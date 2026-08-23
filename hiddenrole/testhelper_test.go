package hiddenrole

import (
	"maps"
	"testing"
)

// 内核测试用的辅助。狼人杀那一层的同名辅助留在根包——它们认得
// 「刀口」「被守」这些词，内核不认得。

// mustAddTo 直接向状态添加玩家，失败即终止。
func mustAddTo(t *testing.T, s *gameState, id string, role RoleType) {
	t.Helper()
	if err := s.addPlayer(id, role); err != nil {
		t.Fatalf("addPlayer(%q, %v): %v", id, role, err)
	}
}

// mustAdd 经引擎添加玩家，失败即终止。
func mustAdd(t *testing.T, e *Engine, id string, role RoleType) {
	t.Helper()
	if err := e.AddPlayer(id, role); err != nil {
		t.Fatalf("AddPlayer(%q, %v): %v", id, role, err)
	}
}

// newTestEngine 造一台装了 testConfig 的引擎。
//
// 它不带任何解析器——内核本来就不自带。需要能推进阶段的引擎时，
// 用 withNoopResolvers。
func newTestEngine(t *testing.T, opts ...EngineOption) *Engine {
	t.Helper()
	e, err := NewEngine(testConfig(), opts...)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	return e
}

// noopResolver 什么都不做的解析器，用于让阶段能推进。
type noopResolver struct{}

func (noopResolver) Resolve([]*SkillUse, GameView) []*Effect { return nil }

// withNoopResolvers 给 testConfig 里每个阶段都装一个空解析器。
//
// 内核的 Start 会校验「每个阶段都有解析器」，而内核自己不带任何解析器——
// 这正是拆分之后的正确行为，测试要推进阶段就得自己补上。
func withNoopResolvers() []EngineOption {
	cfg := testConfig()
	opts := make([]EngineOption, 0, len(cfg.Phases))
	for phase := range cfg.Phases {
		opts = append(opts, WithResolver(phase, noopResolver{}))
	}
	return opts
}

// setRoundVar / markRound 直接铺回合状态，供不经解析器的单元测试用。
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

// sameVars 比较两份自定义状态是否完全一致。
func sameVars(a, b map[string]string) bool { return maps.Equal(a, b) }

// newViewGame 一台开好局、装了空解析器的引擎，供视图与受众的测试用。
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
