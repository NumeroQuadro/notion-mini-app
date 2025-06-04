#
# ─── BUILDER STAGE ────────────────────────────────────────────────────
#
# 1. Use a small “builder” image that has Go installed
# 2. Download dependencies before copying the rest, so that rebuilds are cached
#
FROM golang:1.24-alpine AS builder

# Install git (if you’re pulling any modules via git) and stall certificates if needed
RUN apk add --no-cache git

WORKDIR /src

# Copy go.mod / go.sum first to leverage Docker layer cache for dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy everything else and build
COPY . .

# Build a statically-linked binary; place it at /notion-mini-app
RUN CGO_ENABLED=0 \
    GOOS=linux \
    go build -o /notion-mini-app ./cmd/main.go


#
# ─── FINAL (RUNTIME) STAGE ─────────────────────────────────────────────
#
# 1. Start from scratch (empty) or a minimal base (alpine, distroless, etc.)
# 2. Copy only the compiled binary into it
# 3. Expose port 9000 and set ENTRYPOINT to run the binary directly
#
FROM scratch

# If your binary needs CA certificates (e.g. for HTTPS calls), copy them in:
# COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the compiled Go binary from the builder stage
COPY --from=builder /notion-mini-app /notion-mini-app
# Define your listening port
EXPOSE 8080

# Launch the binary by default
ENTRYPOINT ["/notion-mini-app"]
