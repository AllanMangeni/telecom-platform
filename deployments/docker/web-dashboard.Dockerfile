# Build stage
FROM node:22-alpine AS builder

LABEL maintainer="nutcas3 <nutcas3@users.noreply.github.com>" \
      description="Web Dashboard for Telecom Platform"

WORKDIR /build

RUN corepack enable

# Copy dependency files for better layer caching
COPY package.json pnpm-workspace.yaml pnpm-lock.yaml ./

# Install dependencies
RUN pnpm install --frozen-lockfile

# Copy source code
COPY apps/web-dashboard ./apps/web-dashboard

# Build with optimizations
RUN pnpm --filter web-dashboard build

# Runtime stage
FROM node:22-alpine

LABEL maintainer="nutcas3 <nutcas3@users.noreply.github.com>" \
      description="Web Dashboard for Telecom Platform" \
      version="1.0.0"

RUN corepack enable

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy built application from builder
COPY --from=builder /build/apps/web-dashboard/.next ./apps/web-dashboard/.next
COPY --from=builder /build/apps/web-dashboard/public ./apps/web-dashboard/public
COPY --from=builder /build/apps/web-dashboard/package.json ./apps/web-dashboard/
COPY --from=builder /build/apps/web-dashboard/next.config.js ./apps/web-dashboard/

# Copy production dependencies only
COPY --from=builder /build/node_modules ./node_modules

WORKDIR /app/apps/web-dashboard

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 3000

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:3000 || exit 1

# Set environment variables
ENV NODE_ENV=production

# Signal handling
STOPSIGNAL SIGTERM

CMD ["pnpm", "start"]
