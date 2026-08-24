package missions

import (
	"testing"

	"github.com/Zereker/hiddenrole"
)

// scars_test.go pins the cost of each workaround as runnable evidence.
//
// These tests assert **what the current implementation gets wrong**, not the
// desired behaviour. They exist so that every item in SCARS.md can be
// verified rather than taken on my word. Once the kernel gains the
// corresponding capability they go red -- and that is when to rewrite them as
// positive assertions.

// TestOnlyNamedActorsMayAct: only the players named may act, and the kernel
// enforces it.
//
// **This used to be scar 1**, and the most expensive of the six -- the cost
// landed directly on what players are shown: the kernel decided actors from
// (phase, role, skill) alone, a role is fixed at seating time, and any set
// chosen at runtime was inexpressible. It bit this ruleset twice (the leader,
// the mission team) and werewolf once (the hunter's shot, for which the kernel
// opened the detour queue as a one-player special case).
//
// The consequence was the kernel lying to unqualified players: AllowedSkills
// said they could act, PhaseReadiness waited on them, and SubmitSkillUse
// accepted submissions for the resolver to throw away.
//
// Once the kernel gained NewSetActorsEffect, the three questions (validation,
// AllowedSkills, PhaseReadiness) read from one place. This test watches all
// three together.
func TestOnlyNamedActorsMayAct(t *testing.T) {
	e := fivePlayer(t)

	// 1. The nomination phase: the leader alone.
	leader := leaderID(e.View())
	for _, id := range allPlayerIDs(e) {
		allowed := e.AllowedSkills(id)
		if id == leader {
			if len(allowed) == 0 {
				t.Errorf("the leader %s should be able to nominate, AllowedSkills is empty", id)
			}
			continue
		}
		if len(allowed) != 0 {
			t.Errorf("%s is not the leader, yet AllowedSkills gave %v", id, allowed)
		}
		if err := e.SubmitSkillUse(&hiddenrole.SkillUse{
			PlayerID: id, Skill: SkillPropose, Targets: []string{"a"},
		}); err == nil {
			t.Errorf("%s is not the leader, yet the kernel accepted their nomination", id)
		}
	}

	// 2. The mission phase: team members alone.
	proposeAndApprove(t, e, "a", "b")
	team := map[string]bool{"a": true, "b": true}
	for _, id := range allPlayerIDs(e) {
		allowed := e.AllowedSkills(id)
		if team[id] {
			if len(allowed) == 0 {
				t.Errorf("the team member %s should be able to vote, AllowedSkills is empty", id)
			}
			continue
		}
		if len(allowed) != 0 {
			t.Errorf("%s is not on the mission, yet AllowedSkills gave %v", id, allowed)
		}
		if err := e.SubmitSkillUse(&hiddenrole.SkillUse{
			PlayerID: id, Skill: SkillMissionFail,
		}); err == nil {
			t.Errorf("%s is not on the mission, yet the kernel accepted their fail vote", id)
		}
	}

	// 3. Readiness waits on team members only.
	for _, p := range e.PhaseReadiness().Pending {
		if !team[p.PlayerID] {
			t.Errorf("PhaseReadiness is waiting on %s, who is not on the mission", p.PlayerID)
		}
	}
}

// TestRejectedProposalGoesStraightBackToPropose: a rejected nomination goes
// straight back to nomination without idling through the mission phase.
//
// **This used to be scar 2**: phase transitions were a static graph with no
// way to express "go to the mission if the vote passes, back to nomination
// otherwise", so a rejected nomination had to idle through the mission phase.
//
// Once the kernel handed "where to go next" to the rules
// (NewGotoPhaseEffect), the vote resolver says where to go and the scar
// closed. This test flipped from asserting a defect to asserting a fix.
func TestRejectedProposalGoesStraightBackToPropose(t *testing.T) {
	e := fivePlayer(t)
	propose(t, e, "a", "b")
	for _, id := range allPlayerIDs(e) {
		mustSubmit(t, e, &hiddenrole.SkillUse{PlayerID: id, Skill: SkillReject})
	}
	mustEnd(t, e)

	if got := e.Status().Phase; got != PhasePropose {
		t.Fatalf("phase = %v, want a direct return to PROPOSE (no idle pass through the mission phase)", got)
	}
}

