// Package missions is this engine's second rules package: the mission-based
// social deduction of The Resistance and its Avalon variant.
//
// It does not exist to be another game, it exists to **test whether the kernel
// is really general**. Before it, the kernel had been validated by werewolf
// alone -- which proves the kernel does not know the word "witch", and proves
// nothing about whether it knows the structure "night follows day, and some
// number of people die each round".
//
// This ruleset is the opposite of werewolf in almost every respect: nobody is
// ever eliminated, the outcome counts nobody, discussion is public throughout,
// one round takes three phases (nomination, vote, mission), and there is a
// fourth phase that only appears once the good side has three successes. Every
// place it rubbed against the kernel is recorded in SCARS.md.
//
// # Where the rules come from
//
// Based on the English Wikipedia article for The Resistance (game), pinned
// clause by clause in the tests:
// https://en.wikipedia.org/wiki/The_Resistance_(game)
//
// The Chinese article gets Merlin wrong; see vocab.go.
package missions

import "github.com/Zereker/hiddenrole"

// Options assembles the kernel into a game of this ruleset.
//
// Like werewolf.Options, everything goes on through public options with no
// back doors. That both rulesets use the same set of entry points is
// guaranteed by the compiler -- this package sits outside the kernel.
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

// New creates a game of this ruleset.
func New(extra ...hiddenrole.EngineOption) (*hiddenrole.Engine, error) {
	return hiddenrole.NewEngine(DefaultConfig(), append(Options(), extra...)...)
}

// MustNew is New, panicking on error. For when the configuration is a
// constant.
func MustNew(extra ...hiddenrole.EngineOption) *hiddenrole.Engine {
	e, err := New(extra...)
	if err != nil {
		panic(err)
	}
	return e
}

// Restore rebuilds a game of this ruleset from a snapshot.
func Restore(snap *hiddenrole.Snapshot, extra ...hiddenrole.EngineOption) (*hiddenrole.Engine, error) {
	return hiddenrole.RestoreEngine(DefaultConfig(), snap, append(Options(), extra...)...)
}

// Replay rebuilds a game of this ruleset from an effect log.
func Replay(log []*hiddenrole.Effect, extra ...hiddenrole.EngineOption) (*hiddenrole.Engine, error) {
	return hiddenrole.ReplayEngine(DefaultConfig(), log, append(Options(), extra...)...)
}
