BINARY_NAME=awsbatch-failed-job-notifier

all: build
run:
	go run *.go --config config.dev.yml
build:
	go build -o bin/$(BINARY_NAME) -v
test:
	go test -v ./...