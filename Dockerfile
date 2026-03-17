# ===========================================================================
# Stage 1 — Builder
# ===========================================================================
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /src

# Cache dependency downloads
COPY go.mod go.sum ./
RUN go mod download

# Build the binary (static linking where possible)
COPY . .
RUN CGO_ENABLED=1 go build \
    -ldflags="-s -w" \
    -o /build/ai-news-hub .

# ===========================================================================
# Stage 2 — Runtime
# ===========================================================================
FROM alpine:3.21

# CA certs for outbound HTTPS (RSS feeds)
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary
COPY --from=builder /build/ai-news-hub .

# Copy default config
COPY config/ ./config/

# Create data directory
RUN mkdir -p /app/data

# Expose port
EXPOSE 8080

# Run as non-root user
RUN adduser -D -H appuser
USER appuser

ENTRYPOINT ["./ai-news-hub"]
