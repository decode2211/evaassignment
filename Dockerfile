# Multi-stage build: compile a static binary, then run it on a minimal image.

FROM golang:1.23-alpine AS builder

WORKDIR /src

# Copy manifests first so module downloads are cached separately from code changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 gives a static binary (modernc.org/sqlite is pure Go),
# so the final image needs no C runtime.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.20

# wget is used by HEALTHCHECK below.
RUN apk add --no-cache wget ca-certificates

RUN addgroup -S app && adduser -S app -G app

WORKDIR /app

RUN mkdir -p /app/data && chown -R app:app /app

COPY --from=builder /out/server /app/server

USER app

ENV PORT=8080
ENV DB_PATH=/app/data/tickets.db

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/server"]
