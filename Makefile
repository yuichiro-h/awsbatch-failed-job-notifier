BINARY_NAME=awsbatch-failed-job-notifier

all: build
run:
	CONFIG_PATH=config.dev.yml go run *.go
build:
	go build -o bin/$(BINARY_NAME) -v
test:
	go test -v ./...