smoke-test: build
    ./runvoy exec --skip-git "echo 'Hello, World! $(date -u +"%Y-%m-%d %H:%M:%S")'"

build:
    go build -o runvoy

deploy:
    ./scripts/update-lambda.sh
    ./scripts/update-cloudformation.sh
