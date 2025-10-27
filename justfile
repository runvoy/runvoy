bucket := 'runvoy-releases'

[working-directory: 'cmd/cli']
build-cli:
    go build -o runvoy

create-lambda-bucket:
    aws cloudformation deploy \
        --stack-name runvoy-releases-bucket \
        --template-file infra/runvoy-bucket.yaml

[working-directory: 'backend/orchestrator']
update-lambda:
    rm -f function.zip bootstrap
    GOARCH=arm64 GOOS=linux go build -o bootstrap
    zip function.zip bootstrap
    aws s3 cp function.zip s3://{{bucket}}/bootstrap.zip
    aws lambda update-function-code --function-name runvoy-orchestrator --zip-file fileb://function.zip > /dev/null
    aws lambda wait function-updated --function-name runvoy-orchestrator

init:
    aws cloudformation deploy \
        --stack-name runvoy-backend \
        --template-file infra/cloudformation-backend.yaml \
        --parameter-overrides LambdaCodeBucket={{bucket}} JWTSecret=$(openssl rand -hex 32) \
        --capabilities CAPABILITY_NAMED_IAM

destroy:
    aws cloudformation delete-stack --stack-name runvoy-backend
    aws cloudformation wait stack-delete-complete --stack-name runvoy-backend
