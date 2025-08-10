FROM golang:1.24.4

WORKDIR /app

RUN apt-get update && apt-get install -y ffmpeg && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

RUN go build .

EXPOSE 8080

CMD ["./video-api"]