# Build stage
FROM golang:1.26-alpine AS builder

LABEL maintainer="nutcas3 <nutcas3@users.noreply.github.com>" \
      description="CLI for Telecom Platform"

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy dependency files
COPY apps/cli/go.mod apps/cli/go.sum ./
RUN go mod download

# Copy source code
COPY apps/cli/ ./

# Build with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -extldflags '-static'" \
    -trimpath \
    -o telecom-cli .

# Runtime stage
FROM alpine:3.20

LABEL maintainer="nutcas3 <nutcas3@users.noreply.github.com>" \
      description="CLI for Telecom Platform" \
      version="1.0.0"

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/telecom-cli .

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Signal handling
STOPSIGNAL SIGTERM

CMD ["./telecom-cli"]
