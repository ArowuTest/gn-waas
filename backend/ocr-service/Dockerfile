# Build stage — context must be repo root so shared/go replace directive resolves
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy shared module first (required by replace directive in go.mod)
COPY shared/go/ ./shared/go/

# Copy service module files and download dependencies
COPY backend/ocr-service/go.mod backend/ocr-service/go.sum ./backend/ocr-service/
WORKDIR /app/backend/ocr-service
RUN GOWORK=off go mod download

# Copy service source and build
COPY backend/ocr-service/ ./
RUN GOWORK=off CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /app/ocr-service \
    ./cmd/main.go

# Runtime stage
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /app/ocr-service /ocr-service

ENV TZ=Africa/Accra

EXPOSE 3005

ENTRYPOINT ["/ocr-service"]
