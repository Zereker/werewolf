# 内核 API

> **这份文档就是冻结的对象。** 它列出 `github.com/Zereker/werewolf/engine`
> 的**全部**导出名，逐个说清它承诺什么。
>
> 设计意图见 [DESIGN.md](DESIGN.md)，实施次序见 [ROADMAP.md](ROADMAP.md)。
> 这里只说**契约**：有什么、怎么用、变不变。
>
> 当前规模：**55 个类型、24 个包级函数、56 个方法、62 个常量与变量。**
> 附录 A 是完整清单，用于冻结后的比对。

---

## 0. 两层，先搞清楚你在哪一层

| 你要做的事 | import 什么 |
|---|---|
| 开一局狼人杀、读它的状态 | 只要 `werewolf` |
| **改规则**：写解析器、换胜负判定、加角色、拆快照、按错误码分支 | 还要 `werewolf/engine` |

`werewolf` 包的 `alias.go` 是一份**刻意很短**的别名清单——只收录本包导出
API 用得到的名字。一旦要改规则，调用点上就会出现 `engine.` 这个前缀。
**这不是遗漏，是想让边界在代码里看得见。**

下面说的全部是 `engine` 包。

---

## 1. 一眼看完

| 组 | 名字 | 什么时候用 |
|---|---|---|
| **词汇表** | `PhaseType` `RoleType` `SkillType` `EventType` `Camp` | 定义你这套游戏有哪些阶段、角色、技能 |
| **建局** | `Config` `PhaseConfig` `PhaseStep` | 描述阶段怎么流转、每个阶段谁该动 |
| **开局** | `NewEngine` `MustNewEngine` `RestoreEngine` `ReplayEngine` `EngineOption` | 造一台引擎 |
| **推进** | `Engine.AddPlayer` `.Start` `.SubmitSkillUse` `.EndPhase` `.Apply` | 把一局游戏跑起来 |
| **读局面** | `Engine.Status` `.Var` `.PlayerInfo` `.AlivePlayerIDs` `.RoundContext` `.PhaseInfo` `.PhaseReadiness` `.View` | 问引擎「现在怎么样了」 |
| **玩家视角** | `Engine.PlayerView` `.AllowedSkills` `.Teammates` `.AudienceOf` | 给玩家发什么 |
| **发言** | `Engine.SendMessage` `.MessageReceivers` `Message` | 玩家说话 |
| **回调** | `Engine.OnEvent` `.OnMessage` `EventHandler` `MessageHandler` `Event` | 事件推给宿主 |
| **规则写的东西** | `Resolver` `GameView` `Effect` `VarScope` `SkillUse` | 一个阶段怎么结算 |
| **八个扩展点** | `With*` 八个 + 八个接口 + 八个 `*Func` | 加角色、换判定、改边界 |
| **存档回放** | `Engine.Snapshot` `.EffectLog` + 五个 `*Snapshot` 类型 | 存盘、复盘、审计 |
| **错误** | `ErrorCode` `GameError` `Err*` 20 个 `Code*` 18 个 `HasCode` `CodeOf` `WrapError` | 按错误码分支 |
| **测试辅助** | `Board` `Seat` `Mark` | 单测你自己的解析器 |

---

## 2. 词汇表

**五个类型，内核几乎不拥有取值。** 底层都是字符串。零值是空串，语义即「未指定」。

```go
type PhaseType string   // 阶段：NIGHT_WOLF / PROPOSE / …
type RoleType  string   // 角色：WEREWOLF / MERLIN / …
type SkillType string   // 技能：KILL / APPROVE / …
type EventType string   // 「发生了什么」：KILL / MISSION_FAIL / …
type Camp      string   // 一个「边」：GOOD / EVIL / …
```

五个都有 `String()`，未指定时打印 `"UNSPECIFIED"`。

### 内核自己拥有的取值

**只有这些。这张表只能变短。** 每一个的理由见 [DESIGN.md §7](DESIGN.md)。

