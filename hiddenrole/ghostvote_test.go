package hiddenrole

import "testing"

// ghostVote lets an eliminated player act in the vote phase -- Blood on the
// Clocktower's ghost vote.
type ghostVote struct {
	dead  string
	alive string
	done  *bool
}

func (r ghostVote) Resolve(_ []*SkillUse, _ GameView) []*Effect {
	if *r.done {
		return nil
	}
	*r.done = true
	return []*Effect{
		NewSetAliveEffect(r.dead, false),
		NewSetActorsEffect(phaseVote, r.dead, r.alive),
	}
}

// TestNamedActorsMayBeDead: whoever the rules name may act -- the eliminated
// included.
//
// **Being alive is the default qualification to act, not the law.** Only the
// kernel's own detour queue used to be able to step over it (the hunter
// shooting after being killed) while the rules naming actors could not -- one
// kernel letting its own mechanism move the dead while forbidding the rules'
// mechanism from doing the same is the kernel deciding "may the dead act" on
// the rules' behalf.
//
// What that blocks is real play: the dead in Blood on the Clocktower keep a
// ghost vote, and werewolf has a last-words phase. This test is that ghost
// vote.
func TestNamedActorsMayBeDead(t *testing.T) {
	done := false
	opts := append(withNoopResolvers(),
		WithResolver(phaseNightGuard, ghostVote{dead: "w1", alive: "g", done: &done}))
	e := newTestEngine(t, opts...)
	mustAdd(t, e, "w1", roleWerewolf)
	mustAdd(t, e, "w2", roleWerewolf)
	mustAdd(t, e, "g", roleGuard)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := e.EndPhase(); err != nil { // resolve: w1 is eliminated, and the vote phase gets named actors
		t.Fatalf("EndPhase: %v", err)
	}
	if p, _ := e.PlayerInfo("w1"); p.Alive {
		t.Fatal("premise broken: w1 should be eliminated")
	}

	// Advance to the vote phase.
	for i := 0; i < 20 && e.Status().Phase != phaseVote; i++ {
		if _, err := e.EndPhase(); err != nil {
			t.Fatalf("EndPhase: %v", err)
		}
	}
	if e.Status().Phase != phaseVote {
		t.Fatalf("never reached the vote phase, stopped at %v", e.Status().Phase)
	}

	// w1 is eliminated but named, so w1 may vote.
	if got := e.AllowedSkills("w1"); len(got) == 0 {
		t.Error("w1 is eliminated but named by the rules and should be able to vote -- AllowedSkills is empty")
	}

	// Readiness counts w1 among those still to act -- checked **before** the
	// vote is cast.
	//
	// Where this line sits was forced out by mutation testing: it used to be
	// after the vote, by which point w1 had acted, and "filter w1 out by
	// aliveness" and "correctly wait for w1" looked identical -- the mutation
	// slipped straight under the test.
	pendingBefore := map[string]bool{}
	for _, p := range e.PhaseReadiness().Pending {
		pendingBefore[p.PlayerID] = true
	}
	if !pendingBefore["w1"] {
		t.Errorf("readiness should be waiting on the eliminated-but-named w1, it is waiting on %v", pendingBefore)
	}

	if err := e.SubmitSkillUse(&SkillUse{
		PlayerID: "w1", Skill: skillVote, Targets: []string{"g"},
	}); err != nil {
		t.Errorf("w1's ghost vote was rejected: %v", err)
	}

	// w2 is alive but not named and may not vote -- naming is an allow-list,
	// not an "add these as well".
	if got := e.AllowedSkills("w2"); len(got) != 0 {
		t.Errorf("w2 was not named and should not be able to vote, got %v", got)
	}

	// Readiness honours the same list.
	for _, p := range e.PhaseReadiness().Pending {
		if p.PlayerID != "g" {
			t.Errorf("readiness is waiting on %s, but the list holds only w1 and g (and w1 has voted)", p.PlayerID)
		}
	}
}
