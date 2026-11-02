# syntax=docker/dockerfile:1

# ==========================================
# STAGE 1: Build the Go Application Binary
# ==========================================
# Pinning explicit versions of Go and Alpine ensures build determinism and consistency across environments.
FROM golang:1.26-alpine3.24 AS builder

# Install root CA certificates required for HTTPS external API requests.
RUN apk add --no-cache git ca-certificates && update-ca-certificates

WORKDIR /src

# Separate dependency downloading into its own step to leverage Docker layer caching.
# This prevents downloading modules again if only application code changes.
COPY go.mod go.sum ./
RUN go mod download

# Copy all directories, including cmd/ and internal/, maintaining project hierarchy.
COPY . .

# Compile a highly optimized, statically linked binary.
# - CGO_ENABLED=0 creates a self-contained binary with zero dynamic C-library dependencies.
# - ldflags "-s -w" strips debug symbols and troubleshooting metrics, dropping the final size by ~40%.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /bin/server \
    ./cmd/api/main.go


# ==========================================
# STAGE 2: Secure, Minimal Runtime Environment
# ==========================================
# Using the minimal Alpine runtime to eliminate vulnerabilities present in full Linux distributions.
FROM alpine:3.24.3

# Create an explicit unprivileged user and group using a fixed high-number ID (10001).
# This prevents host system UID collision and satisfies enterprise container security scanning policies.
RUN addgroup -g 10001 appgroup && \
    adduser -u 10001 -G appgroup -h /home/appuser -D appuser

WORKDIR /app

# Extract only the necessary structural assets from the builder stage.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder --chown=appuser:appgroup /bin/server ./server

# Switch context completely away from root to the secure user.
USER appuser:appgroup

# Inform container runtimes that the application listens on port 8080.
EXPOSE 8080

# Utilizing exec form directly executes the application as PID 1.
# This allows standard Unix signals (like SIGTERM) to reach Go directly for graceful shutdown management.
ENTRYPOINT ["./server"]
