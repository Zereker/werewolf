package hiddenrole

import (
	"encoding/json"
	"testing"
)

// replay_test.go covers effect-log replay and host-level writes, verified by
// **the kernel itself**.
//
// This batch fills a real hole: after the split, ReplayEngine had **0%**
// coverage in the kernel's own tests -- its entire path had only ever been
// driven by the rules packages downstream.
//
// The three replay bugs fixed this round (the winner not restored, the actor
// list not consumed on ending, the detour queue not consumed) were all caught
// by a rules package's random games. The kernel's correctness being proven
// downstream is fragile: the day a rules package stops running, nobody is
// watching this path.
//
// Everything below uses the kernel's own vocabulary (see vocab_test.go) and
// knows no game.

// scoreResolver writes one piece of game-long state in a given phase and
// names the next phase's actors.
//
// It is used to build an effect log with **enough in it**: a state change, an
// actor list, and one of the rules' own events -- only with all three can
// replay show that what was rebuilt is the same board.
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

// replayFixture builds an engine that has run a few steps, together with the
// config and options needed to rebuild it.
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

// TestReplayEngine_RebuildsTheSameBoard: what replay produces must be the
// same board.
//
// Snapshots are compared byte for byte, not just phase and round: with a
// field missing from the snapshot, both sides still walk a whole game in
// lockstep, only the rules judge differently.
func TestReplayEngine_RebuildsTheSameBoard(t *testing.T) {
	e, cfg, opts := replayFixture(t)

	replayed, err := ReplayEngine(cfg, e.EffectLog(), opts...)
	if err != nil {
		t.Fatalf("ReplayEngine: %v", err)
	}

	if got, want := replayed.Status(), e.Status(); got != want {
		t.Errorf("Status after replay = %+v, original %+v", got, want)
	}
	a, _ := json.Marshal(e.Snapshot())
	b, _ := json.Marshal(replayed.Snapshot())
	if string(a) != string(b) {
		t.Errorf("the board differs after replay:\n  original %s\n  replayed %s", a, b)
	}
}

// TestReplayEngine_CarriesStateActorsAndBehaviour: state, the actor list and
// behaviour must all travel.
//
// Matching snapshot bytes do not mean matching behaviour -- and the **actor
// list** is the point here: it decides who may act, and without it the
// replayed engine tells everyone "you may act".
func TestReplayEngine_CarriesStateActorsAndBehaviour(t *testing.T) {
	e, cfg, opts := replayFixture(t)

	replayed, err := ReplayEngine(cfg, e.EffectLog(), opts...)
	if err != nil {
		t.Fatalf("ReplayEngine: %v", err)
	}

	if got := replayed.Var(ScopeGame, "probe.score"); got != "7" {
		t.Errorf("game-long state did not travel, read %q", got)
	}
	// The previous phase named actors: only w1 can act in the wolf phase,
	// not w2.
	for _, id := range []string{"w1", "w2", "v"} {
		x, y := len(e.AllowedSkills(id)), len(replayed.AllowedSkills(id))
		if x != y {
			t.Errorf("what %s may do differs: original %d, replayed %d", id, x, y)
		}
	}
	if len(replayed.AllowedSkills("w2")) != 0 {
		t.Error("the list names only w1, so w2 should not be able to act -- the actor list did not travel through replay")
	}
}

// endedGame builds an engine whose game is already over.
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
		t.Fatal("this test needs a game that is already over")
	}
	return e, cfg, opts
}

// TestReplayEngine_CarriesTheWinner: replaying a finished game must still
// know who won.
//
// **This was a real bug.** Who won is settled by the VictoryChecker at the
// moment the game ends and does not change afterwards, and replay does not
// run the check again -- the GAME_ENDED effect carries the winner, only
// nobody used to read it. Replay produced Over=true with an empty Winner, and
// diverged from the original.
func TestReplayEngine_CarriesTheWinner(t *testing.T) {
	e, cfg, opts := endedGame(t)

	replayed, err := ReplayEngine(cfg, e.EffectLog(), opts...)
	if err != nil {
		t.Fatalf("ReplayEngine: %v", err)
	}
	if got, want := replayed.Status(), e.Status(); got != want {
		t.Errorf("Status after replay = %+v, original %+v", got, want)
	}
	if replayed.Status().Winner == CampUnspecified {
		t.Error("the winner did not travel in the effect log")
	}
}

