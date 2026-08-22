// alias.go 把内核的公开 API 在本包再导出一遍。
//
// 拆包之后内核住在 werewolf/engine，但使用者不该被迫 import 两个包才能
// 写出一局狼人杀——`werewolf.Effect`、`werewolf.GameView` 与
// `werewolf.RoleWitch` 出现在同一段代码里是很自然的事。
//
// 这一层是纯别名，没有包装、没有转换：`werewolf.Effect` 与
// `engine.Effect` 是同一个类型，互相传递不需要转换。需要直接依赖内核的
// 使用者（写一套自己的规则包）照常 import werewolf/engine 即可。
//
// 它同时是一份清单：内核对外暴露的全部东西都在这里，一眼可数。

package werewolf

import "github.com/Zereker/werewolf/engine"

// ==================== 词汇表的类型 ====================
//
// 类型在内核，取值在本包（见 vocab.go）——内核不知道有「女巫」这个角色。

type (
	PhaseType    = engine.PhaseType
	RoleType     = engine.RoleType
	SkillType    = engine.SkillType
	EventType    = engine.EventType
	Camp         = engine.Camp
	RoleCategory = engine.RoleCategory
	ErrorCode    = engine.ErrorCode
)

// 内核自己拥有的取值：生命周期阶段、主持人、通用动作。
const (
	PhaseUnspecified = engine.PhaseUnspecified
	PhaseStart       = engine.PhaseStart
	PhaseEnd         = engine.PhaseEnd

	RoleUnspecified = engine.RoleUnspecified
	RoleGod         = engine.RoleGod

	SkillUnspecified = engine.SkillUnspecified
	SkillSkip        = engine.SkillSkip
	SkillAnnounce    = engine.SkillAnnounce

	EventUnspecified = engine.EventUnspecified
	EventGameStarted = engine.EventGameStarted
	EventGameEnded   = engine.EventGameEnded

	EventAbilityTriggered  = engine.EventAbilityTriggered
	EventPlayerAdded       = engine.EventPlayerAdded
	EventPhaseChanged      = engine.EventPhaseChanged
	EventSetPlayerVar      = engine.EventSetPlayerVar
	EventSetRoundVar       = engine.EventSetRoundVar
	EventSetAlive          = engine.EventSetAlive
	EventSetPlayerRoundVar = engine.EventSetPlayerRoundVar

	CampUnspecified         = engine.CampUnspecified
	RoleCategoryUnspecified = engine.RoleCategoryUnspecified

	VarCamp     = engine.VarCamp
	VarCategory = engine.VarCategory
	VarPresent  = engine.VarPresent
)

// DefaultPhaseTimeout 未给出 PhaseConfig.Timeout 时的兜底建议值。
// 各阶段的建议值在 board.go——那是板子数据。
const DefaultPhaseTimeout = engine.DefaultPhaseTimeout

// ==================== 引擎 ====================

// Engine 游戏引擎。就是内核的那一个——本包不包装它。
//
// 拆包时曾想过让本包套一层自己的 Engine，好把 NightKillTarget 这类
// 狼人杀专属读法留成方法。没有那么做：两个同名类型互相不能赋值，
// 使用者迟早会在 RestoreEngine 的返回值上撞到。狼人杀的便利读法
// 因此是包级函数（见 nightstate.go）。
type Engine = engine.Engine

// 内核的构造入口。狼人杀的组装入口是 New / NewWith / MustNew /
// MustNewWith / Restore / Replay，见 rules.go——它们只是把
// Options(rules) 拼在选项前面，没有走任何后门。
var (
	NewEngine     = engine.NewEngine
	MustNewEngine = engine.MustNewEngine
	RestoreEngine = engine.RestoreEngine
	ReplayEngine  = engine.ReplayEngine
)

// ==================== 局面与配置 ====================

type (
	GameConfig  = engine.GameConfig
	PhaseConfig = engine.PhaseConfig
	PhaseStep   = engine.PhaseStep
	SkillUse    = engine.SkillUse
)

// ==================== 效果与事件 ====================

type (
	Effect       = engine.Effect
	Event        = engine.Event
	EventHandler = engine.EventHandler
	Message      = engine.Message
	// MessageHandler 收到一条发言，以及它该发给哪些玩家。
	MessageHandler = engine.MessageHandler
)

// 产出效果的构造函数。规则自己命名发生了什么（NewEffect），
// 再用下面几个原语之一真正改状态。
var (
	NewEffect                  = engine.NewEffect
	NewSetAliveEffect          = engine.NewSetAliveEffect
	NewSetPlayerVarEffect      = engine.NewSetPlayerVarEffect
	NewSetRoundVarEffect       = engine.NewSetRoundVarEffect
	NewSetPlayerRoundVarEffect = engine.NewSetPlayerRoundVarEffect
	NewAbilityTriggerEffect    = engine.NewAbilityTriggerEffect
)

// ==================== 视图 ====================

type (
	GameView         = engine.GameView
	PlayerInfo       = engine.PlayerInfo
	PlayerView       = engine.PlayerView
	SelfInfo         = engine.SelfInfo
	PublicPlayerInfo = engine.PublicPlayerInfo
	PhaseInfo        = engine.PhaseInfo
	RolePhaseInfo    = engine.RolePhaseInfo
	PhaseReadiness   = engine.PhaseReadiness
	PendingAction    = engine.PendingAction
	RoundContext     = engine.RoundContext
	PendingTrigger   = engine.PendingTrigger
)

// ==================== 扩展点 ====================

