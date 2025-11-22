# Deployment recipes

# Deploy all binaries
deploy: deploy-backend deploy-development-frontend

# Deploy backend binaries
deploy-backend: deploy-orchestrator deploy-event-processor

# Deploy development webapp
deploy-development-frontend: deploy-development-webapp

# Deploy orchestrator lambda function
[working-directory: 'dist']
deploy-orchestrator: build-orchestrator-zip
    just deploy-lambda runvoy-orchestrator

# Deploy event processor lambda function
[working-directory: 'dist']
deploy-event-processor: build-event-processor-zip
    just deploy-lambda runvoy-event-processor

# Deploy lambda function to S3 and update the function code
# NOTICE: upload to the primary region, deploy from the current region bucket
[working-directory: 'dist']
deploy-lambda lambda-name:
    aws s3 cp {{lambda-name}}.zip s3://{{bucket}}/0.0.0-development/{{lambda-name}}-{{build_version}}.zip

    aws s3api wait object-exists \
        --bucket runvoy-releases-{{region}} \
        --key 0.0.0-development/{{lambda-name}}-{{build_version}}.zip

    aws lambda update-function-code \
        --function-name {{lambda-name}} \
        --s3-bucket runvoy-releases-{{region}} \
        --s3-key 0.0.0-development/{{lambda-name}}-{{build_version}}.zip > /dev/null

    aws lambda wait function-updated --function-name {{lambda-name}}

# Deploy development webapp to S3
[working-directory: 'cmd/webapp']
deploy-development-webapp: build-development-webapp
    netlify deploy --no-build --dir dist

# Deploy production webapp to S3
[working-directory: 'cmd/webapp']
deploy-production-webapp: build-production-webapp
    netlify deploy --no-build --dir dist --prod