// TestReplayEngine_RejectsABrokenLog: a broken effect log must be rejected
// rather than quietly rebuilding half a game.
func TestReplayEngine_RejectsABrokenLog(t *testing.T) {
	e, cfg, opts := replayFixture(t)
	log := e.EffectLog()

	t.Run("a nil entry", func(t *testing.T) {
		broken := append([]*Effect{}, log...)
		broken = append(broken, nil)
		if _, err := ReplayEngine(cfg, broken, opts...); !HasCode(err, CodeInvalidEffectLog) {
			t.Errorf("should be rejected as %v, got %v", CodeInvalidEffectLog, CodeOf(err))
		}
	})

	t.Run("a PHASE_CHANGED carrying no phase", func(t *testing.T) {
		broken := append([]*Effect{}, log...)
		broken = append(broken, NewEffect(EventPhaseChanged, "", ""))
		if _, err := ReplayEngine(cfg, broken, opts...); !HasCode(err, CodeInvalidEffectLog) {
			t.Errorf("should be rejected as %v, got %v", CodeInvalidEffectLog, CodeOf(err))
		}
	})

	t.Run("an invalid configuration", func(t *testing.T) {
		if _, err := ReplayEngine(&Config{}, log, opts...); err == nil {
			t.Error("an invalid configuration should not rebuild an engine")
		}
	})
}

// TestReplayEngine_LogIsPreserved: the replayed engine carries the same
// history.
//
// Otherwise "save again after replaying" gives something different from the
// original, and chained replays drift further with every hop.
func TestReplayEngine_LogIsPreserved(t *testing.T) {
	e, cfg, opts := replayFixture(t)

	once, err := ReplayEngine(cfg, e.EffectLog(), opts...)
	if err != nil {
		t.Fatalf("ReplayEngine: %v", err)
	}
	twice, err := ReplayEngine(cfg, once.EffectLog(), opts...)
	if err != nil {
		t.Fatalf("replaying again: %v", err)
	}

	if len(once.EffectLog()) != len(e.EffectLog()) {
		t.Errorf("history length after replay %d, original %d", len(once.EffectLog()), len(e.EffectLog()))
	}
	a, _ := json.Marshal(e.Snapshot())
	b, _ := json.Marshal(twice.Snapshot())
	if string(a) != string(b) {
		t.Errorf("replaying twice diverged from the original:\n  original %s\n  twice    %s", a, b)
	}
}

// TestApply_GoesThroughTheSameWritePoint: a host-level write takes the same
// write point.
//
// Engine.Apply bypasses phase resolution and is a tool with an edge -- a host
// really does meet "the player disconnected, count them dead" and "an admin
// kicked someone". Its whole value is in the phrase **still the same write
// point**: effects enter the history, vetoed ones do not take hold, and
// kernel primitives are not sent out. None of those three used to be checked
// in the kernel's own tests (Apply had 0% coverage).
func TestApply_GoesThroughTheSameWritePoint(t *testing.T) {
	e := newTestEngine(t, withNoopResolvers()...)
	mustAdd(t, e, "v", roleVillager)
	mustAdd(t, e, "w", roleWerewolf)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	before := len(e.EffectLog())

	t.Run("the effect really takes hold", func(t *testing.T) {
		e.Apply(NewSetAliveEffect("v", false))
		if p, _ := e.PlayerInfo("v"); p.Alive {
			t.Error("SET_ALIVE should eliminate them")
		}
	})

	t.Run("it enters the history, so replay reproduces it", func(t *testing.T) {
		if len(e.EffectLog()) <= before {
			t.Fatal("an effect from Apply should enter the effect log")
		}
		replayed, err := ReplayEngine(testConfig(), e.EffectLog(), withNoopResolvers()...)
		if err != nil {
			t.Fatalf("ReplayEngine: %v", err)
		}
		if p, _ := replayed.PlayerInfo("v"); p.Alive {
			t.Error("state changed by Apply was not rebuilt by replay")
		}
	})

	t.Run("a vetoed effect does not take hold", func(t *testing.T) {
		vetoed := NewSetAliveEffect("w", false)
		vetoed.Cancel("test: intercept this one")
		e.Apply(vetoed)
		if p, _ := e.PlayerInfo("w"); !p.Alive {
			t.Error("a vetoed effect should change no state")
		}
	})

	t.Run("kernel primitives are not sent out", func(t *testing.T) {
		var seen []EventType
		e.OnEvent(func(ev *Event) { seen = append(seen, ev.Type) })
		e.Apply(NewSetVarEffect(ScopeGame, "probe", "1"))
		for _, typ := range seen {
			if typ == EventSetVar {
				t.Error("a state primitive should not reach OnEvent, and the Apply path is no exception")
			}
		}
	})
}

