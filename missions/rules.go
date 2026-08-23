// Package avalon 是这个引擎的第二套规则：抵抗组织·这套规则。
//
// 它存在的理由不是「再做一个游戏」，而是**验证内核到底通不通用**。
// 在它之前，内核只被狼人杀一套规则验证过——那证明得了内核不认得「女巫」
// 这个词，证明不了内核不认得「夜晚接白天、每轮死几个人」这个结构。
//
// 这套规则与狼人杀几乎处处相反：没有人会出局，胜负不数人头，全程公开讨论，
// 一轮里要走提名、表决、任务三个阶段，还有一个只在好人赢下三次之后
// 才出现的刺杀阶段。它撞到内核的每一处别扭都记在 SCARS.md 里。
//
// # 规则来源
//
// 以英文维基百科 The Resistance (game) 条目为基准，逐条固化在测试里：
// https://en.wikipedia.org/wiki/The_Resistance_(game)
//
// 中文条目在梅林那一条上有误，见 vocab.go 的说明。
package missions

import "github.com/Zereker/hiddenrole"

// Options 把内核装配成一局这套规则。
//
// 与狼人杀的 werewolf.Options 一样，全部经公开选项装上去，没有后门。
// 两套规则用的是同一批入口，这件事由编译器保证——本包在内核之外。
func Options() []hiddenrole.EngineOption {
	opts := []hiddenrole.EngineOption{
		hiddenrole.WithResolver(PhasePropose, proposeResolver{}),
		hiddenrole.WithResolver(PhaseTeamVote, teamVoteResolver{}),
		hiddenrole.WithResolver(PhaseMission, missionResolver{}),
		hiddenrole.WithResolver(PhaseAssassin, assassinResolver{}),
		hiddenrole.WithVictoryChecker(victoryChecker{}),
		hiddenrole.WithAudience(hiddenrole.AudienceFunc(audience)),
		hiddenrole.WithTeammates(hiddenrole.TeammateFunc(teammates)),
		hiddenrole.WithSpeech(hiddenrole.SpeechFunc(speech)),
		hiddenrole.WithGameSetup(hiddenrole.GameSetupFunc(gameSetup)),
	}
	for role, setup := range builtinRoleSetup {
		opts = append(opts, hiddenrole.WithRoleSetup(role, setup))
	}
	for role, provider := range builtinRoleInfo {
		opts = append(opts, hiddenrole.WithRoleInfo(role, provider))
	}
	return opts
}

// New 造一局这套规则。
func New(extra ...hiddenrole.EngineOption) (*hiddenrole.Engine, error) {
	return hiddenrole.NewEngine(DefaultConfig(), append(Options(), extra...)...)
}

// MustNew 同 New，出错时 panic。配置是常量时可用。
func MustNew(extra ...hiddenrole.EngineOption) *hiddenrole.Engine {
	e, err := New(extra...)
	if err != nil {
		panic(err)
	}
	return e
}

// Restore 从快照恢复一局这套规则。
func Restore(snap *hiddenrole.Snapshot, extra ...hiddenrole.EngineOption) (*hiddenrole.Engine, error) {
	return hiddenrole.RestoreEngine(DefaultConfig(), snap, append(Options(), extra...)...)
}

// Replay 按效果流重建一局这套规则。
func Replay(log []*hiddenrole.Effect, extra ...hiddenrole.EngineOption) (*hiddenrole.Engine, error) {
	return hiddenrole.ReplayEngine(DefaultConfig(), log, append(Options(), extra...)...)
}
