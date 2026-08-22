package werewolf

import (
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
	check := NewEffect(EventCheck, "s", "w1").ToEvent()

	t.Run("默认只给行动者", func(t *testing.T) {
		e := newBoundaryGame(t)
		got, known := e.AudienceOf(check)
		if !known || len(got) != 1 || got[0] != "s" {
			t.Fatalf("查验默认只给预言家，实际 known=%v got=%v", known, got)
		}
	})

	t.Run("换掉之后全场可见", func(t *testing.T) {
		e := newBoundaryGame(t, WithAudience(AudienceFunc(
			func(ev *Event, view GameView) ([]string, bool) {
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
	e := newBoundaryGame(t, WithAudience(AudienceFunc(
		func(ev *Event, view GameView) ([]string, bool) {
			return allPlayerIDs(view), true
		})))

	for _, ef := range []*Effect{
		NewSetAliveEffect("v1", false),
		NewSetPlayerVarEffect("wi", VarWitchAntidote, ""),
		NewSetPlayerRoundVarEffect("v1", PlayerRoundVarProtected, VarPresent),
		NewSetRoundVarEffect(RoundVarKillTarget, "v1"),
		NewAbilityTriggerEffect("h", PhaseNightHunter),
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
	oneWay := WithTeammates(TeammateFunc(
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
	if role := revealedRole(e.PlayerView("w2"), "w1"); role != RoleUnspecified {
		t.Errorf("w2 不该看得到 w1 的身份，实际 %v", role)
	}

	// WolfTeammates 与 PhaseInfo 走的是同一个判定，不该各说各话
	if got := e.WolfTeammates("w2"); len(got) != 0 {
		t.Errorf("WolfTeammates 应与 PlayerView 一致，实际 %v", got)
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
		t.Errorf("PhaseInfo 应与 PlayerView 一致，实际 w2 的队友是 %v", got)
	}
	if got := ri.Teammates["w1"]; len(got) != 1 || got[0] != "w2" {
		t.Errorf("PhaseInfo 里 w1 的队友应当是 [w2]，实际 %v", got)
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
		e := newBoundaryGame(t, WithSpeech(SpeechFunc(
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
		"audience":  WithAudience(nil),
		"teammates": WithTeammates(nil),
		"speech":    WithSpeech(nil),
	} {
		if _, err := NewEngine(DefaultGameConfig(), opt); err == nil {
			t.Errorf("With%s(nil) 应当报错", name)
		} else if code := CodeOf(err); code != CodeInvalidConfig {
			t.Errorf("With%s(nil): 期望 CodeInvalidConfig，实际 %v", name, code)
		}
	}
}

// revealedRole 在某人的视角里，另一名玩家的身份是否公开。
func revealedRole(v *PlayerView, id string) RoleType {
	for _, p := range v.Players {
		if p.ID == id {
			return p.Role
		}
	}
	return RoleUnspecified
}

// TestBareEngine_KnowsNothing 只装了内核的引擎一无所知。
//
// 这是拆分的核心断言，也是「狼人杀只是第一个规则包」这句话的可执行说法：
// NewEngine 造出来的东西没有解析器、不会判出胜负、不认得任何角色的初始状态、
// 不知道谁和谁是一边的、不知道谁能听到谁说话。狼人杀的一切由 werewolf.Options
// 经公开选项装上去——它自己也没有走任何后门。
//
// 这条测试的价值在变异下才看得出来：把内核的缺省胜负判定换回
// DefaultVictoryChecker，别的测试一条都不会红（默认规则下行为一样），
// 只有它会。
func TestBareEngine_KnowsNothing(t *testing.T) {
	e, err := NewEngine(DefaultGameConfig())
	if err != nil {
		t.Fatalf("内核构造不该失败: %v", err)
	}

	t.Run("没有解析器", func(t *testing.T) {
		if got := len(e.phase.resolvers); got != 0 {
			t.Errorf("内核不该自带解析器，实际 %d 个", got)
		}
	})

	t.Run("不会判出胜负", func(t *testing.T) {
		// 一副只有狼、按狼人杀规则早就该结束的板子
		bare := MustNewEngine(DefaultGameConfig())
		for id, role := range map[string]RoleType{
			"w1": RoleWerewolf, "w2": RoleWerewolf,
		} {
			if err := bare.AddPlayer(id, role); err != nil {
				t.Fatal(err)
			}
		}
		if over, winner := bare.victory.CheckVictory(newStateView(bare.state)); over {
			t.Errorf("内核不知道什么叫赢，实际判出 winner=%v", winner)
		}
	})

	t.Run("阶段缺解析器时开局被拒", func(t *testing.T) {
		// 内核不自带解析器，因此默认阶段配置对它是不完整的——
		// 这一条会在 Start 时报出来，而不是让游戏推进到一半静默停住
		bare := MustNewEngine(DefaultGameConfig())
		if err := bare.AddPlayer("w1", RoleWerewolf); err != nil {
			t.Fatal(err)
		}
		if err := bare.Start(); err == nil {
			t.Error("阶段没有解析器时开局应当被拒")
		}
	})

	t.Run("不认得任何角色", func(t *testing.T) {
		if got := len(e.roleSetup); got != 0 {
			t.Errorf("内核不该自带角色初始状态，实际 %d 项", got)
		}
		if got := len(e.roleInfo); got != 0 {
			t.Errorf("内核不该自带角色专属信息，实际 %d 项", got)
		}
	})

	t.Run("不划分信息边界", func(t *testing.T) {
		if e.audience != nil {
			t.Error("内核不该自带受众划分")
		}
		if e.teammates != nil {
			t.Error("内核不该自带队友判定")
		}
		if e.speech != nil {
			t.Error("内核不该自带发言范围")
		}
	})
}

// TestWerewolfOptions_GoThroughThePublicDoor 狼人杀那一套全部走公开选项。
//
// 逐项比对：装上 Options 之后有的东西，正是内核缺的那些。中间没有第二条路径。
func TestWerewolfOptions_GoThroughThePublicDoor(t *testing.T) {
	bare := MustNewEngine(DefaultGameConfig())
	full := MustNew(DefaultRules())

	if len(bare.phase.resolvers) != 0 || len(full.phase.resolvers) != 9 {
		t.Errorf("解析器: 内核 %d 个，装上规则后 %d 个",
			len(bare.phase.resolvers), len(full.phase.resolvers))
	}
	if len(bare.roleSetup) != 0 || len(full.roleSetup) != 6 {
		t.Errorf("角色初始状态: 内核 %d 项，装上规则后 %d 项",
			len(bare.roleSetup), len(full.roleSetup))
	}
	if _, ok := full.victory.(DefaultVictoryChecker); !ok {
		t.Errorf("装上规则后应当是狼人杀的判定，实际 %T", full.victory)
	}
	if _, ok := bare.victory.(neverEnds); !ok {
		t.Errorf("内核的缺省应当是「永不结束」，实际 %T", bare.victory)
	}
	if full.audience == nil || full.teammates == nil || full.speech == nil {
		t.Error("装上规则后三个 provider 都该有")
	}
}
