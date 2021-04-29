FROM golang:alpine

WORKDIR /go/src/go_like_bot
COPY . .

RUN go build .

ENTRYPOINT ["/go/src/go_like_bot/go_like_bot"]