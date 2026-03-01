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
    -o /app/cdc-ingestor \
    ./cmd/main.go

# ─── Stage 2: Minimal runtime ─────────────────────────────────────────────────
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /app/cdc-ingestor /cdc-ingestor

# CDC Ingestor environment variables
ENV TZ=Africa/Accra
ENV PORT=3006
ENV GWL_DB_URL=""
ENV TARGET_DB_URL=""
ENV REPLICATION_SLOT=gnwaas_cdc_slot
ENV PUBLICATION_NAME=gnwaas_publication
ENV LOG_LEVEL=info

EXPOSE 3006

ENTRYPOINT ["/cdc-ingestor"]
