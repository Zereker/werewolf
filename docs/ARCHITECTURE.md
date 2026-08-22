# Werewolf 游戏引擎架构

## 概述

本引擎是一个**无时钟、无 IO、无并发调度**的纯状态机库。它不决定「什么时候
天亮」，只回答「在当前阶段，谁能做什么，做完之后世界变成什么样」。计时、网络、
持久化、AI 决策全部由调用方负责。

这条边界决定了后面所有的设计。

## 设计理念

### 1. Phase 为中心，而非 Role 为中心

规则挂在**阶段**上，不挂在角色上。「守卫能守谁」不是守卫这个类的方法，而是
`NIGHT_GUARD` 阶段的配置加上 `GuardResolver` 的判定。

好处是加角色不需要改引擎：新角色 = 新的 `PhaseConfig` + 新的 `Resolver`。

### 2. 状态变更一律经由 Effect

Resolver 拿到的是只读的 `GameView`，只能产出 `Effect` 描述「想发生什么」，
由内部唯一的写入点统一落地。这条约束**由签名保证**，不是靠约定——
曾经它只写在文档里，而 Resolver 收到的是可变的状态对象，
任何实现都能绕开整条管线。

```
SkillUse ──► Resolver ──► []*Effect ──► State.ApplyEffect ──► 新状态
  (输入)      (纯函数)      (描述)         (唯一写入点)
```

这带来三个直接收益：

- **可测**：Resolver 是纯函数，给定输入必得确定的 Effect 列表
- **可审计**：一局游戏就是一串 Effect，可序列化、可回放、可做战报
- **可取消**：`Effect.Canceled` + `Reason` 让「为什么没生效」成为一等公民
  （被守护、无解药、连守限制……都是取消原因而不是静默丢弃）

历史教训：曾经 `WolfResolver` 在目标被守护时直接不产出刀口 Effect，
导致「同守同救」这个局面在引擎里根本无法构成。现在刀口一律记录，
生死判定统一收敛到结算阶段——**判定要集中，信息不要提前丢弃**。

### 3. 配置是数据，不是代码

阶段流转、每阶段可用技能、规则变体全部是可序列化的数据结构。
`GameConfig` 里每个布尔开关都对应维基条目中的一条规则变体。

### 4. 单一真相来源

「玩家此刻能用什么技能」只有一个来源：`PhaseConfig.Steps`。
`SubmitSkillUse` 的校验、`PhaseInfo` 的对外宣告、`PlayerView`
的视角、`PhaseReadiness` 的就绪判定，全部由它派生。

历史教训：这两者曾各自硬编码技能列表，导致 `PhaseInfo` 宣告猎人可以
`SKIP`、而 `SubmitSkillUse` 拒绝 `SKIP` 的自相矛盾。

### 5. 信息边界属于引擎

调用方作为主持人需要上帝视角，但它不该被迫自己实现「投影」——
那是整局游戏最安全攸关的一段逻辑，放在库外意味着每个使用者都要重写一遍，
而且错一次游戏就废了。`PlayerView` 与 `AudienceOf` 把这件事收回库内。

### 6. 引擎不认识具体角色

猎人曾被写死在阶段流转里：`calculateNextPhase` 有它的分支，
`RoundContext` 有它的专属字段。每加一个死亡触发角色就要再改一遍引擎。
现在引擎只认识「谁、去哪个阶段结算」这个抽象，具体是谁由 Resolver 决定。

## 架构图

```
                     ┌──────────────────────────────┐
   调用方             │           Engine             │
  （计时/网络/AI）  ──►│  ┌────────────────────────┐  │
                     │  │ SubmitSkillUse         │  │  收集技能
                     │  │   └─ Phase.Validate    │  │  （校验依据 Steps）
                     │  ├────────────────────────┤  │
                     │  │ EndPhase               │  │  推进游戏
                     │  │   1. Resolver.Resolve  │──┼──► []*Effect
                     │  │   2. State.ApplyEffect │  │
                     │  │   3. 猎人待结算？        │  │
                     │  │   4. CheckVictory      │  │
                     │  │   5. 流转下一阶段        │  │
                     │  ├────────────────────────┤  │
                     │  │ PhaseInfo           │──┼──► 谁该行动、能用什么
                     │  │ SendMessage            │──┼──► 按阶段路由发言
                     │  └────────────────────────┘  │
                     └───────┬──────────────┬───────┘
                             │              │
                     ┌───────▼──────┐  ┌────▼─────────┐
                     │    State     │  │    Phase     │
                     │  玩家/回合上下文 │  │ 配置 + 解析器注册 │
                     └──────────────┘  └──────────────┘
```

## 核心模块

### Engine（engine.go）

轻量状态机，对外只有三类方法：