| 取值 | 是什么 |
|---|---|
| `PhaseUnspecified` `PhaseStart` `PhaseEnd` | 状态机的生命周期。`AddPlayer` 只在 `START` 里允许 |
| `RoleUnspecified` | 在 `PhaseStep` 上表示「所有角色」 |
| `RoleSystem` + `SkillAnnounce` | 「这一步没有玩家承担」，一个标记而不是身份。就绪判定不数它。想要一个叫「上帝」的角色，在规则包里起名（`werewolf.RoleGod` 就是） |
| `SkillUnspecified` `SkillSkip` | `SKIP` 是唯一不需要目标的动作 |
| `CampUnspecified` | 还没分出胜负，或这名玩家不属于任何一边 |
| `VarCamp = "camp"` | 标准键：值会填进 `PlayerInfo.Camp` / `SelfInfo.Camp` |
| `VarPresent = "1"` | 「有 / 没有」这类状态的约定值（空串等同删除，所以要一个非空值） |
| 九个 `Event*` | 内核自己的事件，见 §9 |

---

## 3. 建局

```go
type Config struct {
	StartPhase     PhaseType                   // 开局进哪个阶段
	Phases         map[PhaseType]*PhaseConfig  // 全部阶段
	DefaultTimeout time.Duration               // 建议值，引擎不据此计时
}
func (c *Config) Validate() error
func (c *Config) PhaseTimeout(phase PhaseType) time.Duration
```

```go
type PhaseConfig struct {
	Type      PhaseType
	Steps     []PhaseStep
	Timeout   time.Duration
	NextPhase PhaseType   // 默认出口，可被 GOTO_PHASE 改写

	EndsRound       bool  // 这个阶段结束时，回合数 +1
	ClearsRoundVars bool  // 进入这个阶段之前，回合级变量全部清空
}
```

**只有会转圈的阶段图才需要回合边界。** `Validate()` 沿 `NextPhase` 从
`StartPhase` 走一遍：走得到 `PhaseEnd` 就是一条直线，每个阶段只经过一次，
第二个回合根本不存在，也就不要求声明。一夜狼人就是这样一副图（整局一个
夜晚、一次讨论、一次投票）。转圈的图仍然两者各至少要有一个。

**`EndsRound` 与 `ClearsRoundVars` 是两件事，分开声明。** 绝大多数板子两者
重合（狼人杀：投票阶段结束既是新回合、也该清空），阿瓦隆不重合——队伍标记
要活到下一次提名，而回合数跟着第几轮任务走。`Validate()` 要求两者**各至少
有一个阶段声明**，否则回合永远是 1、回合变量永远不清。

```go
type PhaseStep struct {
	Role  RoleType   // 哪个角色；RoleUnspecified 表示所有角色
	Skill SkillType

	Required bool    // 不满足就不就绪（只影响 PhaseReadiness，不影响 EndPhase）
	Multiple bool    // true=所有合格行动者都要提交；false=任意一人即可
	Group    string  // 互斥备选组：同组内提交任意一个即算完成（开枪 / 不开枪）

	AllowDeadTarget bool  // 这个技能能否指向已出局的玩家（女巫的解药）
}
```

---

## 4. 开局

```go
func NewEngine(config *Config, opts ...EngineOption) (*Engine, error)
func MustNewEngine(config *Config, opts ...EngineOption) *Engine
func RestoreEngine(config *Config, snap *Snapshot, opts ...EngineOption) (*Engine, error)
func ReplayEngine(config *Config, log []*Effect, opts ...EngineOption) (*Engine, error)

type EngineOption func(*Engine) error
```

四个入口都吃同一批选项。**选项只能在构造时给出**：引擎交到调用方手上之后，
解析器、判定器、四个 provider 就不再变。

`RestoreEngine` / `ReplayEngine` 的 `config` 与 `opts` **必须与录制时一致**
——快照与效果流记的是「发生了什么」，不含规则。

---

## 5. Engine

23 个方法。**全部可并发调用。**

### 5.1 推进

```go
func (e *Engine) AddPlayer(id string, role RoleType) error  // 只在 START 阶段
func (e *Engine) Start() error
func (e *Engine) SubmitSkillUse(use *SkillUse) error
func (e *Engine) EndPhase() ([]*Effect, error)
func (e *Engine) Apply(effects ...*Effect) []*Effect
```

`SubmitSkillUse` **在提交时就拦**，不是收下来再让规则事后丢掉——否则
`AllowedSkills` 会对没资格的玩家说谎、`PhaseReadiness` 会等一群不可能行动的人。