// TestSetsAlive_IsTheInterceptionPoint: intercepting a death means
// intercepting the primitive, independently of the cause.
//
// The idiot surviving an exile by flipping their card takes this route:
// vetoing the lethal primitive. Intercepting the primitive rather than the
// word "exile" is what lets one piece of code stop any ruleset's way of
// dying. This ability used to have 0% coverage in the kernel's own tests.
func TestSetsAlive_IsTheInterceptionPoint(t *testing.T) {
	kill := NewSetAliveEffect("v", false)
	revive := NewSetAliveEffect("v", true)

	if alive, ok := kill.SetsAlive(); !ok || alive {
		t.Errorf("this is a lethal primitive, SetsAlive should report (false, true), got (%v, %v)", alive, ok)
	}
	if alive, ok := revive.SetsAlive(); !ok || !alive {
		t.Errorf("this is a reviving primitive, SetsAlive should report (true, true), got (%v, %v)", alive, ok)
	}

	// The rules' own events are not primitives -- an "exile" kills nobody.
	if _, ok := NewEffect(EventType("LYNCH"), "", "v").SetsAlive(); ok {
		t.Error("an event the rules named should not be taken for a lethal primitive")
	}
	var nilEffect *Effect
	if _, ok := nilEffect.SetsAlive(); ok {
		t.Error("a nil effect should not be recognised")
	}

	// Actually intercept one: cancel the lethal primitive and nobody dies.
	e := newTestEngine(t, withNoopResolvers()...)
	mustAdd(t, e, "v", roleVillager)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	blocked := NewSetAliveEffect("v", false)
	if alive, ok := blocked.SetsAlive(); ok && !alive {
		blocked.Cancel("the idiot flips their card and is not eliminated")
	}
	e.Apply(blocked)
	if p, _ := e.PlayerInfo("v"); !p.Alive {
		t.Error("with the lethal primitive vetoed, nobody should die")
	}
}

// TestMustNewEngine: a valid configuration gives an engine, an invalid one
// panics.
//
// It exists because "a misconfiguration must blow up on the spot, not halfway
// through a game". And that used to be unverified.
func TestMustNewEngine(t *testing.T) {
	t.Run("a valid configuration", func(t *testing.T) {
		if e := MustNewEngine(testConfig(), withNoopResolvers()...); e == nil {
			t.Fatal("a valid configuration should give an engine")
		}
	})

	t.Run("an invalid one panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("an invalid configuration should panic -- that is what Must means")
			}
		}()
		MustNewEngine(&Config{})
	})
}

// TestBoard_IsAFaithfulMiniatureOfTheEngine
// A board laid out with Board must go through exactly the same write point as
// the engine.
//
// Board / Seat / Mark are the kernel's **public API** for a rules package's
// unit tests -- their whole value is in the phrase "the same path as the
// engine": otherwise a rules package's unit tests go green and only a full
// game reveals that the effect never landed. And it used never to be called
// once in the kernel's own tests.
func TestBoard_IsAFaithfulMiniatureOfTheEngine(t *testing.T) {
	b := Board{
		Round: 2, Phase: phaseNightWolf,
		Vars: map[string]string{"probe.game": "1"},
		Players: []PlayerInfo{
			Seat("w", roleWerewolf, true, VarCamp, string(campEvil)),
			Mark(Seat("v", roleVillager, true), testMarkA),
		},
	}

	t.Run("the board laid out reads back", func(t *testing.T) {
		view := b.View()
		if got := view.Round(); got != 2 {
			t.Errorf("round = %d, want 2", got)
		}
		if got := view.Var(ScopeGame, "probe.game"); got != "1" {
			t.Errorf("game-long state does not read back: %q", got)
		}
		if got := view.Var(ScopeRound.Of("v"), testMarkA); got == "" {
			t.Error("a round marker set by Mark does not read back")
		}
		if p, ok := view.Player("w"); !ok || p.Vars[VarCamp] != string(campEvil) {
			t.Errorf("the initial state handed out by Seat does not read back: %+v", p)
		}
	})

	t.Run("Apply takes the engine's write point", func(t *testing.T) {
		after := b.Apply([]*Effect{
			NewSetAliveEffect("v", false),
			NewSetVarEffect(ScopeGame, "probe.game", "2"),
		})
		if p, _ := after.Player("v"); p.Alive {
			t.Error("SET_ALIVE should take hold on a Board too")
		}
		if got := after.Var(ScopeGame, "probe.game"); got != "2" {
			t.Errorf("SET_VAR should take hold on a Board too, read %q", got)
		}
	})

	t.Run("a vetoed effect changes nothing", func(t *testing.T) {
		vetoed := NewSetAliveEffect("w", false)
		vetoed.Cancel("test")
		if p, _ := b.Apply([]*Effect{vetoed}).Player("w"); !p.Alive {
			t.Error("a vetoed effect should not change the board -- exactly what Board is meant to verify")
		}
	})

	t.Run("a type the kernel does not recognise changes nothing", func(t *testing.T) {
		unknown := NewEffect(EventType("SOMETHING_RULES_MADE_UP"), "", "w")
		if p, _ := b.Apply([]*Effect{unknown}).Player("w"); !p.Alive {
			t.Error("an event the rules named should change no state")
		}
	})
}
