// alias.go 把内核的一小部分名字在本包再导出一遍。
//
// 只有一条收录规则：**本包自己的导出 API 用得到的类型，才在这里出现**——
// 出现在导出函数的签名里（`Resolve(uses []*SkillUse, view GameView) []*Effect`）、
// 出现在导出类型的字段里（`PhaseConfig.Steps []PhaseStep`），或者是词汇表
// 本身的类型与取值（`PhaseType` 与 `PhaseStart`）。
//
// 因此这份清单不是内核的目录，恰恰相反：它短到能一眼看完，是想让边界在
// 代码里看得见。开一局狼人杀、读它的状态，只 import 本包就够；一旦要
// **改**规则——自己写解析器、换胜负判定、接日志与指标、按错误码分支、
// 拆快照——那是内核的事，import github.com/Zereker/hiddenrole，
// 在调用点上写出 hiddenrole. 这个前缀。本包自己的 resolver.go、rolesetup.go
// 就是这么写的。
//
// 这一层是纯别名，没有包装、没有转换：`werewolf.Effect` 与 `hiddenrole.Effect`
// 是同一个类型，两个包之间互相传递不需要任何转换。

package werewolf

import "github.com/Zereker/hiddenrole"

// ==================== 词汇表 ====================
//
// 类型在内核，取值在本包（见 vocab.go）——内核不知道有「女巫」这个角色。

type (
	PhaseType = hiddenrole.PhaseType
	RoleType  = hiddenrole.RoleType
	SkillType = hiddenrole.SkillType
	EventType = hiddenrole.EventType
	Camp      = hiddenrole.Camp
)

// 词汇表里内核自己拥有的那几个取值：开局前与结束后这两个生命周期阶段、
// 以及任何板子都要有的两个通用动作。它们与 vocab.go 里狼人杀的取值同属
// 一张表，拆开放会让人以为拼一副板子要 import 两个包。
//
// RoleGod 是个例外：它不是内核的取值，是狼人杀给内核那个标记起的名字。
const (
	PhaseStart = hiddenrole.PhaseStart
	PhaseEnd   = hiddenrole.PhaseEnd

	// RoleGod 上帝，也就是主持人。
	//
	// 内核认得的是 RoleSystem——「这一步没有玩家承担」，一个标记而不是
	// 一个身份。它不认得「主持人」：任务制那一套根本没有人主持，血染钟楼叫
	// 说书人。「上帝」是狼人杀给那个标记起的名字，所以定在本包。
	RoleGod = hiddenrole.RoleSystem

	SkillSkip     = hiddenrole.SkillSkip
	SkillAnnounce = hiddenrole.SkillAnnounce

	// VarCamp 与 VarPresent 是内核唯二认识的玩家变量名：前者供胜负判定
	// 数阵营，后者标记「这一局有没有这个角色」。取值同样在本包。
	VarCamp    = hiddenrole.VarCamp
	VarPresent = hiddenrole.VarPresent
)

// 变量作用域的两个取值。读一局的状态要用到它们——`e.Var(ScopeRound, key)`
// ——所以它们和词汇表一样属于「只 import 本包就够」的那一层。
// 加 .Of(playerID) 得到属于某个玩家的那两格，四格拼法见 hiddenrole.VarScope。
var (
	ScopeGame  = hiddenrole.ScopeGame
	ScopeRound = hiddenrole.ScopeRound
)

// ==================== 板子 ====================

type (
	// GameConfig 阶段机的配置，由 DefaultGameConfig 与 Standard*Phase 系列
	// 产出。内核里它叫 Config——那边不需要「Game」这个前缀来区分，这边
	// 留着是因为狼人杀还有一个 Rules。
	GameConfig = hiddenrole.Config

	PhaseConfig = hiddenrole.PhaseConfig
	PhaseStep   = hiddenrole.PhaseStep
)

// ==================== 开一局 ====================

// Engine 游戏引擎。就是内核的那一个——本包不包装它。
//
// 拆包时曾想过让本包套一层自己的 Engine，好把 NightKillTarget 这类
// 狼人杀专属读法留成方法。没有那么做：两个同名类型互相不能赋值，
// 使用者迟早会在 hiddenrole.RestoreEngine 的返回值上撞到。狼人杀的便利读法
// 因此是包级函数（见 nightstate.go）。
type Engine = hiddenrole.Engine

type (
	// EngineOption 装配引擎的选项。本包的 Options(rules) 返回一整套；
	// 要再加一条（比如 hiddenrole.WithLogger），从内核包取。
	EngineOption = hiddenrole.EngineOption

	// SkillUse 一次技能提交，Engine.SubmitSkillUse 的入参。
	SkillUse = hiddenrole.SkillUse

	// GameView 解析器与胜负判定看到的局面。
	GameView = hiddenrole.GameView

	// VarScope 变量作用域，Engine.Var 与 GameView.Var 的第一个参数。
	// 取值是下面的 ScopeGame / ScopeRound，可加 .Of(playerID)。
	VarScope = hiddenrole.VarScope

	// Effect 状态变更。Replay 吃的就是它的日志。
	Effect = hiddenrole.Effect

	// Snapshot 存档，Restore 的入参。
	Snapshot = hiddenrole.Snapshot
)
