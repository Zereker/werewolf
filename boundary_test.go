package werewolf

import (
	"github.com/Zereker/werewolf/engine"
	"testing"
)

// 本文件验证信息边界的三个问题都由规则回答，内核只守一条底线：
// 自己的状态原语永远不外发。
//
// 「都由规则回答」不是口号——三个 provider 每一个换掉之后，行为都要
// 真的跟着变。换不动的那一个，就说明内核里还藏着一条写死的判定。

func newBoundaryGame(t *testing.T, opts ...EngineOption) *Engine {
	t.Helper()
	g := newRuleGameWith(t, nil, opts, seats(
		wolf("w1"), wolf("w2"), seer("s"), witch("wi"), guard("g"),
		villagers("v1", "v2", "v3"),
	)...)
	return g.e
}

// TestWithAudience_Replaceable 可见性划分可以整个换掉。
//
// 默认「查验只给预言家」是狼人杀的规矩。换一套规则——比如一个查验结果
// 全场公开的变体——不该需要改引擎。
func TestWithAudience_Replaceable(t *testing.T) {
	check := engine.NewEffect(EventCheck, "s", "w1").ToEvent()

	t.Run("默认只给行动者", func(t *testing.T) {
		e := newBoundaryGame(t)
		got, known := e.AudienceOf(check)
		if !known || len(got) != 1 || got[0] != "s" {
			t.Fatalf("查验默认只给预言家，实际 known=%v got=%v", known, got)
		}
	})

	t.Run("换掉之后全场可见", func(t *testing.T) {
		e := newBoundaryGame(t, engine.WithAudience(engine.AudienceFunc(
			func(ev *engine.Event, view GameView) ([]string, bool) {
				if ev.Type != EventCheck {
					return wolfAudience(ev, view)
				}
				return allPlayerIDs(view), true
			})))

		got, known := e.AudienceOf(check)
		if !known || len(got) != 8 {
			t.Fatalf("换掉之后查验应当全场可见，实际 known=%v got=%v", known, got)
		}
	})
}

// TestAudienceOf_KernelPrimitivesAreNeverPublic 状态原语不外发，这一条不可配置。
//
// 这是内核在信息边界上唯一保留的判断，也是它必须保留的那一个：原语是
// 状态机的记账（谁的存活位翻了、谁身上多了个标记），把它推给玩家等于
// 把上帝视角直接发出去。一个恶意或粗心的 provider 不该有能力打开这个口子。
func TestAudienceOf_KernelPrimitivesAreNeverPublic(t *testing.T) {
	// 一个「什么都给全场」的 provider
	e := newBoundaryGame(t, engine.WithAudience(engine.AudienceFunc(
		func(ev *engine.Event, view GameView) ([]string, bool) {
			return allPlayerIDs(view), true
		})))

	for _, ef := range []*Effect{
		engine.NewSetAliveEffect("v1", false),
		engine.NewSetPlayerVarEffect("wi", VarWitchAntidote, ""),
		engine.NewSetPlayerRoundVarEffect("v1", PlayerRoundVarProtected, VarPresent),
		engine.NewSetRoundVarEffect(RoundVarKillTarget, "v1"),
		engine.NewAbilityTriggerEffect("h", PhaseNightHunter),
	} {
		got, known := e.AudienceOf(ef.ToEvent())
		if !known {
			t.Errorf("%v 应当是明确的判定，不是「不知道」", ef.Type)
		}
		if len(got) != 0 {
			t.Errorf("%v 是内核的状态原语，不该发给任何人，实际 %v", ef.Type, got)
		}
	}
}