`EndPhase` 是一整套：解析技能 → 应用效果 → 判胜负 → 流转。返回本阶段产生
的全部效果（含被否决的）。

`Apply` 绕开阶段结算，直接施加效果。**这是一把有刃的工具**，但必需：宿主真
会遇到「玩家掉线判死」「管理员踢人」。它走的仍是**同一个写入点**——效果进
效果流、被否决的不生效、内核原语不外发。

### 5.2 读局面

```go
func (e *Engine) Status() Status   // 摘要，四项在同一个读锁里取出
type Status struct {
	Phase  PhaseType
	Round  int
	Over   bool
	Winner Camp
}

func (e *Engine) Var(scope VarScope, key string) string
func (e *Engine) PlayerInfo(playerID string) (PlayerInfo, bool)
func (e *Engine) AlivePlayerIDs() []string
func (e *Engine) RoundContext() *RoundContext
func (e *Engine) PhaseInfo() *PhaseInfo          // 上帝视角
func (e *Engine) PhaseReadiness() PhaseReadiness // 还差谁行动
func (e *Engine) View() GameView                 // 完整只读局面（会 clone）
```

**为什么 `Status` 是一个结构体而不是四个方法**：四个方法各取一次读锁，
宿主要渲染「第 3 回合的白天」得连问两次，中间另一个 goroutine 结算掉一个
阶段的话，读到的是一组**从来不曾同时成立**的值。四项标量、一次锁、不分配。

**为什么 `View()` 之外还有便宜的读法**：`View()` 会 clone 整个局面，问一句
「现在第几回合」不该付那个代价。这是性能分层，不是重复。

### 5.3 玩家视角与边界

```go
func (e *Engine) PlayerView(playerID string) *PlayerView
func (e *Engine) AllowedSkills(playerID string) []SkillType
func (e *Engine) Teammates(playerID string) []string
func (e *Engine) AudienceOf(event *Event) ([]string, bool)
```

`AudienceOf` 的第二个返回值区分两件必须分得开的事：

| 返回 | 意思 | 调用方该做什么 |
|---|---|---|
| `(nil, true)` | **明确不给任何人看**（内核状态原语） | 什么都别发 |
| `(ids, true)` | 明确给这些人 | 发给他们 |
| `(nil, false)` | **不知道**（规则没装 provider） | 自己路由 |

### 5.4 发言

```go
func (e *Engine) SendMessage(senderID, content string) error
func (e *Engine) MessageReceivers(senderID string) []string
type Message struct { /* 发送者、内容、阶段、回合 */ }
```

发言**不走技能通道**。可听范围由 `SpeechProvider` 决定；没装 provider 时
退回「活人都能听到」。

### 5.5 回调

```go
func (e *Engine) OnEvent(handler EventHandler)
func (e *Engine) OnMessage(handler MessageHandler)

type EventHandler   func(event *Event)
type MessageHandler func(msg *Message, receiverIDs []string)
```

**回调一律在释放锁之后执行**，handler 列表在锁内快照——既不会死锁（回调里
可以安全调用 `Engine` 的方法），也不会与并发注册产生竞争。单个 handler
panic 被隔离并记 Error 日志，不影响其他 handler。

### 5.6 存档与回放

```go
func (e *Engine) Snapshot() *Snapshot   // 纯数据，可直接 json.Marshal
func (e *Engine) EffectLog() []*Effect  // 自建局以来的完整效果流
```

**两者分工**：快照是**状态**，效果流是**历史**。要持久化用快照——
`Effect.Data` 是 `map[string]interface{}`，经 JSON 往返类型会退化，
效果流的设计目标是进程内的回放与审计，不是存储格式。

---

## 6. 规则写的东西

### 6.1 Resolver

```go
type Resolver interface {
	Resolve(uses []*SkillUse, view GameView) []*Effect
}
type ResolverFunc func(uses []*SkillUse, view GameView) []*Effect
```

**这个签名是整个库最重要的一行**：只能读 `GameView`、只能返回 `Effect`。
「状态变更一律经由 Effect」因此由**签名**保证，不靠约定。

**不要在实现里回调 `Engine` 的任何方法**——扩展点在引擎持锁期间被同步调用，
Go 的读写锁不可重入，后果是**挂住，不是报错**。签名是刻意收窄的：
扩展点拿不到 `*Engine`。

### 6.2 GameView

