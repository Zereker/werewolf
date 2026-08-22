# Werewolf Game Engine

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

一个纯 Go 实现的狼人杀游戏引擎库。状态机驱动，声明式配置，规则对齐维基百科条目。

## 特性

- **状态机驱动** - 清晰的阶段流转，声明式规则配置
- **规则对齐维基** - 以维基百科「狼人殺」条目为基准，规则逐条有测试覆盖
- **规则可配置** - 女巫自救、守卫连守、同守同救、屠边/屠城均可切换
- **可扩展** - 自定义角色与阶段无需 fork：注册解析器即可
- **管住信息** - 提供玩家视角与效果受众，不必自己实现信息过滤
- **单包设计** - 只需 `import "github.com/Zereker/werewolf"`
- **零依赖** - 只用标准库
- **可存档** - 局面可完整导出为 JSON，恢复后继续推进
- **线程安全** - 引擎的所有导出方法都可并发调用

## 安装

```bash
go get github.com/Zereker/werewolf
```

## 快速开始

```go
package main

import (
	"fmt"
	"log"

	"github.com/Zereker/werewolf"
)

func main() {
	// 1. 创建引擎（nil 表示使用默认配置）。
	//    配置会先经校验，残缺的阶段流转图在这里就会被拒绝
	engine, err := werewolf.NewEngine(nil)
	if err != nil {
		log.Fatal(err)
	}

	// 2. 添加玩家：2 狼、4 神、2 民（阵营与角色类别由角色推导）
	for id, role := range map[string]werewolf.RoleType{
		"w1":     werewolf.RoleWerewolf,
		"w2":     werewolf.RoleWerewolf,
		"seer":   werewolf.RoleSeer,
		"witch":  werewolf.RoleWitch,
		"guard":  werewolf.RoleGuard,
		"hunter": werewolf.RoleHunter,
		"v1":     werewolf.RoleVillager,
		"v2":     werewolf.RoleVillager,
	} {
		must(engine.AddPlayer(id, role))
	}

	// 3. 开始游戏，进入第一夜的守卫阶段
	if err := engine.Start(); err != nil {
		log.Fatal(err)
	}

	// 4. 按阶段推进：每个阶段先提交技能，再调用 EndPhase 结算
	//    Start() 之后是 NIGHT_GUARD，各阶段可提交的技能由 PhaseInfo 给出
	must(engine.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "guard", Skill: werewolf.SkillProtect, TargetID: "seer",
	}))
	next(engine) // NIGHT_GUARD -> NIGHT_WOLF

	must(engine.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "w1", Skill: werewolf.SkillKill, TargetID: "v1",
	}))
	must(engine.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "w2", Skill: werewolf.SkillKill, TargetID: "v1",
	}))
	next(engine) // NIGHT_WOLF -> NIGHT_WITCH

	// 女巫此刻可以看到刀口（解药还在手上）
	fmt.Printf("女巫看到的刀口: %s\n", witchSees(engine))
	next(engine) // NIGHT_WITCH -> NIGHT_SEER

	must(engine.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "seer", Skill: werewolf.SkillCheck, TargetID: "w1",
	}))
	next(engine) // NIGHT_SEER -> NIGHT_RESOLVE

	// 5. 夜晚结算：死亡在此产生
	effects, err := engine.EndPhase()
	if err != nil {
		log.Fatal(err)
	}
	for _, e := range effects {
		fmt.Printf("效果: %v -> %s (canceled=%v)\n", e.Type, e.TargetID, e.Canceled)
	}

	v1, _ := engine.PlayerInfo("v1")
	fmt.Printf("天亮了，当前阶段=%v，v1 存活=%v\n", engine.Phase(), v1.Alive)
}

func next(e *werewolf.Engine) {
	if _, err := e.EndPhase(); err != nil {
		log.Fatal(err)
	}
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
```

输出：

```
女巫看到的刀口: v1
效果: KILL -> v1 (canceled=false)
天亮了，当前阶段=DAY，v1 存活=false
```

完整示例见 [example/main.go](example/main.go)。

## 核心概念

### Engine（游戏引擎）

轻量级状态机，负责：
- 管理游戏状态
- 收集技能使用
- 驱动阶段流转
- 判定胜负条件

