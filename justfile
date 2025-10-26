smoke-test: build
    ./mycli exec --skip-git "echo 'Hello, World! $(date -u +"%Y-%m-%d %H:%M:%S")'"

build:
    go build -o mycli
