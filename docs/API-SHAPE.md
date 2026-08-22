# API 里藏着哪些对象

这份文档不提改法，只做一件事：**把当前 API 里那些「概念存在、但代码里没有对应物」
的地方摊开**，供决定要不要动、动哪些。

数据取自 `go doc -all ./engine`：**53 个类型、25 个包级函数、59 个方法**
（收敛作用域之后；此前是 52 / 28 / 57，总数不变）。

判据只有一条：

> **一个概念要画成表格、或者要靠命名前缀才讲得清，说明它在代码里没有对应物。**

---

## 一、变量作用域：一张 2×2 的表，摊成八个平铺的名字 —— 已收敛

这一条最刺眼，因为**证据是我们自己反复写的文档**——`state.go`、`view.go`、
`SCARS.md`、CHANGELOG 里都画过这张表：

|  | 无主 | 属于某个玩家 |
|---|---|---|
| **整局有效** | `GameVar` | `PlayerVar` |
| **本回合有效** | `RoundVar` | `PlayerRoundVar` |

而代码里它是八个互不相干的名字：

|  | 写 | 读 |
|---|---|---|
| 整局·无主 | `NewSetGameVarEffect(k,v)` | `GameVar(k)` |
| 整局·某人 | `NewSetPlayerVarEffect(id,k,v)` | `PlayerVar(id,k)` |
| 本回合·无主 | `NewSetRoundVarEffect(k,v)` | `RoundVar(k)` |
| 本回合·某人 | `NewSetPlayerRoundVarEffect(id,k,v)` | `PlayerRoundVar(id,k)` |

**症状是可验证的**：疤 4 之所以是「缺了一格」，正因为没有任何东西强制这张表完整。
作用域若是一个类型，缺一格根本写不出来；因为是四个函数，少写一个谁也不会发现
——事实上就是少写了，直到阿瓦隆撞上。

「一个概念」= 作用域（时间尺度 × 有没有主人）。
「代码里的对应物」= 没有。

**已收敛**：作用域现在是 `VarScope` 这个类型，四格由两个值叉乘一个方法
得出，写走 `NewSetVarEffect(scope, k, v)`，读走 `Var(scope, k)`：

|  | 无主 | 属于某个玩家 |
|---|---|---|
| **整局有效** | `ScopeGame` | `ScopeGame.Of(id)` |
| **本回合有效** | `ScopeRound` | `ScopeRound.Of(id)` |

顺带发现同一个毛病还在另外两处：`Engine` 上只有两格（有主的读不到），
`Board` 少了「整局·无主」（摆不出带比分的局面）。两处都补齐了。

名字总数没减（描述这张表的从 15 个降到 11 个，内核导出总数仍是 137），
换到的是**完整性**：四格能枚举，缺一格测试先撞上，不必等下一个规则包。

---

## 二、效果构造器：六个自由函数，混着两类东西

```
NewEffect                     规则给「发生了什么」起名字
NewSetAliveEffect             改状态
NewSetVarEffect               改状态
NewAbilityTriggerEffect       下指令：把某人排进某个阶段
NewGotoPhaseEffect            下指令：下一步去哪
NewSetActorsEffect            下指令：谁能在某阶段行动
```

（四个 Var 构造器收敛成一个之后从九个降到六个，但这一条的毛病没变：
两类东西仍然平铺在一起。）

两类东西平铺在一起，没有任何类型区分：**改状态的**和**下指令的**。

后果已经出现过一次：`GOTO_PHASE` 被放进了 `kernelPrimitives` 表——那张表的文档写着
「它们是状态机的记账（谁的存活位翻了、谁身上多了个标记）」，而 `GOTO_PHASE`
**在 `applyEffect` 里根本没有分支，它不改任何状态**。行为是对的（永不外发），
分类是错的。

**分类已收敛，构造器没有。** `kernelPrimitives`（`map[EventType]bool`）换成了
`kernelEvents`（`map[EventType]eventKind`），三类：

```
kindStateWrite   SET_ALIVE / SET_VAR / SET_ACTORS / ABILITY_TRIGGERED
kindControl      GOTO_PHASE           —— 不改状态，只影响下一步去哪
kindReplay       PLAYER_ADDED / PHASE_CHANGED —— 只在回放那条路上有意义
```

原来的二分（改状态 / 下指令）自己也不准：`PLAYER_ADDED` 与 `PHASE_CHANGED`
哪一类都不是，它们是回放记账。

类别成为一个值之后，那句注释就能断言了：每条 `kindStateWrite` 拿一份干净状态
试一遍，改不动就是分错了类；每条非 `kindStateWrite` 应用完状态必须逐字段不变。
把 `GOTO_PHASE` 改回 `kindStateWrite`（也就是今天这个错误）会立刻变红。

`eventKind` 是**未导出的**：外面没有任何调用方需要它，`isInternalEvent` 是它
唯一的出口。概念有了对应物，不必顺手扩一圈公开 API。

**构造器那一面没动**：六个 `NewXxxEffect` 仍然平铺。要不要给它们也上类型
（比如 `Effect` 分成两个类型）是下一个问题——代价是所有规则包的返回值签名。

「一个概念」= 规则对内核说的话，分三类。
「代码里的对应物」= `eventKind`（未导出），构造器仍是平铺的。

