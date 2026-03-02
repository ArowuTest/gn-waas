# DOCKER-2 fix: Accept VITE_API_URL as build argument so it can be injected at build time.
# DOCKER-3 fix: Use --legacy-peer-deps to resolve react-leaflet@5 / React 18 conflict.
FROM node:22-alpine AS builder
WORKDIR /app

# Accept build-time API URL (injected by docker-compose build args)
ARG VITE_API_URL=http://localhost:3000
ARG VITE_KEYCLOAK_URL=http://localhost:8080
ARG VITE_KEYCLOAK_REALM=gnwaas
ARG VITE_KEYCLOAK_CLIENT_ID=gnwaas-portal
ARG VITE_MAPBOX_TOKEN=""

# Make build args available to Vite at build time
ENV VITE_API_URL=$VITE_API_URL
ENV VITE_KEYCLOAK_URL=$VITE_KEYCLOAK_URL
ENV VITE_KEYCLOAK_REALM=$VITE_KEYCLOAK_REALM
ENV VITE_KEYCLOAK_CLIENT_ID=$VITE_KEYCLOAK_CLIENT_ID
ENV VITE_MAPBOX_TOKEN=$VITE_MAPBOX_TOKEN

COPY package*.json ./
# DOCKER-3 fix: --legacy-peer-deps resolves react-leaflet@5 requiring React 19
RUN npm ci --legacy-peer-deps
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 8083
CMD ["nginx", "-g", "daemon off;"]
