# Deployment recipes

# Deploy all binaries
deploy: deploy-backend deploy-frontend

# Deploy backend binaries
deploy-backend: deploy-orchestrator deploy-event-processor

# Deploy webapp
deploy-frontend: deploy-webapp

# Deploy orchestrator lambda function
[working-directory: 'dist']
deploy-orchestrator: build-orchestrator-zip
    aws s3 cp bootstrap.zip s3://{{bucket}}/0.0.0-development/runvoy-orchestrator.zip
    aws lambda update-function-code \
        --function-name runvoy-orchestrator \
        --s3-bucket {{bucket}} \
        --s3-key 0.0.0-development/runvoy-orchestrator.zip > /dev/null
    aws lambda wait function-updated --function-name runvoy-orchestrator

# Deploy event processor lambda function
[working-directory: 'dist']
deploy-event-processor: build-event-processor-zip
    aws s3 cp event-processor.zip s3://{{bucket}}/0.0.0-development/runvoy-event-processor.zip
    aws lambda update-function-code \
        --function-name runvoy-event-processor \
        --s3-bucket {{bucket}} \
        --s3-key 0.0.0-development/runvoy-event-processor.zip > /dev/null
    aws lambda wait function-updated --function-name runvoy-event-processor

# Deploy webapp to S3
[working-directory: 'cmd/webapp']
deploy-webapp: build-webapp
    netlify deploy --no-build --dir dist --prod
