# Werewolf Game Engine

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

一个纯 Go 实现的狼人杀游戏引擎库。状态机驱动，声明式配置，零外部依赖。

## 特性

- **状态机驱动** - 清晰的阶段流转，声明式规则配置
- **规则可配置** - 女巫自救、守卫连守等规则可自定义
- **单包设计** - 只需 `import "github.com/Zereker/werewolf"`
- **零外部依赖** - 仅使用 Go 标准库
- **线程安全** - 所有状态操作都有锁保护

## 安装

```bash
go get github.com/Zereker/werewolf
```

## 快速开始

```go
package main

import (
    "fmt"

    "github.com/Zereker/werewolf"
    pb "github.com/Zereker/werewolf/proto"
)

func main() {
    // 1. 创建引擎（使用默认配置）
    engine := werewolf.NewEngine(nil)

    // 2. 添加玩家
    engine.AddPlayer("p1", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
    engine.AddPlayer("p2", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
    engine.AddPlayer("p3", pb.RoleType_ROLE_TYPE_SEER, pb.Camp_CAMP_GOOD)
    engine.AddPlayer("p4", pb.RoleType_ROLE_TYPE_WITCH, pb.Camp_CAMP_GOOD)
    engine.AddPlayer("p5", pb.RoleType_ROLE_TYPE_GUARD, pb.Camp_CAMP_GOOD)
    engine.AddPlayer("p6", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

    // 3. 开始游戏
    engine.Start()

    // 4. 提交技能使用
    engine.SubmitSkillUse(&werewolf.SkillUse{
        PlayerID: "p1",
        Skill:    pb.SkillType_SKILL_TYPE_KILL,
        TargetID: "p6",
    })

    // 5. 结束阶段，解析技能效果
    effects, _ := engine.EndPhase()

    for _, effect := range effects {
        fmt.Printf("Effect: %v -> %v\n", effect.Type, effect.TargetID)
    }
}
```

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

### GameConfig（游戏配置）

声明式规则配置：

```go
config := &werewolf.GameConfig{
    WitchCanSaveSelf:     false, // 女巫不能自救
    GuardCanProtectSelf:  true,  // 守卫可以自守
    GuardCanRepeat:       false, // 守卫不能连续守同一人
    SameGuardKillIsEmpty: true,  // 同守同杀是空刀
}
```

### PhaseConfig（阶段配置）

声明式阶段步骤：

```go
nightPhase := &werewolf.PhaseConfig{
    Type: pb.PhaseType_PHASE_TYPE_NIGHT,
    Steps: []werewolf.PhaseStep{
        {Role: pb.RoleType_ROLE_TYPE_GUARD, Skill: pb.SkillType_SKILL_TYPE_PROTECT, Order: 1},
        {Role: pb.RoleType_ROLE_TYPE_WEREWOLF, Skill: pb.SkillType_SKILL_TYPE_KILL, Order: 2, Multiple: true},
        {Role: pb.RoleType_ROLE_TYPE_WITCH, Skill: pb.SkillType_SKILL_TYPE_ANTIDOTE, Order: 3},
        {Role: pb.RoleType_ROLE_TYPE_WITCH, Skill: pb.SkillType_SKILL_TYPE_POISON, Order: 4},
        {Role: pb.RoleType_ROLE_TYPE_SEER, Skill: pb.SkillType_SKILL_TYPE_CHECK, Order: 5},
    },
}
```

### Resolver（冲突解析器）

处理技能冲突的核心逻辑：

- **NightResolver** - 夜晚技能解析（守卫保护 > 狼人击杀 > 女巫救人/毒人）
- **VoteResolver** - 投票结果解析（统计票数，处理平票）
- **DayResolver** - 白天发言（无状态变化）

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

## 游戏流程

```
Start → Night → Day → Vote → Night → ... → End
         ↑                      │
         └──────────────────────┘
```

1. **Night（夜晚）** - 狼人杀人、预言家查验、女巫救人/毒人、守卫守护
2. **Day（白天）** - 玩家发言讨论
3. **Vote（投票）** - 所有玩家投票驱逐

## 支持的角色

| 角色 | 阵营 | 技能 |
|------|------|------|
| Werewolf（狼人） | 狼人阵营 | 夜晚杀人 |
| Seer（预言家） | 好人阵营 | 夜晚查验身份 |
| Witch（女巫） | 好人阵营 | 解药救人、毒药杀人 |
| Guard（守卫） | 好人阵营 | 夜晚守护 |
| Hunter（猎人） | 好人阵营 | 死亡时可以开枪 |
| Villager（村民） | 好人阵营 | 无特殊技能 |

## 项目结构

```
werewolf/
├── config.go         # 游戏配置、阶段配置
├── effect.go         # 效果类型定义
├── engine.go         # 核心引擎（状态机）
├── errors.go         # 错误定义
├── phase_manager.go  # 阶段管理器
├── resolver.go       # 冲突解析器
├── state.go          # 游戏状态
├── proto/            # Protobuf 定义
└── docs/             # 文档
    └── ARCHITECTURE.md
```

## 架构设计

详见 [ARCHITECTURE.md](docs/ARCHITECTURE.md)

**设计理念：**
- **状态机驱动** - 不是事件驱动，而是显式的阶段流转
- **Phase 为中心** - 阶段决定规则，而非角色
- **声明式配置** - 规则用数据描述，而非代码

## 测试

```bash
go test ./...
```

## 许可证

MIT License. 详见 [LICENSE](LICENSE)。

---

**Made with Go by Zereker**
