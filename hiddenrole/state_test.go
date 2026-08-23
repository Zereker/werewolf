package hiddenrole

import (
	"testing"
)

func TestNewState(t *testing.T) {
	state := newState()

	if state.Phase != PhaseStart {
		t.Errorf("expected Phase=START, got %v", state.Phase)
	}
	if state.Round != 0 {
		t.Errorf("expected Round=0, got %d", state.Round)
	}
	if len(state.players) != 0 {
		t.Errorf("expected empty players, got %d", len(state.players))
	}
}

func TestAddPlayer(t *testing.T) {
	state := newState()

	mustAddTo(t, state, "p1", roleWerewolf)

	player, ok := state.getPlayer("p1")
	if !ok {
		t.Fatal("player not found after AddPlayer")
	}
	if player.ID != "p1" {
		t.Errorf("expected ID=p1, got %s", player.ID)
	}
	if player.Role != roleWerewolf {
		t.Errorf("expected Role=WEREWOLF, got %v", player.Role)
	}
	// Kernel seating records only the ID, the role and aliveness. Camp,
	// items and other initial state are handed out by the rules' RoleSetup
	// (see Engine.AddPlayer), and a bare gameState does not go through that
	// step.
	if len(player.Vars) != 0 {
		t.Errorf("kernel seating should hand out no state, got %v", player.Vars)
	}
	if !player.Alive {
		t.Error("expected Alive=true")
	}
}

func TestGetPlayer_Exists(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "p1", roleSeer)

	player, ok := state.getPlayer("p1")
	if !ok {
		t.Error("expected ok=true for existing player")
	}
	if player == nil {
		t.Error("expected player not nil")
	}
}

func TestGetPlayer_NotExists(t *testing.T) {
	state := newState()

	player, ok := state.getPlayer("nonexistent")
	if ok {
		t.Error("expected ok=false for non-existing player")
	}
	if player != nil {
		t.Error("expected player to be nil")
	}
}

// TestApplyEffect_KernelPrimitives covers the kernel's state primitives.
//
// applyEffect used to recognise a dozen effect types -- a wolf kill, a
// poisoning, an exile and a gunshot each their own way to die, PROTECT and
// SAVE each marking a thing. That wrote "what happens in a game of werewolf"
// into the state machine: change the ruleset and not one is any use, while a
// new rule wanting to express its own state change had to come and edit
// this.
func TestApplyEffect_KernelPrimitives(t *testing.T) {
	t.Run("SET_ALIVE changes aliveness", func(t *testing.T) {
		state := newState()
		mustAddTo(t, state, "p1", roleVillager)

		state.applyEffect(NewSetAliveEffect("p1", false))
		if p, _ := state.getPlayer("p1"); p.Alive {
			t.Error("should be eliminated after SET_ALIVE(false)")
		}
		state.applyEffect(NewSetAliveEffect("p1", true))
		if p, _ := state.getPlayer("p1"); !p.Alive {
			t.Error("should be alive again after SET_ALIVE(true)")
		}
	})

	t.Run("SET_VAR marks this round", func(t *testing.T) {
		state := newState()
		mustAddTo(t, state, "p1", roleVillager)

		state.applyEffect(NewSetVarEffect(ScopeRound.Of("p1"), testMarkA, VarPresent))
		if !markedInA(state, "p1") {
			t.Error("the marker should read back after being set")
		}
		state.applyEffect(NewSetVarEffect(ScopeRound.Of("p1"), testMarkA, ""))
		if markedInA(state, "p1") {
			t.Error("an empty value should be equivalent to deletion")
		}
	})

	t.Run("the round boundary clears markers", func(t *testing.T) {
		state := newState()
		mustAddTo(t, state, "p1", roleVillager)

		state.applyEffect(NewSetVarEffect(ScopeRound.Of("p1"), testMarkB, VarPresent))
		setRoundVar(state, testKillTarget, "p1")
		state.resetRoundState()

		if markedInB(state, "p1") {
			t.Error("a player's round markers should be cleared with the round")
		}
		if got := killTargetOfState(state); got != "" {
			t.Errorf("round variables should be cleared with the round, got %q", got)
		}
	})
}