// TestApprovedProposalGoesToMission: an approved vote goes to the mission.
//
// A pair with the one above: the same resolver computes two different exits,
// which is exactly what a static graph cannot express.
func TestApprovedProposalGoesToMission(t *testing.T) {
	e := fivePlayer(t)
	propose(t, e, "a", "b")
	for _, id := range allPlayerIDs(e) {
		mustSubmit(t, e, &hiddenrole.SkillUse{PlayerID: id, Skill: SkillApprove})
	}
	mustEnd(t, e)

	if got := e.Status().Phase; got != PhaseMission {
		t.Fatalf("phase = %v, want MISSION", got)
	}
}

// TestRoundEqualsMissionNumber: the engine's round equals this package's
// mission number.
//
// **This used to be scar 3**: the kernel welded "increment the round" onto
// "the phase cycle returned to the start phase", and this ruleset goes round
// the loop once per nomination, so Round became a nomination counter, out by
// as much as a factor of five from "which mission", and it was handed to
// players verbatim in PlayerView.Round.
//
// Two changes together closed it, and neither alone would have:
//   - PhaseConfig.EndsRound lets the board declare its own round boundary
//     (this ruleset declares it on the mission phase);
//   - NewGotoPhaseEffect sends a rejected nomination straight back to the
//     nomination phase instead of idling through the mission phase -- with
//     only the first, the idle pass would still advance the round and the scar
//     would be half open.
//
// That the two scars are coupled is itself a finding: they share a root, both
// being the kernel making a decision only the rules can answer.
func TestRoundEqualsMissionNumber(t *testing.T) {
	e := fivePlayer(t)

	// Two consecutive rejections first: no mission has been played, so the
	// round number must not move.
	for i := 0; i < 2; i++ {
		propose(t, e, "a", "b")
		for _, id := range allPlayerIDs(e) {
			mustSubmit(t, e, &hiddenrole.SkillUse{PlayerID: id, Skill: SkillReject})
		}
		mustEnd(t, e)
	}
	if got, want := e.Status().Round, 1; got != want {
		t.Errorf("Round = %d after two rejections, want still %d -- no mission has been played", got, want)
	}

	// Play two missions, and the round follows the missions.
	runMission(t, e, 0, "a", "b")
	runMission(t, e, 0, "a", "b", "c")

	if got, want := e.Status().Round, missionOf(e); got != want {
		t.Errorf("Round = %d and this package's mission number = %d; they should be equal", got, want)
	}
	if v := e.PlayerView("a"); v.Round != e.Status().Round {
		t.Errorf("PlayerView.Round = %d, the engine says %d", v.Round, e.Status().Round)
	}
}

// TestGameProgressLivesInGameVars: the game's progress lives in the game scope
// and hangs off no player.
//
// **This used to be scar 4**: the kernel's variable scopes are a 2x2 table and
// the "whole game, unowned" cell was missing. This package's five counters
// (which mission, how many succeeded, how many failed, how many consecutive
// rejections, who leads) could only be filed under the lexicographically
// smallest player's own state as a ledger -- global facts under one person's
// name, with five fields appearing out of nowhere in that player's view with
// nothing to do with them.
//
// Once the kernel gained the fourth cell, the ledger was deleted outright.
// This test watches two things: that the progress really is in the game scope,
// and that **no player has any of it stuck to them**.
func TestGameProgressLivesInGameVars(t *testing.T) {
	e := fivePlayer(t)
	proposeAndApprove(t, e, "a", "b")
	for _, id := range []string{"a", "b"} {
		mustSubmit(t, e, &hiddenrole.SkillUse{PlayerID: id, Skill: SkillMissionSuccess})
	}
	mustEnd(t, e)

	// 1. The progress reads back, and it is in the game scope.
	if got := successes(e.View()); got != 1 {
		t.Fatalf("successes = %d, want 1", got)
	}
	if e.Var(hiddenrole.ScopeGame, varSuccess) == "" {
		t.Errorf("the success count should live in the game scope, %q is empty", varSuccess)
	}

	// 2. No player has any of this package's game-long counters stuck to them.
	for _, id := range e.AlivePlayerIDs() {
		p, _ := e.PlayerInfo(id)
		for k := range p.Vars {
			if k != hiddenrole.VarCamp {
				t.Errorf("player %s should not carry %q -- that is game-long state and belongs to nobody", id, k)
			}
		}
	}
}

