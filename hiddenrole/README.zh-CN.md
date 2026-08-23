# hiddenrole

[English](README.md) · **中文**

**社会推理游戏内核**，纯 Go，**零依赖**。它不知道狼人杀是什么。

[![Go Reference](https://pkg.go.dev/badge/github.com/Zereker/hiddenrole.svg)](https://pkg.go.dev/github.com/Zereker/hiddenrole)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

```
go get github.com/Zereker/hiddenrole
```

它知道的是：有一批玩家，有一个阶段环，每个阶段结束时问一下那个阶段的
解析器「发生了什么」，然后把结果折进状态。以及最难的那件事——**谁有权
知道什么**。

角色、技能、死法、胜负、信息边界，全部由规则包经公开选项装上来。

## 「它真的不知道」这件事可以查

本包的非测试源码里，`RoleType` 的取值一共两个（`RoleUnspecified`、
`RoleSystem`），`PhaseType` 三个、`SkillType` 三个，全在
[`types.go`](types.go) 里。「女巫」「狼人」「NIGHT_WITCH」一个都没有。

更硬的证据是**三套互不相干的规则包**跑在它上面，没有一个取值相同：

| 规则包 | 玩的是什么 | 它证明了什么 |
|---|---|---|
| [werewolf](https://github.com/Zereker/werewolf) | 狼人杀 | 出局是核心机制，八个阶段成环 |
| [werewolf/missions](https://github.com/Zereker/werewolf/tree/main/missions) | 任务制（提名 / 表决 / 任务 / 刺杀） | **一个人都不出局**也能跑；阶段流转由结算结果决定 |
| [werewolf/onenight](https://github.com/Zereker/werewolf/tree/main/onenight) | 单夜换牌制 | 身份**分两层**：发到手的那张定夜里做什么，手上那张定结算算哪边 |

第三套写下来只逼出**零个破坏性 API 变更**——[API 已冻结](API.md)，
由 `TestAPI_SurfaceIsPinned` 与 [`testdata/api.golden`](testdata/api.golden)
守着：名字或签名变了，测试就红。

## 从哪儿开始读

| 想知道 | 看哪份 |
|---|---|
| **有哪些 API、各承诺什么** | [API.md](API.md) 🔒 已冻结 |
| **该长成什么样、为什么这么抽象** | [DESIGN.md](DESIGN.md) |
| 现在的代码怎么组织的 | [ARCHITECTURE.md](ARCHITECTURE.md) |
| 别人怎么做的，我们哪里强、哪里欠 | [PRIOR-ART.md](PRIOR-ART.md) |
| 写规则包时撞到过什么 | [missions](https://github.com/Zereker/werewolf/blob/main/missions/SCARS.md) · [onenight](https://github.com/Zereker/werewolf/blob/main/onenight/SCARS.md) |

写自己的规则包时，[`enginetest`](enginetest/) 提供随机对局与七条通用不变量
（`RunFuzz`）——它们一条都不认识任何游戏，只验内核层面的事：存了再读回来
一样吗、回放走到同一个局面吗、说不能行动的人是不是真的不能行动。

## 一台什么都不知道的状态机

`NewEngine` 造出来的引擎能推进阶段，但永远不会分出胜负、不认得任何角色、
不划分信息边界。下面是一套完整的、两页纸的规则：红蓝两队，每回合一次
公投，票多者出局，一方全灭即结束。

```go
const (
	phaseVote = hiddenrole.PhaseType("VOTE")
	roleRed   = hiddenrole.RoleType("RED")
	roleBlue  = hiddenrole.RoleType("BLUE")
	skillVote = hiddenrole.SkillType("VOTE")
	eventOut  = hiddenrole.EventType("OUT")
	campRed   = hiddenrole.Camp("RED")
	campBlue  = hiddenrole.Camp("BLUE")
)

// 这个阶段结束时发生了什么。只读 GameView，只返回 Effect。
type vote struct{}

func (vote) Resolve(uses []*hiddenrole.SkillUse, _ hiddenrole.GameView) []*hiddenrole.Effect {
	tally := map[string]int{}
	for _, u := range uses {
		if u.Skill == skillVote {
			tally[u.Target()]++
		}
	}
	out, best := "", 0
	for id, n := range tally {
		if n > best || (n == best && id < out) { // 顺序必须由局面唯一决定
			out, best = id, n
		}
	}
	if out == "" {
		return nil
	}
	return []*hiddenrole.Effect{
		hiddenrole.NewEffect(eventOut, "", out),  // 规则给「发生了什么」起的名字
		hiddenrole.NewSetAliveEffect(out, false), // 真正改状态的那一条
	}
}

// 一方全灭即结束。
type lastSideStanding struct{}

func (lastSideStanding) CheckVictory(view hiddenrole.GameView) (bool, hiddenrole.Camp) {
	red, blue := 0, 0
	for _, p := range view.AlivePlayers() {
		if p.Role == roleRed {
			red++
		} else {
			blue++
		}
	}
	switch {
	case blue == 0:
		return true, campRed
	case red == 0:
		return true, campBlue
	}
	return false, hiddenrole.CampUnspecified
}

func main() {
	cfg := &hiddenrole.Config{
		StartPhase: phaseVote,
		Phases: map[hiddenrole.PhaseType]*hiddenrole.PhaseConfig{
			phaseVote: {
				Type: phaseVote,
				Steps: []hiddenrole.PhaseStep{
					{Role: roleRed, Skill: skillVote, Required: true, Multiple: true},
					{Role: roleBlue, Skill: skillVote, Required: true, Multiple: true},
				},
				NextPhase:       phaseVote, // 阶段环，转回自己
				EndsRound:       true,      // 这个阶段结束就是一回合
				ClearsRoundVars: true,      // 而它开始时是干净的
			},
		},
	}

	e := hiddenrole.MustNewEngine(cfg,
		hiddenrole.WithResolver(phaseVote, vote{}),
		hiddenrole.WithVictoryChecker(lastSideStanding{}))

	_ = e.AddPlayer("r1", roleRed)
	_ = e.AddPlayer("r2", roleRed)
	_ = e.AddPlayer("b1", roleBlue)
	_ = e.Start()

	for _, id := range []string{"r1", "r2", "b1"} {
		_ = e.SubmitSkillUse(&hiddenrole.SkillUse{PlayerID: id, Skill: skillVote, Targets: []string{"b1"}})
	}
	effects, _ := e.EndPhase()
	for _, ef := range effects {
		fmt.Println(ef.Type, ef.TargetID) // OUT b1 / SET_ALIVE b1 / GAME_ENDED
	}
	st := e.Status()
	fmt.Println("结束:", st.Over, "赢家:", st.Winner) // true RED
}
```

不给 `WithVictoryChecker`，这局游戏永远不会结束；不给 `WithResolver`，
`Start()` 直接报错。**内核什么都不知道**是可以被这样验证的，不是一句口号。

## 唯一的写入点

```
SubmitSkillUse  ->  Resolver.Resolve  ->  []*Effect  ->  applyEffect
   收集技能          裁决（纯函数）       状态变更的描述     唯一的写入点
```

`Resolver` 拿到的是只读的 `GameView`，只能通过返回 `Effect` 表达状态变更。
这条约束由**签名**保证而非靠约定：状态的每一次改变都经由同一个写入点，
快照、回放、审计这些能力才成立。

`Resolve` 与 `CheckVictory` 都在引擎持锁期间被调用，实现中不要回调
`Engine` 的任何方法。返回的效果顺序必须由局面唯一决定（上面的
`id < out` 就是为此），否则回放与快照比对失去确定性。

## 状态机认得的两条原语

| 构造函数 | 改什么 | 读回来 |
|---|---|---|
| `NewSetAliveEffect(id, alive)` | 存活 | `GameView.Player(id).Alive` |
| `NewSetVarEffect(scope, k, v)` | 一项自定义状态 | `GameView.Var(scope, k)` |

作用域是一张 2×2 的表——时间尺度乘以有没有主人，四格由两个值叉乘
一个方法得出（见 `VarScope`）：

| | 无主 | 属于某个玩家 |
|---|---|---|
| **整局有效** | `ScopeGame` | `ScopeGame.Of(id)` |
| **本回合有效** | `ScopeRound` | `ScopeRound.Of(id)` |

这张表此前只存在于注释里，代码里是八个平铺的名字（四个构造器加四个读法）
——于是没有任何东西强制它完整，少了「整局·无主」那一格很久没人发现，
直到任务制那一套撞上。现在缺一格根本写不出来。

外加一条 `NewDetourEffect(id, phase)`，排一笔欠账
（猎人被刀之后的那一枪就是它）。

变量的值是字符串，**空串在写入点等同删除**，因此「有 / 没有」这类状态
只需要一个非空值（约定用 `VarPresent`）。

「狼刀」「放逐」「开枪」这些是规则给「发生了什么」起的名字，状态机不认得
——一个 `KILL` 效果单独发出去，**谁都不会死**。规则要让人出局，就在它旁边
产出一条 `SET_ALIVE`。两个效果，两件事：前者给受众与效果流看，后者给
状态机看。上面例子里 `OUT` 和 `SET_ALIVE` 成对出现就是这个意思。

## 谁能知道什么

```go
e.PlayerView(id)      // 某个玩家有权知道的一切，可以原样发给他
e.AudienceOf(event)   // 一件事该发给哪些玩家
```

划分由规则给：`AudienceProvider`（一件事该告诉谁）、`TeammateProvider`
（谁和谁是一边的，允许不对称）、`SpeechProvider`（发言谁能听到）。

内核在这一层只守一条底线，且**不可配置**：自己的状态原语永远不外发。
它们是状态机的记账，推给玩家等于把上帝视角直接发出去。

面向玩家的 `PlayerView` / `AudienceOf` 与上帝视角的 `PhaseInfo` /
`PlayerInfo` 是两套读法，别混用：前者可以直接发给玩家，后者不行。

## 八个扩展点

| 想加什么 | 用什么 |
|---|---|
| 某个阶段怎么结算 | `WithResolver(phase, resolver)` |
| 某个角色带着什么入座 | `WithRoleSetup(role, setup)`，写进该玩家的 `Vars` |
| 怎么算赢 | `WithVictoryChecker(checker)` |
| 角色专属信息 | `WithRoleInfo(role, provider)`，出现在 `PlayerView.RoleInfo` |
| 一件事该告诉谁 | `WithAudience(provider)` |
| 谁和谁是一边的 | `WithTeammates(provider)` |
| 发言谁能听到 | `WithSpeech(provider)` |
| 日志 | `WithLogger(l)` |

外加两个不走选项的：局中的状态变更走 `Effect` 原语，宿主级的
状态修改走 `Engine.Apply`（同样经唯一写入点，但绕开阶段结算——是把
锋利的刀）。

**八个扩展点都能用一个普通函数装上**：`ResolverFunc` / `VictoryFunc` /
`RoleSetupFunc` / `GameSetupFunc` / `RoleInfoFunc` / `AudienceFunc` /
`TeammateFunc` / `SpeechFunc`。前两个是后补的——此前只有它们两个没有
适配器，没有理由，只是历史，于是「装一个只有几行的解析器」得先声明
一个空结构体。

全部只能在构造时给出：引擎交到调用方手上之后，这些就不再变了。
`NewEngine` / `MustNewEngine` / `RestoreEngine` / `ReplayEngine`
四个入口都接受它们。

## 内核不替规则做的两个决定

一局游戏怎么推进，有两个决定**只有规则知道答案**：

| 决定 | 谁说了算 |
|---|---|
| 下一步去哪个阶段 | `PhaseConfig.NextPhase` 是默认出口，规则可用 `NewGotoPhaseEffect` 在结算时改写 |
| 这一步之后是不是新回合 | `PhaseConfig.EndsRound` 声明 |

这两件事此前都是内核自己定的：出口查一张静态图，回合边界猜「绕回起始阶段
就算」。狼人杀里两个猜测都恰好成立（夜→昼→夜），换一套规则就不成立。

判据是一句话：**内核能不能在不知道这是什么游戏的情况下，独立判断这件事
对不对？**「状态改了没有」能判断，归内核；「现在是不是新回合」判断不了，
归规则。

```go
// 表决通过就去任务，否则回提名——结果由本阶段的结算算出来，静态图表达不了
if approved {
	effects = append(effects, hiddenrole.NewGotoPhaseEffect(phaseMission))
} else {
	effects = append(effects, hiddenrole.NewGotoPhaseEffect(phasePropose))
}
```

出口的优先级：**待结算的绕道队列 > `GOTO_PHASE` > `NextPhase`**。触发排最前
是因为队列必须排空——胜负判定与回合边界都等着它，中途跳走会把还没结算的
死亡技能丢掉。目标阶段不在配置里时记一条错误日志并退回默认出口。

## 谁能在这个阶段行动

两层，优先级从高到低：

| | 谁 |
|---|---|
| 规则点到名的人 | `NewSetActorsEffect(phase, ids...)`，或死亡触发在进入阶段时写下的那一份。**存活与否由规则负责**，内核不再二次否决 |
| 默认 | `PhaseStep.Role` 匹配上的**活人** |

技能校验、`AllowedSkills`、`PhaseReadiness`、`PhaseInfo` 四处共用
`actorsForStep` 这一个取数点——四个问题一个来源，才不会出现「内核收下了
他的提交，却告诉别人他不该行动」。

绕道（`NewDetourEffect`）此前是这里的**第三层**，与点名回答同一个
问题、实现也几乎逐字相同。现在它不再回答「谁能行动」：进入它要去的那个阶段时，
内核按队首把触发者写成该阶段的名单，之后一切照点名走。写在进入阶段而不是
写在效果的写入点，是因为队列里可以有多条指向同一个阶段的触发（两名猎人同一夜
出局），在写入点各写一次会互相覆盖，只剩最后一个人能行动。

## 扩展点不能回头找引擎

八个扩展点全部在引擎**持锁期间**被同步调用。实现里回调 `Engine` 的任何
方法，后果是**挂住**，不是报错——Go 的读写锁不可重入，那一局从此没有响应。

它们不需要回调：想知道的一切都在参数里。签名是刻意收窄的,扩展点拿不到
`*Engine`,要绕过这条约束得自己把引擎存进结构体,那是一个有意的动作。

要在回调里问引擎,用 `OnEvent` / `OnMessage` 的处理器——事件与消息都在
**锁外**发布:

```go
e.OnEvent(func(ev *hiddenrole.Event) {
	audience, known := e.AudienceOf(ev) // 安全：这里没有持锁
	if !known {
		return // 引擎认不得的第三方事件类型，自己路由，别默认广播
	}
	for _, id := range audience {
		send(id, ev)
	}
})
```

把引擎接进一个服务端正是这么做的,见 `example/netserver`。

## 单测自己的解析器

不用整局跑起来。`Board` 让你手工摆一副局面：

```go
b := hiddenrole.Board{
	Players: []hiddenrole.PlayerInfo{
		hiddenrole.Seat("r1", roleRed, true),
		hiddenrole.Seat("b1", roleBlue, true),
	},
	Round: 1,
	Phase: phaseVote,
}

effects := vote{}.Resolve([]*hiddenrole.SkillUse{
	{PlayerID: "r1", Skill: skillVote, Targets: []string{"b1"}},
}, b.View())

after := b.Apply(effects)          // 把效果折回去
p, _ := after.Player("b1")
// p.Alive == false
```

`Seat(id, role, alive, vars...)` 摆一名玩家，`Mark(p, keys...)` 给他打上
本回合的标记，`Board.Var(scope, k)` 读四格里的任意一格。

## 存档、回放与错误

```go
snap := e.Snapshot()                              // 纯数据，可直接 json.Marshal
e2, err := hiddenrole.RestoreEngine(cfg, snap, opts...) // 选项要与建局时一致

log := e.EffectLog()                              // 自建局以来的完整效果流
e3, err := hiddenrole.ReplayEngine(cfg, log, opts...)  // 按流重建
```

效果流是**历史**，快照是**状态**：持久化用 `Snapshot`，进程内的回放、
复盘与排查用 `EffectLog`。快照带版本号（`SnapshotVersion`），读不懂的
格式会明确拒绝而不是猜。

错误都带错误码，`errors.Is` 与 `HasCode` 都可用来判别：

```go
if err := e.SubmitSkillUse(use); err != nil {
	switch {
	case errors.Is(err, hiddenrole.ErrPlayerDead):
		...
	case hiddenrole.HasCode(err, hiddenrole.CodeSkillNotAllowed):
		...
	}
}
```

自己的规则报错用 `WrapError(code, format, args...)`，与内核同一套。

## 内核不做什么

- **不计时。** `PhaseConfig.Timeout` 只是建议值，什么时候调 `EndPhase`
  完全由调用方决定；`PhaseReadiness()` 告诉你还差谁。
- **不联网、不做房间、不做匹配。**
- **不做存储。** `Snapshot` 导出局面、`RestoreEngine` 重建，存到哪是使用者的事。
- **不知道任何游戏的规则。** 那是规则包的事。

## 完整 API

```
go doc github.com/Zereker/hiddenrole
```

包文档见 [`doc.go`](doc.go)。一份真实的、跑得起来的规则包见
[Zereker/werewolf](https://github.com/Zereker/werewolf)——它用的每一个入口，你也能用。

## 许可证

MIT License. 详见 [LICENSE](LICENSE)。
