# Build stage — context must be repo root so shared/go replace directive resolves
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy shared module first (required by replace directive in go.mod)
COPY shared/go/ ./shared/go/

# Copy service module files and download dependencies
COPY backend/api-gateway/go.mod backend/api-gateway/go.sum ./backend/api-gateway/
WORKDIR /app/backend/api-gateway
RUN GOWORK=off go mod download

# Copy service source and build
COPY backend/api-gateway/ ./
RUN GOWORK=off CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /app/api-gateway \
    ./cmd/main.go

# Runtime stage
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /app/api-gateway /api-gateway

ENV TZ=Africa/Accra

EXPOSE 3000

ENTRYPOINT ["/api-gateway"]