```go
type GameView interface {
	Player(id string) (PlayerInfo, bool)
	AlivePlayers() []PlayerInfo
	AllPlayers() []PlayerInfo          // 含已出局的：屠神判定要数死人
	AlivePlayerIDsByRole(role RoleType) []string
	RoundContext() RoundContext
	Var(scope VarScope, key string) string
	Round() int
	Phase() PhaseType
}
```

**视图只提供事实，不提供判断。** 「还剩几个神职」是规则的判断，自己数。

### 6.3 VarScope：一张 2×2 的表

```go
type VarScope struct{ /* 字段未导出 */ }

var ScopeGame  VarScope  // 整局有效、无主
var ScopeRound VarScope  // 本回合有效、无主

func (s VarScope) Of(playerID string) VarScope  // 绑到某个玩家，时间尺度不变
func (s VarScope) String() string               // game / round / game:p1 / round:p1
```

| | 无主 | 属于某个玩家 |
|---|---|---|
| **整局有效** | `ScopeGame` | `ScopeGame.Of(id)` |
| **本回合有效** | `ScopeRound` | `ScopeRound.Of(id)` |

```go
// 写
engine.NewSetVarEffect(engine.ScopeGame,        "score",    "3")
engine.NewSetVarEffect(engine.ScopeGame.Of(id), "antidote", "used")
engine.NewSetVarEffect(engine.ScopeRound,       "kill",     target)
engine.NewSetVarEffect(engine.ScopeRound.Of(id),"guarded",  engine.VarPresent)

// 读
view.Var(engine.ScopeRound.Of(id), "guarded")
```

**值是字符串，空串等同删除，四格同一个口径。** 键名由规则自己定。

四格由两个值叉乘一个方法得出，**缺一格根本写不出来**——这张表此前只存在于
注释里、代码里是八个平铺的名字，于是少了「整局·无主」那一格很久没人发现。

### 6.4 Effect

```go
type Effect struct {
	Type     EventType
	SourceID string
	TargetID string
	Data     map[string]interface{}
	Canceled bool
	Reason   string
}

func NewEffect(eventType EventType, sourceID, targetID string) *Effect
func (e *Effect) WithData(key string, value interface{}) *Effect
func (e *Effect) Cancel(reason string)
func (e *Effect) ToEvent() *Event
```

**六个构造器，两类东西：**

| 构造器 | 类别 | 状态机认得吗 |
|---|---|---|
| `NewEffect` | 规则给「发生了什么」起名字 | 不认得，推给 `OnEvent` |
| `NewSetAliveEffect(id, alive)` | 改状态 | ✅ |
| `NewSetVarEffect(scope, k, v)` | 改状态 | ✅ |
| `NewSetActorsEffect(phase, ids...)` | 改状态（写行动者名单） | ✅ |
| `NewDetourEffect(id, phase)` | 下指令（排一笔绕道的欠账） | ✅ |
| `NewGotoPhaseEffect(phase)` | 下指令（改写下一阶段） | ✅ 但不改任何状态 |

**「狼刀」不会让人死。** 一条 `KILL` 单独发出去，状态机不认得。规则要让人
出局，就在它旁边产出一条 `SET_ALIVE`。两个效果，两件事——前者给受众与效果
流看，后者给状态机看。

**两个检视方法**，给想拦下某一类变更的扩展用：

```go
func (e *Effect) SetsAlive() (alive, ok bool)
func (e *Effect) SetsVar() (scope VarScope, key, value string, ok bool)
```

白痴被投票放逐时翻牌不出局，靠的就是把那条致死的原语否决掉。**拦原语而不是
拦「放逐」这个说法**，好处是与死因无关——同一段代码能挡住狼刀、毒杀、枪口
和任何第三方规则的死法。

### 6.5 SkillUse

```go
type SkillUse struct {
	PlayerID string
	Skill    SkillType
	Targets  []string
	Phase    PhaseType  // 由引擎填
	Round    int        // 由引擎填
}
func (u *SkillUse) Target() string  // 单目标读法：Targets[0]，空则空串
```

---

## 7. 八个扩展点

**八个都能用一个普通函数装上。内置角色没有特权**——它们走同一批门。

