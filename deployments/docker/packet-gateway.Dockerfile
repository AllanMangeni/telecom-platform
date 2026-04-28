# Build stage
FROM rust:1.95-slim AS builder

LABEL maintainer="nutcas3 <nutcas3@users.noreply.github.com>" \
      description="Packet Gateway for Telecom Platform"

WORKDIR /build

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    pkg-config \
    libssl-dev \
    clang \
    llvm \
    libelf-dev \
    libbpf-dev \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Copy Cargo files for better layer caching
COPY Cargo.toml Cargo.lock ./

# Create a dummy app to cache dependencies
RUN mkdir -p apps/packet-gateway/src && \
    echo "fn main() {}" > apps/packet-gateway/src/main.rs && \
    cargo build --release --bin packet-gateway && \
    rm -rf apps/packet-gateway/src

# Copy actual source code
COPY apps/packet-gateway ./apps/packet-gateway
COPY apps/charging-engine/src/circuit_breaker.rs ./apps/charging-engine/src/circuit_breaker.rs

# Build with optimizations
RUN cargo build --release --bin packet-gateway && \
    strip /build/target/release/packet-gateway

# Runtime stage
FROM debian:bookworm-slim

LABEL maintainer="nutcas3 <nutcas3@users.noreply.github.com>" \
      description="Packet Gateway for Telecom Platform" \
      version="1.0.0"

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    libssl3 \
    ca-certificates \
    tzdata \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN groupadd -r appuser && \
    useradd -r -g appuser -u 1000 appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/target/release/packet-gateway /app/packet-gateway

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 9090

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9090/health || exit 1

# Set environment variables
ENV RUST_LOG=info

# Signal handling
STOPSIGNAL SIGTERM

CMD ["/app/packet-gateway"]
