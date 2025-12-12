# AWS Infrastructure recipes

# Setup CloudFormation StackSet prerequisites (run once)
create-stackset-prerequisites:
    aws cloudformation deploy \
        --region us-east-1 \
        --stack-name runvoy-stackset-prerequisites \
        --template-file deploy/release/runvoy-stackset-prerequisites.yaml \
        --capabilities CAPABILITY_NAMED_IAM

# Create releases bucket with multi-region replication
create-releases-buckets: create-stackset-prerequisites
    aws cloudformation create-stack-set \
        --region us-east-1 \
        --stack-set-name runvoy-releases-bucket \
        --template-body file://deploy/release/runvoy-bucket.yaml

    aws cloudformation create-stack-instances \
        --region us-east-1 \
        --stack-set-name runvoy-releases-bucket \
        --accounts 587453114450 \
        --regions {{regions}}

    @echo "Waiting for all stack instances to complete in all regions..."
    @while [ $(aws cloudformation list-stack-instances \
        --region us-east-1 \
        --stack-set-name runvoy-releases-bucket \
        --query "length(Summaries[?Status != 'CURRENT'])" \
        --output text) -gt 0 ]; do \
        pending=$(aws cloudformation list-stack-instances \
            --region us-east-1 \
            --stack-set-name runvoy-releases-bucket \
            --query "length(Summaries[?Status != 'CURRENT'])" \
            --output text); \
        echo "Still waiting... ($pending stacks pending)"; \
        sleep 10; \
    done
    @echo "All stack instances completed!"

    aws cloudformation deploy \
        --region us-east-1 \
        --stack-name runvoy-releases-bucket-replication \
        --template-file deploy/release/runvoy-bucket-replication.yaml \
        --capabilities CAPABILITY_NAMED_IAM \
        --parameter-overrides ReplicationTargetRegions="{{replication_target_regions}}"

# Create/update backend infrastructure via cloudformation
create-backend-infra:
    aws cloudformation deploy \
        --stack-name {{stack_name}} \
        --template-file deploy/providers/aws/cloudformation-backend.yaml \
        --parameter-overrides LogRetentionDays=7 \
        --capabilities CAPABILITY_NAMED_IAM
    just create-config-file {{stack_name}}
    just seed-admin-user {{admin_email}} {{stack_name}}

# Destroy backend infrastructure via cloudformation
destroy-backend-infra:
    aws cloudformation delete-stack \
        --stack-name {{stack_name}}
    aws cloudformation wait stack-delete-complete \
        --stack-name {{stack_name}}

# Initialize backend infrastructure and seed admin user
init: create-backend-infra deploy
    @echo ""
    @echo "==================================================================="
    @echo "Backend infrastructure initialized successfully!"
    @echo "==================================================================="
    @echo ""
    @echo "Next step: Register the default Docker image"
    @echo ""
    @echo "To register the default image, use the CLI:"
    @echo "  runvoy images register public.ecr.aws/docker/library/ubuntu:22.04 --set-default"
    @echo ""
    @echo "You can also list all registered images:"
    @echo "  runvoy images list"
    @echo ""

# Create/update config file with API endpoint from CloudFormation stack
create-config-file:
    go run scripts/create-config-file/main.go {{stack_name}}

# Seed initial admin user into DynamoDB (idempotent)
seed-admin-user email:
    go run scripts/seed-admin-user/main.go {{email}} {{stack_name}}

# Create/update axiom forwarder infrastructure prerequisites via cloudformation
create-axiom-forwarder-prerequisites:
    aws cloudformation deploy \
        --stack-name axiom-forwarder-prerequisites \
        --template-file deploy/axiom/axiom-forwarder-prerequisites.yaml \
        --capabilities CAPABILITY_NAMED_IAM \
        --parameter-overrides BucketName={{axiom_bucket}}

# Create/update axiom forwarder infrastructure via cloudformation
create-axiom-forwarder-infra: create-axiom-forwarder-prerequisites upload-axiom-forwarder
    aws cloudformation deploy \
        --stack-name axiom-forwarder \
        --template-file deploy/axiom/axiom-forwarder.yaml \
        --capabilities CAPABILITY_NAMED_IAM \
        --parameter-overrides LambdaCodeBucket={{axiom_bucket}} \
            AxiomToken={{axiom_token}} \
            AxiomDataset={{axiom_dataset}} \
            OrchestratorLogGroup={{orchestrator_log_group}} \
            EventProcessorLogGroup={{event_processor_log_group}}