```go
engine := werewolf.NewEngine(config)
engine.Start()
engine.SubmitSkillUse(use)
effects, _ := engine.EndPhase()
```

`EndPhase` 是推进游戏的唯一入口，阶段流转与猎人等动态阶段都由它处理。

### 添加玩家

```go
// 只能在 Start 之前调用。内置角色与扩展角色走同一个入口——
// 阵营、类别、道具这些初始状态写在角色自己的 RoleSetup 里
err := engine.AddPlayer("p1", werewolf.RoleWerewolf)
```

以下情况会返回错误，而不是静默生效：

| 情况 | 错误 |
|------|------|
| ID 为空 | `ErrInvalidPlayerID` |
| ID 重复 | `PLAYER_EXISTS` |
| 角色是上帝或未指定 | `INVALID_ROLE` |
| 游戏已开始 | `ErrGameAlreadyStarted` |

`Start` 同样会校验板子：缺狼返回 `ErrNoWerewolf`，缺好人返回 `ErrNoGoodPlayer`，
重复调用返回 `ErrGameAlreadyStarted`。

### GameConfig（游戏配置）

声明式规则配置：

```go
config := &werewolf.GameConfig{
    WitchCanSaveSelf:       false, // 女巫不能自救
    WitchCanUseBothPotions: false, // 女巫不能在同一夜同时用解药和毒药
    GuardCanProtectSelf:    true,  // 守卫可以自守
    GuardCanRepeat:         false, // 守卫不能连续守同一人
    SameGuardKillIsEmpty:   true,  // 守卫守住刀口时空刀（守护生效）
    GuardSaveTogetherDies:  true,  // 同守同救，目标依然死亡

    VictoryMode: werewolf.VictoryModeSideWipe, // 屠边判定
}
```

### 胜负判定

| 模式 | 狼人胜利条件 |
|------|--------------|
| `VictoryModeSideWipe`（默认，屠边） | 所有平民出局 **或** 所有神职出局 |
| `VictoryModeTownWipe`（屠城） | 好人存活数 <= 狼人存活数 |

好人阵营的胜利条件与模式无关：狼人全部出局即获胜。

屠边需要区分神职与平民。类别是玩家身上的一项状态（键 `VarCategory`），
由角色的 `RoleSetup` 在入座时发放——内置角色是 `builtinRoleSetup` 表里的
一行（预言家/女巫/猎人/守卫为神职，村民为平民），扩展角色用
`WithRoleSetup` + `CampVars` 登记自己的一份。没有登记的角色不属于任何
阵营，也就不参与胜负计数。

屠边判定只对开局就存在的类别生效：没有神职的板子不会因「神职全灭」在开局
瞬间判负，平民同理。
```

### PhaseConfig（阶段配置）

阶段用数据描述：谁、在什么时候、能用什么技能，以及下一阶段是什么。

```go
nightWolf := &werewolf.PhaseConfig{
    Type: werewolf.PhaseNightWolf,
    Steps: []werewolf.PhaseStep{
        {Role: werewolf.RoleGod, Skill: werewolf.SkillAnnounce},
        {Role: werewolf.RoleWerewolf, Skill: werewolf.SkillKill},
    },
    Timeout:   werewolf.WolfPhaseTimeout,
    NextPhase: werewolf.PhaseNightWitch,
}
```

`Steps` 同时是技能校验的唯一依据：`SubmitSkillUse` 只放行当前阶段声明过的技能，
`PhaseInfo` 对外宣告的可用技能也由它派生，两者不会出现分歧。

`Timeout` 是给调用方参考的建议值——**引擎自身不计时**，阶段何时结束完全由
调用方决定（调用 `EndPhase`）。

### Resolver（冲突解析器）

每个阶段一个解析器，只产出 `Effect`、不直接改状态：

| Resolver | 阶段 | 职责 |
|----------|------|------|
| `GuardResolver` | NIGHT_GUARD | 守护，处理连守限制与自守限制 |
| `WolfResolver` | NIGHT_WOLF | 按票数取狼队共识，记录刀口 |
| `WitchResolver` | NIGHT_WITCH | 解药/毒药，处理自救与同夜双开药限制 |
| `SeerResolver` | NIGHT_SEER | 查验阵营 |
| `NightResolveResolver` | NIGHT_RESOLVE | 夜晚结算：刀口生死、毒杀、猎人触发 |
| `HunterResolver` | NIGHT_HUNTER / DAY_HUNTER | 猎人开枪或放弃 |
| `DayResolver` | DAY | 发言阶段，无状态变化 |
| `VoteResolver` | VOTE | 统计票数，处理平票 |

刀口的最终生死统一在 `NightResolveResolver` 判定：

| 被守卫守护 | 被女巫解药 | 结果 |
|:---:|:---:|------|
| ✓ | ✓ | 由 `GuardSaveTogetherDies` 决定（默认死亡，即同守同救） |
| ✓ | ✗ | 由 `SameGuardKillIsEmpty` 决定守护是否生效 |
| ✗ | ✓ | 救回 |
| ✗ | ✗ | 死亡 |

### Effect（效果）

技能执行的结果描述：

```go
type Effect struct {
    Type     EffectType  // Kill, Protect, Save, Poison, Check, Vote, Eliminate
    SourceID string      // 效果来源
    TargetID string      // 效果目标
    Canceled bool        // 是否被取消（如被守卫保护）
    Reason   string      // 取消原因
}
```

## 玩家视角

狼人杀最难的部分是「谁能知道什么」。引擎把这件事收在库内，
`PlayerView` 返回的内容可以直接发给该玩家：

```go
v := engine.PlayerView("p1")

