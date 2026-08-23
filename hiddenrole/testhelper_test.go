package hiddenrole

import (
	"maps"
	"testing"
)

// Helpers for the kernel's own tests. The similarly named helpers of the
// werewolf layer stay in the root package -- they know the words "kill" and
// "guarded", and the kernel does not.

// mustAddTo adds a player straight to the state, failing the test on error.
func mustAddTo(t *testing.T, s *gameState, id string, role RoleType) {
	t.Helper()
	if err := s.addPlayer(id, role); err != nil {
		t.Fatalf("addPlayer(%q, %v): %v", id, role, err)
	}
}

// mustAdd adds a player through the engine, failing the test on error.
func mustAdd(t *testing.T, e *Engine, id string, role RoleType) {
	t.Helper()
	if err := e.AddPlayer(id, role); err != nil {
		t.Fatalf("AddPlayer(%q, %v): %v", id, role, err)
	}
}

// newTestEngine builds an engine carrying testConfig.
//
// It comes with no resolvers -- the kernel never ships any. When a test needs
// an engine that can advance phases, use withNoopResolvers.
func newTestEngine(t *testing.T, opts ...EngineOption) *Engine {
	t.Helper()
	e, err := NewEngine(testConfig(), opts...)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	return e
}

// noopResolver does nothing, and exists so that phases can advance.
type noopResolver struct{}

func (noopResolver) Resolve([]*SkillUse, GameView) []*Effect { return nil }

// withNoopResolvers installs an empty resolver on every phase of testConfig.
//
// The kernel's Start checks that every phase has a resolver, and the kernel
// ships none -- which is the correct behaviour after the split, so a test
// that wants to advance phases has to supply them itself.
func withNoopResolvers() []EngineOption {
	cfg := testConfig()
	opts := make([]EngineOption, 0, len(cfg.Phases))
	for phase := range cfg.Phases {
		opts = append(opts, WithResolver(phase, noopResolver{}))
	}
	return opts
}

// setRoundVar / markRound lay out round state directly, for unit tests that
// do not go through a resolver.
func setRoundVar(s *gameState, key, value string) {
	s.applyEffect(NewSetVarEffect(ScopeRound, key, value))
}

func markRound(s *gameState, playerID, key string) {
	s.applyEffect(NewSetVarEffect(ScopeRound.Of(playerID), key, VarPresent))
}

func roundVarOfState(s *gameState, key string) string { return s.varOf(ScopeRound, key) }

func markedIn(s *gameState, playerID, key string) bool {
	return s.varOf(ScopeRound.Of(playerID), key) != ""
}

func markedInA(s *gameState, id string) bool { return markedIn(s, id, testMarkA) }
func markedInB(s *gameState, id string) bool { return markedIn(s, id, testMarkB) }

func killTargetOfState(s *gameState) string { return roundVarOfState(s, testKillTarget) }

// sameVars reports whether two sets of custom state are identical.
func sameVars(a, b map[string]string) bool { return maps.Equal(a, b) }

// newViewGame is a started engine with empty resolvers, for the view and
// audience tests.
func newViewGame(t *testing.T) *Engine {
	t.Helper()
	e := newTestEngine(t, withNoopResolvers()...)
	for id, role := range map[string]RoleType{
		"w1": roleWerewolf, "w2": roleWerewolf, "s": roleSeer,
		"wi": roleWitch, "g": roleGuard,
		"v1": roleVillager, "v2": roleVillager, "v3": roleVillager,
	} {
		mustAdd(t, e, id, role)
	}
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return e
}
