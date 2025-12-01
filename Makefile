SHELL := /bin/bash
NAME=promalert
VERSION=1.2.16
GCR_HOST=gcr.io/bugsnag-155907
AWS_PROFILE="insighthub-production"
ECR_REGION=${ECR_REGION:-us-east-1}
AWS_ACCOUNT_ID="357581182020"
ECR_REGISTRY="${AWS_ACCOUNT_ID}.dkr.ecr.${ECR_REGION}.amazonaws.com"
GCR_IMAGE=$(GCR_HOST)/$(NAME):$(VERSION)
ECR_IMAGE=$(ECR_REGISTRY)/$(NAME):$(VERSION)


.DEFAULT_GOAL := help
.PHONY: test pipfile
# Tasks
test: ## Run unit tests
	@echo "---> [Executing go tests]"
	@go test . -race -timeout 30m -p 1

image: ## Build and push image given VERSION
	@echo "---> [Executing docker build]"
	@docker buildx build --platform linux/amd64,linux/arm64 --push . -t $(GCR_IMAGE) -t $(ECR_IMAGE)
	@echo "pushed to GCR: $(GCR_IMAGE)"
	@echo "pushed to ECR: $(ECR_IMAGE)"

build: test image ## Creates a unit tested image
	@echo "---> [All done!]"

help: ## Print help for all functions of Makefile
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
