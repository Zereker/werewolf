package engine

import "testing"

// TestExtensionPoints_AllHaveFuncAdapters 八个扩展点必须都能用一个普通函数装上。
//
// 此前只有 Resolver 与 VictoryChecker 没有这层适配器——没有理由，只是历史，
// 于是「装一个只有几行的解析器」得先声明一个空结构体。这个测试不检查名字，
// 它直接把八个函数字面量装进一台引擎：少一个适配器就编译不过。
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

// TestResolverFunc_IsCalled 装上去的函数真的被调用，产出真的落进状态。
//
// 适配器本身只有一行，但一行也能写反（调用了别的、把返回值丢了）。
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
		t.Error("ResolverFunc 装上了却没被调用")
	}
	if got := e.Var(ScopeGame, "probe"); got != "1" {
		t.Errorf("ResolverFunc 的产出没有落进状态，读到 %q", got)
	}
}

// TestVictoryFunc_IsCalled 装上去的判定真的被问到，结论真的被采纳。
func TestVictoryFunc_IsCalled(t *testing.T) {
	called := 0
	opts := append(withNoopResolvers(),
		WithVictoryChecker(VictoryFunc(func(GameView) (bool, Camp) {
			called++
			// 开局那一次必须说「还没结束」——否则引擎会拒绝开局
			// （「board is already decided before the game starts」）。
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
		t.Fatal("VictoryFunc 装上了却没被问到")
	}
	if !e.Status().Over {
		t.Error("VictoryFunc 说结束了，引擎却没结束")
	}
	if got := e.Status().Winner; got != Camp("PROBE") {
		t.Errorf("胜者应当原样报出去，得到 %v", got)
	}
}
