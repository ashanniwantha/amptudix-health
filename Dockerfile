# syntax=docker/dockerfile:1

# ==========================================
# STAGE 1: Build the Go binaries
# ==========================================
# Pin a specific minor version for reproducible builds. Upgrading Go is an
# explicit commit, not something that silently changes when a tag is re-pushed.
FROM golang:1.27-alpine3.24 AS builder

# CA certificates are needed only if the build ever fetches over HTTPS.
# They are copied into the distroless runtime by the base image itself.
RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Cache dependencies separately from source so a code-only change does not
# re-download modules.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 produces a fully static binary with no libc dependency,
# which is what allows the distroless/static runtime to contain only the
# binary and its data.
# -trimpath removes absolute build paths from the binary.
# -s -w strips the symbol table and DWARF debug info (~40% smaller).
# Both api and migrate are built from the same source tree.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/api \
    ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/migrate \
    ./cmd/migrate


# ==========================================
# STAGE 2: Minimal, non-root runtime
# ==========================================
# gcr.io/distroless/static-debian12:nonroot is ~2 MB, contains CA
# certificates and timezone data, has no shell or package manager, and runs
# as a non-root user out of the box. For a statically linked Go binary this
# is the smallest practical attack surface.
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

WORKDIR /app

# Copy both binaries. --chown ensures they are owned by the nonroot user
# that the image runs as.
COPY --from=builder --chown=nonroot:nonroot /out/api     ./api
COPY --from=builder --chown=nonroot:nonroot /out/migrate ./migrate

# Distroless:nonroot already sets USER nonroot:nonroot; this line is
# explicit for clarity and for scanners that require it.
USER nonroot:nonroot

EXPOSE 8080

# Exec form: the process becomes PID 1 and receives SIGTERM directly,
# which is required for graceful shutdown.
ENTRYPOINT ["./api"]
