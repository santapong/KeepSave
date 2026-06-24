# Build stage. Digest-pinned for supply-chain integrity (INF-2).
FROM node:20-alpine@sha256:fb4cd12c85ee03686f6af5362a0b0d56d50c58a04632e6c0fb8363f609372293 AS build
# tag: node:20-alpine
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
# Digest-pinned to the 1.27-alpine release (INF-2). The unprivileged image
# already runs as non-root; readOnlyRootFilesystem in Helm is satisfied by
# the emptyDir mounts declared in deployment-frontend.yaml.
FROM nginxinc/nginx-unprivileged:1.27-alpine@sha256:65e3e85dbaed8ba248841d9d58a899b6197106c23cb0ff1a132b7bfe0547e4c0
# tag: nginxinc/nginx-unprivileged:1.27-alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 3000
CMD ["nginx", "-g", "daemon off;"]