// TestWithTeammates_Replaceable 「谁和谁是一边的」可以换掉，而且允许不对称。
//
// 不对称是刻意支持的：血染钟楼的恶魔认得爪牙，爪牙不知道恶魔是谁。
// 内核不检查两边是否一致——它根本不知道「阵营」这个概念。
func TestWithTeammates_Replaceable(t *testing.T) {
	// w1 认得 w2，w2 谁都不认得
	oneWay := engine.WithTeammates(engine.TeammateFunc(
		func(playerID string, view GameView) []string {
			if playerID == "w1" {
				return []string{"w2"}
			}
			return nil
		}))

	e := newBoundaryGame(t, oneWay)

	if got := e.PlayerView("w1").Teammates; len(got) != 1 || got[0] != "w2" {
		t.Fatalf("w1 应当认得 w2，实际 %v", got)
	}
	if got := e.PlayerView("w2").Teammates; len(got) != 0 {
		t.Fatalf("w2 不该认得任何人，实际 %v", got)
	}

	// 身份可见性跟着走：w1 看得到 w2 的身份，反过来不行
	if role := revealedRole(e.PlayerView("w1"), "w2"); role != RoleWerewolf {
		t.Errorf("w1 应当看得到 w2 的身份，实际 %v", role)
	}
	if role := revealedRole(e.PlayerView("w2"), "w1"); role != engine.RoleUnspecified {
		t.Errorf("w2 不该看得到 w1 的身份，实际 %v", role)
	}

	// WolfTeammates 与 PhaseInfo 走的是同一个判定，不该各说各话
	if got := e.Teammates("w2"); len(got) != 0 {
		t.Errorf("WolfTeammates 应与 engine.PlayerView 一致，实际 %v", got)
	}
	e2 := newBoundaryGame(t, oneWay)
	for e2.Phase() != PhaseNightWolf {
		if _, err := e2.EndPhase(); err != nil {
			t.Fatal(err)
		}
	}
	ri, ok := e2.PhaseInfo().RoleInfos[RoleWerewolf]
	if !ok {
		t.Fatal("狼人阶段应当有狼人的阶段信息")
	}
	if got := ri.Teammates["w2"]; len(got) != 0 {
		t.Errorf("engine.PhaseInfo 应与 engine.PlayerView 一致，实际 w2 的队友是 %v", got)
	}
	if got := ri.Teammates["w1"]; len(got) != 1 || got[0] != "w2" {
		t.Errorf("engine.PhaseInfo 里 w1 的队友应当是 [w2]，实际 %v", got)
	}
}

// TestWithSpeech_Replaceable 发言的可听范围可以换掉。
func TestWithSpeech_Replaceable(t *testing.T) {
	t.Run("默认夜里只有狼队交流", func(t *testing.T) {
		e := newBoundaryGame(t)
		for e.Phase() != PhaseNightWolf {
			if _, err := e.EndPhase(); err != nil {
				t.Fatal(err)
			}
		}
		if got := e.MessageReceivers("w1"); len(got) != 2 {
			t.Errorf("狼人阶段狼队内部可听，实际 %v", got)
		}
		if got := e.MessageReceivers("v1"); len(got) != 0 {
			t.Errorf("狼人阶段平民说不了话，实际 %v", got)
		}
	})

	t.Run("换成全程公开", func(t *testing.T) {
		e := newBoundaryGame(t, engine.WithSpeech(engine.SpeechFunc(
			func(senderID string, view GameView) []string {
				out := make([]string, 0)
				for _, p := range view.AlivePlayers() {
					out = append(out, p.ID)
				}
				return out
			})))
		for e.Phase() != PhaseNightWolf {
			if _, err := e.EndPhase(); err != nil {
				t.Fatal(err)
			}
		}
		if got := e.MessageReceivers("v1"); len(got) != 8 {
			t.Errorf("换掉之后平民夜里也能说话且全场可听，实际 %v", got)
		}
	})
}

// TestBoundaryProviders_NilRejected 三个注册入口与其余选项一致，拒绝 nil。
func TestBoundaryProviders_NilRejected(t *testing.T) {
	for name, opt := range map[string]EngineOption{
		"audience":  engine.WithAudience(nil),
		"teammates": engine.WithTeammates(nil),
		"speech":    engine.WithSpeech(nil),
	} {
		if _, err := engine.NewEngine(DefaultGameConfig(), opt); err == nil {
			t.Errorf("With%s(nil) 应当报错", name)
		} else if code := engine.CodeOf(err); code != engine.CodeInvalidConfig {
			t.Errorf("With%s(nil): 期望 engine.CodeInvalidConfig，实际 %v", name, code)
		}
	}
}

// revealedRole 在某人的视角里，另一名玩家的身份是否公开。
func revealedRole(v *engine.PlayerView, id string) RoleType {
	for _, p := range v.Players {
		if p.ID == id {
			return p.Role
		}
	}
	return engine.RoleUnspecified
}

