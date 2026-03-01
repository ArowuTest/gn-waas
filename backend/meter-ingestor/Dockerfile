# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /workspace

# Copy go workspace files
COPY go.work go.work.sum* ./

# Copy all modules (workspace requires all modules present)
COPY shared/go/ shared/go/
COPY backend/meter-ingestor/ backend/meter-ingestor/

# Download dependencies
WORKDIR /workspace/backend/meter-ingestor
RUN GONOSUMCHECK=* GONOSUMDB=* GOWORK=on go mod download

# Build the binary
RUN GONOSUMCHECK=* GONOSUMDB=* GOWORK=on CGO_ENABLED=0 GOOS=linux \
    go build -ldflags="-w -s" -o /meter-ingestor ./cmd/

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates wget tzdata

WORKDIR /app

COPY --from=builder /meter-ingestor /app/meter-ingestor

# Non-root user
RUN addgroup -S gnwaas && adduser -S gnwaas -G gnwaas
USER gnwaas

EXPOSE 3007 50051

HEALTHCHECK --interval=30s --timeout=10s --start-period=15s --retries=3 \
    CMD wget -q --spider http://localhost:3007/health || exit 1

ENTRYPOINT ["/app/meter-ingestor"]
