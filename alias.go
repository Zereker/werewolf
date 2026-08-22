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
// 拆快照——那是内核的事，import github.com/Zereker/werewolf/engine，
// 在调用点上写出 engine. 这个前缀。本包自己的 resolver.go、rolesetup.go
// 就是这么写的。
//
// 这一层是纯别名，没有包装、没有转换：`werewolf.Effect` 与 `engine.Effect`
// 是同一个类型，两个包之间互相传递不需要任何转换。

package werewolf

import "github.com/Zereker/werewolf/engine"

// ==================== 词汇表 ====================
//
// 类型在内核，取值在本包（见 vocab.go）——内核不知道有「女巫」这个角色。

type (
	PhaseType = engine.PhaseType
	RoleType  = engine.RoleType
	SkillType = engine.SkillType
	EventType = engine.EventType
	Camp      = engine.Camp
)

// 词汇表里内核自己拥有的那几个取值：开局前与结束后这两个生命周期阶段、
// 主持人、以及任何板子都要有的两个通用动作。它们与 vocab.go 里狼人杀的
// 取值同属一张表，拆开放会让人以为拼一副板子要 import 两个包。
const (
	PhaseStart = engine.PhaseStart
	PhaseEnd   = engine.PhaseEnd

	RoleGod = engine.RoleGod

	SkillSkip     = engine.SkillSkip
	SkillAnnounce = engine.SkillAnnounce

	// VarCamp 与 VarPresent 是内核唯二认识的玩家变量名：前者供胜负判定
	// 数阵营，后者标记「这一局有没有这个角色」。取值同样在本包。
	VarCamp    = engine.VarCamp
	VarPresent = engine.VarPresent
)

// ==================== 板子 ====================

type (
	// GameConfig 阶段机的配置，由 DefaultGameConfig 与 Standard*Phase 系列
	// 产出。内核里它叫 Config——那边不需要「Game」这个前缀来区分，这边
	// 留着是因为狼人杀还有一个 Rules。
	GameConfig = engine.Config

	PhaseConfig = engine.PhaseConfig
	PhaseStep   = engine.PhaseStep
)

// ==================== 开一局 ====================

// Engine 游戏引擎。就是内核的那一个——本包不包装它。
//
// 拆包时曾想过让本包套一层自己的 Engine，好把 NightKillTarget 这类
// 狼人杀专属读法留成方法。没有那么做：两个同名类型互相不能赋值，
// 使用者迟早会在 engine.RestoreEngine 的返回值上撞到。狼人杀的便利读法
// 因此是包级函数（见 nightstate.go）。
type Engine = engine.Engine

type (
	// EngineOption 装配引擎的选项。本包的 Options(rules) 返回一整套；
	// 要再加一条（比如 engine.WithLogger），从内核包取。
	EngineOption = engine.EngineOption

	// SkillUse 一次技能提交，Engine.SubmitSkillUse 的入参。
	SkillUse = engine.SkillUse

	// GameView 解析器与胜负判定看到的局面。
	GameView = engine.GameView

	// Effect 状态变更。Replay 吃的就是它的日志。
	Effect = engine.Effect

	// Snapshot 存档，Restore 的入参。
	Snapshot = engine.Snapshot
)
