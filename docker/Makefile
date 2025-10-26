.PHONY: build push test clean

IMAGE_NAME ?= mycli/executor
IMAGE_TAG ?= latest
PLATFORM ?= linux/amd64,linux/arm64

# Build the Docker image for local testing (amd64)
build:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

# Build multi-architecture image
build-multi:
	docker buildx build --platform $(PLATFORM) -t $(IMAGE_NAME):$(IMAGE_TAG) --load .

# Push to ECR Public (requires AWS_ACCOUNT_ID and ECR_ALIAS)
push-ecr:
	@echo "Logging into ECR Public..."
	@aws ecr-public get-login-password --region us-east-1 | docker login --username AWS --password-stdin public.ecr.aws
	@echo "Tagging image..."
	docker tag $(IMAGE_NAME):$(IMAGE_TAG) public.ecr.aws/$(ECR_ALIAS)/mycli-executor:$(IMAGE_TAG)
	@echo "Pushing image..."
	docker push public.ecr.aws/$(ECR_ALIAS)/mycli-executor:$(IMAGE_TAG)

# Test with a public repository
test:
	docker run --rm \
		-e REPO_URL=https://github.com/hashicorp/terraform-guides \
		-e REPO_BRANCH=main \
		-e USER_COMMAND="ls -la && pwd" \
		$(IMAGE_NAME):$(IMAGE_TAG)

# Test with custom command
test-custom:
	docker run --rm \
		-e REPO_URL=$(REPO_URL) \
		-e REPO_BRANCH=$(REPO_BRANCH) \
		-e USER_COMMAND="$(COMMAND)" \
		-e GITHUB_TOKEN=$(GITHUB_TOKEN) \
		$(IMAGE_NAME):$(IMAGE_TAG)

# Clean up Docker images
clean:
	docker rmi $(IMAGE_NAME):$(IMAGE_TAG) || true

# Show help
help:
	@echo "Available targets:"
	@echo "  build         - Build Docker image for local testing (amd64)"
	@echo "  build-multi   - Build multi-architecture image (amd64 + arm64)"
	@echo "  push-ecr      - Push to AWS ECR Public (requires ECR_ALIAS env var)"
	@echo "  test          - Run a simple test with a public repository"
	@echo "  test-custom   - Test with custom repo (set REPO_URL, REPO_BRANCH, COMMAND)"
	@echo "  clean         - Remove local Docker image"
	@echo ""
	@echo "Examples:"
	@echo "  make build"
	@echo "  make test"
	@echo "  ECR_ALIAS=myalias make push-ecr"
	@echo "  REPO_URL=https://github.com/user/repo COMMAND='make test' make test-custom"
