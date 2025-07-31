BUILD_FLAGS     :=
GOBIN           := $(shell go env GOPATH)/bin

BINARY_NAME=notion-mini-app
MAIN_FILE=cmd/main.go
GO_FILES=$(wildcard *.go)

DOCKER_IMAGE=notion-mini-app
DOCKER_TAG=latest

# Load environment variables from .env file
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

all: lint build

docker-build:
	@echo "Building from Dockerfile..."
	@docker build -t notion-mini-app .

docker-run: docker-build
	@if [ -f .env ]; then \
		docker run -d --rm --name notion-mini-app \
		--env-file .env \
		-p 8080:8080 \
		-p 443:443 \
		-v /etc/letsencrypt/live/tralalero-tralala.ru/fullchain.pem:/app/certs/fullchain.pem:ro \
		-v /etc/letsencrypt/live/tralalero-tralala.ru/privkey.pem:/app/certs/privkey.pem:ro \
		notion-mini-app; \
	else \
		@echo "Error: .env file not found. Please create one from .env.example."; \
		exit 1; \
	fi

docker-logs:
	@echo "Getting Docker container logs..."
	@docker logs -f notion-mini-app

docker-stop:
	@echo "Stopping Docker container..."
	@docker stop notion-mini-app

docker-rm:
	@echo "Deleting Docker container..."
	@docker rm notion-mini-app

deps:
	go mod tidy
	go mod download

clean:
	rm -f $(BINARY_NAME)