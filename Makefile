.PHONY: test build clean

test:
	go test -v ./...

test-cover:
	go test -cover ./...

build:
	go build ./...

clean:
	go clean

fmt:
	go fmt ./...