| 类别 | 方法 | 说明 |
|------|------|------|
| 建局 | `AddPlayer` / `AddCustomPlayer` | 只能在 `Start` 之前调用，全部返回 error |
| 推进 | `Start` / `EndPhase` | `EndPhase` 是唯一推进入口 |
| 输入 | `SubmitSkillUse` / `SendMessage` | 技能与发言两条独立通道 |
| 读取 | `PhaseInfo` / `PlayerInfo` / `RoundContext` … | 一律返回只读副本 |

**非法输入一律返回 error，不静默生效**：重复 ID、空 ID、把上帝当玩家、
开局后再加人、缺狼或缺好人的板子，都在入口处拒绝。阵营与角色类别由角色推导
（`CampOf` / `CategoryOf`），调用方无从传错；扩展角色走 `AddCustomPlayer`
显式指定。

**并发模型**：所有导出方法可并发调用。用户回调（`OnEvent` / `OnMessage`）
一律在**释放锁之后**执行，且 handler 列表在锁内快照——既不会死锁（回调里
可以安全调用 Engine 方法），也不会与 `OnEvent` 的并发注册产生竞争。
单个 handler panic 被隔离并记录 Error 日志，不影响其他 handler。

### State（state.go）

持有全部游戏状态，是**唯一的写入点**（`ApplyEffect`）。

- `PlayerState`：身份、存活、女巫药剂、守卫上回合目标
- `RoundContext`：回合内的临时状态（刀口、被守、被救、被毒、猎人触发），
  每进入新的一夜重建
- `RoleCategory`：神职 / 平民 / 狼人，屠边判定需要这个维度，
  而 `pb.Camp` 只有好人/狼人两值，表达不了

**猎人触发标记是一次性的**：由死亡结算置位，进入猎人阶段后必须
`ConsumeHunterTrigger` 消费。不消费的话它会在整个回合内持续为真，
导致夜里开过枪的猎人在当天投票后被再次拉进 `DAY_HUNTER`。

### Phase（phase.go）

阶段配置的查询入口 + 解析器注册表 + 技能校验（`ValidateSkillUse`）。

### Resolver（resolver.go）

每阶段一个，签名统一：

```go
Resolve(uses []*SkillUse, state *State, config *GameConfig) []*Effect
```

夜晚的判定顺序值得单独说明。**信息在前面的阶段收集，判定在结算阶段集中**：

```
NIGHT_GUARD    GuardResolver   ──► PROTECT（可能被连守/自守限制取消）
NIGHT_WOLF     WolfResolver    ──► SET_NIGHT_KILL（无论是否被守都记录）
NIGHT_WITCH    WitchResolver   ──► SAVE / USE_POISON（同夜只能用一瓶药）
NIGHT_SEER     SeerResolver    ──► CHECK（只报阵营，不报角色）
NIGHT_RESOLVE  NightResolve    ──► KILL / POISON / HUNTER_TRIGGERED
                                   ↑ 刀口生死在这里才定
```

刀口结算表：

| 被守 | 被救 | 结果 |
|:---:|:---:|------|
| ✓ | ✓ | `GuardSaveTogetherDies`（默认死亡，即同守同救） |
| ✓ | ✗ | `SameGuardKillIsEmpty`（默认空刀） |
| ✗ | ✓ | 救回 |
| ✗ | ✗ | 死亡 |

### Effect（effect.go）

状态变更的描述。事件类型按编号分两类：

- `< 100`：外部可见事件，会通过 `OnEvent` 推给调用方
- `>= 100`：内部状态变更（`SET_NIGHT_KILL`、`USE_ANTIDOTE`、
  `HUNTER_TRIGGERED` 等），不外发

## 数据流

### 一个阶段的完整生命周期

```
1. 调用方读 PhaseInfo()      ──► 得知本阶段谁该行动、能用什么技能
2. 调用方收集玩家决策            ──► （超时、AI、网络，都是调用方的事）
3. SubmitSkillUse() × N         ──► Phase.ValidateSkillUse 校验后入队
4. 调用方调 EndPhase()
   ├─ Resolver.Resolve(pendingUses) ──► []*Effect
   ├─ State.ApplyEffect(每个 effect)
   ├─ 计算下一阶段（含猎人动态触发）
   ├─ CheckVictory —— 若下一阶段是猎人阶段则推迟
   └─ 锁外分发外部可见事件
5. 回到 1
```

**为什么胜负判定要推迟**：猎人被刀时可能已经构成屠神，但他那一枪可能带走
最后一只狼、让好人反胜。判定必须排在死亡技能结算之后。

## 扩展点

### 添加新角色

