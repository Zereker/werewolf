# 更新日志

本项目遵循 [语义化版本](https://semver.org/lang/zh-CN/)，破坏性变更在每个版本的开头单独列出。

> 发版由 GitHub Actions 完成：**Actions → Release → Run workflow**，填版本号即可。
> tag 与 Release 都由 workflow 创建，发布说明取自本文件对应的小节——
> 没有小节就发不出去。

> 公开的 tag 只有 `v1.0.0` 与 `v1.2.0`。`v1.0.0` 到 `v1.2.0` 之间的全部改动
> 都归入 `v1.2.0` 一节——对使用者而言，中间没有可取用的版本。

## 未发布

### 破坏性变更

- **枚举的 JSON 从编号改成名字。** 此前 `json.Marshal` 出来是 `{"role":2,"phase":21}`，
  每个客户端都得自己维护一张对照表——`example/netserver` 推给客户端的就是这个。
  现在是 `{"role":"WEREWOLF","phase":"NIGHT_GUARD"}`。第三方的自定义取值（1000 起）
  没有名字，仍按编号写；读的时候名字与编号都接受，不认识的名字直接报错而不是
  静默变成零值。**快照版本 3 → 4**，不兼容旧存档。
- **`AudienceOf` 的参数从 `*Effect` 改为 `*Event`。** 这个问题问的是「外面的人该看到
  什么」，而 `OnEvent` 推给调用方的正是 `Event`——此前想按受众路由推送，得手工把
  Event 拼回一个 Effect。手里拿着 Effect（`EndPhase` 的返回值）时用 `Effect.ToEvent()`。

- **事件类型的编号改成三段。** 内部段此前写成「≥ 100」，与「自定义取值从 1000 起」
  这条约定直接打架：第三方定义的每一个事件类型都会被判成引擎内部事件，
  于是白痴翻牌、狼王自爆这类本该全场可见的事情，扩展**根本发不出去**。
  现在 `1..99` 是引擎外部事件、`100..999` 是引擎内部状态变更、`1000` 起是扩展的
  地盘（照常推给 `OnEvent`，`AudienceOf` 回答「不知道」）。

### 新增

- `example/extension`：第三个真实使用者，加了一个**白痴**——被投票放逐时翻牌、
  不出局、此后失去投票权。它的形状与狼王完全不同（不是「死后触发」而是
  「阻止一次死亡再改变往后的能力」），走的是包装内置解析器、否决效果、
  自定义事件类型这条路。上面那条事件编号的 bug 就是写它时撞出来的。
- `example/netserver`：TCP 长连接的服务端，库的第二个真实使用者。命令行主持台
  碰不到的那半边——事件推送、每条连接一份视图、并发、断线重连、超时真的触发——
  由它来压，七条端到端测试全在 `-race` 下跑。

## v1.3.0 — 2026-08-22

本版含破坏性变更。按 Go 的模块规则，v2 及以上需要更换模块路径（`/v2`），
本项目刻意不走那条路——模块路径保持 `github.com/Zereker/werewolf`，
破坏性变更放在 minor 版本里。升级前请通读下面这一节。

### 破坏性变更

- **移除 protobuf 依赖，改用原生 Go 类型。** 这个库从未真正序列化过 protobuf
  （没有 `Marshal`、没有 gRPC），protobuf 的全部作用是声明 6 个枚举和 1 个 struct。
  现在依赖归零，`go.sum` 也随之删除。

  ```go
  pb.RoleType_ROLE_TYPE_WEREWOLF       →  werewolf.RoleWerewolf
  pb.PhaseType_PHASE_TYPE_NIGHT_GUARD  →  werewolf.PhaseNightGuard
  pb.SkillType_SKILL_TYPE_ANTIDOTE     →  werewolf.SkillAntidote
  ```

  `pb.Event` 变成原生的 `Event`，字段 `SourceId`/`TargetId` 按 Go 约定改为
  `SourceID`/`TargetID`，并带上 json tag。枚举数值逐个保持不变，快照格式不受影响。
  `String()` 的输出去掉类型前缀（`PHASE_TYPE_NIGHT_GUARD` → `NIGHT_GUARD`）。

- **删除四个冗余的导出 API**，各有替代：

  | 删除 | 用什么代替 |
  |---|---|
  | `Engine.SetLogger` | `WithLogger` 构造选项 |
  | `Engine.SetMetrics` | `WithMetrics` 构造选项 |
  | `Engine.RegisterResolver` | `WithResolver` 构造选项 |
  | `NewGameError` | `WrapError`（且它会正确挂上哨兵，`errors.Is` 可比对） |

- `IsErrorCode` → `HasCode`，`ErrorCode(err)` → `CodeOf(err)`（与 `ErrorCode` 类型重名）。
- `AudienceOf` 返回 `([]string, bool)`，第二个值表示引擎是否认得该事件类型。
- `PlayerView.Self` 的类型从 `PlayerInfo` 改为 `SelfInfo`，不再包含只有上帝该知道的
  `Protected` 字段。
- `NewEngine` / `MustNewEngine` / `RestoreEngine` / `ReplayEngine` 接受构造选项。
- 快照版本 2 → 3（`PlayerSnapshot` 增加 `LastProtectedRound`）。

### 新增

- `EventVoteTied`：平票有了自己的事件类型（此前挂在 `UNSPECIFIED` 上，受众为空）。
- `PhaseReadiness.Optional`：列出「可以动但还没动」的人。`Ready`/`Pending` 只管必需
  行动，只看它们驱动游戏的话，默认配置下守卫、女巫、预言家一整局都不会被叫到。
- `PhaseStep.Group`：互斥备选组，猎人的「开枪」与「不开枪」提交任一即算完成。
- `GameConfig.PhaseTimeout(phase)`：取某个阶段的建议超时。
- `Event.Canceled` / `Event.Reason`：被规则否决的行动不再与成功的无法区分。
- `example/cli`：一个能真的从头玩完一局的命令行主持台。
- 包文档（`doc.go`）、基准测试、本更新日志。

### 修复

- **回合边界写死了守卫阶段。** 阶段环里没有它时（没有守卫的板子），回合数永远停在 1、
  回合上下文永不重置——女巫用掉的那瓶解药会一夜又一夜地把同一个人救回来。
- **第三方 Resolver 返回的切片里混进 `nil` 会 panic。**
- **`EndPhase()` 的返回值里没有 `GAME_ENDED`**，照着返回值路由的调用方不知道谁赢了。
- **被否决的用毒会广播给全场**，女巫当场暴露。
- **出局的女巫仍能读到今晚的刀口。**
- **被守护的玩家能从自己的视图里读出来。**
- `Validate()` 补上：漏填 `NextPhase`、越界的 `VictoryMode`、`UNSPECIFIED` 与具体角色
  的步骤重叠、跨角色的互斥备选组。
- 指向未配置阶段的死亡触发就地否决，而不是把游戏带进一个空阶段静默收场。
- `IsErrorCode`/`ErrorCode` 改用 `errors.As`，`GameError` 实现 `Unwrap`——此前有五个
  预定义哨兵从未出现在任何返回路径上。
- `PhaseInfo` 的行动者名单顺序不再随机。
- 毒杀结算按 ID 排序，效果流可确定地比对。
- 狼队判定改为按阵营，`AddCustomPlayer` 加入的狼王、狼美人现在真的算狼队。
- `WithData` 撞 nil map、`applyEffect(nil)` 的 panic。

### 测试

- 新增随机对局的不变量测试（`fuzz_test.go`）：随机板子、规则开关、胜负方式、阶段环
  与自定义角色，每一步核对 9 条不变量——视图一致性、快照往返、效果流回放、身份不泄漏、
  受众不外扩、名单顺序稳定、回合边界。5000 局约 5 秒。
- 覆盖率 91.4% → 94.2%，`golangci-lint` 覆盖全部代码（不再排除任何路径）。

## v1.2.0

### 破坏性变更

- 导出方法去掉 `Get` 前缀（`GetPhaseInfo` → `PhaseInfo` 等，共 16 个）。
- `NewEngine` 返回 `error`；`State` 收进包内不再导出。
- Resolver 拿到的是只读的 `GameView` 而非可变状态。

### 新增

- `Engine.PlayerView` / `Engine.AudienceOf`：信息边界收进库内。
- `Engine.Snapshot` / `RestoreEngine`：局面存档与恢复。
- `Engine.EffectLog` / `ReplayEngine`：效果流与回放。
- `Engine.PhaseReadiness`：还差谁行动。
- `VictoryMode`（屠边 / 屠城）与 `RoleCategory`（神职 / 平民 / 狼人）。
- 死亡触发抽象：引擎不再认识猎人，只认识「谁、去哪个阶段结算」。
- `WitchCanUseBothPotions` / `GuardSaveTogetherDies` 等规则开关。

### 修复

- 规则以维基百科「狼人殺」条目为基准逐条核对并固化为测试（`rules_test.go`）。
- 连守判定、猎人开枪的触发条件、屠边计数等多处与规则不符的判定。
- `State` 与 `Engine` 的嵌套双锁；回调改为在锁外执行。
- 主干上必然 panic 的 example；CI 增加实际运行示例的步骤。

## v1.0.0

首个版本：声明式阶段配置、六个内置角色、消息系统、猎人被动触发、Logger/Metrics 接口。