type (
	Resolver         = engine.Resolver
	VictoryChecker   = engine.VictoryChecker
	RoleInfoProvider = engine.RoleInfoProvider
	RoleInfoFunc     = engine.RoleInfoFunc
	RoleSetup        = engine.RoleSetup
	RoleSetupFunc    = engine.RoleSetupFunc
	AudienceProvider = engine.AudienceProvider
	AudienceFunc     = engine.AudienceFunc
	TeammateProvider = engine.TeammateProvider
	TeammateFunc     = engine.TeammateFunc
	SpeechProvider   = engine.SpeechProvider
	SpeechFunc       = engine.SpeechFunc
	EngineOption     = engine.EngineOption
)

var (
	WithResolver       = engine.WithResolver
	WithVictoryChecker = engine.WithVictoryChecker
	WithRoleInfo       = engine.WithRoleInfo
	WithRoleSetup      = engine.WithRoleSetup
	WithAudience       = engine.WithAudience
	WithTeammates      = engine.WithTeammates
	WithSpeech         = engine.WithSpeech
	WithLogger         = engine.WithLogger
	WithMetrics        = engine.WithMetrics
)

// ==================== 解析器的单元测试 ====================
//
// 规则包要单元测试自己的解析器时需要它们：手工摆一副局面，转成 GameView
// 喂给解析器，再把产出的效果折回去看局面变成了什么样。没有这个入口，
// 规则的解析器就只能整局跑起来才测得动。

type Board = engine.Board

var (
	NewGameView = engine.NewGameView
	Seat        = engine.Seat
	Mark        = engine.Mark
)

// ==================== 存档与回放 ====================

type (
	Snapshot               = engine.Snapshot
	PlayerSnapshot         = engine.PlayerSnapshot
	RoundCtxSnapshot       = engine.RoundCtxSnapshot
	SkillUseSnapshot       = engine.SkillUseSnapshot
	PendingTriggerSnapshot = engine.PendingTriggerSnapshot
)

// SnapshotVersion 当前快照格式的版本号。
const SnapshotVersion = engine.SnapshotVersion

// ==================== 日志与指标 ====================

type (
	Logger  = engine.Logger
	Metrics = engine.Metrics
	Field   = engine.Field
)

var (
	NewNopLogger  = engine.NewNopLogger
	NewNopMetrics = engine.NewNopMetrics
	F             = engine.F
	PlayerField   = engine.PlayerField
	PhaseField    = engine.PhaseField
	SkillField    = engine.SkillField
	TargetField   = engine.TargetField
	EventField    = engine.EventField
	RoundField    = engine.RoundField
)

// ==================== 错误 ====================

type GameError = engine.GameError

// 错误码。规则用它们给自己的错误分类，与内核同一套。
const (
	CodeUnspecified        = engine.CodeUnspecified
	CodePlayerNotFound     = engine.CodePlayerNotFound
	CodePlayerDead         = engine.CodePlayerDead
	CodeTargetNotFound     = engine.CodeTargetNotFound
	CodeTargetDead         = engine.CodeTargetDead
	CodeSkillNotAllowed    = engine.CodeSkillNotAllowed
	CodeGameNotStarted     = engine.CodeGameNotStarted
	CodeGameEnded          = engine.CodeGameEnded
	CodeInvalidPhase       = engine.CodeInvalidPhase
	CodeMessageNotAllowed  = engine.CodeMessageNotAllowed
	CodePlayerExists       = engine.CodePlayerExists
	CodeInvalidPlayerId    = engine.CodeInvalidPlayerId
	CodeInvalidRole        = engine.CodeInvalidRole
	CodeGameAlreadyStarted = engine.CodeGameAlreadyStarted
	CodeInvalidBoard       = engine.CodeInvalidBoard
	CodeInvalidSnapshot    = engine.CodeInvalidSnapshot
	CodeInvalidConfig      = engine.CodeInvalidConfig
	CodeInvalidEffectLog   = engine.CodeInvalidEffectLog
)

// 哨兵错误。errors.Is 与 HasCode 都可用来判别，见内核 errors.go。
var (
	ErrPlayerNotFound    = engine.ErrPlayerNotFound
	ErrPlayerDead        = engine.ErrPlayerDead
	ErrTargetNotFound    = engine.ErrTargetNotFound
	ErrTargetDead        = engine.ErrTargetDead
	ErrSkillNotAllowed   = engine.ErrSkillNotAllowed
	ErrGameNotStarted    = engine.ErrGameNotStarted
	ErrGameEnded         = engine.ErrGameEnded
	ErrInvalidPhase      = engine.ErrInvalidPhase
	ErrMessageNotAllowed = engine.ErrMessageNotAllowed

	ErrPlayerExists        = engine.ErrPlayerExists
	ErrInvalidPlayerID     = engine.ErrInvalidPlayerID
	ErrInvalidRole         = engine.ErrInvalidRole
	ErrGameAlreadyStarted  = engine.ErrGameAlreadyStarted
	ErrInvalidBoard        = engine.ErrInvalidBoard
	ErrBoardAlreadyDecided = engine.ErrBoardAlreadyDecided

	ErrInvalidSnapshot  = engine.ErrInvalidSnapshot
	ErrNilSnapshot      = engine.ErrNilSnapshot
	ErrInvalidEffectLog = engine.ErrInvalidEffectLog

	ErrInvalidConfig = engine.ErrInvalidConfig
)

var (
	WrapError = engine.WrapError
	CodeOf    = engine.CodeOf
	HasCode   = engine.HasCode
)
