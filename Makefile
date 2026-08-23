# 本地开发的常用命令。CI 跑的是同一批检查（见 .github/workflows/ci.yml），
# 提 PR 之前跑一次 make check，可以省掉一轮红灯。
#
# # 这里有两个 module
#
# 引擎独立成了一个 module（hiddenrole/，见 go.mod 里的 replace）。
# Go 的命令**不会**跨 module 边界：在仓库根目录跑 `go test ./...`
# 一个引擎的测试都不会跑到。所以下面每个目标都跑两遍——漏掉哪一遍，
# 那半边就悄悄没人管了。
.PHONY: test test-cover race bench lint vet fmt fmt-check examples check build clean

ENGINE := hiddenrole

# 默认目标：跑一遍完整检查
all: check

build:
	go build ./...
	cd $(ENGINE) && go build ./...

test:
	go test ./...
	cd $(ENGINE) && go test ./...

# 两个 module 各自统计。
#
# 规则包这边 -coverpkg 指到三套规则：内核在另一个 module 里，算不进来。
#
# 引擎那边只统计**内核包自己**，不含 enginetest 子包：那是给规则包用的
# 测试支架，它自己没有测试、也不该有——驱动它的代码在另一个 module 里，
# 跨 module 的覆盖率本来就统计不到。把它算进来会把数字从 87.8% 拉到
# 76.9%，而那 11 个百分点是统计口径的假象，不是没测。
#
# 引擎这个数字**此前被规则包的测试掩盖着**（自测 73.9% 显示成 94%）。
# 独立成库之后它必须自己站住，所以要单独看。
test-cover:
	go test -coverpkg=github.com/Zereker/werewolf,github.com/Zereker/werewolf/missions,github.com/Zereker/werewolf/onenight \
		-coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1
	cd $(ENGINE) && go test -coverprofile=coverage.out . && go tool cover -func=coverage.out | tail -1

race:
	go test -race ./...
	cd $(ENGINE) && go test -race ./...

# 每个基准各跑一次，只确认它们还能跑通，不比对耗时
bench:
	go test -bench=. -benchtime=1x -run '^$$' ./...
	cd $(ENGINE) && go test -bench=. -benchtime=1x -run '^$$' ./...

vet:
	go vet ./...
	cd $(ENGINE) && go vet ./...

lint:
	golangci-lint run
	cd $(ENGINE) && golangci-lint run

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
	cd $(ENGINE) && go clean
