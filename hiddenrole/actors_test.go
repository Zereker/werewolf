package hiddenrole

import "testing"

// setActorsOnce names actors on the first resolution only, and never again.
type setActorsOnce struct {
	phase PhaseType
	ids   []string
	done  *bool
}

func (r setActorsOnce) Resolve([]*SkillUse, GameView) []*Effect {
	if *r.done {
		return nil
	}
	*r.done = true
	return []*Effect{NewSetActorsEffect(r.phase, r.ids...)}
}

// TestSetActors_IsConsumedAfterThePhaseResolves: a list is spent on use and
// does not carry into the next visit.
//
// An actor list is nearly always "computed this round" -- the missions
// package's team is chosen by this round's nomination, its leader is this
// round's rotation. Inheriting the previous round's list is nearly always
// wrong, and wrong in a well-hidden way: the game runs on as usual, only with
// a different set of people acting who should not be.
//
// This test was forced out by mutation testing: with consumeActors removed,
// **not one test went red**, because both rules packages name actors again
// before every visit to that phase, so a stale list was always overwritten. A
// rule with no test is only a comment.
func TestSetActors_IsConsumedAfterThePhaseResolves(t *testing.T) {
	done := false
	opts := append(withNoopResolvers(),
		WithResolver(phaseNightGuard, setActorsOnce{
			phase: phaseNightWolf, ids: []string{"w1"}, done: &done,
		}))
	e := newTestEngine(t, opts...)
	mustAdd(t, e, "w1", roleWerewolf)
	mustAdd(t, e, "w2", roleWerewolf)
	mustAdd(t, e, "g", roleGuard)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// First visit to the wolf phase: the list names w1, so w2 must not act.
	if _, err := e.EndPhase(); err != nil { // NIGHT_GUARD -> NIGHT_WOLF
		t.Fatalf("EndPhase: %v", err)
	}
	if e.Status().Phase != phaseNightWolf {
		t.Fatalf("phase = %v, want %v", e.Status().Phase, phaseNightWolf)
	}
	if got := e.AllowedSkills("w2"); len(got) != 0 {
		t.Fatalf("w2 is not on the named list, yet AllowedSkills gave %v", got)
	}

	// Go all the way round back to the wolf phase. This time nobody names
	// actors, so it should fall back to computing by role -- both wolves can
	// act. Inheriting the previous round's list would keep w2 shut out.
	for i := 0; i < 20 && e.Status().Phase != phaseNightWolf; i++ {
		if _, err := e.EndPhase(); err != nil {
			t.Fatalf("EndPhase: %v", err)
		}
	}
	for i := 0; i < 20; i++ {
		if _, err := e.EndPhase(); err != nil {
			t.Fatalf("EndPhase: %v", err)
		}
		if e.Status().Phase == phaseNightWolf {
			break
		}
	}
	if e.Status().Phase != phaseNightWolf {
		t.Fatalf("never got back round to the wolf phase, stopped at %v", e.Status().Phase)
	}
	if got := e.AllowedSkills("w2"); len(got) == 0 {
		t.Error("nobody named actors this round, so w2 should be able to act by role -- the previous round's list was inherited")
	}
}

// TestSetActors_EmptyListIsNotTheSameAsUnset: naming an empty list is not the
// same as naming nobody.
//
// "The rules said, and nobody can act in this phase" and "the rules did not
// say, compute by role" are two different things. Representing the former as
// nil collapses it into the latter, and suddenly the whole table can act.
func TestSetActors_EmptyListIsNotTheSameAsUnset(t *testing.T) {
	done := false
	opts := append(withNoopResolvers(),
		WithResolver(phaseNightGuard, setActorsOnce{
			phase: phaseNightWolf, ids: nil, done: &done,
		}))
	e := newTestEngine(t, opts...)
	mustAdd(t, e, "w1", roleWerewolf)
	mustAdd(t, e, "g", roleGuard)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := e.EndPhase(); err != nil {
		t.Fatalf("EndPhase: %v", err)
	}
	if got := e.AllowedSkills("w1"); len(got) != 0 {
		t.Errorf("an empty list was named, so nobody can act, yet w1 got %v", got)
	}
}

// detourTwice enqueues two players into the same phase on the first
// resolution.
type detourTwice struct {
	phase PhaseType
	a, b  string
	done  *bool
}

func (r detourTwice) Resolve([]*SkillUse, GameView) []*Effect {
	if *r.done {
		return nil
	}
	*r.done = true
	return []*Effect{
		NewDetourEffect(r.a, r.phase),
		NewDetourEffect(r.b, r.phase),
	}
}