// TestBareEngine_KnowsNothing 只装了内核的引擎一无所知。
//
// 拆包之后，「内核不认识狼人杀」这件事由**编译器**保证：内核包里连
// DefaultVictoryChecker 这个名字都写不出来。所以这条测试不再去数字段，
// 改成验可观察的行为——那才是使用者会撞到的东西。
//
// 用的全是公开 API：没有 e.victory、没有 e.phase.resolvers。规则包本来
// 就只能这么看内核，测试也该站在同一个位置。
func TestBareEngine_KnowsNothing(t *testing.T) {
	seat := func(t *testing.T, e *Engine) {
		t.Helper()
		for id, role := range map[string]RoleType{
			"w1": RoleWerewolf, "w2": RoleWerewolf,
			"s": RoleSeer, "v1": RoleVillager,
		} {
			if err := e.AddPlayer(id, role); err != nil {
				t.Fatal(err)
			}
		}
	}

	t.Run("阶段缺解析器时开局被拒", func(t *testing.T) {
		// 内核不自带任何解析器，因此狼人杀那副阶段图对它是不完整的——
		// 这一条在 Start 时报出来，而不是让游戏推进到一半静默停住
		bare := engine.MustNewEngine(DefaultGameConfig())
		seat(t, bare)
		if err := bare.Start(); err == nil {
			t.Fatal("阶段没有解析器时开局应当被拒")
		}
	})

	// 补上空解析器之后能推进，但内核对这局游戏依然一无所知
	bare := engine.MustNewEngine(DefaultGameConfig(), noopResolvers()...)
	seat(t, bare)
	if err := bare.Start(); err != nil {
		t.Fatalf("补上解析器后应当能开局: %v", err)
	}

	t.Run("不会判出胜负", func(t *testing.T) {
		// 一副按狼人杀规则早就该结束的局面：好人全死光
		bare.Apply(engine.NewSetAliveEffect("s", false), engine.NewSetAliveEffect("v1", false))
		for i := 0; i < 30; i++ {
			if _, err := bare.EndPhase(); err != nil {
				t.Fatalf("EndPhase: %v", err)
			}
		}
		if bare.IsGameOver() {
			t.Error("内核不知道什么叫赢，这局不该结束")
		}
	})

	t.Run("不认得任何角色", func(t *testing.T) {
		// 女巫没有药、狼人没有阵营——初始状态由规则的 RoleSetup 发
		wi := engine.MustNewEngine(DefaultGameConfig(), noopResolvers()...)
		if err := wi.AddPlayer("wi", RoleWitch); err != nil {
			t.Fatal(err)
		}
		p, _ := wi.PlayerInfo("wi")
		if len(p.Vars) != 0 {
			t.Errorf("内核入座不该发放任何状态，实际 %v", p.Vars)
		}
	})

	t.Run("不划分信息边界", func(t *testing.T) {
		if got := bare.PlayerView("w1").Teammates; len(got) != 0 {
			t.Errorf("内核不知道谁和谁是一边的，实际 %v", got)
		}
		if _, known := bare.AudienceOf(engine.NewEffect(EventKill, "", "v1").ToEvent()); known {
			t.Error("内核不该声称认得 KILL 这个事件")
		}
		if got := bare.MessageReceivers("w1"); len(got) != 0 {
			t.Errorf("内核不知道谁能听到谁说话，实际 %v", got)
		}
	})
}

// noopResolvers 给默认板子的每个阶段装一个空解析器。
//
// 内核的 Start 会校验「每个阶段都有解析器」，而内核自己不带任何解析器。
// 想拿一台裸引擎推进阶段，就得自己补上——这正是拆分之后的正确行为。
func noopResolvers() []EngineOption {
	cfg := DefaultGameConfig()
	opts := make([]EngineOption, 0, len(cfg.Phases))
	for phase := range cfg.Phases {
		opts = append(opts, engine.WithResolver(phase, noopResolver{}))
	}
	return opts
}

type noopResolver struct{}

func (noopResolver) Resolve([]*SkillUse, GameView) []*Effect { return nil }

// TestWerewolfOptions_GoThroughThePublicDoor 狼人杀那一套全部走公开选项。
//
// 与上一条对照：同一副阶段图，装上 Options 之后每一样都工作了。
// 中间没有第二条路径——werewolf 包在内核之外，它能用的入口使用者也能用。
func TestWerewolfOptions_GoThroughThePublicDoor(t *testing.T) {
	e := MustNew(DefaultRules())
	for id, role := range map[string]RoleType{
		"w1": RoleWerewolf, "w2": RoleWerewolf,
		"wi": RoleWitch, "s": RoleSeer,
		"v1": RoleVillager, "v2": RoleVillager,
	} {
		if err := e.AddPlayer(id, role); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.Start(); err != nil {
		t.Fatalf("装上规则之后应当能开局: %v", err)
	}

	if p, _ := e.PlayerInfo("wi"); p.Var(VarWitchAntidote) == "" {
		t.Error("女巫应当带着解药入座")
	}
	if got := e.Teammates("w1"); len(got) != 1 || got[0] != "w2" {
		t.Errorf("狼队应当互相认得，实际 %v", got)
	}
	if _, known := e.AudienceOf(engine.NewEffect(EventKill, "", "v1").ToEvent()); !known {
		t.Error("装上规则之后应当认得 KILL")
	}

	// 好人全死光 -> 狼人胜
	e.Apply(engine.NewSetAliveEffect("wi", false), engine.NewSetAliveEffect("s", false),
		engine.NewSetAliveEffect("v1", false), engine.NewSetAliveEffect("v2", false))
	for i := 0; i < 30 && !e.IsGameOver(); i++ {
		if _, err := e.EndPhase(); err != nil {
			t.Fatalf("EndPhase: %v", err)
		}
	}
	if !e.IsGameOver() {
		t.Error("好人全灭，装上规则之后应当判出胜负")
	}
}