| 想加什么 | 选项 | 接口 | 函数适配器 |
|---|---|---|---|
| 某个阶段怎么结算 | `WithResolver(phase, r)` | `Resolver` | `ResolverFunc` |
| 怎么算赢 | `WithVictoryChecker(c)` | `VictoryChecker` | `VictoryFunc` |
| 某个角色带着什么入座 | `WithRoleSetup(role, s)` | `RoleSetup` | `RoleSetupFunc` |
| 开局那一刻铺什么 | `WithGameSetup(s)` | `GameSetup` | `GameSetupFunc` |
| 一件事该告诉谁 | `WithAudience(p)` | `AudienceProvider` | `AudienceFunc` |
| 谁和谁是一边的 | `WithTeammates(p)` | `TeammateProvider` | `TeammateFunc` |
| 发言谁能听到 | `WithSpeech(p)` | `SpeechProvider` | `SpeechFunc` |
| 角色额外看到什么 | `WithRoleInfo(role, p)` | `RoleInfoProvider` | `RoleInfoFunc` |

外加一个不算扩展点的：`WithLogger(l)` / `Logger` / `Field`——那是宿主的接线。

**`RoleSetup` 与 `GameSetup` 的分工**：前者是「这个角色带着什么入座」
（女巫的两瓶药），后者是「开局那一刻整个局面该铺成什么样」——它能看到
`GameView`，因此做得了 `RoleSetup` 做不到的事，比如「第一个队长是几号座位」。

**允许不对称**：恶魔认得爪牙、反过来不成立；阿瓦隆的奥伯伦既不认识同伙也不
被同伙认识。内核不假设对称。

---

## 8. 信息边界的类型

### 一名玩家的三副面孔

```go
type PlayerInfo struct {           // 上帝 / 规则
	ID    string
	Role  RoleType
	Alive bool
	Vars      map[string]string
	RoundVars map[string]string
}

type SelfInfo struct {             // 他自己
	ID    string
	Role  RoleType
	Alive bool
	Camp  Camp
}                                  // 注意：没有 Vars

type PublicPlayerInfo struct {     // 别人
	ID    string
	Alive bool
	Role  RoleType  // 仅在对本视角公开时填充
}
```

**这三个不合并。** `PublicPlayerInfo` **在类型上就装不下 `Vars`**——
「这一项该不该给他看」因此是签名问题，不是运行时判断。

要给玩家看的私有状态，由角色经 `RoleInfoProvider` **显式投射**。

### PlayerView

```go
type PlayerView struct {
	PlayerID string
	Round    int
	Phase    PhaseType
	Self          SelfInfo
	Players       []PublicPlayerInfo  // 按 ID 排序
	AllowedSkills []SkillType         // 永不为 nil；空切片=还没轮到我
	Teammates     []string
	RoleInfo      map[string]string   // 角色显式投射的东西
}
```

### 上帝视角

```go
type PhaseInfo struct { /* 阶段、回合、活跃角色、每个角色的信息 */ }
func (p *PhaseInfo) NeedsGodAnnouncement() bool
func (p *PhaseInfo) GodAnnouncementStep() *PhaseStep
func (p *PhaseInfo) PlayerActionSteps() []PhaseStep

type RolePhaseInfo struct { /* 可用技能、该行动的人、队友、角色专属信息 */ }

type PhaseReadiness struct {
	Phase    PhaseType
	Round    int
	Ready    bool             // 所有 Required 步骤都满足了吗
	Pending  []PendingAction  // 还差谁「必须」动
	Optional []PendingAction  // 谁「可以」动但还没动
	Acted    []string
}
type PendingAction struct { PlayerID string; Role RoleType; Skill SkillType }
```

**`Pending` 与 `Optional` 分开**，因为「还差谁必须动」和「本阶段谁可以动」
是两回事：默认配置里只有狼刀与投票是 `Required`，守卫、女巫、预言家、猎人
全都可以不动。只看 `Pending` 驱动游戏的话，这几个角色一整局都不会被叫到。

`Ready == false` **不会**让 `EndPhase` 拒绝——引擎不计时，是否按超时推进由
调用方决定。

---

## 9. 事件

```go
type Event struct { /* 类型、来源、目标、数据、阶段、回合 */ }
```

**九个内核事件。前七个永不外发，且这一条不可配置。**

