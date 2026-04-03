FROM node:22-alpine AS builder
    
WORKDIR /app

RUN corepack enable

COPY package.json pnpm-workspace.yaml pnpm-lock.yaml ./
COPY apps/web-dashboard ./apps/web-dashboard

RUN pnpm install --frozen-lockfile

RUN pnpm --filter web-dashboard build

FROM node:22-alpine

WORKDIR /app

RUN corepack enable

COPY --from=builder /app/apps/web-dashboard/.next ./apps/web-dashboard/.next
COPY --from=builder /app/apps/web-dashboard/public ./apps/web-dashboard/public
COPY --from=builder /app/apps/web-dashboard/package.json ./apps/web-dashboard/
COPY --from=builder /app/apps/web-dashboard/next.config.js ./apps/web-dashboard/
COPY --from=builder /app/node_modules ./node_modules

WORKDIR /app/apps/web-dashboard

EXPOSE 3000

CMD ["pnpm", "start"]
