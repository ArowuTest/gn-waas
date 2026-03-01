# ─── Stage 1: Build ───────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Cache dependencies separately from source
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.version=${VERSION:-dev}" \
    -o /app/gra-bridge \
    ./cmd/main.go

# ─── Stage 2: Minimal runtime ─────────────────────────────────────────────────
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /app/gra-bridge /gra-bridge

# GRA Bridge environment variables
ENV TZ=Africa/Accra
ENV PORT=3003
ENV GRA_VSDC_URL=https://vsdc.gra.gov.gh/api/v1
ENV GRA_API_KEY=""
ENV DB_URL=""
ENV LOG_LEVEL=info

EXPOSE 3003

ENTRYPOINT ["/gra-bridge"]
