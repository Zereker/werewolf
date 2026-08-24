package onenight

import (
	"testing"

	"github.com/Zereker/hiddenrole"
)

// TestSnapshot_RoundTrip: the board matches after a save round trip, and
// playing on from either side reaches the same outcome.
//
// Almost all of this ruleset's state lives in the two game-long cells (the
// card in each hand, the three centre cards, who saw what). A snapshot that
// dropped them would give a restored game that is wrong from the first move.
func TestSnapshot_RoundTrip(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleWerewolf, RoleVillager, RoleTanner},
		at("s", RoleSeer), at("r", RoleRobber),
		at("w", RoleWerewolf), at("v", RoleVillager))

	g.advance(PhaseNightSeer)
	g.use("s", SkillSeerCenter02)
	g.advance(PhaseNightRobber)
	g.use("r", SkillRob, "w")
	g.advance(PhaseDay)

	snap := g.e.Snapshot()
	restored, err := hiddenrole.RestoreEngine(GameConfig(), snap,
		Options([CenterCount]hiddenrole.RoleType{RoleWerewolf, RoleVillager, RoleTanner})...)
	if err != nil {
		t.Fatalf("RestoreEngine: %v", err)
	}

	// The card in each hand.
	for _, id := range []string{"s", "r", "w", "v"} {
		want := card(g.e.View(), id)
		if got := card(restored.View(), id); got != want {
			t.Errorf("%s: the card in hand restored wrongly: %v, want %v", id, got, want)
		}
	}
	// The three centre cards.
	for i := 0; i < CenterCount; i++ {
		want := centerCard(g.e.View(), i)
		if got := centerCard(restored.View(), i); got != want {
			t.Errorf("centre card %d restored wrongly: %v, want %v", i, got, want)
		}
	}
	// Who saw what.
	if got := restored.PlayerView("s").RoleInfo["learn.center.0"]; got != string(RoleWerewolf) {
		t.Errorf("what the seer saw did not travel with the snapshot, read %q", got)
	}
	if got := restored.PlayerView("r").RoleInfo["learn.self"]; got != string(RoleWerewolf) {
		t.Errorf("what the robber saw did not travel with the snapshot, read %q", got)
	}

	// Play on from both, and the outcomes match.
	finish := func(e *hiddenrole.Engine) hiddenrole.Camp {
		t.Helper()
		for _, id := range []string{"s", "r", "w", "v"} {
			target := "r"
			if id == "r" {
				target = "w"
			}
			if err := e.SubmitSkillUse(&hiddenrole.SkillUse{
				PlayerID: id, Skill: SkillVote, Targets: []string{target},
			}); err != nil {
				t.Fatalf("%s voting: %v", id, err)
			}
		}
		if _, err := e.EndPhase(); err != nil {
			t.Fatalf("EndPhase: %v", err)
		}
		return e.Status().Winner
	}

	if err := func() error { _, err := g.e.EndPhase(); return err }(); err != nil {
		t.Fatalf("EndPhase: %v", err)
	}
	if _, err := restored.EndPhase(); err != nil {
		t.Fatalf("EndPhase: %v", err)
	}
	if a, b := finish(g.e), finish(restored); a != b {
		t.Errorf("the restored game reached a different outcome: %v vs %v", a, b)
	}
}

// TestReplay_RebuildsGame: replaying the effect log reaches the same board.
func TestReplay_RebuildsGame(t *testing.T) {
	center := [CenterCount]hiddenrole.RoleType{RoleVillager, RoleWerewolf, RoleVillager}
	g := newGame(t, center,
		at("t", RoleTroublemaker), at("w", RoleWerewolf),
		at("d", RoleDrunk), at("v", RoleVillager))

	g.advance(PhaseNightTroublemake)
	g.use("t", SkillMeddle, "w", "v")
	g.advance(PhaseNightDrunk)
	g.use("d", SkillDrinkCenter1)
	g.advance(PhaseDay)

	replayed, err := hiddenrole.ReplayEngine(GameConfig(), g.e.EffectLog(), Options(center)...)
	if err != nil {
		t.Fatalf("ReplayEngine: %v", err)
	}

	if got, want := replayed.Status().Phase, g.e.Status().Phase; got != want {
		t.Errorf("phase after replay = %v, want %v", got, want)
	}
	for _, id := range []string{"t", "w", "d", "v"} {
		want := card(g.e.View(), id)
		if got := card(replayed.View(), id); got != want {
			t.Errorf("%s: the card in hand replayed wrongly: %v, want %v", id, got, want)
		}
	}
	for i := 0; i < CenterCount; i++ {
		want := centerCard(g.e.View(), i)
		if got := centerCard(replayed.View(), i); got != want {
			t.Errorf("centre card %d replayed wrongly: %v, want %v", i, got, want)
		}
	}
}

// TestConfig_IsValid: the phase graph is internally consistent.
func TestConfig_IsValid(t *testing.T) {
	if err := GameConfig().Validate(); err != nil {
		t.Fatalf("the phase graph is invalid: %v", err)
	}
}

// TestRoundNeverAdvances: this ruleset has exactly one round.
//
// Both earlier packages have cyclic phase graphs and their round numbers climb.
// This one is a straight line and Round is 1 from start to finish -- and this
// configuration declares no round boundary at all, precisely because it does
// not need one (SCARS.md, scar 2).
func TestRoundNeverAdvances(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("w", RoleWerewolf), at("v1", RoleVillager), at("v2", RoleVillager))

	for _, phase := range []hiddenrole.PhaseType{
		PhaseNightMinion, PhaseNightMason, PhaseNightSeer, PhaseNightRobber,
		PhaseNightTroublemake, PhaseNightDrunk, PhaseNightInsomniac,
		PhaseDay, PhaseVote,
	} {
		g.advance(phase)
		if got := g.e.Status().Round; got != 1 {
			t.Fatalf("round = %d on reaching %v; this ruleset has exactly one round", got, phase)
		}
	}
}
