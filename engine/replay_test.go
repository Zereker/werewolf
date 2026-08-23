package engine

import (
	"encoding/json"
	"testing"
)

// replay_test.go 效果流回放与宿主级写入，由**内核自己**验。
//
// 这一批测试补的是一个真实的窟窿：拆包之后，`ReplayEngine` 在内核自己的
// 测试里覆盖率是 **0%**——它整条路径只被下游的规则包驱动过。
//
// 这一轮修掉的三个回放 bug（不恢复赢家、结束那一步不消费行动者名单、
// 不消费绕道队列）全都是规则包的随机对局抓到的。内核的正确性由下游证明，
// 那是脆的：哪天规则包不跑了，这条路就没人守了。
//
// 下面每一条都只用内核自己的词汇表（见 vocab_test.go），不认识任何游戏。

// scoreResolver 在指定阶段写一笔整局状态，并点名下一阶段的行动者。
//
// 用它造出一条**内容够丰富**的效果流：状态变更、行动者名单、规则自己的
// 事件三样都有——只有全都有，回放才验得出「重建出来的是同一个局面」。
type scoreResolver struct {
	phase PhaseType
	key   string
	value string
	names []string
}

func (r scoreResolver) Resolve(_ []*SkillUse, _ GameView) []*Effect {
	out := []*Effect{
		NewEffect(EventType("SCORED"), "", "").WithData("v", r.value),
		NewSetVarEffect(ScopeGame, r.key, r.value),
	}
	if len(r.names) > 0 {
		out = append(out, NewSetActorsEffect(r.phase, r.names...))
	}
	return out
}

// replayFixture 造一台跑过几步的引擎，外加重建它所需的配置与选项。
func replayFixture(t *testing.T) (*Engine, *Config, []EngineOption) {
	t.Helper()

	cfg := testConfig()
	opts := append(withNoopResolvers(),
		WithResolver(phaseNightGuard, scoreResolver{
			phase: phaseNightWolf, key: "probe.score", value: "7",
			names: []string{"w1"},
		}))

	e, err := NewEngine(cfg, opts...)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	mustAdd(t, e, "w1", roleWerewolf)
	mustAdd(t, e, "w2", roleWerewolf)
	mustAdd(t, e, "v", roleVillager)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := e.EndPhase(); err != nil { // NIGHT_GUARD -> NIGHT_WOLF
		t.Fatalf("EndPhase: %v", err)
	}
	return e, cfg, opts
}

// TestReplayEngine_RebuildsTheSameBoard 回放出来的必须是同一个局面。
//
// 逐字节比快照，不是只比阶段与回合：快照漏掉一个字段，两边照样能同步地
// 走完一整局，只是规则判定不一样了。
func TestReplayEngine_RebuildsTheSameBoard(t *testing.T) {
	e, cfg, opts := replayFixture(t)

	replayed, err := ReplayEngine(cfg, e.EffectLog(), opts...)
	if err != nil {
		t.Fatalf("ReplayEngine: %v", err)
	}

	if got, want := replayed.Status(), e.Status(); got != want {
		t.Errorf("回放后 Status = %+v，原局 %+v", got, want)
	}
	a, _ := json.Marshal(e.Snapshot())
	b, _ := json.Marshal(replayed.Snapshot())
	if string(a) != string(b) {
		t.Errorf("回放后局面不一致:\n  原   %s\n  回放 %s", a, b)
	}
}

// TestReplayEngine_CarriesStateActorsAndBehaviour 状态、行动者名单、行为三样都要跟着走。
//
// 快照字节一样不等于行为一样——**行动者名单**是这条的重点：它决定「谁能
// 行动」，漏了它，回放出来的引擎会对所有人说「你可以动」。
func TestReplayEngine_CarriesStateActorsAndBehaviour(t *testing.T) {
	e, cfg, opts := replayFixture(t)

	replayed, err := ReplayEngine(cfg, e.EffectLog(), opts...)
	if err != nil {
		t.Fatalf("ReplayEngine: %v", err)
	}

	if got := replayed.Var(ScopeGame, "probe.score"); got != "7" {
		t.Errorf("整局状态没跟着走，读到 %q", got)
	}
	// 上一阶段点了名：只有 w1 能在狼人阶段行动，w2 不能。
	for _, id := range []string{"w1", "w2", "v"} {
		x, y := len(e.AllowedSkills(id)), len(replayed.AllowedSkills(id))
		if x != y {
			t.Errorf("%s 能做的事不一样：原 %d，回放 %d", id, x, y)
		}
	}
	if len(replayed.AllowedSkills("w2")) != 0 {
		t.Error("名单里只点了 w1，w2 不该能行动——行动者名单没跟着回放走")
	}
}

