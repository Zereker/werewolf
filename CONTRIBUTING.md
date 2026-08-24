# 贡献指南

Contributions are welcome. This document is in Chinese; feel free to open issues
and pull requests in either Chinese or English.


## 内核在另一个 module 里

引擎已独立成 [`hiddenrole`](https://github.com/Zereker/hiddenrole)，有自己的 `go.mod`、自己的
[CONTRIBUTING](https://github.com/Zereker/hiddenrole/blob/master/CONTRIBUTING.md)、**已冻结的 API**。

本仓库按版本依赖它，与其他第三方依赖没有区别，`go test ./...` 跑的是
下载下来的那一份。

**要连着内核一起改**，在本地把它检出到并排的目录，加一条 replace：

```sh
git clone https://github.com/Zereker/hiddenrole ../hiddenrole
go mod edit -replace github.com/Zereker/hiddenrole=../hiddenrole
```

验完撤掉（`go mod edit -dropreplace github.com/Zereker/hiddenrole`）——
**这条 replace 不要提交**，它会让 CI 与使用者拿到不同的引擎。

改内核请读[它自己的 CONTRIBUTING](https://github.com/Zereker/hiddenrole/blob/master/CONTRIBUTING.md)
——那边的纪律更紧（API 冻结、变异验证、三套规则包一起验）。

## 用什么语言写

**这个仓库面向中文开发者，但注释用什么语言按包分，跟着规则的出身走。**

| 包 | 语言 | 实现的是 |
|---|---|---|
| 根包 | **中文** | 狼人杀的中文规则 |
| [`missions/`](missions) | **英文** | The Resistance 与它的 Avalon 变体 |
| [`onenight/`](onenight) | **英文** | One Night Ultimate Werewolf |

理由不是偏好，是**规则原文是什么语言**。根包实现的是中文桌上那一套：
屠边屠城、同守同救、守卫不能连守、上帝、12 人标准板——这些概念的准确
表述本来就是中文的。「屠边」译成 side-wipe 已经丢了「神职 / 平民」那层
结构，注释里写「预言家验人」比 "the seer checks someone" 更贴规则原文。

另外两套反过来。梅林、派西维尔、莫德雷德、莫甘娜、奥伯伦、盗贼、捣蛋鬼、
皮匠——这些名字的**原文就是英文**，中文是译名，而且译名不止一种。写
Merlin 不是崇洋，是写规则书上那个词；写「梅林」反而要求读者先做一次
回译才能对上英文规则书与几乎全部社区讨论。

**内核那边一律英文**（见
[hiddenrole](https://github.com/Zereker/hiddenrole)）。它是给陌生人
import 的库，注释承载的是判断过程——为什么内核不认得任何取值、
为什么三副面孔不合并——那些论证只有中文的话，一半价值锁死在语言里。

同一条判据贯穿全部四处：**注释服务于读它的人和它描述的规则，不服务于
仓库的统一。** 所以这不是「一个翻了一个没翻」；改一个包的注释语言之前，
先说清那个包的规则原文是什么语言。

### 两份 README

| | 是什么 | 谁维护 |
|---|---|---|
| [`README.md`](README.md) | 完整参考，八百多行，本仓库的正本 | 改动跟着代码走 |
| [`README.en.md`](README.en.md) | **独立的短英文简介**，两百行 | 只在「这是什么、怎么起步」变了时才动 |

**后者不是前者的翻译，也不要把它补成翻译。** 它是给英文读者的门面：
说清这是什么、能不能用、去哪看内核，然后指向
[hiddenrole](https://github.com/Zereker/hiddenrole)（那边是英文的）。
正因为它不追着中文版跑，它才不会漂移。

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

内核的状态原语只有两条：改存活、写变量（四种作用域见下）。外加两条控制指令：改写下一阶段、排一笔绕道的欠账。
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
