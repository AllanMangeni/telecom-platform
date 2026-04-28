# Build stage
FROM golang:1.26-alpine AS builder

LABEL maintainer="nutcas3 <nutcas3@users.noreply.github.com>" \
      description="API Server for Telecom Platform"

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy dependency files
COPY apps/api-server/go.mod apps/api-server/go.sum ./
RUN go mod download

# Copy source code
COPY apps/api-server/ ./

# Build with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -extldflags '-static'" \
    -trimpath \
    -o api-server .

# Runtime stage
FROM alpine:3.20

LABEL maintainer="nutcas3 <nutcas3@users.noreply.github.com>" \
      description="API Server for Telecom Platform" \
      version="1.0.0"

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/api-server .

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8000

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8000/v1/health || exit 1

# Set environment variables
ENV GIN_MODE=release

# Signal handling
STOPSIGNAL SIGTERM

CMD ["./api-server"]