// endedGame 造一台已经结束的引擎。
func endedGame(t *testing.T) (*Engine, *Config, []EngineOption) {
	t.Helper()
	cfg := testConfig()
	opts := append(withNoopResolvers(),
		WithVictoryChecker(VictoryFunc(func(view GameView) (bool, Camp) {
			return view.Round() > 1, campEvil
		})))
	e, err := NewEngine(cfg, opts...)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	mustAdd(t, e, "w1", roleWerewolf)
	mustAdd(t, e, "v", roleVillager)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	for i := 0; i < 30 && !e.Status().Over; i++ {
		if _, err := e.EndPhase(); err != nil {
			t.Fatalf("EndPhase: %v", err)
		}
	}
	if !e.Status().Over {
		t.Fatal("这个测试要一局已经结束的对局")
	}
	return e, cfg, opts
}

// TestReplayEngine_CarriesTheWinner 已经结束的一局，回放出来必须还知道谁赢了。
//
// **这是一个真出过的 bug。** 谁赢是结束那一刻由 VictoryChecker 定下的、
// 此后不再变，而回放不会再跑一次判定——GAME_ENDED 效果里带着赢家，
// 只是此前没人读。回放出来是 Over=true 而 Winner 为空，与原局分叉。
func TestReplayEngine_CarriesTheWinner(t *testing.T) {
	e, cfg, opts := endedGame(t)

	replayed, err := ReplayEngine(cfg, e.EffectLog(), opts...)
	if err != nil {
		t.Fatalf("ReplayEngine: %v", err)
	}
	if got, want := replayed.Status(), e.Status(); got != want {
		t.Errorf("回放后 Status = %+v，原局 %+v", got, want)
	}
	if replayed.Status().Winner == CampUnspecified {
		t.Error("赢家没跟着效果流走")
	}
}

// TestReplayEngine_RejectsABrokenLog 坏掉的效果流要被拒绝，不能悄悄重建出半局游戏。
func TestReplayEngine_RejectsABrokenLog(t *testing.T) {
	e, cfg, opts := replayFixture(t)
	log := e.EffectLog()

	t.Run("nil 条目", func(t *testing.T) {
		broken := append([]*Effect{}, log...)
		broken = append(broken, nil)
		if _, err := ReplayEngine(cfg, broken, opts...); !HasCode(err, CodeInvalidEffectLog) {
			t.Errorf("应当拒成 %v，实际 %v", CodeInvalidEffectLog, CodeOf(err))
		}
	})

	t.Run("PHASE_CHANGED 不带阶段", func(t *testing.T) {
		broken := append([]*Effect{}, log...)
		broken = append(broken, NewEffect(EventPhaseChanged, "", ""))
		if _, err := ReplayEngine(cfg, broken, opts...); !HasCode(err, CodeInvalidEffectLog) {
			t.Errorf("应当拒成 %v，实际 %v", CodeInvalidEffectLog, CodeOf(err))
		}
	})

	t.Run("配置本身不合法", func(t *testing.T) {
		if _, err := ReplayEngine(&Config{}, log, opts...); err == nil {
			t.Error("配置不合法时不该重建出引擎")
		}
	})
}

