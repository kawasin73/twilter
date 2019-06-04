FROM golang:1.12 as build

WORKDIR /go/src/github.com/kawasin73/twilter

COPY . ./

WORKDIR /go/src/github.com/kawasin73/twilter/cmd/twilter

ENV GO111MODULE=on

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app .

FROM alpine:latest

LABEL maintainer="kawasin73@gmail.com"

RUN apk add --no-cache ca-certificates

COPY --from=build /app /usr/local/bin/twilter

ENTRYPOINT ["/usr/local/bin/twilter"]
