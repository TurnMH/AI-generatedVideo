# syntax=docker/dockerfile:1
# ============================================================
# frontend/Dockerfile — Next.js 14 生产构建
# ============================================================

FROM node:20-alpine AS deps
WORKDIR /app

# 安装依赖（利用缓存层）
COPY package.json package-lock.json* ./
RUN npm config set registry https://registry.npmmirror.com \
    && npm ci --prefer-offline

# ── 构建阶段 ─────────────────────────────────────────────────
FROM node:20-alpine AS builder
WORKDIR /app

COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN mkdir -p public

# 构建时注入环境变量（运行时变量通过容器 env 传入）
ARG NEXT_PUBLIC_API_URL=/
ARG NEXT_PUBLIC_WS_URL=
ARG API_PROXY_TARGET=http://gateway:8000

ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_WS_URL=$NEXT_PUBLIC_WS_URL
ENV API_PROXY_TARGET=$API_PROXY_TARGET
ENV NEXT_TELEMETRY_DISABLED=1

RUN npm run build

# ── 运行阶段 ─────────────────────────────────────────────────
FROM node:20-alpine AS runner
WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

# 只复制运行所需文件
RUN addgroup --system --gid 1001 nodejs && \
    adduser  --system --uid 1001 nextjs

COPY --from=builder /app/public          ./public
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static     ./.next/static

USER nextjs

EXPOSE 3000
ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

CMD ["node", "server.js"]
