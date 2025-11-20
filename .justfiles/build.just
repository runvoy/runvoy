# Build recipes

# Build all binaries
build: build-cli build-local build-backend build-frontend

# Build backend binaries (Lambda functions)
build-backend: build-orchestrator build-event-processor

# Build frontend binary
build-frontend: build-webapp

# Build CLI client
[working-directory: 'cmd/cli']
build-cli:
    go build \
        -ldflags '{{build_flags}}' \
        -o ../../bin/runvoy

# Build orchestrator backend service (Lambda function)
[working-directory: 'cmd/backend/providers/aws/orchestrator']
build-orchestrator:
    GOARCH=arm64 GOOS=linux go build \
        -ldflags '{{build_flags}}' \
        -o ../../../../../dist/bootstrap

# Build event processor backend service (Lambda function)
[working-directory: 'cmd/backend/providers/aws/processor']
build-event-processor:
    GOARCH=arm64 GOOS=linux go build \
        -ldflags '{{build_flags}}' \
        -o ../../../../../dist/bootstrap

# Build local development server
[working-directory: 'cmd/local']
build-local:
    go build \
        -ldflags '{{build_flags}}' \
        -o ../../bin/local

# Build orchestrator zip file
[working-directory: 'dist']
build-orchestrator-zip: build-orchestrator
    rm -f bootstrap.zip
    zip bootstrap.zip bootstrap

# Build event processor zip file
[working-directory: 'dist']
build-event-processor-zip: build-event-processor
    rm -f event-processor.zip
    zip event-processor.zip bootstrap

# Build webapp
[working-directory: 'cmd/webapp']
build-webapp: lint-webapp
    VITE_RUNVOY_VERSION={{build_version}} npx vite build
