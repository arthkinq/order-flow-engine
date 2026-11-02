FROM golang:alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/worker ./cmd/worker


FROM alpine:3.19 AS api

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app

COPY --from=builder /bin/api /app/api

USER appuser
EXPOSE 50051 2112
ENTRYPOINT ["/app/api"]


FROM alpine:3.19 AS worker

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app

COPY --from=builder /bin/worker /app/worker

USER appuser
EXPOSE 2113
ENTRYPOINT ["/app/worker"]
