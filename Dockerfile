FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git ca-certificates gcc musl-dev

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o api-server ./cmd/server

FROM alpine:3.18

RUN apk add --no-cache ca-certificates curl tzdata

RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /build/api-server .
COPY --from=builder /build/migrations ./migrations

RUN chown -R appuser:appgroup /app && chmod +x ./api-server

USER appuser

EXPOSE 8080

CMD ["./api-server"]