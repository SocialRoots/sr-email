# Build stage
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /go/bin/api ./cmd/api
RUN CGO_ENABLED=0 go build -o /go/bin/cron ./cmd/cron

# Runtime stage
FROM alpine:3.19
RUN apk add --no-cache ca-certificates

COPY --from=builder /go/bin/api /go/bin/api
COPY --from=builder /go/bin/cron /go/bin/cron
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["/entrypoint.sh"]