// TestDetours_QueuedForTheSamePhaseEachGetTheirTurn
// Two detours enqueued into the same phase on the same night must each get
// their own turn; the last one must not be all that is left.
//
// The detour queue no longer answers "who may act" itself: **on entering a
// phase** it writes an actor list from the head of the queue
// (gameState.nameDetourActor). It is written on entering the phase rather
// than at the DETOUR write point precisely because of this test: with two
// hunters eliminated on one night the queue holds two detours pointing at the
// same phase, and writing at the enqueue point would have them overwrite each
// other, leaving only the second able to shoot while the first one's shot
// vanishes.
func TestDetours_QueuedForTheSamePhaseEachGetTheirTurn(t *testing.T) {
	done := false
	opts := append(withNoopResolvers(),
		WithResolver(phaseNightResolve, detourTwice{
			phase: phaseNightHunter, a: "h1", b: "h2", done: &done,
		}))
	e := newTestEngine(t, opts...)
	mustAdd(t, e, "h1", roleHunter)
	mustAdd(t, e, "h2", roleHunter)
	mustAdd(t, e, "v1", roleVillager)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Advance to the end of the resolution phase, enqueueing both detours.
	for i := 0; i < 20 && e.Status().Phase != phaseNightResolve; i++ {
		if _, err := e.EndPhase(); err != nil {
			t.Fatalf("EndPhase: %v", err)
		}
	}
	if _, err := e.EndPhase(); err != nil { // NIGHT_RESOLVE -> NIGHT_HUNTER
		t.Fatalf("EndPhase: %v", err)
	}

	// First trip: the head is h1, and only h1 can act.
	if e.Status().Phase != phaseNightHunter {
		t.Fatalf("the first detour did not route to the hunter phase, got %v", e.Status().Phase)
	}
	assertOnlyActor(t, e, "h1", "first trip")

	if _, err := e.EndPhase(); err != nil { // consume h1, leaving h2 in the queue
		t.Fatalf("EndPhase: %v", err)
	}

	// Second trip: it must come back to the same phase, and this time it is
	// h2's turn.
	if e.Status().Phase != phaseNightHunter {
		t.Fatalf("the second detour did not route back to the hunter phase, got %v -- "+
			"with two detours pointing at one phase, the second overwrote the first", e.Status().Phase)
	}
	assertOnlyActor(t, e, "h2", "second trip")

	if _, err := e.EndPhase(); err != nil { // consume h2, draining the queue
		t.Fatalf("EndPhase: %v", err)
	}
	if e.Status().Phase == phaseNightHunter {
		t.Error("the queue is drained and it should not return to the hunter phase")
	}
}

// assertOnlyActor: exactly one player, want, can act in this phase, and all
// three paths agree.
//
// The three paths are AllowedSkills, PhaseInfo and SubmitSkillUse's
// validation. They each used to carry their own three-layer decision and now
// share the single actorsForStep read point -- but whether they really share
// it has to be said by a test; reading the code does not show it.
func assertOnlyActor(t *testing.T, e *Engine, want, when string) {
	t.Helper()

	if got := e.AllowedSkills(want); len(got) == 0 {
		t.Errorf("%s: %s should be able to act, yet AllowedSkills is empty", when, want)
	}
	ids := e.PhaseInfo().RoleInfos[roleHunter].PlayerIDs
	if len(ids) != 1 || ids[0] != want {
		t.Errorf("%s: %s should act in this phase, PhaseInfo gave %v", when, want, ids)
	}
	for _, other := range []string{"h1", "h2", "v1"} {
		if other == want {
			continue
		}
		if got := e.AllowedSkills(other); len(got) != 0 {
			t.Errorf("%s: %s should not be able to act, yet AllowedSkills gave %v", when, other, got)
		}
		err := e.SubmitSkillUse(&SkillUse{PlayerID: other, Skill: skillShoot, Targets: []string{"v1"}})
		if err == nil {
			t.Errorf("%s: %s should not be able to act, yet the submission was accepted", when, other)
		}
	}
}

// TestNameDetourActor_OnlyNamesItsOwnPhase: a detour names actors only in its
// own phase.
//
// Normal progression cannot get past this: while the queue is non-empty,
// calculateNextPhase always makes the next stop the head's phase, so
// "entering some other phase with a detour still pending" is unreachable. But
// that is precisely nameDetourActor's premise, and a premise written into the
// code deserves a test pinning it -- otherwise somebody later changes the
// transition order (letting GOTO_PHASE outrank the queue, say), the detour's
// player gets named in a completely unrelated phase, and every integration
// test stays green.
//
// So this goes around the transition and calls the state directly, checking
// the function's own contract.
func TestNameDetourActor_OnlyNamesItsOwnPhase(t *testing.T) {
	s := newState()
	mustAddTo(t, s, "h1", roleHunter)
	s.startAt(phaseNight)
	s.applyEffect(NewDetourEffect("h1", phaseNightHunter))

	// Enter a phase unrelated to the detour: nobody should be named.
	s.nextPhase(phaseDay, false, false)
	if ids, ok := s.actorsFor(phaseDay); ok {
		t.Errorf("the detour points at %v, yet entering %v named actors: %v", phaseNightHunter, phaseDay, ids)
	}

	// Enter the phase the detour is for: its player is named.
	s.nextPhase(phaseNightHunter, false, false)
	ids, ok := s.actorsFor(phaseNightHunter)
	if !ok || len(ids) != 1 || ids[0] != "h1" {
		t.Errorf("entering %v should name h1, got %v (present=%v)", phaseNightHunter, ids, ok)
	}
}
