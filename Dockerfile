FROM golang:1.16.7-buster

WORKDIR /app
COPY . ./

RUN go mod download
RUN go test ./...
RUN go build -o /app/server 

EXPOSE 6060

CMD ["/app/server"]
