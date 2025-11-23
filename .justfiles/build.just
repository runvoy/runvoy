# Build recipes

# Build all binaries
build: build-cli build-local build-backend build-frontend

# Build backend binaries (Lambda functions)
build-backend: build-orchestrator build-event-processor

# Build frontend binary
build-frontend: build-development-webapp

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
    just build-lambda-zip runvoy-orchestrator

# Build event processor zip file
[working-directory: 'dist']
build-event-processor-zip: build-event-processor
    just build-lambda-zip runvoy-event-processor

# Build axiom forwarder backend service (Lambda function)
[working-directory: 'scripts/axiom-forwarder']
build-axiom-forwarder:
    GOARCH=arm64 GOOS=linux go build \
        -o ../../dist/bootstrap

# Build axiom forwarder zip file
[working-directory: 'dist']
build-axiom-forwarder-zip: build-axiom-forwarder
    just build-lambda-zip axiom-forwarder

# Build lambda zip file
[working-directory: 'dist']
build-lambda-zip lambda-name:
    rm -f {{lambda-name}}.zip
    zip {{lambda-name}}.zip bootstrap
    rm -f bootstrap

# Build development webapp
[working-directory: 'cmd/webapp']
build-development-webapp: type-check-webapp lint-webapp
    VITE_RUNVOY_VERSION={{build_version}} npx vite build

# Build production webapp
[working-directory: 'cmd/webapp']
build-production-webapp: type-check-webapp lint-webapp
    VITE_RUNVOY_VERSION={{version}} npx vite build
