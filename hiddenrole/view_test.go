package hiddenrole

import (
	"testing"
)

// TestGameView_IsReadOnly: a view cannot be turned back into the mutable
// state object.
//
// This is the type-level guarantee behind "every state change goes through an
// Effect": stateView is an unexported value type with unexported fields, so
// nothing outside the package can assert a GameView into anything that
// changes state.
func TestGameView_IsReadOnly(t *testing.T) {
	st := newState()
	if err := st.addPlayer("v1", roleVillager); err != nil {
		t.Fatal(err)
	}
	view := newStateView(st)

	// Go through any to get past the compile-time check before asserting:
	// *gameState does not implement GameView at all, so nothing that can
	// change state comes out. (Writing view.(*gameState) directly is
	// rejected by the compiler as an impossible type assertion, which is
	// itself proof that the constraint holds.)
	if _, ok := any(view).(*gameState); ok {
		t.Fatal("GameView should not be assertable back to *gameState")
	}
	if _, ok := any(view).(interface{ applyEffect(*Effect) }); ok {
		t.Fatal("GameView should expose no state-changing method")
	}
}

func TestGameView_ReadsThrough(t *testing.T) {
	st := newState()
	for id, role := range map[string]RoleType{
		"w1": roleWerewolf,
		"g":  roleGuard,
		"v1": roleVillager,
		"v2": roleVillager,
	} {
		if err := st.addPlayer(id, role); err != nil {
			t.Fatal(err)
		}
	}
	st.applyEffect(NewSetAliveEffect("v2", false))

	st.Round = 2
	setRoundVar(st, testKillTarget, "v1")
	st.applyEffect(NewSetVarEffect(ScopeGame.Of("g"), testVarStock, "v1"))

	view := newStateView(st)

	if got := len(view.AlivePlayers()); got != 3 {
		t.Errorf("living players: want 3, got %d", got)
	}
	if got := view.AlivePlayerIDsByRole(roleWerewolf); len(got) != 1 || got[0] != "w1" {
		t.Errorf("werewolf list: want [w1], got %v", got)
	}
	if got := view.Var(ScopeGame.Of("g"), testVarStock); got != "v1" {
		t.Errorf("player state: want v1, got %q", got)
	}
	if got := view.Var(ScopeGame.Of("nobody"), testVarStock); got != "" {
		t.Errorf("a player who does not exist should read empty, got %q", got)
	}
	if got := view.Var(ScopeRound, testKillTarget); got != "v1" {
		t.Errorf("round state: want v1, got %q", got)
	}
	if _, ok := view.Player("nobody"); ok {
		t.Error("a player who does not exist should return false")
	}
}

// TestGameView_RoundContextIsCopy: the round context a view returns is a
// copy, and changing it does not affect the engine's state.
func TestGameView_RoundContextIsCopy(t *testing.T) {
	st := newState()
	if err := st.addPlayer("v1", roleVillager); err != nil {
		t.Fatal(err)
	}
	setRoundVar(st, testKillTarget, "v1")

	view := newStateView(st)
	rc := view.RoundContext()
	rc.Vars[testKillTarget] = "tampered"
	rc.Vars["conjured-out-of-nowhere"] = "1"

	if got := view.Var(ScopeRound, testKillTarget); got != "v1" {
		t.Errorf("changing the copy affected the engine's state: %q", got)
	}
	if fresh := view.RoundContext(); fresh.Vars["conjured-out-of-nowhere"] != "" {
		t.Error("changing the copy's map affected the engine's state")
	}
}