| 事件 | 类别 |
|---|---|
| `EventSetAlive` `EventSetVar` `EventSetActors` `EventAbilityTriggered` | 改状态 |
| `EventGotoPhase` | 控制指令（不改任何状态） |
| `EventPlayerAdded` `EventPhaseChanged` | 回放记账 |
| `EventGameStarted` `EventGameEnded` | **公开**，玩家该看到 |

**判断依据是内核那张表，不是编号区间也不是名字前缀。** 早先写成
「>= 100 即内部」，与「第三方取值从 1000 起」直接打架——扩展定义的每个事件
都被判成内部事件，于是扩展的事件根本发不出去。

规则定义的任何其他 `EventType` 一律是外部事件，受众交给 `AudienceProvider`。

---

## 10. 存档

```go
const SnapshotVersion = 13

type Snapshot struct { /* 版本、阶段、回合、整局变量、行动者、赢家、玩家、回合上下文、未结算提交 */ }
type PlayerSnapshot struct{ ... }
type RoundCtxSnapshot struct{ ... }
type SkillUseSnapshot struct{ ... }
type DetourSnapshot struct{ ... }
```

**五个 `*Snapshot` 影子类型的存在是刻意的**：快照是写进存储的格式，字段名
必须稳定，不能随内部重构漂移。这是唯一一处「同一批数据两个类型」被明确
认可的地方。

**快照不含** `Config`、`Logger` 与回调——恢复时要把同一批选项传回去。

**快照含赢家**（v13 起）。谁赢是结束那一刻由 `VictoryChecker` 定下的、此后
不再变，而恢复出来的引擎不会再跑一次判定——不带它的话，一局已经结束的对局
恢复出来是 `Over=true` 而 `Winner` 为空，`Status` 那四项号称来自同一个瞬间，
在这条路上却对不上。这是一个真 bug，v13 修掉。

---

## 11. 错误

```go
type ErrorCode string
type GameError struct{ Code ErrorCode; Message string; ... }

func WrapError(code ErrorCode, format string, args ...interface{}) *GameError
func HasCode(err error, code ErrorCode) bool
func CodeOf(err error) ErrorCode
```

18 个 `Code*`，20 个 `Err*` 哨兵。**两种用法都支持**：

```go
if errors.Is(err, engine.ErrPlayerDead) { ... }        // 哨兵
if engine.HasCode(err, engine.CodePlayerDead) { ... }  // 错误码（跨进程友好）
```

带上下文的错误经 `Unwrap()` 仍能被 `errors.Is` 穿透。

---

## 12. 测试辅助

```go
type Board struct {
	Players   []PlayerInfo
	Round     int
	Phase     PhaseType
	Vars      map[string]string  // 整局·无主
	RoundVars map[string]string  // 本回合·无主
}
func (b Board) View() GameView
func (b Board) Apply(effects []*Effect) Board
func (b Board) Player(id string) (PlayerInfo, bool)
func (b Board) Var(scope VarScope, key string) string

func Seat(id string, role RoleType, alive bool, vars ...string) PlayerInfo
func Mark(p PlayerInfo, keys ...string) PlayerInfo
```

**名字不以 `Test` 开头，因为它是正经的公开 API**：规则包在内核之外，拿不到
内部状态，没有这个入口它的解析器就只能整局跑起来才测得动——那测的是集成。

```go
b = b.Apply(resolver.Resolve(uses, b.View()))
```

走的是与引擎**完全相同**的那个写入点，因此「效果没生效」这类问题在单元测试
里就会暴露。

---

## 13. 稳定性承诺

### 不会变的（改它需要一条撞到过的具体理由）

1. **`Resolver` 的签名**——只能读 `GameView`、只能返回 `Effect`
2. **信息边界的地板**——内核状态原语永不外发，不可配置
3. **三副面孔不合并**——`PublicPlayerInfo` 装不下 `Vars` 是编译期保证
4. **五个 `*Snapshot` 与内部结构解耦**
5. **词汇表只有类型、取值在规则包**
6. **扩展点只能在构造时给出**
7. **`Effect` 是唯一的写入路径**（`Engine.Apply` 是同一个写入点，不是第二个）

### 会变的（[ROADMAP.md 阶段二](ROADMAP.md)）

