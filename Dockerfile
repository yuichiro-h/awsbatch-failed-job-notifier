FROM golang:1.11-alpine3.7 AS builder
ADD . /go/src/github.com/yuichiro-h/awsbatch-failed-job-notifier
WORKDIR /go/src/github.com/yuichiro-h/awsbatch-failed-job-notifier
RUN go build -ldflags "-s -w" -o bin/awsbatch-failed-job-notifier -v

FROM alpine

RUN apk add --update \
    ca-certificates && \
    rm -rf /var/cache/apk/*

COPY --from=builder \
    /go/src/github.com/yuichiro-h/awsbatch-failed-job-notifier/bin/awsbatch-failed-job-notifier \
    /awsbatch-failed-job-notifier

ENTRYPOINT ["/awsbatch-failed-job-notifier"]