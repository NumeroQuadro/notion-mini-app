#
# ─── BUILDER STAGE ────────────────────────────────────────────────────
#
#  - Starts from a small Alpine/Go image
#  - Installs git (for go modules) and ca-certificates (for HTTPS at build time, if any)
#  - Downloads Go dependencies first (caching)
#  - Copies the rest of the source + .env
#  - Builds a fully static Linux binary at /notion-mini-app
#
FROM golang:1.24-alpine AS builder

# 1) Install git (for private modules, if any) and ca-certificates (if any HTTPS is needed during build)
RUN apk add --no-cache git ca-certificates

WORKDIR /src

# 2) Copy go.mod/go.sum and download dependencies (cached as long as these two files don't change)
COPY go.mod go.sum ./
RUN go mod download

# 3) Copy .env explicitly (so your code can call godotenv.Load() or similar if you rely on a .env file)
#    If you prefer passing env vars at runtime via `--env-file`, you can remove this line
COPY .env ./

# 4) Copy the rest of your source code
COPY . ./

# 5) Build a statically-linked binary (CGO_ENABLED=0 → no dynamic libraries needed)
RUN CGO_ENABLED=0 \
    GOOS=linux \
    go build -o /notion-mini-app ./cmd/main.go


#
# ─── FINAL (RUNTIME) STAGE ─────────────────────────────────────────────
#
#  - Starts from scratch (empty) to keep the image as small as possible
#  - Copies in only:
#      a) the compiled Go binary from the builder
#      b) the CA certificate bundle (so ‘tls: failed to verify certificate’ goes away)
#      c) the .env file (so your app can read it if it’s coded to do so)
#  - Exposes port 8080 and runs /notion-mini-app by default
#
FROM scratch

# 1) Copy the CA certificate bundle from the builder → /etc/ssl/certs/ca-certificates.crt

# 2) Copy the compiled Go binary
COPY --from=builder /notion-mini-app /notion-mini-app

# 3) Copy the .env file (if your app expects to find it at runtime)
COPY --from=builder /src/.env /app/.env

# 4) Set a working directory (optional, but if your code expects to run from /app, it can be useful)
WORKDIR /app

# 5) Expose port 8080 (adjust if your application listens on a different port)
EXPOSE 8080
EXPOSE 443


# 6) Run the Go binary by default; it will load /app/.env automatically if your code calls godotenv.Load()
ENTRYPOINT ["/notion-mini-app"]
