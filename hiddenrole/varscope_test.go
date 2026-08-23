package hiddenrole

import "testing"

// allScopes is the complete set of four scope cells, written out as
// (lifetime, ownership) crossed.
//
// This function is the point of these tests: scopes used to be four unrelated
// constructors, nothing anywhere could say how many cells there were, and a
// missing one was nobody's job to notice (in fact "whole game, unowned" was
// missing until the mission-based rules ran into it). It is a type now, the
// full set can be enumerated, and every property below asserts over the full
// set -- so the next missing cell is caught by a test rather than by the next
// rules package.
func allScopes(ownerID string) []struct {
	name     string
	scope    VarScope
	perRound bool
	owned    bool
} {
	return []struct {
		name     string
		scope    VarScope
		perRound bool
		owned    bool
	}{
		{"whole game, unowned", ScopeGame, false, false},
		{"whole game, one player", ScopeGame.Of(ownerID), false, true},
		{"this round, unowned", ScopeRound, true, false},
		{"this round, one player", ScopeRound.Of(ownerID), true, true},
	}
}

// TestVarScope_AllFourCellsRoundTrip: all four cells can be written, read
// back, and cleared by writing an empty string.
func TestVarScope_AllFourCellsRoundTrip(t *testing.T) {
	for _, c := range allScopes("p1") {
		t.Run(c.name, func(t *testing.T) {
			state := newState()
			mustAddTo(t, state, "p1", roleVillager)

			if got := state.varOf(c.scope, "k"); got != "" {
				t.Fatalf("never written, so it should be empty, got %q", got)
			}
			state.applyEffect(NewSetVarEffect(c.scope, "k", "v"))
			if got := state.varOf(c.scope, "k"); got != "v" {
				t.Fatalf("written then read back should be v, got %q", got)
			}
			state.applyEffect(NewSetVarEffect(c.scope, "k", ""))
			if got := state.varOf(c.scope, "k"); got != "" {
				t.Fatalf("an empty string should be equivalent to deletion, got %q", got)
			}
			// "Deleted" and "set to empty" both read back as the empty string
			// and cannot be told apart from the outside -- telling them apart
			// means looking at the map underneath. The difference shows in the
			// snapshot: setting an empty string leaves an empty entry, so saves
			// grow without bound and stop being byte-identical to a save where
			// the key was never written.
			if vars, _ := state.varsFor(c.scope); vars != nil {
				if _, exists := vars["k"]; exists {
					t.Fatalf("an empty string should delete the key rather than leave an empty entry: %v", vars)
				}
			}
		})
	}
}

// TestVarScope_CellsDoNotLeakIntoEachOther: the same key in different cells
// does not interfere.
//
// With the four cells sharing one event type and one set of data keys, this
// is no longer obvious: a write point that reads the scope wrongly would run
// all four together.
func TestVarScope_CellsDoNotLeakIntoEachOther(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "p1", roleVillager)
	mustAddTo(t, state, "p2", roleVillager)

	cells := allScopes("p1")
	for i, c := range cells {
		state.applyEffect(NewSetVarEffect(c.scope, "k", c.name))
		_ = i
	}
	for _, c := range cells {
		if got := state.varOf(c.scope, "k"); got != c.name {
			t.Errorf("%s was overwritten by another cell: want %q, got %q", c.name, c.name, got)
		}
	}

	// The two owned cells know who they belong to: what was written for p1 is
	// not readable by p2.
	for _, c := range cells {
		if !c.owned {
			continue
		}
		other := ScopeGame.Of("p2")
		if c.perRound {
			other = ScopeRound.Of("p2")
		}
		if got := state.varOf(other, "k"); got != "" {
			t.Errorf("%s leaked onto another player: p2 read %q", c.name, got)
		}
	}
}