v.Self            // 自己的身份、阵营、存活状态
v.Players         // 全场公开信息；身份只对自己与狼队友可见
v.AllowedSkills   // 本阶段自己能提交的技能，为空即「还没轮到我」
v.Teammates       // 狼人可见：队友
v.RoleInfo        // 角色专属信息：女巫的刀口 v.RoleInfo[RoleInfoKillTarget]、
                  // 药剂存量 v.RoleInfo[RoleInfoAntidote]，扩展角色的键由自己定
```

配套的 `AudienceOf` 回答「发生的事该告诉谁」：

```go
// 推的一路：OnEvent 给的就是 Event，直接问
engine.OnEvent(func(ev *werewolf.Event) {
    audience, known := engine.AudienceOf(ev)
    if !known {
        return // 第三方角色自定义的事件类型，引擎无从判断，调用方自己路由
    }
    for _, id := range audience {
        send(id, ev)   // 死亡全场可见；查验/守护/解药只给行动者
    }
})

// 拉的一路：EndPhase 给的是内部的 Effect，转一下
for _, effect := range effects {
    audience, _ := engine.AudienceOf(effect.ToEvent())
    ...
}
```

被规则否决的行动（`effect.Canceled`）只发给行动者本人：
「女巫想毒人但今晚已经用过解药」若按类型当成公开死讯广播出去，
女巫当场暴露。

`PhaseInfo` / `PlayerInfo` / `WolfTeammates` / `NightKillTarget`
是**上帝视角**接口，供调用方作为主持人使用，不可整体转发给玩家。

「谁和谁是一边的」由 `TeammateProvider` 回答（`WithTeammates` 可换掉），
狼人杀的默认实现按**阵营**而非角色：自定义的狼王、狼美人同样看得到队友、
夜里也能和狼队互通。`PlayerView`、`PhaseInfo`、`WolfTeammates` 三处共用
这一个判定。

## 阶段就绪

引擎不计时，但它知道谁还没行动：

```go
r := engine.PhaseReadiness()

for _, p := range r.Pending {
    fmt.Println("必须等:", p.PlayerID, p.Skill)   // 不动就不能推进
}
for _, p := range r.Optional {
    fmt.Println("可以催:", p.PlayerID, p.Skill)   // 不动也合法
}

