# syntax=docker/dockerfile:1

FROM node:22-alpine AS build

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build

FROM nginx:1.27-alpine

RUN printf 'server {\n    listen 3000;\n    root /usr/share/nginx/html;\n    index index.html;\n    location / {\n        try_files $uri $uri/ /index.html;\n    }\n}\n' > /etc/nginx/conf.d/default.conf

COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 3000

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=5 \
    CMD wget -qO- "http://127.0.0.1:3000/" >/dev/null || exit 1
