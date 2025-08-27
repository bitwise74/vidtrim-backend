FROM golang:1.24.4-alpine AS builder

WORKDIR /app

RUN apk add --no-cache gcc g++ musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN go build -ldflags="-s -w" -o video-api .

FROM nvidia/cuda:12.4.1-runtime-ubuntu22.04

WORKDIR /app

RUN apt-get update && \
    apt-get install -y ffmpeg ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/video-api .

EXPOSE 8080

ENTRYPOINT ["./video-api"]