engine.EndPhase()   // 未就绪也不会被拒绝，是否超时推进由调用方决定
```

**`Ready` 不表示「所有人都动过了」**：默认配置里只有狼人商刀与投票是
`Required`，守卫、女巫、预言家、猎人都可以不动。所以「还差谁**必须**动」
看 `Pending`，「本阶段谁**可以**动」看 `Optional`——只看前者来驱动游戏，
那几个角色一整局都不会被叫到。

由 `PhaseStep.Required` / `Multiple` / `Group` 声明。没有合格行动者的
必需步骤（守卫已出局）视为自动满足；互斥备选组（猎人的开枪与不开枪）
提交任一即算完成，只报一条。

## 扩展新角色

内置六个角色只是一套默认板子。加入狼王、白痴、骑士等角色不需要 fork：

```go
const (
    roleWolfKing  = werewolf.RoleType(1000)   // 自定义取值一律从 1000 起
    skillWolfClaw = werewolf.SkillType(1000)
    phaseWolfKing = werewolf.PhaseType(1000)
)

cfg := werewolf.DefaultGameConfig()
cfg.Phases[phaseWolfKing] = &werewolf.PhaseConfig{
    Type:      phaseWolfKing,
    Steps:     []werewolf.PhaseStep{{Role: roleWolfKing, Skill: skillWolfClaw}},
    NextPhase: werewolf.PhaseNightGuard,
}

engine, _ := werewolf.NewEngine(cfg,
    werewolf.WithResolver(phaseWolfKing, &wolfKingResolver{}),
    // 阵营与类别写在角色自己身上，入座时不用再给一遍
    werewolf.WithRoleSetup(roleWolfKing, werewolf.RoleSetupFunc(
        func(string, werewolf.RoleType) map[string]string {
            return werewolf.CampVars(werewolf.CampEvil, werewolf.RoleCategoryWolf)
        })))
engine.AddPlayer("wk", roleWolfKing)
```

`WithResolver` 是构造选项，`NewEngine` / `RestoreEngine` / `ReplayEngine`
三个入口都接受它，`WithLogger` / `WithMetrics` 同理。解析器、日志与指标
都只能在构造时给出：引擎交到调用方手上之后，这些就不再变了。

扩展能改动的八处，都由构造选项给出：

| 想加什么 | 用什么 |
|---|---|
| 新角色的行为 | `WithResolver(phase, resolver)`，可包装内置解析器复用逻辑 |
| 角色的初始状态 | `WithRoleSetup(role, setup)`，入座时发放，写进该玩家的 `Vars`；阵营与类别也在这里（`CampVars`） |
| 角色自身的状态 | `NewSetPlayerVarEffect`（跟着玩家一整局）/ `NewSetRoundVarEffect`（每回合清零），读走 `GameView.PlayerVar` / `RoundVar` |
| 新的胜利条件 | `WithVictoryChecker(checker)`，包一层 `DefaultVictoryChecker` 就能在内置规则之上再加一条 |
| 角色专属信息 | `WithRoleInfo(role, provider)`，结果出现在 `PlayerView.RoleInfo` 与 `RolePhaseInfo.RoleInfo` |
| 一件事该告诉谁 | `WithAudience(provider)`，`AudienceOf` 的判定 |
| 谁和谁是一边的 | `WithTeammates(provider)`，允许不对称（恶魔认得爪牙，反过来不成立） |
| 发言谁能听到 | `WithSpeech(provider)`，`MessageReceivers` 的判定 |

内置角色在这些事上**没有特权**，女巫是现成的样本：

- 开局两瓶药由 `builtinRoleSetup` 发，与 `WithRoleSetup` 同一张表——注册一个
  空的 setup，她就真的空手上桌；
- 药存在 `Vars` 里（`VarWitchAntidote` / `VarWitchPoison`），与第三方角色同一份存储；
- 刀口与药剂存量经 `WithRoleInfo` 投射给玩家，可以被换掉；
- 队友经 `TeammateProvider` 按**阵营**给，自定义的狼王一样拿得到。

加一个角色不需要改引擎里任何一行。

**存储与投射是分开的**：状态只有 `Vars` 一种存法，谁都能写；要给玩家看成
什么样由角色的 `RoleInfoProvider` 决定。默认不给——往 `Vars` 里放什么由角色
决定，自动交给玩家等于让每个角色自己去想「这一项能不能给他看」。

状态一定要走 Var 而不是存在 Resolver 的字段里：`Resolver` 接口要求
「只能通过返回 Effect 表达状态变更」，存在字段里的东西快照带不上、回放
也重建不出，恢复出来的对局是错的**还不会报错**。

**事件类型的编号是分段的**，这一点关系到扩展的事件能不能发出去：

| 段 | 归谁 | 会不会推给 `OnEvent` |
|---|---|---|
| `1..99` | 引擎的外部可见事件 | 会 |
| `100..999` | 引擎的内部状态变更 | 不会 |
| **`1000` 起** | **第三方扩展** | **会**；`AudienceOf` 回答「不知道」，路由由扩展自己决定 |

死亡时触发的能力由 Resolver 产出 `NewAbilityTriggerEffect(playerID, phase)`，
引擎会自动流转到该阶段，并把胜负判定推迟到技能结算之后。

可运行的例子：[example/extension](example/extension) 加了一个**白痴**（被投票放逐时
翻牌、不出局、此后失去投票权），演示包装内置解析器、否决一个效果、自定义事件类型
与存档恢复；[extension_test.go](extension_test.go) 用狼王再走一遍死亡触发那条分支。

## 命令行主持台

`example/cli` 是一个能真的从头玩完一局的主持台，也是这个库的第一个真实使用者——
超时、消息路由、存档落盘这些库刻意不管的事，都由它自己解决：

```console
$ go run ./example/cli
狼人杀 · 命令行主持台
9 人局：3 狼 + 预言家/女巫/守卫/猎人 + 2 民

  [公告] 天黑请闭眼。守卫请睁眼，你要守护谁？
  守卫: [6号]  可用技能 守护

