.PHONY: test build clean proto

# Proto 文件路径
PROTO_DIR := proto
PROTO_FILES := $(wildcard $(PROTO_DIR)/*.proto)

# 生成 Go 代码
proto:
	protoc --go_out=. --go_opt=paths=source_relative $(PROTO_FILES)

# 安装 protoc-gen-go（如果尚未安装）
proto-deps:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

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
