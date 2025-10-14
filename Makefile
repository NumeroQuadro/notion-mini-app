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
		echo "Starting Docker container..."; \
		docker run -d --name notion-mini-app \
		--env-file .env \
		-p 8081:8080 \
		-p 8443:443 \
		-v /etc/letsencrypt/live/tralalero-tralala.ru/fullchain.pem:/app/certs/fullchain.pem:ro \
		-v /etc/letsencrypt/live/tralalero-tralala.ru/privkey.pem:/app/certs/privkey.pem:ro \
		notion-mini-app; \
		echo "Waiting for container to start..."; \
		sleep 3; \
		if docker ps --filter "name=notion-mini-app" --format "{{.Names}}" | grep -q notion-mini-app; then \
			echo "✅ Container is running!"; \
			echo "View logs with: make docker-logs"; \
		else \
			echo "❌ Container crashed! Showing logs:"; \
			docker logs notion-mini-app 2>&1 || echo "Container was removed too quickly. Try: make docker-run-debug"; \
			docker rm -f notion-mini-app 2>/dev/null || true; \
			exit 1; \
		fi; \
	else \
		echo "Error: .env file not found. Please create one from .env.example."; \
		exit 1; \
	fi

docker-run-debug: docker-build
	@echo "Running container in foreground for debugging..."
	@echo "Press Ctrl+C to stop"
	@echo "==========================================="
	@if [ -f .env ]; then \
		docker run --rm --name notion-mini-app-debug \
		--env-file .env \
		-p 8081:8080 \
		-p 8443:443 \
		notion-mini-app; \
	else \
		echo "Error: .env file not found."; \
		exit 1; \
	fi

docker-logs:
	@echo "Getting Docker container logs..."
	@docker logs -f notion-mini-app

docker-status:
	@echo "Docker container status:"
	@docker ps -a --filter "name=notion-mini-app" --format "table {{.ID}}\t{{.Status}}\t{{.Names}}\t{{.Ports}}"
	@echo ""
	@if docker ps --filter "name=notion-mini-app" --format "{{.Names}}" | grep -q notion-mini-app; then \
		echo "✅ Container is RUNNING"; \
	else \
		if docker ps -a --filter "name=notion-mini-app" --format "{{.Names}}" | grep -q notion-mini-app; then \
			echo "❌ Container STOPPED/CRASHED - Last logs:"; \
			docker logs --tail 20 notion-mini-app 2>&1; \
		else \
			echo "⚠️  No container found. Run: make docker-run"; \
		fi; \
	fi

docker-stop:
	@echo "Stopping Docker container..."
	@docker stop notion-mini-app 2>/dev/null || echo "Container not running"
	@docker rm notion-mini-app 2>/dev/null || echo "Container already removed"

docker-clean:
	@echo "Cleaning up Docker containers and images..."
	@docker stop notion-mini-app 2>/dev/null || true
	@docker rm notion-mini-app 2>/dev/null || true
	@docker rm notion-mini-app-debug 2>/dev/null || true
	@echo "✅ Cleanup complete"

docker-rm:
	@echo "Deleting Docker container..."
	@docker rm notion-mini-app

deps:
	go mod tidy
	go mod download

clean:
	rm -f $(BINARY_NAME)