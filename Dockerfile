FROM nvidia/cuda:12.2.0-runtime-ubuntu22.04

WORKDIR /app

RUN apt-get update && \
    apt-get install -y gcc g++ ffmpeg ca-certificates git make && \
    rm -rf /var/lib/apt/lists/*

COPY . .

RUN go mod download
RUN go build -o video-api .

EXPOSE 8080

ENTRYPOINT ["./video-api"]
