FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server .

FROM alpine:3.20
RUN apk add --no-cache tzdata ca-certificates chromium font-liberation ttf-freefont \
 && ln -s /usr/bin/chromium /usr/bin/google-chrome
ENV TZ=Asia/Makassar
WORKDIR /app
COPY --from=builder /app/server .
EXPOSE 8080
CMD ["./server"]