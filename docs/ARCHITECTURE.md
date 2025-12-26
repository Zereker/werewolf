# Werewolf 游戏引擎架构文档

## 概述

Werewolf 游戏引擎采用**状态机驱动**的架构设计，以**Phase（阶段）为中心**，使用**声明式配置**描述游戏规则。

## 设计理念

### 核心原则

1. **状态机驱动** - 游戏流程是显式的阶段流转，而非事件驱动
2. **Phase 为中心** - 阶段决定规则（谁能做什么），而非角色
3. **声明式配置** - 规则用数据描述，不硬编码在代码中
4. **单包设计** - 用户只需导入一个包

### 架构演进

```
之前（命令式、多包）           之后（声明式、单包）
├── event/                    ├── config.go
├── executor/                 ├── effect.go
├── phase/          ──────►   ├── engine.go
├── player/                   ├── phase_manager.go
├── role/                     ├── resolver.go
├── skill/                    ├── state.go
└── engine.go                 └── proto/
```

## 架构图

```
┌─────────────────────────────────────────────────────────────┐
│                         Engine                               │
│                    （轻量状态机）                              │
│                                                              │
│   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│   │ GameState   │  │PhaseManager │  │  GameConfig │        │
│   │ （游戏状态） │  │ （阶段管理） │  │ （规则配置） │        │
│   └──────┬──────┘  └──────┬──────┘  └──────┬──────┘        │
│          │                │                │                │
│          └────────────────┼────────────────┘                │
│                           │                                  │
│                    ┌──────▼──────┐                          │
│                    │  Resolver   │                          │
│                    │ （冲突解析） │                          │
│                    └──────┬──────┘                          │
│                           │                                  │
│                    ┌──────▼──────┐                          │
│                    │   Effect    │                          │
│                    │ （效果描述） │                          │
│                    └─────────────┘                          │
└─────────────────────────────────────────────────────────────┘
```

## 核心模块

### 1. Engine（引擎）

**文件**: `engine.go`

**职责**: 轻量级状态机，协调所有模块

```go
type Engine struct {
    config       *GameConfig     // 规则配置
    state        *GameState      // 游戏状态
    phaseManager *PhaseManager   // 阶段管理
    pendingUses  []*SkillUse     // 待处理技能
    eventHandlers []EventHandler // 事件处理器
}
```

**核心方法**:
- `NewEngine(config)` - 创建引擎
- `AddPlayer(id, role, camp)` - 添加玩家
- `Start()` - 开始游戏
- `SubmitSkillUse(use)` - 提交技能使用
- `EndPhase()` - 结束阶段，解析技能，流转状态

**设计要点**:
- Engine 只做协调，不做具体逻辑
- 技能解析委托给 Resolver
- 状态管理委托给 GameState

---

### 2. GameState（游戏状态）

**文件**: `state.go`

**职责**: 管理游戏状态，线程安全

```go
type GameState struct {
    Phase   pb.PhaseType            // 当前阶段
    Round   int                     // 当前回合
    Players map[string]*PlayerState // 玩家状态
}

type PlayerState struct {
    ID        string
    Role      pb.RoleType
    Camp      pb.Camp
    Alive     bool
    Protected bool
}
```

**核心方法**:
- `AddPlayer()` - 添加玩家
- `GetPlayerInfo()` - 获取玩家信息只读副本
- `ApplyEffect()` - 应用效果（改变状态）
- `CheckVictory()` - 检查胜利条件
- `NextPhase()` - 切换阶段

---

### 3. GameConfig（游戏配置）

**文件**: `config.go`

**职责**: 声明式规则配置

```go
type GameConfig struct {
    // 规则变体
    WitchCanSaveSelf     bool  // 女巫能否自救
    GuardCanProtectSelf  bool  // 守卫能否自守
    GuardCanRepeat       bool  // 守卫能否连续守同一人
    SameGuardKillIsEmpty bool  // 同守同杀是否空刀

    // 阶段配置
    Phases map[pb.PhaseType]*PhaseConfig
}
```

**PhaseConfig（阶段配置）**:

```go
type PhaseConfig struct {
    Type    pb.PhaseType  // 阶段类型
    Steps   []PhaseStep   // 步骤列表
    Timeout time.Duration // 超时时间
}

type PhaseStep struct {
    Role     pb.RoleType   // 哪个角色
    Skill    pb.SkillType  // 使用什么技能
    Order    int           // 执行顺序
    Required bool          // 是否必须行动
    Multiple bool          // 是否允许多个玩家
}
```

**标准夜晚配置示例**:

```go
Steps: []PhaseStep{
    {Role: Guard,    Skill: Protect,  Order: 1},
    {Role: Werewolf, Skill: Kill,     Order: 2, Multiple: true},
    {Role: Witch,    Skill: Antidote, Order: 3},
    {Role: Witch,    Skill: Poison,   Order: 4},
    {Role: Seer,     Skill: Check,    Order: 5},
}
```

---

### 4. PhaseManager（阶段管理器）

**文件**: `phase_manager.go`

**职责**: 管理阶段配置和解析器

```go
type PhaseManager struct {
    config    *GameConfig
    resolvers map[pb.PhaseType]Resolver
}
```

**核心方法**:
- `GetPhaseConfig()` - 获取阶段配置
- `GetResolver()` - 获取阶段解析器
- `GetAllowedSkills()` - 获取角色允许的技能
- `NextPhase()` - 计算下一阶段
- `ValidateSkillUse()` - 验证技能使用

---

### 5. Resolver（冲突解析器）

