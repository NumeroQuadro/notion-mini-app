BUILD_FLAGS     :=
GOBIN           := $(shell go env GOPATH)/bin

BINARY_NAME=notion-mini-app
MAIN_FILE=cmd/main.go
GO_FILES=$(wildcard *.go)

DOCKER_IMAGE=notion-mini-app
DOCKER_TAG=latest

all: lint build

docker-build:
	@echo "Building from Dockerfile..."
	@docker build -t notion-mini-app .

docker-run: docker-build
	@docker run --rm \
	-e TELEGRAM_BOT_TOKEN=_ \
	-e NOTION_API_KEY=_ \
	-e NOTION_TASKS_DATABASE_ID=_ \
	-e NOTION_NOTES_DATABASE_ID=_ \
	-e NOTION_JOURNAL_DATABASE_ID=_ \
	-e NOTION_PROJECTS_DATABASE_ID=_ \
	-e MINI_APP_URL=_ \
	-e AUTHORIZED_USER_ID=_ \
	-e PORT=8080 \
	-e HOST=0.0.0.0 \
	-p 8080:8080 \
	notion-mini-app

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