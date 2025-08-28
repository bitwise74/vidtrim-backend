FROM nvidia/cuda:12.2.0-runtime-ubuntu22.04

WORKDIR /app

RUN apt-get update && \
    apt-get install -y gcc g++ ffmpeg ca-certificates git make wget tar && \
    rm -rf /var/lib/apt/lists/*

RUN wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz && \
    rm go1.24.0.linux-amd64.tar.gz

ENV PATH=$PATH:/usr/local/go/bin

COPY . .

RUN go mod download
RUN go build -o video-api .

EXPOSE 8080

ENTRYPOINT ["./video-api"]
