# Next.js dashboard image using the standalone output for a small runtime.
FROM node:22-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci || npm install

FROM node:22-alpine AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
# NEXT_PUBLIC_* is inlined into the client bundle at build time.
ARG NEXT_PUBLIC_API_BASE_URL=http://localhost:8080
ENV NEXT_PUBLIC_API_BASE_URL=$NEXT_PUBLIC_API_BASE_URL
# Selects the white-label brand (src/brand). Unset/unknown falls back to "beacon".
# Build a branded image with:  --build-arg NEXT_PUBLIC_BRAND=sysops
ARG NEXT_PUBLIC_BRAND=beacon
ENV NEXT_PUBLIC_BRAND=$NEXT_PUBLIC_BRAND
# "Sign in with Google" client id for the default (beacon) brand — baked into the
# bundle. Empty hides the button. Branded builds normally hardcode it in
# src/brand/<brand>.ts instead; this arg is the env-driven fallback for the default
# brand (handy for local dev and single-brand deploys).
ARG NEXT_PUBLIC_GOOGLE_CLIENT_ID=""
ENV NEXT_PUBLIC_GOOGLE_CLIENT_ID=$NEXT_PUBLIC_GOOGLE_CLIENT_ID
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build

FROM node:22-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
RUN addgroup -g 1001 nodejs && adduser -D -u 1001 -G nodejs nextjs

COPY --from=build /app/public ./public
COPY --from=build --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=build --chown=nextjs:nodejs /app/.next/static ./.next/static

USER nextjs
EXPOSE 3000
ENV PORT=3000
CMD ["node", "server.js"]
