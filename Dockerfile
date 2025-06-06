FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

RUN CGO_ENABLED=0 \
    GOOS=linux \
    go build -o /notion-mini-app ./cmd/main.go

EXPOSE 8080
EXPOSE 443


ENTRYPOINT ["/notion-mini-app"]
