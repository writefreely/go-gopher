FROM golang:alpine

EXPOSE 70/tcp

ENTRYPOINT ["gopherd"]
CMD []

RUN \
    apk add --update git && \
    rm -rf /var/cache/apk/*

RUN mkdir -p /go/src/github.com/prologic/gopherproxy
WORKDIR /go/src/github.com/prologic/go-gopher

COPY . /go/src/github.com/prologic/go-gopher

RUN go get -v -d
RUN go install -v github.com/prologic/go-gopher/...
