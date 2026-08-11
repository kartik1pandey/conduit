FROM node:20-slim AS deps
WORKDIR /app
# argon2's npm package has no prebuilt binary for this exact platform/node
# combination, so it compiles from source via node-gyp — which needs
# Python and a C++ toolchain that node:20-slim doesn't ship by default.
RUN apt-get update && apt-get install -y --no-install-recommends python3 make g++ \
    && rm -rf /var/lib/apt/lists/*
COPY package.json package-lock.json ./
RUN npm ci

FROM node:20-slim AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
# `next build` imports every route module to collect its metadata, with no
# real runtime environment available — src/lib/db.ts's Pool is
# constructed lazily specifically so that doesn't require live
# infrastructure, but Auth.js's own config still wants non-empty strings
# present. These are build-time placeholders only, overridden by real
# values at container runtime (see docker-compose.yml / .env.example).
ENV AUTH_SECRET=build-time-placeholder-not-used-at-runtime
ENV AUTH_GITHUB_ID=build-time-placeholder
ENV AUTH_GITHUB_SECRET=build-time-placeholder
ENV CONDUIT_CORE_URL=http://build-time-placeholder
RUN npm run build

FROM node:20-slim AS runtime
WORKDIR /app
RUN useradd --uid 10001 --create-home conduit
ENV NODE_ENV=production
COPY --from=build /app/.next/standalone ./
# argon2 loads its compiled .node binary via a runtime directory scan
# (node-gyp-build/prebuildify), not a statically analyzable require path —
# Next.js's standalone output tracer (@vercel/nft) can't see that
# reference and silently omits the binary, which crashes the process (a
# segfault, not a clean error) the moment anything imports argon2. Copying
# the fully-installed package from the deps stage, where npm ci actually
# ran node-gyp against this container's real architecture, overwrites
# whatever partial copy the trace produced.
COPY --from=deps /app/node_modules/argon2 ./node_modules/argon2
COPY --from=build /app/.next/static ./.next/static
COPY --from=build /app/public ./public
COPY --from=build /app/migrations ./migrations
COPY --from=build /app/scripts ./scripts
COPY --from=build /app/docker-entrypoint.sh ./
RUN chmod +x docker-entrypoint.sh
USER conduit
EXPOSE 3000
ENTRYPOINT ["./docker-entrypoint.sh"]
