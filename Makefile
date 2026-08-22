# 本地开发的常用命令。CI 跑的是同一批检查（见 .github/workflows/ci.yml），
# 提 PR 之前跑一次 make check，可以省掉一轮红灯。
.PHONY: test test-cover race bench lint vet fmt fmt-check examples check build clean

# 默认目标：跑一遍完整检查
all: check

build:
	go build ./...

test:
	go test ./...

test-cover:
	go test -cover ./...

race:
	go test -race ./...

# 每个基准各跑一次，只确认它们还能跑通，不比对耗时
bench:
	go test -bench=. -benchtime=1x -run '^$$' ./...

vet:
	go vet ./...

lint:
	golangci-lint run

fmt:
	gofmt -w .

fmt-check:
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then echo "以下文件未格式化："; echo "$$out"; exit 1; fi

# 只编译不运行是不够的：示例里一个必然 panic 的调用曾就此合入主干
examples:
	go run ./example > /dev/null
	go run ./example/cli -seed 7 < example/cli/testdata/demo.txt > /dev/null
	go run ./example/extension > /dev/null

check: build vet fmt-check lint test race examples

clean:
	go clean
