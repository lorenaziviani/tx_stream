FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o txstream ./cmd/txstream

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

RUN addgroup -g 1001 -S txstream && \
    adduser -u 1001 -S txstream -G txstream

WORKDIR /app

COPY --from=builder /app/txstream .

COPY --from=builder /app/migrations ./migrations

RUN chown -R txstream:txstream /app

USER txstream

EXPOSE 8080

ENV SERVER_PORT=8080
ENV SERVER_HOST=0.0.0.0

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./txstream"] 