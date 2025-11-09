# Multi-stage build for tapo-data-logger
# Stage 1: Build the application
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY *.go ./
COPY static/ ./static/

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-w -s" -o tapo-data-logger .

# Stage 2: Create minimal runtime image
FROM alpine:latest

# Install ca-certificates for HTTPS connections
RUN apk --no-cache add ca-certificates tzdata

# Create a non-root user
RUN addgroup -S tapo && adduser -S tapo -G tapo

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/tapo-data-logger .

# Copy config example (users can mount their own config)
COPY config.example.json /app/config.example.json

# Change ownership
RUN chown -R tapo:tapo /app

# Switch to non-root user
USER tapo

# Expose web UI port (default 8080)
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ || exit 1

# Set default environment variables
ENV CONFIG_FILE=/app/config.json

# Run the application
CMD ["./tapo-data-logger"]
