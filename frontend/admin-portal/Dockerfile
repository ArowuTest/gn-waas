# DOCKER-2 fix: Accept VITE_API_URL as build argument so it can be injected at build time.
# Use --legacy-peer-deps to resolve peer dependency conflicts.
FROM node:22-alpine AS builder
WORKDIR /app

# Accept build-time API URL (injected by docker-compose build args)
ARG VITE_API_URL=""
ARG VITE_KEYCLOAK_URL=http://localhost:8080
ARG VITE_KEYCLOAK_REALM=gnwaas
ARG VITE_KEYCLOAK_CLIENT_ID=gnwaas-portal
ARG VITE_MAPBOX_TOKEN=""
# FE-H01 fix: explicitly default VITE_DEV_MODE to false in production builds.
# This prevents dev-only login buttons and debug features from being exposed
# in the production container. Override with --build-arg VITE_DEV_MODE=true
# ONLY for local development builds.
ARG VITE_DEV_MODE=false

# Make build args available to Vite at build time
ENV VITE_API_URL=$VITE_API_URL
ENV VITE_KEYCLOAK_URL=$VITE_KEYCLOAK_URL
ENV VITE_KEYCLOAK_REALM=$VITE_KEYCLOAK_REALM
ENV VITE_KEYCLOAK_CLIENT_ID=$VITE_KEYCLOAK_CLIENT_ID
ENV VITE_MAPBOX_TOKEN=$VITE_MAPBOX_TOKEN
ENV VITE_DEV_MODE=$VITE_DEV_MODE

COPY package*.json ./
# --legacy-peer-deps resolves peer dependency conflicts (react-leaflet@4 + React 18)
RUN npm ci --legacy-peer-deps
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
