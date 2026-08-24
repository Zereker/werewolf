# 本地开发的常用命令。CI 跑的是同一批检查（见 .github/workflows/ci.yml），
# 提 PR 之前跑一次 make check，可以省掉一轮红灯。
#
# # 引擎在另一个仓库
#
# 内核是独立的 module：github.com/Zereker/hiddenrole，按版本依赖，
# 与其他第三方依赖没有区别。要连着内核一起改，在本地检出它然后加一条
# replace（见 CONTRIBUTING.md），验完再撤掉。
.PHONY: all test test-cover race bench lint vet fmt fmt-check examples check build clean

# 默认目标：跑一遍完整检查
all: check

build:
	go build ./...

test:
	go test ./...

# -coverpkg 只指三套规则包：根包，加 example/ 下的 missions 与 onenight。
# 内核在另一个 module 里，算不进来，它的覆盖率由它自己的 CI 统计。
# example/ 下另外三个（cli、netserver、extension）不算——那三个是使用者，
# 不是被测对象，算进去只会稀释数字。
test-cover:
	go test -coverpkg=github.com/Zereker/werewolf,github.com/Zereker/werewolf/example/missions,github.com/Zereker/werewolf/example/onenight \
		-coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1

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