[第1回合 守卫] > act 6号 守 5号
  已记下: 6号 守护 5号
[第1回合 守卫] > end
  守卫 结束 -> 狼人
  [私信 [6号]] 6号 -> 5号 被守护
[第1回合 狼人] > act 1号 刀 5号
[第1回合 狼人] > end
[第1回合 女巫] > view 2号
  你是 2号，女巫，存活
  解药 还在，毒药 还在
  今晚被刀的是: 5号
  你现在可以: 解药/毒药
[第1回合 女巫] > act 2号 救 5号
...
  [全场] 5号 被刀，同守同救，依然死亡
```

`view <玩家>` 出来的内容可以原样发给他，`[私信]` / `[全场]` 的分发依据是
`AudienceOf`。`run` 让它自己随机跑完一局，`save` / `load` 演示服务重启。
也可以照脚本跑：`go run ./example/cli < example/cli/testdata/demo.txt`。

## TCP 服务端

`example/netserver` 是一个 TCP 长连接的服务端，也是这个库的第二个真实使用者。
命令行主持台验证的是「一个人主持一局」，有一整类东西它碰不到——事件推送、
每条连接一份视图、多局并发、断线重连、超时真的触发——都由它来压。

协议是 TCP + 一行一条 JSON，`nc` 就能玩：

```console
$ go run ./example/netserver &
$ nc localhost 9000
{"type":"join","player":"p1"}
<- {"type":"phase","phase":"NIGHT_GUARD","round":1,"deadline_ms":...}
<- {"type":"view","view":{...}}          // 只有 p1 有权知道的那一份
{"type":"act","skill":"protect","target":"p5"}
{"type":"say","text":"我是好人"}
<- {"type":"event","event":{...}}        // 只推给 AudienceOf 划出来的人
```

房间用单 goroutine 串行化对引擎的访问（actor），而不是加锁：引擎的回调是在
`EndPhase` 内部、释放引擎锁之后触发的，房间若自己也加锁就会撞上
「持房间锁 → EndPhase → 回调 → 想再拿房间锁」的自锁。

## 效果流与回放

```go
log := engine.EffectLog()                    // 自建局以来的完整事件流
replayed, _ := werewolf.ReplayEngine(cfg, log) // 按流重建局面
```

效果流是**历史**，快照是**状态**：持久化用 `Snapshot`，
进程内的回放、复盘与排查用 `EffectLog`。

## 存档与恢复

`Engine.Snapshot()` 导出完整局面，`RestoreEngine` 从快照重建引擎。
快照是纯数据结构，可直接用 `encoding/json` 序列化。

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/Zereker/werewolf"
)

func main() {
	config := werewolf.DefaultGameConfig()
	engine := werewolf.MustNewEngine(config) // 配置是常量时可用 Must 版本
	for id, role := range map[string]werewolf.RoleType{
		"w1": werewolf.RoleWerewolf,
		"wi": werewolf.RoleWitch,
		"v1": werewolf.RoleVillager,
		"v2": werewolf.RoleVillager,
	} {
		if err := engine.AddPlayer(id, role); err != nil {
			log.Fatal(err)
		}
	}
	if err := engine.Start(); err != nil {
		log.Fatal(err)
	}
	engine.EndPhase() // -> NIGHT_WOLF
	engine.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "w1", Skill: werewolf.SkillKill, TargetID: "v1",
	})

	// 保存：技能已提交、尚未结算，快照会把它一并带上
	data, err := json.Marshal(engine.Snapshot())
	if err != nil {
		log.Fatal(err)
	}

	// 恢复：配置由调用方提供，必须与保存时一致
	var snap werewolf.Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		log.Fatal(err)
	}
	restored, err := werewolf.RestoreEngine(config, &snap)
	if err != nil {
		log.Fatal(err)
	}

	restored.EndPhase() // 结算狼刀
	fmt.Printf("恢复后阶段=%v，女巫看到的刀口=%s\n", restored.Phase(), witchSees(restored))
}

// witchSees 上帝视角下女巫看到的刀口。
//
// 角色专属信息一律经 RoleInfo 出来，键名由角色自己定——引擎不认识女巫，
// 也就没有一个叫 KillTarget 的具名字段。
func witchSees(e *werewolf.Engine) string {
	ri, ok := e.PhaseInfo().RoleInfos[werewolf.RoleWitch]
	if !ok {
		return ""
	}
	for _, info := range ri.RoleInfo {
		if t := info[werewolf.RoleInfoKillTarget]; t != "" {
			return t
		}
	}
	return ""
}
```