| 会怎么变 | 影响谁 |
|---|---|
| `PlayerInfo.Alive` / `.Role` 从存储字段变成**派生字段** | 读法不变；写法从 `SET_ALIVE` 并入 `SET_VAR` |
| `SnapshotVersion` 提升，快照格式变更 | 旧存档读不了（当前零使用者） |
| ~~绕道队列相关的名字~~ | **已完成**（§14 第 3 条） |

### 没有承诺的

- **性能**。没有任何真实负载说它慢，优化在测量之后。
- **`Effect.Data` 的具体键名**。它们是内核的内部约定，读效果请用
  `SetsAlive()` / `SetsVar()` 这类方法，不要直接翻 `Data`。

---

## 14. 冻结前的清账（**七条，已全部处理**）

写这份文档时逐条过 API 才发现的七处不一致。全部已清。

| # | 问题 | 做法 |
|---|---|---|
| 1 | `CodeInvalidPlayerId` 与 `ErrInvalidPlayerID` 大小写不一致 | 统一为 `CodeInvalidPlayerID` |
| 2 | `PlayerInfo.Var(key)` / `.RoundVar(key)` 不吃 `VarScope`，与其他读法不一致 | **两个方法删掉**。`Vars` / `RoundVars` 是导出字段，读 nil map 在 Go 里本来就安全，这两个方法是零价值的糖——它们唯一的作用是让 `Var` 在两个类型上意思不同 |
| 3 | `PendingTrigger` / `NewAbilityTriggerEffect` 的文档还在说「死亡技能」 | 改名 `Detour` / `NewDetourEffect`，事件值 `ABILITY_TRIGGERED` → `DETOUR`，快照字段 `pending_triggers` → `detours`，`SnapshotVersion` 11 → 12 |
| 4 | `RoleGod` 的名字暗示「主持人」这个身份 | 内核改名 `RoleSystem`（值 `"GOD"` → `"SYSTEM"`）。「上帝」是狼人杀给这个标记起的名字，定在规则包（`werewolf.RoleGod`） |
| 5 | `Engine.SendMessage` 的文档说「玩家已死亡」会报错 | 改写：那是**没装 `SpeechProvider` 时的默认**，装了就由 provider 说了算 |
| 6 | `Engine.PlayerInfo` 的注释写着「（推荐使用）」 | 改写成它实际的语义：上帝视角，含 `Vars`，**不是**给玩家看的 |
| 7 | 没有任何东西钉住这份导出清单 | **`TestAPI_SurfaceIsPinned`**：`go/ast` 解析包内全部非测试源码，收集导出名，与 `engine/testdata/api.golden` 比对 |

### 第 7 条是执法机制

没有它，这份文档一定会和代码漂移——与这个项目其他「规矩只写在注释里」的
伤口是同一类问题。

钉住的是**名字加签名**（含接口的方法集）。只钉名字的话，「把 `CheckVictory`
的返回值从一个 `Camp` 改成一组」这种改动会溜过去——导出名一个都不增不减，
而所有实现者都会编译不过。参数改名不算变更，参数**类型**改了才算。

它不判断 API 好不好，只保证**变更不会悄悄发生**：

```
$ go test ./engine
--- FAIL: TestAPI_SurfaceIsPinned
    内核的导出面变了。
    新增：[func SneakyExport]
    删除：[]

    这不是错误，是提醒：导出面是 docs/API.md 声称冻结的东西。
    确认这次变更是有意的，然后一起做两件事——
      1. go test ./engine -run TestAPI_SurfaceIsPinned -update-api-golden
      2. 更新 docs/API.md（正文与附录 A）
```

「悄悄新增」「悄悄删除」「悄悄改签名」三个方向都验证过会变红——最后那个
用的是一个**能编译通过**的变异（给 `CodeOf` 加一个可变参，所有现有调用照样
编译），因为编译不过的变异证明不了这个测试本身。

---

## 附录 A：完整导出名清单

**冻结基线。** 由 `TestAPI_SurfaceIsPinned` 与 `engine/testdata/api.golden`
守着——**名字或签名**变了，测试就变红。

合计 **55 类型 / 24 包级函数 / 56 方法 /
20 个接口方法 / 62 常量与变量**。
下面按名字列出；带签名的完整清单在 `engine/testdata/api.golden`。

### 类型（55）

