package hiddenrole

import "testing"

// TestExtensionPoints_AllHaveFuncAdapters: all eight extension points must be
// installable with a plain function.
//
// Resolver and VictoryChecker used to be the only two without such an adapter
// -- no reason, just history -- which meant installing a three-line resolver
// first required declaring an empty struct. This test checks no names; it
// installs eight function literals into one engine, so a missing adapter
// fails to compile.
func TestExtensionPoints_AllHaveFuncAdapters(t *testing.T) {
	cfg := testConfig()
	opts := []EngineOption{
		WithVictoryChecker(VictoryFunc(func(GameView) (bool, Camp) {
			return false, CampUnspecified
		})),
		WithAudience(AudienceFunc(func(*Event, GameView) ([]string, bool) {
			return nil, false
		})),
		WithTeammates(TeammateFunc(func(string, GameView) []string { return nil })),
		WithSpeech(SpeechFunc(func(string, GameView) []string { return nil })),
		WithRoleInfo(roleVillager, RoleInfoFunc(func(string, GameView) map[string]string {
			return nil
		})),
		WithRoleSetup(roleVillager, RoleSetupFunc(func(string, RoleType) map[string]string {
			return nil
		})),
		WithGameSetup(GameSetupFunc(func(GameView) []*Effect { return nil })),
	}
	for phase := range cfg.Phases {
		opts = append(opts, WithResolver(phase, ResolverFunc(
			func([]*SkillUse, GameView) []*Effect { return nil })))
	}

	e, err := NewEngine(cfg, opts...)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	mustAdd(t, e, "p1", roleVillager)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
}

// TestResolverFunc_IsCalled: the installed function really is called, and
// what it produces really lands in the state.
//
// The adapter itself is one line, but one line can still be written backwards
// (calling the wrong thing, dropping the return value).
func TestResolverFunc_IsCalled(t *testing.T) {
	called := 0
	cfg := testConfig()
	opts := []EngineOption{
		WithResolver(phaseNightGuard, ResolverFunc(func(uses []*SkillUse, view GameView) []*Effect {
			called++
			return []*Effect{NewSetVarEffect(ScopeGame, "probe", "1")}
		})),
	}
	for phase := range cfg.Phases {
		if phase == phaseNightGuard {
			continue
		}
		opts = append(opts, WithResolver(phase, noopResolver{}))
	}

	e, err := NewEngine(cfg, opts...)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	mustAdd(t, e, "p1", roleVillager)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := e.EndPhase(); err != nil {
		t.Fatalf("EndPhase: %v", err)
	}

	if called == 0 {
		t.Error("ResolverFunc was installed but never called")
	}
	if got := e.Var(ScopeGame, "probe"); got != "1" {
		t.Errorf("ResolverFunc's output did not land in the state, read %q", got)
	}
}

// TestVictoryFunc_IsCalled: the installed checker really is asked, and its
// verdict really is taken.
func TestVictoryFunc_IsCalled(t *testing.T) {
	called := 0
	opts := append(withNoopResolvers(),
		WithVictoryChecker(VictoryFunc(func(GameView) (bool, Camp) {
			called++
			// The first call, at the start, has to say "not decided" -- or the
			// engine refuses to start ("board is already decided before the
			// game starts").
			return called > 1, Camp("PROBE")
		})))

	e := newTestEngine(t, opts...)
	mustAdd(t, e, "p1", roleVillager)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := e.EndPhase(); err != nil {
		t.Fatalf("EndPhase: %v", err)
	}

	if called == 0 {
		t.Fatal("VictoryFunc was installed but never asked")
	}
	if !e.Status().Over {
		t.Error("VictoryFunc said the game was over, the engine disagreed")
	}
	if got := e.Status().Winner; got != Camp("PROBE") {
		t.Errorf("the winner should be reported verbatim, got %v", got)
	}
}