// ==================== Test helpers ====================

func allPlayerIDs(e *hiddenrole.Engine) []string { return e.AlivePlayerIDs() }

func missionOf(e *hiddenrole.Engine) int { return mission(e.View()) }

func mustSubmit(t *testing.T, e *hiddenrole.Engine, use *hiddenrole.SkillUse) {
	t.Helper()
	if err := e.SubmitSkillUse(use); err != nil {
		t.Fatalf("submitting %s/%s: %v", use.PlayerID, use.Skill, err)
	}
}

func mustEnd(t *testing.T, e *hiddenrole.Engine) []*hiddenrole.Effect {
	t.Helper()
	effects, err := e.EndPhase()
	if err != nil {
		t.Fatalf("EndPhase(%v): %v", e.Status().Phase, err)
	}
	return effects
}

func propose(t *testing.T, e *hiddenrole.Engine, members ...string) {
	t.Helper()
	leader := leaderID(e.View())
	mustSubmit(t, e, &hiddenrole.SkillUse{PlayerID: leader, Skill: SkillPropose, Targets: members})
	mustEnd(t, e)
}

func proposeAndApprove(t *testing.T, e *hiddenrole.Engine, members ...string) {
	t.Helper()
	propose(t, e, members...)
	for _, id := range allPlayerIDs(e) {
		mustSubmit(t, e, &hiddenrole.SkillUse{PlayerID: id, Skill: SkillApprove})
	}
	mustEnd(t, e)
}

// TestReadinessKnowsTheWholeTeamIsProposed: readiness can say whether the
// nomination is complete.
//
// **This used to be scar 5**: `SkillUse` carried a single target and the
// leader had to submit N times. That shape was fixed by a sample size of one
// -- werewolf's nine skills happen to have exactly one target each. The
// consequence was readiness knowing only whether the leader had submitted: in
// a seven-player game mission 1 takes 2, and nominating just 1 made it report
// Ready=true.
//
// That is the same class of problem as "AllowedSkills telling an unqualified
// player they may act": the kernel saying something untrue to a player. Since
// scar 1 was fixed by that standard, this one should be too.
//
// One submission now carries the whole team, and nomination and readiness are
// the same thing.
func TestReadinessKnowsTheWholeTeamIsProposed(t *testing.T) {
	e := fivePlayer(t)
	need := MissionSize(5, 1)
	if need < 2 {
		t.Fatalf("this test needs a mission of at least 2, got %d", need)
	}

	if e.PhaseReadiness().Ready {
		t.Fatal("it reported ready before anything was nominated")
	}
	leader := leaderID(e.View())
	mustSubmit(t, e, &hiddenrole.SkillUse{
		PlayerID: leader, Skill: SkillPropose, Targets: []string{"a", "b"},
	})
	if !e.PhaseReadiness().Ready {
		t.Error("the whole team is nominated and it still reports not ready")
	}

	// And what was nominated really is the whole team.
	mustEnd(t, e)
	if got := len(teamIDs(e.View())); got != need {
		t.Errorf("team size = %d, want %d", got, need)
	}
}