```
AudienceFunc  AudienceProvider  Board  Camp  Config  Detour
DetourSnapshot  Effect  Engine  EngineOption  ErrorCode  Event
EventHandler  EventType  Field  GameError  GameSetup  GameSetupFunc
GameView  Logger  Message  MessageHandler  PendingAction  PhaseConfig
PhaseInfo  PhaseReadiness  PhaseStep  PhaseType  PlayerInfo
PlayerSnapshot  PlayerView  PublicPlayerInfo  Resolver  ResolverFunc
RoleInfoFunc  RoleInfoProvider  RolePhaseInfo  RoleSetup  RoleSetupFunc
RoleType  RoundContext  RoundCtxSnapshot  SelfInfo  SkillType  SkillUse
SkillUseSnapshot  Snapshot  SpeechFunc  SpeechProvider  Status
TeammateFunc  TeammateProvider  VarScope  VictoryChecker  VictoryFunc
```

### 包级函数（24）

```
CodeOf  HasCode  Mark  MustNewEngine  NewDetourEffect  NewEffect
NewEngine  NewGotoPhaseEffect  NewSetActorsEffect  NewSetAliveEffect
NewSetVarEffect  ReplayEngine  RestoreEngine  Seat  WithAudience
WithGameSetup  WithLogger  WithResolver  WithRoleInfo  WithRoleSetup
WithSpeech  WithTeammates  WithVictoryChecker  WrapError
```

### 方法（56，按接收者）

```
Engine(23)  AddPlayer  AlivePlayerIDs  AllowedSkills  Apply  AudienceOf  EffectLog  EndPhase  MessageReceivers  OnEvent  OnMessage  PhaseInfo  PhaseReadiness  PlayerInfo  PlayerView  RoundContext  SendMessage  Snapshot  Start  Status  SubmitSkillUse  Teammates  Var  View
Effect(5)  Cancel  SetsAlive  SetsVar  ToEvent  WithData
Board(4)  Apply  Player  Var  View
PhaseInfo(3)  GodAnnouncementStep  NeedsGodAnnouncement  PlayerActionSteps
Config(2)  PhaseTimeout  Validate
GameError(2)  Error  Unwrap
VarScope(2)  Of  String
AudienceFunc(1)  Audience
Camp(1)  String
ErrorCode(1)  String
EventType(1)  String
GameSetupFunc(1)  Setup
PhaseType(1)  String
ResolverFunc(1)  Resolve
RoleInfoFunc(1)  RoleInfo
RoleSetupFunc(1)  Setup
RoleType(1)  String
SkillType(1)  String
SkillUse(1)  Target
SpeechFunc(1)  Receivers
TeammateFunc(1)  Teammates
VictoryFunc(1)  CheckVictory
```

### 常量（41）

```
CampUnspecified  CodeGameAlreadyStarted  CodeGameEnded
CodeGameNotStarted  CodeInvalidBoard  CodeInvalidConfig
CodeInvalidEffectLog  CodeInvalidPhase  CodeInvalidPlayerID
CodeInvalidRole  CodeInvalidSnapshot  CodeMessageNotAllowed
CodePlayerDead  CodePlayerExists  CodePlayerNotFound  CodeSkillNotAllowed
CodeTargetDead  CodeTargetNotFound  CodeUnspecified  DefaultPhaseTimeout
EventDetour  EventGameEnded  EventGameStarted  EventGotoPhase
EventPhaseChanged  EventPlayerAdded  EventSetActors  EventSetAlive
EventSetVar  EventUnspecified  PhaseEnd  PhaseStart  PhaseUnspecified
RoleSystem  RoleUnspecified  SkillAnnounce  SkillSkip  SkillUnspecified
SnapshotVersion  VarCamp  VarPresent
```

### 变量（21）

```
ErrBoardAlreadyDecided  ErrGameAlreadyStarted  ErrGameEnded
ErrGameNotStarted  ErrInvalidBoard  ErrInvalidConfig  ErrInvalidEffectLog
ErrInvalidPhase  ErrInvalidPlayerID  ErrInvalidRole  ErrInvalidSnapshot
ErrMessageNotAllowed  ErrNilSnapshot  ErrPlayerDead  ErrPlayerExists
ErrPlayerNotFound  ErrSkillNotAllowed  ErrTargetDead  ErrTargetNotFound
ScopeGame  ScopeRound
```