// TestApplyEffect_RuleEventsDoNotTouchState: the rules' events change no
// state.
//
// This is the executable form of "the kernel does not know werewolf": KILL /
// POISON / ELIMINATE / SHOOT / PROTECT / SAVE are now only the rules' names
// for what happened, for the audience and the effect log. What actually
// changes state is the primitive alongside them -- so a lone KILL kills
// nobody.
func TestApplyEffect_RuleEventsDoNotTouchState(t *testing.T) {
	for _, typ := range []EventType{
		eventKill, eventPoison, eventEliminate, eventShoot,
		eventProtect, eventSave, eventCheck, eventVoteTied,
	} {
		state := newState()
		mustAddTo(t, state, "p1", roleVillager)

		// Seating already handed out the initial state (camp and category);
		// what is compared is whether anything moved since.
		before := copyVars(state.players["p1"].Vars)

		state.applyEffect(NewEffect(typ, "src", "p1"))

		p, _ := state.getPlayer("p1")
		switch {
		case !p.Alive:
			t.Errorf("%v should not have the kernel change aliveness", typ)
		case len(p.RoundVars) != 0:
			t.Errorf("%v should not have the kernel write round markers, got %v", typ, p.RoundVars)
		case !sameVars(p.Vars, before):
			t.Errorf("%v should not have the kernel change player state; at seating %v, now %v", typ, before, p.Vars)
		}
	}
}

// TestApplyEffect_SaveDoesNotResurrect: the antidote is not a resurrection
// primitive.
//
// Deaths all happen in the night resolution phase, and the target is still
// alive when SAVE takes effect; setting Alive=true here would let any SAVE
// effect drag a long-eliminated player back onto the board.
func TestApplyEffect_SaveDoesNotResurrect(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "p1", roleVillager)
	state.players["p1"].Alive = false

	state.applyEffect(NewEffect(eventSave, "witch", "p1"))

	player, _ := state.getPlayer("p1")
	if player.Alive {
		t.Error("an eliminated player should not be resurrected by the antidote")
	}
}

func TestApplyEffect_Canceled(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "p1", roleVillager)

	effect := NewEffect(eventKill, "wolf", "p1")
	effect.Cancel("protected")
	state.applyEffect(effect)

	player, _ := state.getPlayer("p1")
	if !player.Alive {
		t.Error("canceled effect should not kill player")
	}
}

func TestApplyEffect_InvalidTarget(t *testing.T) {
	state := newState()

	effect := NewEffect(eventKill, "wolf", "nonexistent")
	// Should not panic
	state.applyEffect(effect)
}

func TestResetRoundState(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "p1", roleVillager)
	mustAddTo(t, state, "p2", roleVillager)

	// Set the protection marker through the round context.
	markRound(state, "p1", testMarkA)
	markRound(state, "p2", testMarkA)
	setRoundVar(state, testKillTarget, "p1")

	state.resetRoundState()

	// The round context should have been reset.
	if markedInA(state, "p1") {
		t.Error("expected p1 not protected after reset")
	}
	if markedInA(state, "p2") {
		t.Error("expected p2 not protected after reset")
	}
	if killTargetOfState(state) != "" {
		t.Errorf("expected empty KillTarget after reset, got %s", killTargetOfState(state))
	}
}

func TestNextPhase_ToDay(t *testing.T) {
	state := newState()
	state.Phase = phaseNight
	state.Round = 1

	state.nextPhase(phaseDay, false, false) // the previous phase declared neither

	if state.Phase != phaseDay {
		t.Errorf("expected Phase=DAY, got %v", state.Phase)
	}
	if state.Round != 1 {
		t.Errorf("expected Round=1, got %d", state.Round)
	}
}

func TestNextPhase_ToNightGuard_IncrementsRound(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "p1", roleVillager)
	markRound(state, "p1", testMarkA)
	setRoundVar(state, testKillTarget, "p1")
	state.Phase = phaseVote
	state.Round = 1

	// The second argument is "was the phase just resolved the end of this
	// round", declared by PhaseConfig.EndsRound -- the kernel no longer
	// guesses it from the phase cycle.
	state.nextPhase(phaseNightGuard, true, true)

	if state.Phase != phaseNightGuard {
		t.Errorf("expected Phase=NIGHT_GUARD, got %v", state.Phase)
	}
	if state.Round != 2 {
		t.Errorf("expected Round=2, got %d", state.Round)
	}
	// The round context should have been reset.
	if markedInA(state, "p1") {
		t.Error("expected NightContext to be reset")
	}
	if killTargetOfState(state) != "" {
		t.Error("expected KillTarget to be reset")
	}
}
