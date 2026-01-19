# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install git for go mod download
RUN apk add --no-cache git

# Copy source code first (for go mod tidy)
COPY . .

# Download dependencies
RUN go mod tidy

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /share-server ./cmd/server

# Runtime stage
FROM alpine:3.19

# Add CA certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /share-server /app/share-server

# Create data directory
RUN mkdir -p /data

# Non-root user
RUN adduser -D -u 1000 share
RUN chown -R share:share /app /data
USER share

# Expose port
EXPOSE 8080

# Environment variables
ENV SHARE_CONFIG=/etc/share/config.yaml

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ || exit 1

# Run server
ENTRYPOINT ["/app/share-server"]