---

## 三、扩展点：八件事，二十四个名字 —— 已补齐一半

八个扩展点，每个都摊成 2-3 个名字：

| 扩展点 | 接口 | Func 适配器 | With 选项 |
|---|---|---|---|
| `Resolver` | ✓ | ✓ | ✓ |
| `VictoryChecker` | ✓ | ✓ | ✓ |
| `AudienceProvider` | ✓ | ✓ | ✓ |
| `TeammateProvider` | ✓ | ✓ | ✓ |
| `SpeechProvider` | ✓ | ✓ | ✓ |
| `RoleInfoProvider` | ✓ | ✓ | ✓ |
| `RoleSetup` | ✓ | ✓ | ✓ |
| `GameSetup` | ✓ | ✓ | ✓ |

**已补齐**：`Resolver` 与 `VictoryChecker` 此前没有 Func 适配器，另外六个有。
这个不齐整没有理由，只是历史，现在补上了 `ResolverFunc` 与 `VictoryFunc`
——`TestExtensionPoints_AllHaveFuncAdapters` 把八个函数字面量直接装进一台
引擎，少一个适配器就编译不过。

**剩下的那半还在**：一个扩展点仍是三个名字（接口 + 适配器 + 选项）。
这一半要不要动是另一个问题——三个名字各有各的用处（接口给类型、适配器
给便利、选项给装配），不像作用域那样是同一件事被摊开。

「一个概念」= 一个扩展点。
「代码里的对应物」= 一个接口 + 一个适配器 + 一个选项函数，三个名字。

---

## 四、影子类型：同一批数据的三套形状

同一批游戏状态，在代码里有三副面孔：

| 内部 | 对外只读 | 存档 |
|---|---|---|
| `playerState`（未导出） | `PlayerInfo` / `PublicPlayerInfo` / `SelfInfo` | `PlayerSnapshot` |
| `RoundContext` | `RoundContext` | `RoundCtxSnapshot` |
| `SkillUse` | `SkillUse` | `SkillUseSnapshot` |
| `PendingTrigger` | `PendingTrigger` | `PendingTriggerSnapshot` |

四个 `*Snapshot` 影子类型的存在是**刻意的**（快照是写进存储的格式，字段名必须稳定，
不能随内部重构漂移——这一条写在 `snapshot.go` 里，我仍然认为是对的）。

但视图那一列不是：`PlayerInfo` / `PublicPlayerInfo` / `SelfInfo` 三个类型描述的都是
「一名玩家，按看的人不同露出不同的字段」——**「谁在看」这个维度没有对应物**，
于是变成了三个类型名。

---

## 五、`Engine` 的方法 —— 摘要那一组已收敛

原来是 27 个，其中一串是**同一件事的不同粒度**：`Phase()` `Round()` `Var()`
`PlayerInfo()` `AlivePlayerIDs()` `RoundContext()` 与 `View()` 问的是同一批问题。

此前的辩护是「`View()` 会 clone 整个状态，问一句『现在第几回合』不该付那个代价」
——性能分层，不是重复。这个辩护成立，但它解释的是**为什么有两条路**，
没解释**为什么那条便宜的路要摊成七个方法**。

**已收敛的那一组，理由不是名字多，是会撕裂。** `Phase()` / `Round()` /
`IsGameOver()` / `Winner()` 各取一次读锁：宿主要渲染「第 3 回合的白天」得连问
两次，中间另一个 goroutine 结算掉一个阶段的话，读到的是一组**从来不曾同时
成立**的值。四个合成一个 `Status()`，四项标量在同一个读锁里取出，不分配内存。

```
Status{ Phase, Round, Over, Winner }
```

`TestStatus_IsAtomic` 一边推进阶段一边并发读，断言组合永远合法（结束了就必须
停在 `PhaseEnd`，没结束就不能已经有赢家）。改回四次分别取锁会变红。

**剩下三个没动**：`Var(scope, key)` / `PlayerInfo(id)` / `AlivePlayerIDs()`
带参数或者要分配，不是「摘要字段」，把它们塞进一个结构体只会让每次读都付
不必要的代价。`View()` 那条路照旧。

现在是 23 个方法。

---

## 汇总

| 概念 | 代码里的对应物 | 摊成几个名字 |
|---|---|---|
| 变量作用域（2×2） | ~~没有~~ → `VarScope` | ~~8~~ → 已收敛 |
| 规则对内核说的话（三类） | `eventKind`（未导出） | 6（构造器未收敛） |
| 一个扩展点 | 部分（接口有，装配没有） | 8 × 3 = 24（已齐整，未收敛） |
| 「谁在看这份数据」 | **没有** | 3 |
| 便宜的状态读法（摘要那组） | `Status` | ~~4~~ → 已收敛 |

**53 个导出类型里，相当一部分是「概念没有对应物」摊出来的。**

---

## 有意为之、不该动的

免得后面误伤：

- **四个 `*Snapshot` 影子类型**——快照是写进存储的格式，必须与内部结构解耦。
- **`GameView` 与可变状态分离**——规则拿只读视图、只能返回 `Effect`，这条约束由
  签名保证，是这个库最值钱的性质之一。
- **`Engine` 与 `GameView` 两条读法并存**——性能分层是真的。
