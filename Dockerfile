# syntax=docker/dockerfile:1

########################
# 1) Build stage
########################
ARG GO_VERSION=1.24
FROM golang:${GO_VERSION}-bookworm AS builder

WORKDIR /src

# Cache deps first for faster rebuilds
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy the rest of the source
COPY . .

# Build a static binary for a tiny final image
ENV CGO_ENABLED=0 GOFLAGS="-buildvcs=false"
ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN --mount=type=cache,target=/root/.cache/go-build \
    GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -ldflags="-s -w" -o /out/app ./cmd/server

########################
# 2) Runtime stage
########################
FROM alpine:3.20

# Add non-root user
RUN addgroup -S app && adduser -S app -G app

# CA certs (HTTPS) + tzdata (optional)
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy the compiled binary
COPY --from=builder /out/app /usr/local/bin/app

# Drop privileges
USER app

# --- App configuration (override at runtime) ---
# Gin usually listens on 8080;
ENV PORT=8080

# MinIO settings (override via docker run -e ...)
ENV MINIO_ENDPOINT="http://minio:9000" \
    MINIO_ACCESS_KEY="" \
    MINIO_SECRET_KEY="" \
    MINIO_BUCKET="uploads" \
    MINIO_USE_SSL="false"

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD wget -qO- http://127.0.0.1:${PORT}/health-check || exit 1

ENTRYPOINT ["/usr/local/bin/app"]
