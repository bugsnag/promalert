SHELL := /bin/bash
NAME=promalert
HOST=gcr.io/bugsnag-155907
VER=1.2.14
IMAGE=$(HOST)/$(NAME):$(VER)

.DEFAULT_GOAL := help
.PHONY: test pipfile
# Tasks
test: ## Run unit tests
	@echo "---> [Executing go tests]"
	@go test . -race -timeout 30m -p 1

image: ## Build image given VER
	@echo "---> [Executing docker build]"
	@docker buildx create --use
	@docker buildx build --platform linux/amd64,linux/arm64 --push -t $(IMAGE) .
	@echo $(IMAGE)

build: test image ## Creates a unit tested image
	@echo "---> [All done!]"

.PHONY: up ## Bring up the dev docker stack
up:
	@docker-compose -f docker-compose.dev.yml up -d --remove-orphans --quiet-pull --build
	@docker-compose -f docker-compose.dev.yml logs -f
	
.PHONY: down ## Bring down the dev docker stack
down:
	@docker-compose -f docker-compose.dev.yml down

help: ## Print help for all functions of Makefile
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
