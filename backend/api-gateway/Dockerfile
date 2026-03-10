# Build stage — context must be repo root so shared/go replace directive resolves
FROM golang:1.24-alpine AS builder

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

# Runtime stage — use alpine (not scratch) so we can include migration files
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/api-gateway ./api-gateway

# Copy database migrations and seeds
COPY database/migrations/ ./migrations/
COPY database/seeds/ ./seeds/

ENV TZ=Africa/Accra
ENV MIGRATIONS_DIR=/app/migrations
ENV SEEDS_DIR=/app/seeds

EXPOSE 3000

ENTRYPOINT ["/app/api-gateway"]
