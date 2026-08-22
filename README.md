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
- **依赖极简** - 仅依赖 `google.golang.org/protobuf`（用于事件与枚举定义）
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
	pb "github.com/Zereker/werewolf/proto"
)

func main() {
	// 1. 创建引擎（nil 表示使用默认配置）。
	//    配置会先经校验，残缺的阶段流转图在这里就会被拒绝
	engine, err := werewolf.NewEngine(nil)
	if err != nil {
		log.Fatal(err)
	}

	// 2. 添加玩家：2 狼、4 神、2 民（阵营与角色类别由角色推导）
	for id, role := range map[string]pb.RoleType{
		"w1": pb.RoleType_ROLE_TYPE_WEREWOLF,
		"w2": pb.RoleType_ROLE_TYPE_WEREWOLF,
		"seer":   pb.RoleType_ROLE_TYPE_SEER,
		"witch":  pb.RoleType_ROLE_TYPE_WITCH,
		"guard":  pb.RoleType_ROLE_TYPE_GUARD,
		"hunter": pb.RoleType_ROLE_TYPE_HUNTER,
		"v1":     pb.RoleType_ROLE_TYPE_VILLAGER,
		"v2":     pb.RoleType_ROLE_TYPE_VILLAGER,
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
		PlayerID: "guard", Skill: pb.SkillType_SKILL_TYPE_PROTECT, TargetID: "seer",
	}))
	next(engine) // NIGHT_GUARD -> NIGHT_WOLF

	must(engine.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "w1", Skill: pb.SkillType_SKILL_TYPE_KILL, TargetID: "v1",
	}))
	must(engine.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "w2", Skill: pb.SkillType_SKILL_TYPE_KILL, TargetID: "v1",
	}))
	next(engine) // NIGHT_WOLF -> NIGHT_WITCH

	// 女巫此刻可以看到刀口（解药还在手上）
	info := engine.PhaseInfo()
	fmt.Printf("女巫看到的刀口: %s\n", info.RoleInfos[pb.RoleType_ROLE_TYPE_WITCH].KillTarget)
	next(engine) // NIGHT_WITCH -> NIGHT_SEER

	must(engine.SubmitSkillUse(&werewolf.SkillUse{
		PlayerID: "seer", Skill: pb.SkillType_SKILL_TYPE_CHECK, TargetID: "w1",
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
效果: EVENT_TYPE_KILL -> v1 (canceled=false)
天亮了，当前阶段=PHASE_TYPE_DAY，v1 存活=false
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
// 阵营与角色类别由角色推导，只能在 Start 之前调用
err := engine.AddPlayer("p1", pb.RoleType_ROLE_TYPE_WEREWOLF)

// 扩展角色（隐狼、白痴等）阵营/类别无法从角色推导，显式指定
err = engine.AddCustomPlayer("p2", pb.RoleType_ROLE_TYPE_VILLAGER,
    pb.Camp_CAMP_EVIL, werewolf.RoleCategoryWolf)
```

以下情况会返回错误，而不是静默生效：

| 情况 | 错误 |
|------|------|
| ID 为空 | `ErrInvalidPlayerID` |
| ID 重复 | `ERROR_CODE_PLAYER_EXISTS` |
| 角色是上帝或未指定 | `ERROR_CODE_INVALID_ROLE` |
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

屠边需要区分神职与平民，`PlayerState.Category` 由 `CategoryOf(role)` 自动推导
（预言家/女巫/猎人/守卫为神职，村民为平民）。自定义角色可通过
`State.SetPlayerCategory` 显式指定，未指定的角色不参与屠边判定。

屠边判定只对开局就存在的类别生效：没有神职的板子不会因「神职全灭」在开局
瞬间判负，平民同理。
```

### PhaseConfig（阶段配置）

阶段用数据描述：谁、在什么时候、能用什么技能，以及下一阶段是什么。

```go
nightWolf := &werewolf.PhaseConfig{
    Type: pb.PhaseType_PHASE_TYPE_NIGHT_WOLF,
    Steps: []werewolf.PhaseStep{
        {Role: pb.RoleType_ROLE_TYPE_GOD, Skill: pb.SkillType_SKILL_TYPE_ANNOUNCE},
        {Role: pb.RoleType_ROLE_TYPE_WEREWOLF, Skill: pb.SkillType_SKILL_TYPE_KILL},
    },
    Timeout:   werewolf.WolfPhaseTimeout,
    NextPhase: pb.PhaseType_PHASE_TYPE_NIGHT_WITCH,
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

v.Self            // 自己的身份、阵营、（女巫的）药剂
v.Players         // 全场公开信息；身份只对自己与狼队友可见
v.AllowedSkills   // 本阶段自己能提交的技能，为空即「还没轮到我」
v.Teammates       // 狼人可见：队友
v.KillTarget      // 女巫可见：今晚刀口（解药用完即为空）
```

配套的 `AudienceOf` 回答「发生的事该告诉谁」：

```go
for _, effect := range effects {
    audience, known := engine.AudienceOf(effect)
    if !known {
        // 第三方角色自定义的事件类型，引擎无从判断可见性，调用方自己路由
        continue
    }
    for _, id := range audience {
        send(id, effect)   // 死亡全场可见；查验/守护/解药只给行动者
    }
}
```

被规则否决的行动（`effect.Canceled`）只发给行动者本人：
「女巫想毒人但今晚已经用过解药」若按类型当成公开死讯广播出去，
女巫当场暴露。

`PhaseInfo` / `PlayerInfo` / `WolfTeammates` / `NightKillTarget`
是**上帝视角**接口，供调用方作为主持人使用，不可整体转发给玩家。

## 阶段就绪

引擎不计时，但它知道谁还没行动：

```go
r := engine.PhaseReadiness()
if !r.Ready {
    fmt.Println("还差:", r.Pending)   // 谁、什么角色、什么技能
}
engine.EndPhase()   // 未就绪也不会被拒绝，是否超时推进由调用方决定
```

由 `PhaseStep.Required` / `Multiple` 声明：狼人商刀与投票要求全员参与，
其余步骤可选。没有合格行动者的必需步骤（守卫已出局）视为自动满足。

## 扩展新角色

内置六个角色只是一套默认板子。加入狼王、白痴、骑士等角色不需要 fork：

```go
const (
    roleWolfKing  = pb.RoleType(1000)   // 自定义取值从 1000 起
    skillWolfClaw = pb.SkillType(1000)
    phaseWolfKing = pb.PhaseType(1000)
)

cfg := werewolf.DefaultGameConfig()
cfg.Phases[phaseWolfKing] = &werewolf.PhaseConfig{
    Type:      phaseWolfKing,
    Steps:     []werewolf.PhaseStep{{Role: roleWolfKing, Skill: skillWolfClaw}},
    NextPhase: pb.PhaseType_PHASE_TYPE_NIGHT_GUARD,
}

engine, _ := werewolf.NewEngine(cfg)
engine.RegisterResolver(phaseWolfKing, &wolfKingResolver{})
engine.AddCustomPlayer("wk", roleWolfKing, pb.Camp_CAMP_EVIL, werewolf.RoleCategoryWolf)
```

死亡时触发的能力由 Resolver 产出 `NewAbilityTriggerEffect(playerID, phase)`，
引擎会自动流转到该阶段，并把胜负判定推迟到技能结算之后。
完整可运行的例子见 [extension_test.go](extension_test.go)。

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
	pb "github.com/Zereker/werewolf/proto"
)

func main() {
	config := werewolf.DefaultGameConfig()
	engine := werewolf.MustNewEngine(config) // 配置是常量时可用 Must 版本
	for id, role := range map[string]pb.RoleType{
		"w1": pb.RoleType_ROLE_TYPE_WEREWOLF,
		"wi": pb.RoleType_ROLE_TYPE_WITCH,
		"v1": pb.RoleType_ROLE_TYPE_VILLAGER,
		"v2": pb.RoleType_ROLE_TYPE_VILLAGER,
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
		PlayerID: "w1", Skill: pb.SkillType_SKILL_TYPE_KILL, TargetID: "v1",
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
	fmt.Printf("恢复后阶段=%v，女巫看到的刀口=%s\n",
		restored.Phase(),
		restored.PhaseInfo().RoleInfos[pb.RoleType_ROLE_TYPE_WITCH].KillTarget)
}
```

输出：

```
恢复后阶段=PHASE_TYPE_NIGHT_WITCH，女巫看到的刀口=v1
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

「类别」用于屠边判定，由 `CategoryOf(role)` 自动推导。扩展角色（狼王、白痴、
骑士等）暂未内置，可通过 `State.SetPlayerCategory` 指定类别后参与判定。

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
├── view.go         # Resolver 的只读视图
├── rules_test.go      # 以维基百科规则为基准的一致性测试
├── extension_test.go  # 第三方扩展契约（以狼王为例）
├── proto/          # Protobuf 定义（枚举与事件）
├── example/        # 可运行示例
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

## 许可证

MIT License. 详见 [LICENSE](LICENSE)。

---

**Made with Go by Zereker**
