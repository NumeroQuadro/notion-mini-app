BUILD_FLAGS     :=
GOBIN           := $(shell go env GOPATH)/bin

BINARY_NAME=notion-mini-app
MAIN_FILE=cmd/main.go
GO_FILES=$(wildcard *.go)

DOCKER_IMAGE=notion-mini-app
DOCKER_TAG=latest

TELEGRAM_BOT_TOKEN=
NOTION_API_KEY=
NOTION_TASKS_DATABASE_ID=
NOTION_NOTES_DATABASE_ID=
NOTION_JOURNAL_DATABASE_ID=
NOTION_PROJECTS_DATABASE_ID=
MINI_APP_URL=
AUTHORIZED_USER_ID=
PORT=8080
HOST=

all: lint build

docker-build:
	@echo "Building from Dockerfile..."
	@docker build -t notion-mini-app .

docker-run: docker-build
	@docker run --rm \
	-e TELEGRAM_BOT_TOKEN=$(TELEGRAM_BOT_TOKEN) \
	-e NOTION_API_KEY=$(NOTION_API_KEY) \
	-e NOTION_TASKS_DATABASE_ID=$(NOTION_TASKS_DATABASE_ID) \
	-e NOTION_NOTES_DATABASE_ID=$(NOTION_NOTES_DATABASE_ID) \
	-e NOTION_JOURNAL_DATABASE_ID=$(NOTION_JOURNAL_DATABASE_ID) \
	-e NOTION_PROJECTS_DATABASE_ID=$(NOTION_PROJECTS_DATABASE_ID) \
	-e MINI_APP_URL=$(MINI_APP_URL) \
	-e AUTHORIZED_USER_ID=$(AUTHORIZED_USER_ID) \
	-e PORT=$(PORT) \
	-e HOST=$(HOST) \
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