输出：

```
恢复后阶段=NIGHT_WITCH，女巫看到的刀口=v1
```

要点：

- **快照包含当前阶段已提交但未结算的技能**，可以在阶段中途保存，
  恢复后继续收技能再 `EndPhase`
- **快照不包含规则配置**，`GameConfig` 由调用方在恢复时提供。
  用不同的配置恢复，等于中途换了规则
- 快照是深拷贝，导出后引擎继续推进不会改动它
- 同一局面导出的字节是确定的（集合与玩家列表都做了排序），便于比对与幂等写入
- `SnapshotVersion` 不匹配会直接报错，而不是按新结构误读旧数据

## 游戏流程

引擎把夜晚拆成了若干子阶段，每个子阶段只让一个角色行动：

```
Start
  │
  ▼
NIGHT_GUARD → NIGHT_WOLF → NIGHT_WITCH → NIGHT_SEER → NIGHT_RESOLVE
                                                            │
                                    ┌───────────────────────┤
                                    ▼                       ▼
                             NIGHT_HUNTER ───────────────► DAY
                            （猎人死亡时触发）               │
                                                            ▼
                                                          VOTE
                                                            │
                                    ┌───────────────────────┤
                                    ▼                       ▼
                              DAY_HUNTER ──────────► 下一夜 NIGHT_GUARD
                            （猎人被放逐时触发）
```

顺序不是随意的：守卫必须排在狼人之前才能拦下刀口，女巫必须排在狼人之后
才能看到刀口。猎人阶段由死亡结算动态触发，**胜负判定会推迟到猎人开完枪之后**
——那一枪可能带走最后一只狼，让好人反胜。

## 支持的角色

| 角色 | 阵营 | 类别 | 技能 |
|------|------|------|------|
| Werewolf（狼人） | 狼人 | 狼人 | 夜晚击杀 |
| Seer（预言家） | 好人 | 神职 | 夜晚查验阵营 |
| Witch（女巫） | 好人 | 神职 | 解药救人、毒药杀人（同夜只能用一瓶） |
| Guard（守卫） | 好人 | 神职 | 夜晚守护，不可连续两晚守同一人 |
| Hunter（猎人） | 好人 | 神职 | 死亡时开枪带走一人（被毒杀除外） |
| Villager（村民） | 好人 | 平民 | 无特殊技能 |

