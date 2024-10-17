FROM golang:alpine

RUN apk add make git

RUN mkdir -p /go/src/github.com/berney/rakkess/

WORKDIR /go/src/github.com/berney/rakkess/

CMD git clone --depth 1 https://github.com/berney/rakkess.git . && \
    make all && \
    mv out/* /go/bin