// TestVarScope_RoundBoundaryClearsExactlyTheRoundCells: the round boundary
// clears the two round cells, and only those.
//
// The whole meaning of the "this round" axis is here: clear too much and
// game-long state is lost, clear too little and last night's markers pile up
// into the next -- both have happened.
func TestVarScope_RoundBoundaryClearsExactlyTheRoundCells(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "p1", roleVillager)

	cells := allScopes("p1")
	for _, c := range cells {
		state.applyEffect(NewSetVarEffect(c.scope, "k", "v"))
	}

	state.resetRoundState()

	for _, c := range cells {
		got := state.varOf(c.scope, "k")
		if c.perRound && got != "" {
			t.Errorf("%s is round-scoped and should be cleared past the round boundary, got %q", c.name, got)
		}
		if !c.perRound && got != "v" {
			t.Errorf("%s lives for the whole game and should survive the round boundary, got %q", c.name, got)
		}
	}
}

// TestVarScope_SetsVarReportsTheCell: the scope can be read back out of the
// effect.
//
// With the four cells folded into the single SET_VAR event type, Type alone
// no longer says which cell a write is for. An extension that wants to
// intercept or observe a class of write has only SetsVar -- so it has to
// recognise all four.
func TestVarScope_SetsVarReportsTheCell(t *testing.T) {
	for _, c := range allScopes("p1") {
		t.Run(c.name, func(t *testing.T) {
			ef := NewSetVarEffect(c.scope, "k", "v")
			scope, key, value, ok := ef.SetsVar()
			if !ok {
				t.Fatal("a SET_VAR effect should be recognised by SetsVar")
			}
			if key != "k" || value != "v" {
				t.Errorf("key and value read back wrong: %q=%q", key, value)
			}
			if scope != c.scope {
				t.Errorf("scope read back wrong: want %v, got %v", c.scope, scope)
			}
		})
	}

	if _, _, _, ok := NewSetAliveEffect("p1", false).SetsVar(); ok {
		t.Error("SET_ALIVE does not write a state variable and should not be recognised by SetsVar")
	}
	var nilEffect *Effect
	if _, _, _, ok := nilEffect.SetsVar(); ok {
		t.Error("a nil effect should not be recognised by SetsVar")
	}
}

// TestVarScope_OfDoesNotMutateTheSharedValues: ScopeGame and ScopeRound are
// values, and .Of returns a copy.
//
// They are package-level variables, and if .Of modified the receiver itself,
// one call would permanently bind the global ScopeGame to one player -- every
// subsequent write would go into the wrong cell.
func TestVarScope_OfDoesNotMutateTheSharedValues(t *testing.T) {
	beforeGame, beforeRound := ScopeGame, ScopeRound

	_ = ScopeGame.Of("p1")
	_ = ScopeRound.Of("p1")

	if ScopeGame != beforeGame {
		t.Errorf(".Of modified ScopeGame itself: %v", ScopeGame)
	}
	if ScopeRound != beforeRound {
		t.Errorf(".Of modified ScopeRound itself: %v", ScopeRound)
	}
	if ScopeGame.String() != "game" || ScopeRound.String() != "round" {
		t.Errorf("the two unowned cells print wrongly: %q / %q", ScopeGame, ScopeRound)
	}
	if got := ScopeRound.Of("p1").String(); got != "round:p1" {
		t.Errorf("an owned cell prints wrongly: %q", got)
	}
}

// TestVarScope_UnknownOwnerIsANoOp: a write for a player who does not exist
// changes nothing.
func TestVarScope_UnknownOwnerIsANoOp(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "p1", roleVillager)

	state.applyEffect(NewSetVarEffect(ScopeGame.Of("ghost"), "k", "v"))
	state.applyEffect(NewSetVarEffect(ScopeRound.Of("ghost"), "k", "v"))

	for _, c := range allScopes("p1") {
		if got := state.varOf(c.scope, "k"); got != "" {
			t.Errorf("a write for a player who does not exist changed %s: %q", c.name, got)
		}
	}
}