// TestReplayEngine_LogIsPreserved 回放出来的引擎带着同一份历史。
//
// 否则「回放之后再存一次档」得到的东西与原局不同，链式回放会越走越偏。
func TestReplayEngine_LogIsPreserved(t *testing.T) {
	e, cfg, opts := replayFixture(t)

	once, err := ReplayEngine(cfg, e.EffectLog(), opts...)
	if err != nil {
		t.Fatalf("ReplayEngine: %v", err)
	}
	twice, err := ReplayEngine(cfg, once.EffectLog(), opts...)
	if err != nil {
		t.Fatalf("再回放一次: %v", err)
	}

	if len(once.EffectLog()) != len(e.EffectLog()) {
		t.Errorf("回放后历史长度 %d，原局 %d", len(once.EffectLog()), len(e.EffectLog()))
	}
	a, _ := json.Marshal(e.Snapshot())
	b, _ := json.Marshal(twice.Snapshot())
	if string(a) != string(b) {
		t.Errorf("回放两次之后与原局分叉:\n  原 %s\n  两次 %s", a, b)
	}
}

// TestApply_GoesThroughTheSameWritePoint 宿主级写入走的是同一个写入点。
//
// Engine.Apply 绕开阶段结算，是一把有刃的工具——宿主真的会遇到「玩家掉线
// 判死」「管理员踢人」。它的价值全在「**仍然是同一个写入点**」这一句上：
// 效果进历史、被否决的不生效、内核原语不外发。这三条此前在内核自己的
// 测试里一条都没验过（Apply 覆盖率 0%）。
func TestApply_GoesThroughTheSameWritePoint(t *testing.T) {
	e := newTestEngine(t, withNoopResolvers()...)
	mustAdd(t, e, "v", roleVillager)
	mustAdd(t, e, "w", roleWerewolf)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	before := len(e.EffectLog())

	t.Run("效果真的生效", func(t *testing.T) {
		e.Apply(NewSetAliveEffect("v", false))
		if p, _ := e.PlayerInfo("v"); p.Alive {
			t.Error("SET_ALIVE 应当让他出局")
		}
	})

	t.Run("进历史，因此回放得出来", func(t *testing.T) {
		if len(e.EffectLog()) <= before {
			t.Fatal("Apply 的效果应当进效果流")
		}
		replayed, err := ReplayEngine(testConfig(), e.EffectLog(), withNoopResolvers()...)
		if err != nil {
			t.Fatalf("ReplayEngine: %v", err)
		}
		if p, _ := replayed.PlayerInfo("v"); p.Alive {
			t.Error("Apply 改的状态没能被回放重建")
		}
	})

	t.Run("被否决的不生效", func(t *testing.T) {
		vetoed := NewSetAliveEffect("w", false)
		vetoed.Cancel("测试：拦下这一次")
		e.Apply(vetoed)
		if p, _ := e.PlayerInfo("w"); !p.Alive {
			t.Error("被否决的效果不该改动状态")
		}
	})

	t.Run("内核原语不外发", func(t *testing.T) {
		var seen []EventType
		e.OnEvent(func(ev *Event) { seen = append(seen, ev.Type) })
		e.Apply(NewSetVarEffect(ScopeGame, "probe", "1"))
		for _, typ := range seen {
			if typ == EventSetVar {
				t.Error("状态原语不该到达 OnEvent，Apply 这条路也不例外")
			}
		}
	})
}

// TestSetsAlive_IsTheInterceptionPoint 拦一次死亡靠的是拦原语，与死因无关。
//
// 白痴被投票放逐时翻牌不出局，走的就是这条：把那条致死的原语否决掉。
// 拦原语而不是拦「放逐」这个说法，好处是同一段代码能挡住任何规则的死法。
// 这条能力此前在内核自己的测试里覆盖率 0%。
func TestSetsAlive_IsTheInterceptionPoint(t *testing.T) {
	kill := NewSetAliveEffect("v", false)
	revive := NewSetAliveEffect("v", true)

	if alive, ok := kill.SetsAlive(); !ok || alive {
		t.Errorf("这是一条致死原语，SetsAlive 应当报 (false, true)，实际 (%v, %v)", alive, ok)
	}
	if alive, ok := revive.SetsAlive(); !ok || !alive {
		t.Errorf("这是一条复活原语，SetsAlive 应当报 (true, true)，实际 (%v, %v)", alive, ok)
	}

	// 规则自己的事件不是原语——「放逐」不会让人死。
	if _, ok := NewEffect(EventType("LYNCH"), "", "v").SetsAlive(); ok {
		t.Error("规则自己命名的事件不该被当成致死原语")
	}
	var nilEffect *Effect
	if _, ok := nilEffect.SetsAlive(); ok {
		t.Error("nil 效果不该被认出来")
	}

	// 真拦一次：把致死原语 Cancel 掉，人就不死。
	e := newTestEngine(t, withNoopResolvers()...)
	mustAdd(t, e, "v", roleVillager)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	blocked := NewSetAliveEffect("v", false)
	if alive, ok := blocked.SetsAlive(); ok && !alive {
		blocked.Cancel("白痴翻牌，不出局")
	}
	e.Apply(blocked)
	if p, _ := e.PlayerInfo("v"); !p.Alive {
		t.Error("致死原语被否决之后，人不该死")
	}
}