「阵营」与「类别」都是玩家身上的状态，由角色的 `RoleSetup` 在入座时发放
（见 `builtinRoleSetup`）。扩展角色（狼王、白痴、骑士等）暂未内置，
用 `WithRoleSetup` + `CampVars` 登记之后同样参与判定。

## 项目结构

```
werewolf/
├── config.go       # 游戏配置、阶段配置、规则开关
├── effect.go       # 效果类型定义
├── engine.go       # 核心引擎（状态机）
├── errors.go       # 错误定义
├── logger.go       # 日志与指标接口
├── phase.go        # 阶段管理器、技能校验
├── resolver.go     # 各阶段解析器
├── effectlog.go    # 效果流日志与回放
├── events.go       # 事件通知
├── messaging.go    # 玩家发言的路由
├── phase_info.go   # 阶段信息（上帝视角）
├── player_view.go  # 玩家视角与效果受众
├── readiness.go    # 阶段就绪判定
├── snapshot.go     # 存档导出与恢复
├── state.go        # 游戏状态、角色类别、胜负判定
├── doc.go          # 包文档
├── types.go        # 阶段、阵营、角色、技能
├── event.go        # 对外事件
├── view.go         # Resolver 的只读视图
├── rules_test.go      # 以维基百科规则为基准的一致性测试
├── extension_test.go  # 第三方扩展契约（以狼王为例）
├── example/        # 可运行示例
│   ├── cli/        # 命令行主持台（真实使用者）
│   ├── netserver/  # TCP 服务端（推送、并发、断线重连）
│   └── extension/  # 自定义角色（白痴）
└── docs/
    └── ARCHITECTURE.md
```

## 架构设计

详见 [ARCHITECTURE.md](docs/ARCHITECTURE.md)

**设计理念：**
- **状态机驱动** - 不是事件驱动，而是显式的阶段流转
- **Phase 为中心** - 阶段决定规则，而非角色
- **声明式配置** - 规则用数据描述，而非代码

## 规则依据

本引擎的规则以中文维基百科[「狼人殺」条目](https://zh.wikipedia.org/wiki/狼人殺)
为基准，逐条写成了可执行测试，见 [rules_test.go](rules_test.go)：

| 编号 | 规则 |
|------|------|
| R1 | 预言家每晚查验一名存活玩家的所属阵营 |
| R2 | 解药未使用时才可得知狼人的杀害对象 |
| R3 | 解药和毒药不可以在同一夜使用 |
| R4 | 解药不能用于解救自己 |
| R5 | 守卫可以守护自己或不进行守护 |
| R6 | 守卫不可连续两晚守护同一名玩家 |
| R7 | 同守同救时该名玩家依然会死亡 |
| R8 | 除被毒杀外，猎人以任何方式出局都可以开枪 |
| R9 | 狼人全部出局，好人胜利 |
| R10 | 淘汰所有平民或所有神职，狼人胜利（屠边） |
| R11 | 所有存活玩家发言完毕后投票放逐一名玩家 |

维基未规定、由本引擎自行约定的口径（同样有测试固化）：

- **D1** 白天投票平票 → 无人出局（不进入 PK 发言重投）
- **D2** 狼人刀口平票 → 空刀
- **D3** 夜晚行动顺序固定为 守卫 → 狼人 → 女巫 → 预言家 → 结算

## 测试

```bash
go test ./...           # 全部测试
go test -race ./...     # 并发检查
golangci-lint run       # 静态检查（覆盖测试代码）
go run ./example        # 示例必须能跑通，不是只能编译
```

## 更新日志与发版

见 [CHANGELOG.md](CHANGELOG.md)。破坏性变更在每个版本开头单独列出。

发版走 GitHub Actions：**Actions → Release → Run workflow**，填一个形如 `v1.3.0`
的版本号。workflow 会先跑一遍完整验证，再创建 tag 与 Release，发布说明取自
CHANGELOG 里对应的小节。四道闸：版本号格式、tag 未占用、CHANGELOG 里有对应
小节、v2 及以上必须先改模块路径（本项目刻意不走 `/vN`，所以这一条实际是拦下）。

## 许可证

MIT License. 详见 [LICENSE](LICENSE)。

---

**Made with Go by Zereker**