1. `proto/event.proto` 加 `RoleType`（及所需的 `SkillType`）
2. `CategoryOf` 加类别映射，或调用方用 `State.SetPlayerCategory` 指定
3. 若需要独立行动窗口：加 `PhaseConfig` 并接进流转链
4. 写对应的 `Resolver`，在 `NewPhase` 注册

### 添加规则变体

在 `GameConfig` 加开关，在对应 Resolver 读取。**默认值以维基条目为准**，
并在 `rules_test.go` 里补双向测试（开、关各一条）。

### 自定义阶段流程

`GameConfig.Phases` 是一张 `map[PhaseType]*PhaseConfig`，每个配置声明自己的
`NextPhase`。替换这张表即可改变整个流程，无需改动引擎代码。

## 命名约定

只读方法**不加 `Get` 前缀**——这是 Go 的通行风格（Effective Go）：
`e.Phase()` 而不是 `e.GetCurrentPhase()`，`e.PlayerInfo(id)` 而不是
`e.GetPlayerInfo(id)`。方法名与返回类型同名是可以的，标准库里
`time.Time.Location()` 返回 `*Location` 就是这个形状。

包外用不上的东西一律不导出。阶段管理器（`phaseManager`）没有注入口、
状态对象（`gameState`）是实现细节、投票统计结果只在包内流转——
它们导出只会让 `go doc` 更长，不会让任何人多做成一件事。

## 不做什么

明确排除在范围外的东西，避免误解：

- **不计时**：`Timeout` 字段只是给调用方的建议值
- **不做角色分配**：谁是狼由调用方决定并通过 `AddPlayer` 告知
- **不做发牌随机**：没有洗牌逻辑，座位与身份的对应关系由调用方给定
- **不做存储**：提供 `Snapshot` / `RestoreEngine` 导出与重建局面，
  但存到哪、怎么存由调用方决定
- **不决定阶段何时结束**：`PhaseReadiness` 告诉你还差谁，
  是继续等还是超时推进由调用方决定，`EndPhase` 不会因未就绪而拒绝
- **不做网络与 AI**

## 存档

`Engine.Snapshot()` 导出局面，`RestoreEngine(config, snap)` 重建引擎。

**快照类型与引擎内部类型是刻意分开的两套**：内部结构随重构演进，
而快照是写进存储的格式，字段名必须稳定。转换集中在 `snapshot.go`，
增减字段时那里会显式报错，不会悄悄丢数据。

设计取舍：

- **不含规则配置**。快照只记录局面，`GameConfig` 由调用方在恢复时提供——
  规则的版本管理是调用方的事，把规则混进存档只会让两边都说不清。
- **含未结算技能**。`pendingUses` 一并导出，因此存档点不必卡在阶段边界。
- **枚举按数值序列化**。protobuf 的枚举编号是稳定契约，名称则可能被重命名。
- **输出确定**。集合与玩家列表都排序后导出，同一局面的字节一致，
  便于比对与幂等写入（Go 的 map 遍历顺序是随机的，不排序就做不到）。
- **版本不匹配直接拒绝**，而不是按新结构去解读旧数据——那会得到一个
  看似正常、实则错乱的局面。

## 扩展契约

内置的六个角色是一套默认板子，不是能力上限。第三方加入新角色只需要：

1. 用超出内置枚举的取值定义角色、技能、阶段（建议从 1000 起）
2. 在 `GameConfig.Phases` 里声明该阶段
3. `Engine.RegisterResolver` 注册解析器
4. `Engine.AddCustomPlayer` 显式给出阵营与角色类别

死亡时触发的能力由 Resolver 产出 `NewAbilityTriggerEffect`，
引擎会把它排入待结算队列、自动流转过去，并把胜负判定推迟到结算之后。

`extension_test.go` 用「狼王」把这条路径完整走通，全程只用导出 API。
这个测试的意义不只是覆盖率——它是扩展契约本身的可执行说明，
契约一旦被破坏，它会先于使用者发现。

## 效果流

`EffectLog` 累积自建局以来的全部 Effect，`ReplayEngine` 按流重建局面。
`PLAYER_ADDED` / `GAME_STARTED` / `PHASE_CHANGED` 三个事件让效果流自洽——
不依赖任何外部信息即可完整重建。

与快照的分工：**效果流是历史，快照是状态**。
持久化用快照（`Effect.Data` 经 JSON 往返会退化类型），
进程内的回放、复盘与排查用效果流。

## 规则依据

规则以中文维基百科「狼人殺」条目为基准，逐条固化在 `rules_test.go`（R1–R11），
引擎自定的口径另行编号（D1–D3）。新增规则请先在那里写测试。

`rules_test.go` 中的 `knownDeviations` 用于登记「已知不符、尚未修复」的行为，
登记项默认 Skip；`WEREWOLF_STRICT_RULES=1` 可强制执行，用于驱动修复。
