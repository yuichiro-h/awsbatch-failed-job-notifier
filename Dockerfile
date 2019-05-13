FROM golang:1.12 as build

ADD . /go/src/github.com/yuichiro-h/awsbatch-failed-job-notifier
WORKDIR /go/src/github.com/yuichiro-h/awsbatch-failed-job-notifier
RUN GO111MODULE=on CGO_ENABLED=0 go build -ldflags "-s -w" -o /go/bin/exec

FROM alpine
RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*
COPY --from=build /go/bin/exec /
CMD ["/exec"]