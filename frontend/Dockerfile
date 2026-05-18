FROM node:20-alpine AS build
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
FROM nginxinc/nginx-unprivileged:alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 3000
CMD ["nginx", "-g", "daemon off;"]
