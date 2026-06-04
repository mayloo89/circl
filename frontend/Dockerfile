# Build stage
FROM node:22-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

# Build args are baked into next.config.ts at build time.
# NEXT_PUBLIC_* vars are embedded in the client bundle.
ARG NEXT_PUBLIC_API_URL=http://localhost:8080
ARG NEXT_PUBLIC_WS_URL=ws://localhost:8080
ARG NEXT_PUBLIC_IMAGE_HOSTNAME
ARG NEXT_PUBLIC_IMAGE_PORT
ARG NEXT_PUBLIC_IMAGE_PROTOCOL=http
ARG NEXT_PUBLIC_IMAGE_UNOPTIMIZED=false
ARG NEXT_PUBLIC_TURNSTILE_SITE_KEY

ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_WS_URL=$NEXT_PUBLIC_WS_URL
ENV NEXT_PUBLIC_IMAGE_HOSTNAME=$NEXT_PUBLIC_IMAGE_HOSTNAME
ENV NEXT_PUBLIC_IMAGE_PORT=$NEXT_PUBLIC_IMAGE_PORT
ENV NEXT_PUBLIC_IMAGE_PROTOCOL=$NEXT_PUBLIC_IMAGE_PROTOCOL
ENV NEXT_PUBLIC_IMAGE_UNOPTIMIZED=$NEXT_PUBLIC_IMAGE_UNOPTIMIZED
ENV NEXT_PUBLIC_TURNSTILE_SITE_KEY=$NEXT_PUBLIC_TURNSTILE_SITE_KEY
ENV NEXT_TELEMETRY_DISABLED=1

RUN npm run build

# Runtime stage — minimal standalone server
FROM node:22-alpine AS runner

WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

# Non-root user for security
RUN addgroup --system --gid 1001 nodejs && \
    adduser --system --uid 1001 nextjs

COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static
COPY --from=builder --chown=nextjs:nodejs /app/public ./public

USER nextjs

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD wget -qO- http://localhost:3000/en || exit 1

CMD ["node", "server.js"]
