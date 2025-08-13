FROM golang:1.24.4-alpine AS builder

WORKDIR /app

RUN apk add --no-cache gcc g++ musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN go build -ldflags="-s -w" -o video-api .

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ffmpeg

COPY --from=builder /app/video-api .

EXPOSE 8080

ENTRYPOINT ["./video-api"]
