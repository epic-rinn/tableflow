# TableFlow admin or PWA image (MVP-21). Build from the repository root:
#   docker build -f deploy/next.Dockerfile --build-arg APP=pwa -t tableflow-pwa .
#   docker build -f deploy/next.Dockerfile --build-arg APP=admin \
#     --build-arg NEXT_PUBLIC_PWA_URL=https://pwa.example -t tableflow-admin .
# Runs the Next.js standalone output as the unprivileged `node` user.
ARG APP
FROM node:24.21.0-bookworm-slim AS build
ARG APP
ARG NEXT_PUBLIC_PWA_URL=""
ENV NEXT_TELEMETRY_DISABLED=1 NEXT_PUBLIC_PWA_URL=${NEXT_PUBLIC_PWA_URL}
RUN corepack enable
WORKDIR /app
COPY src/${APP}/package.json src/${APP}/pnpm-lock.yaml src/${APP}/pnpm-workspace.yaml ./
RUN corepack pnpm install --frozen-lockfile
COPY src/${APP}/ ./
RUN corepack pnpm build && mkdir -p public

FROM node:24.21.0-bookworm-slim
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 HOSTNAME=0.0.0.0 PORT=3000
WORKDIR /app
COPY --from=build --chown=node:node /app/.next/standalone ./
COPY --from=build --chown=node:node /app/.next/static ./.next/static
COPY --from=build --chown=node:node /app/public ./public
USER node
EXPOSE 3000
CMD ["node", "server.js"]
