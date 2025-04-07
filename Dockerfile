FROM golang:1.23.0


WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o main ./cmd/server

COPY wait-for-it.sh /wait-for-it.sh

RUN chmod +x /wait-for-it.sh

EXPOSE 8081

CMD ["./main"]