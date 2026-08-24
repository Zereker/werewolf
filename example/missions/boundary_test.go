package missions

import (
	"encoding/json"
	"testing"

	"github.com/Zereker/hiddenrole"
)

// TestMissionFailIsAnonymous: fail votes are anonymous -- the table learns how
// many there were and never who cast them.
//
// This is a tighter information constraint than anything in werewolf: there, a
// vetoed action is at least known to the actor, and here even the *result* may
// only show as an aggregate. It is not implemented with any help from the
// kernel but by **producing no event per vote at all**: the resolver emits one
// aggregate event carrying the count.
func TestMissionFailIsAnonymous(t *testing.T) {
	e := fivePlayer(t) // d=assassin e=Morgana
	effects := runMission(t, e, 1, "d", "e")

	var failed *hiddenrole.Effect
	for _, ef := range effects {
		if ef.Type == EventMissionFailed {
			failed = ef
		}
		// No effect may tie any person to "voted failure". d is the only one
		// who voted failure this round: apart from being nominated, no effect
		// should have them as its source or target.
		if ef.Type != EventProposed && (ef.SourceID == "d" || ef.TargetID == "d") {
			t.Errorf("an effect points at the player who voted failure: %+v", ef)
		}
	}
	if failed == nil {
		t.Fatalf("the mission should have failed, but there is no MISSION_FAILED among the effects: %v", typesOf(effects))
	}
	if failed.SourceID != "" || failed.TargetID != "" {
		t.Errorf("the failure event should carry no source or target, got source=%q target=%q",
			failed.SourceID, failed.TargetID)
	}
	if failed.Data["fails"] != "1" {
		t.Errorf("fail votes = %v, want 1", failed.Data["fails"])
	}

	// The whole table should see it, and see exactly the same thing.
	audience, known := e.AudienceOf(failed.ToEvent())
	if !known || len(audience) != 5 {
		t.Errorf("the failure event should be visible to everyone, got known=%v audience=%v", known, audience)
	}
}

// TestGoodPlayerCannotFail: a good player's fail vote is vetoed, and only they
// know it.
//
// "Only they know" is necessary: if anyone else could see that somebody tried
// to vote failure and was stopped, it would name them on the spot -- only a
// good player is ever stopped.
func TestGoodPlayerCannotFail(t *testing.T) {
	e := fivePlayer(t) // c=loyal servant (good)
	leader := leaderID(e.View())
	mustSubmit(t, e, &hiddenrole.SkillUse{PlayerID: leader, Skill: SkillPropose, Targets: []string{"c", "d"}})
	mustEnd(t, e)
	for _, id := range e.AlivePlayerIDs() {
		mustSubmit(t, e, &hiddenrole.SkillUse{PlayerID: id, Skill: SkillApprove})
	}
	mustEnd(t, e)

	// The good player c tries to vote failure.
	mustSubmit(t, e, &hiddenrole.SkillUse{PlayerID: "c", Skill: SkillMissionFail})
	mustSubmit(t, e, &hiddenrole.SkillUse{PlayerID: "d", Skill: SkillMissionSuccess})
	effects := mustEnd(t, e)

	var rejected *hiddenrole.Effect
	for _, ef := range effects {
		if ef.Type == EventFailRejected {
			rejected = ef
		}
	}
	if rejected == nil {
		t.Fatalf("a good player's fail vote should be vetoed: %v", typesOf(effects))
	}
	if !rejected.Canceled {
		t.Error("the vetoing effect should carry Canceled")
	}
	audience, known := e.AudienceOf(rejected.ToEvent())
	if !known || len(audience) != 1 || audience[0] != "c" {
		t.Errorf("the veto should go to c alone, got known=%v audience=%v", known, audience)
	}

	// And this mission should count as a success -- a good player's fail vote
	// does not take effect.
	var succeeded bool
	for _, ef := range effects {
		if ef.Type == EventMissionSucceeded {
			succeeded = true
		}
	}
	if !succeeded {
		t.Errorf("a good player's fail vote should not fail the mission: %v", typesOf(effects))
	}
}

// TestSnapshotAndReplay: does the workaround survive persistence.
//
// The game's progress was filed under one player's own state (see SCARS.md,
// scar 4). The workaround holds only if that state enters the snapshot and the
// effect log. This test watches that premise -- and shows at the same time
// that scar 4 is ugly without breaking any of the kernel's promises.
func TestSnapshotAndReplay(t *testing.T) {
	e := fivePlayer(t)
	runMission(t, e, 0, "a", "b")
	runMission(t, e, 1, "d", "e", "a") // one success, one failure; the score is 1:1

	wantMission, wantSucc, wantFail := mission(e.View()), successes(e.View()), failures(e.View())
	if wantMission != 3 || wantSucc != 1 || wantFail != 1 {
		t.Fatalf("premise broken: mission %d, %d succeeded, %d failed", wantMission, wantSucc, wantFail)
	}

	// 1. A snapshot round trip.
	raw, err := json.Marshal(e.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	var snap hiddenrole.Snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatal(err)
	}
	restored, err := Restore(&snap)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	checkProgress(t, "restore", restored, wantMission, wantSucc, wantFail)

	// 2. An effect-log replay.
	replayed, err := Replay(e.EffectLog())
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	checkProgress(t, "replay", replayed, wantMission, wantSucc, wantFail)

	// 3. The restored engine plays on and reaches the same result.
	runMission(t, restored, 0, "a", "b")
	runMission(t, e, 0, "a", "b")
	if a, b := successes(e.View()), successes(restored.View()); a != b {
		t.Errorf("success counts differ after playing on: original %d, restored %d", a, b)
	}
}

func checkProgress(t *testing.T, what string, e *hiddenrole.Engine, m, s, f int) {
	t.Helper()
	v := e.View()
	if mission(v) != m || successes(v) != s || failures(v) != f {
		t.Errorf("progress does not match after %s: mission %d with %d succeeded and %d failed, want mission %d with %d succeeded and %d failed",
			what, mission(v), successes(v), failures(v), m, s, f)
	}
}
