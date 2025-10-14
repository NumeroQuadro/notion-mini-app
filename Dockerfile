FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

RUN CGO_ENABLED=0 \
    GOOS=linux \
    go build -o /notion-mini-app ./cmd/main.go

# Use minimal alpine image for final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /notion-mini-app /app/notion-mini-app

# Copy the web directory (CRITICAL - bot needs this!)
COPY --from=builder /src/web /app/web

EXPOSE 8080
EXPOSE 443

ENTRYPOINT ["/app/notion-mini-app"]