**文件**: `resolver.go`

**职责**: 解析技能冲突，产生效果

```go
type Resolver interface {
    Resolve(uses []*SkillUse, state *GameState, config *GameConfig) []*Effect
}
```

**三种解析器**:

| 解析器 | 阶段 | 职责 |
|--------|------|------|
| NightResolver | 夜晚 | 解析守卫/狼人/女巫/预言家的技能冲突 |
| DayResolver | 白天 | 发言阶段，无状态变化 |
| VoteResolver | 投票 | 统计投票，处理平票 |

**NightResolver 优先级**:

```
1. 守卫保护 → 标记目标为 Protected
2. 狼人击杀 → 检查是否被保护，产生 Kill 效果
3. 女巫解药 → 可取消 Kill 效果
4. 女巫毒药 → 产生 Poison 效果
5. 预言家查验 → 产生 Check 效果（仅返回信息）
```

---

### 6. Effect（效果）

**文件**: `effect.go`

**职责**: 描述状态变更

```go
type Effect struct {
    Type     EffectType             // 效果类型
    SourceID string                 // 效果来源
    TargetID string                 // 效果目标
    Data     map[string]interface{} // 附加数据
    Canceled bool                   // 是否被取消
    Reason   string                 // 取消原因
}
```

**效果类型**:

| 类型 | 说明 | 状态变化 |
|------|------|----------|
| EffectKill | 击杀 | Alive = false |
| EffectProtect | 保护 | Protected = true |
| EffectSave | 救活 | Alive = true |
| EffectPoison | 毒杀 | Alive = false |
| EffectCheck | 查验 | 无（返回阵营信息） |
| EffectEliminate | 投票出局 | Alive = false |

---

## 数据流

### 完整游戏流程

```
1. 创建引擎
   NewEngine(config)

2. 添加玩家
   AddPlayer("p1", Werewolf, Evil)
   AddPlayer("p2", Seer, Good)
   ...

3. 开始游戏
   Start() → Phase = Night, Round = 1

4. 夜晚阶段
   SubmitSkillUse(Kill, "p1" → "p3")
   SubmitSkillUse(Check, "p2" → "p1")
   ...

5. 结束阶段
   EndPhase()
   ├── Resolver.Resolve(uses) → [Effect...]
   ├── ApplyEffect(each effect)
   ├── CheckVictory()
   └── NextPhase() → Day

6. 白天阶段
   (发言，无技能提交)
   EndPhase() → Vote

7. 投票阶段
   SubmitSkillUse(Vote, "p1" → "p3")
   SubmitSkillUse(Vote, "p2" → "p3")
   ...
   EndPhase()
   ├── VoteResolver.Resolve()
   ├── ApplyEffect(Eliminate)
   ├── CheckVictory()
   └── NextPhase() → Night (Round 2)

8. 循环直到胜利条件满足
```

### 技能解析流程

```
SubmitSkillUse()
      │
      ▼
ValidateSkillUse()  ← 检查玩家存活、技能允许、目标有效
      │
      ▼
pendingUses.append()
      │
      ▼
EndPhase()
      │
      ▼
Resolver.Resolve()  ← 按规则解析冲突
      │
      ├── 分组：protect, kill, antidote, poison, check
      │
      ├── 处理保护：标记 protectedTarget
      │
      ├── 处理击杀：检查是否被保护
      │
      ├── 处理解药：取消击杀效果
      │
      ├── 处理毒药：产生毒杀效果
      │
      └── 处理查验：返回阵营信息
      │
      ▼
[]*Effect
      │
      ▼
ApplyEffect()  ← 修改 GameState
      │
      ▼
CheckVictory()  ← 判定胜负
```

---

## 设计模式

| 模式 | 位置 | 作用 |
|------|------|------|
| **State Machine** | Engine | 阶段流转 |
| **Strategy** | Resolver | 不同阶段不同解析策略 |
| **Configuration** | GameConfig | 声明式规则 |
| **Observer** | EventHandler | 事件通知（可选） |

---

## 扩展点

### 1. 添加新角色

只需修改 PhaseConfig，添加新的 Step：

```go
Steps: []PhaseStep{
    // ... 现有步骤
    {Role: NewRole, Skill: NewSkill, Order: 6},
}
```

### 2. 添加新技能

1. 在 proto 中定义新的 SkillType
2. 在 Resolver 中添加处理逻辑
3. 在 PhaseConfig 中配置

### 3. 自定义规则变体

修改 GameConfig：

```go
config := &GameConfig{
    WitchCanSaveSelf: true,  // 允许女巫自救
    // ...
}
```

### 4. 自定义阶段流程

实现新的 Resolver，或修改 PhaseManager.NextPhase() 逻辑。

---

## 与旧架构对比

| 方面 | 旧架构 | 新架构 |
|------|--------|--------|
| 驱动方式 | 事件驱动 | 状态机驱动 |
| 中心概念 | Role（角色） | Phase（阶段） |
| 规则定义 | 代码（命令式） | 配置（声明式） |
| 包结构 | 8个包 | 2个包 |
| 技能执行 | Skill + Executor 分离 | Effect 统一表达 |
| 复杂度 | ~6000行 | ~900行 |

---

## 总结

新架构的优势：

- **简单** - 单包设计，核心概念少
- **清晰** - 状态机流转显式可见
- **灵活** - 声明式配置，易于扩展
- **可维护** - 代码量减少 85%

核心思想：**狼人杀的本质是阶段驱动的状态机，规则应该用数据描述，而非代码实现。**
