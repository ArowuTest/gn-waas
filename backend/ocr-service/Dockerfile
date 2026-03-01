FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git ca-certificates tzdata
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/ocr-service ./cmd/main.go

FROM alpine:3.19
RUN apk add --no-cache tesseract-ocr tesseract-ocr-data-eng ca-certificates tzdata
COPY --from=builder /app/ocr-service /ocr-service
ENV TZ=Africa/Accra
EXPOSE 3005
ENTRYPOINT ["/ocr-service"]
