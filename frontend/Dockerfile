# Build stage. Digest-pinned for supply-chain integrity (INF-2).
FROM node:24.21.0-alpine@sha256:ebfe2f90462722a7a4de65e91990e97fe0d401c70e0e762c5b53302f905ec1c1 AS build
# tag: node:24.21.0-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

# nginx-unprivileged runs the master + workers as uid 101 with no root
# anywhere in the stack (per audit S-M8). Listens on a non-privileged
# port out of the box, matching our 3000 in nginx.conf. Only relevant
# for the Option-C self-hosted path - the Vercel deploy ignores this
# image entirely.
#
# Digest-pinned to the 1.30.5-alpine-slim release (INF-2). The unprivileged image
# already runs as non-root; readOnlyRootFilesystem in Helm is satisfied by
# the emptyDir mounts declared in deployment-frontend.yaml.
FROM nginxinc/nginx-unprivileged:1.31.7-alpine-slim@sha256:b94a56c52fd315bbd8f1f4368d64c25983351c978313e02feff2c0b35e856f2d
# tag: nginxinc/nginx-unprivileged:1.30.5-alpine-slim
# The pinned nginx release includes PCRE2 10.48. Patch CVE-2026-103111 with
# the exact fixed Alpine package until the upstream image incorporates it.
USER 0
RUN apk add --no-cache pcre2=10.49-r0
USER 101
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 3000
CMD ["nginx", "-g", "daemon off;"]
