# 贡献指南

Contributions are welcome. This document is in Chinese; feel free to open issues
and pull requests in either Chinese or English.

## 跑起来

```sh
go test ./...          # 单元 + 集成 + 200 局随机对局
go test -race ./...    # TCP 服务端示例在 -race 下跑
make lint              # golangci-lint
make check             # 上面全部，加 gofmt 与 go vet
```

三个示例都能直接跑，改动 API 之后请确认它们还活着：

```sh
go run ./example              # 各接口的用法演示
printf 'run\nquit\n' | go run ./example/cli   # 从头玩完一局
go run ./example/extension    # 一个第三方角色（白痴）
```

## 一个好的改动长什么样

### 测试要能抓到问题

**测试通过不等于测试有用。** 这个仓库里每一个行为改动都做过**变异验证**：
把改动反向去掉，跑测试，确认它真的会红。变异的内容与红灯数写进提交信息和
CHANGELOG，好让别人能核对而不是只能相信。

举个真实的例子：守卫的连守限制曾经漏在快照之外整整一版。当时的随机对局
只比阶段与回合，存档读档两边照样能同步走完一整局，只是规则判定不一样了。
后来改成**逐字节比对两边导出的快照**才抓到。

所以提 PR 时，请顺手说明：如果把你的改动去掉，哪一条测试会红。答不上来，
通常说明测试测的是别的东西。

### 状态变更只能经由 Effect

`Resolver` 拿到的是只读的 `GameView`，只能通过返回 `Effect` 表达状态变更。
这不是风格约定，是快照、回放、审计三件事成立的前提——把状态藏在解析器的
字段里，恢复出来的对局是错的**而且不会报错**。

状态放哪儿看作用域：

写一律走 `NewSetVarEffect(scope, key, value)`，读一律走
`GameView.Var(scope, key)`，四格由作用域挑：

| | 无主 | 属于某个玩家 |
|---|---|---|
| **整局有效** | `ScopeGame`（比分、轮到谁） | `ScopeGame.Of(id)`（女巫的药、白痴翻没翻牌） |
| **本回合有效** | `ScopeRound`（今晚的刀口） | `ScopeRound.Of(id)`（今晚谁被守了） |

### 加角色不该改引擎

这是整个设计的标准。如果你的改动需要在引擎里加一个 `if role == X` 或者
`case EventY`，先停下来想想是不是抽象漏了一个口子——**大概率是**。
八个扩展点见 README；内置角色走的是同一条路，它们没有特权。

内核的状态原语只有四条：改存活、写三种作用域的变量、排队一个死亡触发。
`KILL` / `ELIMINATE` / `SHOOT` 这些是规则给「发生了什么」起的名字，
状态机不认得它们——一个 `KILL` 效果单独发出去，谁都不会死。

### 规则要有依据

内置规则以维基百科「狼人殺」条目为基准。改动规则行为时，请在测试或注释里
写清依据的是哪一条，以及为什么这么解读。桌面上有分歧的规则应该做成配置
（见 `GameConfig`），而不是替使用者选一个。

## 提交信息

用 [Conventional Commits](https://www.conventionalcommits.org/)：
`feat:` / `fix:` / `refactor:` / `docs:` / `test:` / `chore:`，
破坏性变更加 `!`。

正文写**为什么**，不写做了什么——做了什么看 diff 就有。一个改动如果修的是
某个具体的错，把那个错讲清楚：什么情况下会触发、症状是什么、为什么之前没
发现。这些是 diff 里读不出来的东西。

## 兼容性

**v1.5.0 起，公开 API 是承诺。** 破坏性变更需要一个大版本，而按 Go 的模块规则
那意味着更换模块路径（`github.com/Zereker/werewolf/v2`）——每个使用者的 import
都要改一次。这个代价是刻意留着的：它让「要不要破坏」成为一个必须认真回答的问题。

因此：先想清楚能不能不破坏。加一个可选项、加一个新函数、给接口加一个带默认
实现的包装，通常都能达到目的。

快照结构变了要递增 `SnapshotVersion`，即便公开 API 没变。

## 发版

发版由 GitHub Actions 完成：**Actions → Release → Run workflow**，填版本号。
tag 与 Release 都由 workflow 创建，发布说明取自 CHANGELOG 对应的小节——
没有小节就发不出去。本地不需要任何权限。
