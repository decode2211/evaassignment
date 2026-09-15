# This file describes how to package the ticket system server into a
# Docker image. It uses a two-stage build: the first stage ("builder")
# compiles the Go program using the full Go toolchain, and the second
# stage copies only the finished, compiled binary into a tiny final image.
# This keeps the final image small and avoids shipping build tools,
# source code, or caches to production.

# ---- Stage 1: build the static Go binary ----
FROM golang:1.23-alpine AS builder

WORKDIR /src

# Copy just the dependency manifests first so Docker can cache the
# downloaded modules as a separate layer - they only need to be
# re-downloaded when go.mod/go.sum actually change, not on every code edit.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 produces a fully static binary with no C library
# dependency (possible because modernc.org/sqlite is pure Go), which is
# exactly what lets the final image be based on plain alpine with no
# extra runtime libraries. -ldflags="-s -w" strips debug information to
# make the binary smaller.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/server ./cmd/server

# ---- Stage 2: minimal runtime image ----
FROM alpine:3.20

# wget is needed by the HEALTHCHECK below; ca-certificates is included in
# case the app ever needs to make outbound HTTPS calls.
RUN apk add --no-cache wget ca-certificates

# Run as a dedicated, unprivileged user instead of root. If an attacker
# ever found a way to execute code inside this container, running as a
# non-root user limits what damage they could do to the container itself.
RUN addgroup -S app && adduser -S app -G app

WORKDIR /app

# The SQLite database lives under /app/data. We create it (and hand
# ownership to the "app" user) at image-build time so the container can
# still write to it even though it runs as a non-root user.
RUN mkdir -p /app/data && chown -R app:app /app

COPY --from=builder /out/server /app/server

USER app

# Sensible defaults so the container starts with zero required
# environment variables, as the assignment requires.
ENV PORT=8080
ENV DB_PATH=/app/data/tickets.db

EXPOSE 8080

# Docker (and platforms like Render) use this to know whether the
# container is actually healthy, not just "running".
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/server"]
