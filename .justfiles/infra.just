# AWS Infrastructure recipes

# Setup CloudFormation StackSet prerequisites (run once)
create-stackset-prerequisites:
    aws cloudformation deploy \
        --stack-name runvoy-stackset-prerequisites \
        --template-file deploy/providers/aws/runvoy-stackset-prerequisites.yaml \
        --capabilities CAPABILITY_NAMED_IAM

# Create releases bucket with multi-region replication
create-releases-buckets: create-stackset-prerequisites
    aws cloudformation create-stack-set \
        --stack-set-name runvoy-releases-bucket \
        --template-body file://deploy/providers/aws/runvoy-bucket.yaml

    aws cloudformation create-stack-instances \
        --stack-set-name runvoy-releases-bucket \
        --accounts 587453114450 \
        --regions {{regions}}

    @echo "Waiting for all stack instances to complete in all regions..."
    @while [ $(aws cloudformation list-stack-instances \
        --stack-set-name runvoy-releases-bucket \
        --query "length(Summaries[?Status != 'CURRENT'])" \
        --output text) -gt 0 ]; do \
        pending=$(aws cloudformation list-stack-instances \
            --stack-set-name runvoy-releases-bucket \
            --query "length(Summaries[?Status != 'CURRENT'])" \
            --output text); \
        echo "Still waiting... ($pending stacks pending)"; \
        sleep 10; \
    done
    @echo "All stack instances completed!"

    aws cloudformation deploy \
        --stack-name runvoy-releases-bucket-replication \
        --template-file deploy/providers/aws/runvoy-bucket-replication.yaml \
        --capabilities CAPABILITY_NAMED_IAM \
        --parameter-overrides ReplicationTargetRegions="{{replication_target_regions}}"

# Create/update backend infrastructure via cloudformation
create-backend-infra:
    aws cloudformation deploy \
        --stack-name {{stack_name}} \
        --template-file deploy/providers/aws/cloudformation-backend.yaml \
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
create-config-file stack_name:
    go run scripts/create-config-file/main.go {{stack_name}}

# Seed initial admin user into DynamoDB (idempotent)
seed-admin-user email stack_name:
    go run scripts/seed-admin-user/main.go {{email}} {{stack_name}}

# Helper to setup Axiom logging for development
# https://axiom.co/docs/send-data/cloudwatch#install-with-cloudformation-stacks
setup-axiom-logging:
    open https://axiom.co/docs/send-data/cloudwatch#install-with-cloudformation-stacks