// TestMustNewEngine 配置合法就给引擎，不合法就 panic。
//
// 它存在的理由是「配置写错必须当场炸，不能拖到半局」。而这条此前没验过。
func TestMustNewEngine(t *testing.T) {
	t.Run("合法配置", func(t *testing.T) {
		if e := MustNewEngine(testConfig(), withNoopResolvers()...); e == nil {
			t.Fatal("合法配置应当给出引擎")
		}
	})

	t.Run("不合法就 panic", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("配置不合法时应当 panic——这正是 Must 的含义")
			}
		}()
		MustNewEngine(&Config{})
	})
}

// TestBoard_IsAFaithfulMiniatureOfTheEngine
// Board 摆出来的局面，走的必须是与引擎完全相同的那个写入点。
//
// Board / Seat / Mark 是内核给规则包做单测用的**公开 API**——它的价值全在
// 「与引擎同一条路」这一句上：不然规则包的单测会绿，整局跑起来才发现效果
// 没生效。而它此前在内核自己的测试里一次都没被调用过。
func TestBoard_IsAFaithfulMiniatureOfTheEngine(t *testing.T) {
	b := Board{
		Round: 2, Phase: phaseNightWolf,
		Vars: map[string]string{"probe.game": "1"},
		Players: []PlayerInfo{
			Seat("w", roleWerewolf, true, VarCamp, string(campEvil)),
			Mark(Seat("v", roleVillager, true), testMarkA),
		},
	}

	t.Run("摆出来的局面读得回来", func(t *testing.T) {
		view := b.View()
		if got := view.Round(); got != 2 {
			t.Errorf("回合 = %d，期望 2", got)
		}
		if got := view.Var(ScopeGame, "probe.game"); got != "1" {
			t.Errorf("整局状态读不回来：%q", got)
		}
		if got := view.Var(ScopeRound.Of("v"), testMarkA); got == "" {
			t.Error("Mark 打上的本回合标记读不回来")
		}
		if p, ok := view.Player("w"); !ok || p.Vars[VarCamp] != string(campEvil) {
			t.Errorf("Seat 发的初始状态读不回来：%+v", p)
		}
	})

	t.Run("Apply 走的是引擎那个写入点", func(t *testing.T) {
		after := b.Apply([]*Effect{
			NewSetAliveEffect("v", false),
			NewSetVarEffect(ScopeGame, "probe.game", "2"),
		})
		if p, _ := after.Player("v"); p.Alive {
			t.Error("SET_ALIVE 在 Board 上也该生效")
		}
		if got := after.Var(ScopeGame, "probe.game"); got != "2" {
			t.Errorf("SET_VAR 在 Board 上也该生效，读到 %q", got)
		}
	})

	t.Run("被否决的效果什么都不改", func(t *testing.T) {
		vetoed := NewSetAliveEffect("w", false)
		vetoed.Cancel("测试")
		if p, _ := b.Apply([]*Effect{vetoed}).Player("w"); !p.Alive {
			t.Error("被否决的效果不该改动局面——这正是 Board 要验的东西")
		}
	})

	t.Run("内核不认得的类型什么都不改", func(t *testing.T) {
		unknown := NewEffect(EventType("SOMETHING_RULES_MADE_UP"), "", "w")
		if p, _ := b.Apply([]*Effect{unknown}).Player("w"); !p.Alive {
			t.Error("规则自己命名的事件不该改动状态")
		}
	})
}
