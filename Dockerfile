FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o server .

FROM alpine:3.20

RUN apk add --no-cache tzdata ca-certificates \
 && adduser -D -u 10001 appuser

ENV TZ=Asia/Makassar \
    GIN_MODE=release

WORKDIR /app
COPY --from=builder --chown=appuser:appuser /app/server .

USER appuser

EXPOSE 8080
CMD ["./server"]