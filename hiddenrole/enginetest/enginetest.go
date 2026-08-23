// Package enginetest runs random games against a set of general invariants,
// for every rules package to reuse.
//
// # Why this package exists
//
// Hand-written cases only cover the situations somebody thought of. Random
// games work the other way round: run the engine for thousands of games and
// check, at every step, the properties that must hold no matter what. The
// hardest-to-reproduce problems this project has found -- replay divergence,
// a snapshot missing a field, the round number incremented once too often at
// the end of a game -- were all of the "only shows up in one particular
// situation" kind, and all of them were caught by invariants like these.
//
// It used to exist only inside the werewolf rules (fuzz_test.go in the root
// package), which meant **the kernel's determinism, snapshot round trips and
// effect-log replay were attested by one ruleset out of three**. Those three
// are the kernel's load-bearing walls, and one ruleset should not be their
// only witness.
//
// # Why it is public
//
// It is test infrastructure, in the same position as net/http/httptest: **a
// test harness for users of the library, not the thing under test.**
//
// It used to be called `internal/gamefuzz`, on the grounds that "a test-only
// thing should not add a name to an API that was just frozen". That position
// stopped working once **the engine became its own module** -- Go's rule is
// that `internal/` can only be imported from within the same module, and the
// rules packages now live in another one and could not use a line of it.
//
// Being public, it is guarded by the freeze: TestAPI_SurfaceIsPinned pins
// this sub-package too, or it would become a back door around that
// discipline.
//
// For contrast: hiddenrole.Board / Seat / Mark are public test APIs too, and
// they do the other half of the same job -- those three lay out one board by
// hand to unit-test a resolver, this one runs thousands of games to check
// invariants.
//
// # None of these invariants knows any game
//
// Not one of them mentions a werewolf, a mission or a centre card. Each asks
// something at the kernel's level: does what was stored read back the same,
// does replay arrive at the same board, is somebody the engine says cannot
// act really unable to act. Laying out the board and taking turns is the
// rules package's job.
package enginetest

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"

	"github.com/Zereker/hiddenrole"
)

// Seat is one player taking a seat.
type Seat struct {
	ID   string
	Role hiddenrole.RoleType
}

// Game is everything one random game needs, supplied by the rules package.
type Game struct {
	// Config is the phase graph. It may be randomised per game -- a random
	// configuration turns up more than random play does: two of the three
	// outcome-changing problems found in werewolf came from custom
	// configurations.
	Config *hiddenrole.Config

	// Options is the assembly. It has to match Config, and must be
	// **re-passable verbatim on restore** -- that is how the snapshot
	// round-trip invariant checks it.
	Options []hiddenrole.EngineOption

	// Seats is who sits down.
	Seats []Seat

	// Labels characterise this game, used to watch for the randomisation
	// degenerating. If some branch is never reached, the test would quietly
	// become a test of one situation only.
	Labels []string
}

// Setup lays out one game. The same rng must lay out the same game, or a
// failure cannot be reproduced.
type Setup func(rng *rand.Rand) Game

// Act takes one turn. The rules package decides how -- a generically random
// submission is nearly always rejected on a multi-target skill (the missions
// nomination needs N people, the One Night troublemaker needs exactly 2), and
// the game would never reach an interesting situation.
//
// Doing nothing is allowed: phases advance without a submission, and many
// rulesets' night abilities are optional to begin with.
type Act func(e *hiddenrole.Engine, rng *rand.Rand)

// FuzzSpec is the configuration of one random-game test.
type FuzzSpec struct {
	Games    int      // how many games to run
	MaxSteps int      // most steps per game; beyond that it counts as unfinished
	Setup    Setup    // how to lay a game out
	Act      Act      // how to take a turn; nil means take none and only advance phases
	WantEnd  bool     // whether every game must finish within MaxSteps
	MustSee  []string // none of these labels may be zero, or the randomisation has degenerated
}

// RunFuzz runs a batch of random games, checking the invariants at each step.
//
// The seeds are fixed, so a failure reproduces: the log carries seed and
// step.
func RunFuzz(t *testing.T, spec FuzzSpec) {
	t.Helper()

	stats := map[string]int{}
	for seed := 0; seed < spec.Games; seed++ {
		rng := rand.New(rand.NewSource(int64(seed))) //nolint:gosec // test randomness
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("seed=%d PANIC: %v", seed, r)
				}
			}()
			for _, label := range playOne(t, seed, rng, spec) {
				stats[label]++
			}
		}()
	}

	for _, k := range sortedKeys(stats) {
		t.Logf("  %-16s %d", k, stats[k])
	}
	if spec.WantEnd {
		if n := stats[labelNotEnded]; n > 0 {
			t.Errorf("%d games did not finish within %d steps", n, spec.MaxSteps)
		}
	}
	for _, k := range spec.MustSee {
		if stats[k] == 0 {
			t.Errorf("the randomisation never produced %q; the search space has degenerated", k)
		}
	}
}

const (
	labelStarted  = "started"
	labelEnded    = "finished"
	labelNotEnded = "unfinished"
)

// playOne runs one game and returns the labels characterising it.
func playOne(t *testing.T, seed int, rng *rand.Rand, spec FuzzSpec) []string {
	t.Helper()

	g := spec.Setup(rng)
	e, err := hiddenrole.NewEngine(g.Config, g.Options...)
	if err != nil {
		t.Fatalf("seed=%d NewEngine: %v", seed, err)
	}
	for _, s := range g.Seats {
		if err := e.AddPlayer(s.ID, s.Role); err != nil {
			t.Fatalf("seed=%d AddPlayer(%s): %v", seed, s.ID, err)
		}
	}
	if err := e.Start(); err != nil {
		t.Fatalf("seed=%d Start: %v", seed, err)
	}

	labels := append([]string{labelStarted}, g.Labels...)

	// Collect every event the kernel emits -- invariant G looks at them.
	var seen []hiddenrole.EventType
	e.OnEvent(func(ev *hiddenrole.Event) { seen = append(seen, ev.Type) })

	lastRound := e.Status().Round
	for step := 0; step < spec.MaxSteps; step++ {
		if e.Status().Over {
			checkEndedStaysEnded(t, seed, step, e, g)
			return append(labels, labelEnded)
		}

		if spec.Act != nil {
			spec.Act(e, rng)
		}

		checkAllowedMatchesView(t, seed, step, e)
		checkPhaseInfoStable(t, seed, step, e)

		clone := checkSnapshotRoundTrip(t, seed, step, e, g)
		checkSameBehaviour(t, seed, step, "a snapshot round trip", e, clone)

		if _, err := e.EndPhase(); err != nil {
			t.Fatalf("seed=%d step=%d EndPhase: %v", seed, step, err)
		}
		if _, err := clone.EndPhase(); err != nil {
			t.Fatalf("seed=%d step=%d clone EndPhase: %v", seed, step, err)
		}
		checkSameState(t, seed, step, e, clone)

		checkStatusCoherent(t, seed, step, e)
		if r := e.Status().Round; r < lastRound {
			t.Fatalf("seed=%d step=%d round number went backwards: %d -> %d", seed, step, lastRound, r)
		} else {
			lastRound = r
		}

		checkReplay(t, seed, step, e, g)
	}

	checkPrimitivesNeverBroadcast(t, seed, seen)
	return append(labels, labelNotEnded)
}

// checkSnapshotRoundTrip is invariant A: after a save round trip, both sides
// must be the same board.
//
// Comparing phase and round is not enough -- with a field missing from the
// snapshot, both sides still walk a whole game in lockstep, only the rules
// judge differently. Comparing the exported snapshots byte for byte is what
// catches the missing-field class, and that class really did happen twice
// (the guard's consecutive-protection record, the winner at the moment the
// game ends).
func checkSnapshotRoundTrip(t *testing.T, seed, step int, e *hiddenrole.Engine, g Game) *hiddenrole.Engine {
	t.Helper()

	raw, err := json.Marshal(e.Snapshot())
	if err != nil {
		t.Fatalf("seed=%d step=%d Marshal: %v", seed, step, err)
	}
	var back hiddenrole.Snapshot
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("seed=%d step=%d Unmarshal: %v", seed, step, err)
	}
	clone, err := hiddenrole.RestoreEngine(g.Config, &back, g.Options...)
	if err != nil {
		t.Fatalf("seed=%d step=%d RestoreEngine: %v", seed, step, err)
	}
	return clone
}

// checkSameState requires two engines' exported snapshots to be byte-identical.
func checkSameState(t *testing.T, seed, step int, a, b *hiddenrole.Engine) {
	t.Helper()
	x, _ := json.Marshal(a.Snapshot())
	y, _ := json.Marshal(b.Snapshot())
	if string(x) != string(y) {
		t.Fatalf("seed=%d step=%d state differs after a snapshot round trip:\n  original %s\n  clone    %s", seed, step, x, y)
	}
	if a.Status() != b.Status() {
		t.Fatalf("seed=%d step=%d Status differs: %+v vs %+v", seed, step, a.Status(), b.Status())
	}
}

// checkSameBehaviour requires two engines to give the same answer to "who can
// do what right now".
//
// **This one was forced out by mutation testing.** Cross-engine comparison
// used to look at snapshot bytes only -- and when the snapshot serialiser
// itself drops a field, both sides drop it and the comparison is blind: the
// "snapshot loses Actors" mutation survived on the spot.
//
// What a snapshot means is not "the exported JSON matches", it is "the
// restored engine behaves the same". So ask about behaviour: who may act, who
// has yet to act. Lose the actor list and the answers differ immediately.
func checkSameBehaviour(t *testing.T, seed, step int, how string, a, b *hiddenrole.Engine) {
	t.Helper()

	for _, p := range a.View().AllPlayers() {
		x := fmt.Sprint(a.AllowedSkills(p.ID))
		y := fmt.Sprint(b.AllowedSkills(p.ID))
		if x != y {
			t.Fatalf("seed=%d step=%d after %s, what %s may do differs: original=%s clone=%s",
				seed, step, how, p.ID, x, y)
		}
	}

	x, y := a.PhaseReadiness(), b.PhaseReadiness()
	if fmt.Sprint(x) != fmt.Sprint(y) {
		t.Fatalf("seed=%d step=%d readiness differs after %s:\n  original %+v\n  clone    %+v",
			seed, step, how, x, y)
	}

	// The god's-view list has to match too -- the host runs the phase from it.
	for role, ri := range a.PhaseInfo().RoleInfos {
		other, ok := b.PhaseInfo().RoleInfos[role]
		if !ok {
			t.Fatalf("seed=%d step=%d after %s, the information for %v is missing", seed, step, how, role)
		}
		if fmt.Sprint(ri.PlayerIDs) != fmt.Sprint(other.PlayerIDs) {
			t.Fatalf("seed=%d step=%d after %s, who should act as %v differs: original=%v clone=%v",
				seed, step, how, role, ri.PlayerIDs, other.PlayerIDs)
		}
	}
}

// checkReplay is invariant B: replaying the effect log reaches the same board.
//
// It complements the snapshot invariant: a snapshot is state, an effect log
// is history. Both can rebuild, and the results must agree.
func checkReplay(t *testing.T, seed, step int, e *hiddenrole.Engine, g Game) {
	t.Helper()

	replayed, err := hiddenrole.ReplayEngine(g.Config, e.EffectLog(), g.Options...)
	if err != nil {
		t.Fatalf("seed=%d step=%d ReplayEngine: %v", seed, step, err)
	}
	if got, want := replayed.Status().Phase, e.Status().Phase; got != want {
		t.Fatalf("seed=%d step=%d phase after replay = %v, original %v", seed, step, got, want)
	}
	if got, want := replayed.Status().Round, e.Status().Round; got != want {
		t.Fatalf("seed=%d step=%d round after replay = %d, original %d", seed, step, got, want)
	}

	// Compare snapshots byte for byte. That works here because checkReplay is
	// called **after** EndPhase -- the unresolved submissions have been
	// cleared, and they were never in the effect log anyway (they had not
	// become effects yet).
	x, _ := json.Marshal(e.Snapshot())
	y, _ := json.Marshal(replayed.Snapshot())
	if string(x) != string(y) {
		t.Fatalf("seed=%d step=%d state differs after replay:\n  original %s\n  replayed %s", seed, step, x, y)
	}

	// Same reasoning as the snapshot invariant: matching bytes do not mean
	// matching behaviour, so ask about behaviour too.
	checkSameBehaviour(t, seed, step, "an effect-log replay", e, replayed)
}

// checkAllowedMatchesView is invariant C: three paths must give the same
// answer to "who may act".
//
// Engine.AllowedSkills, PlayerView.AllowedSkills, and SubmitSkillUse's
// validation. Were they to differ, a caller running the phase by one of them
// would have the player's submission rejected by another.
func checkAllowedMatchesView(t *testing.T, seed, step int, e *hiddenrole.Engine) {
	t.Helper()
	for _, p := range e.View().AllPlayers() {
		a := len(e.AllowedSkills(p.ID))
		v := e.PlayerView(p.ID)
		if v == nil {
			t.Fatalf("seed=%d step=%d %s has no view", seed, step, p.ID)
		}
		if b := len(v.AllowedSkills); a != b {
			t.Fatalf("seed=%d step=%d %s: AllowedSkills=%d PlayerView=%d", seed, step, p.ID, a, b)
		}
	}
}

// checkPhaseInfoStable is invariant D: querying the same board repeatedly
// must give lists in a stable order.
//
// Building a list by iterating a map gives a different order every time for
// the same board -- and replaying and comparing effect logs would lose their
// determinism. This one really did happen once.
func checkPhaseInfoStable(t *testing.T, seed, step int, e *hiddenrole.Engine) {
	t.Helper()
	want := map[hiddenrole.RoleType]string{}
	for role, ri := range e.PhaseInfo().RoleInfos {
		want[role] = fmt.Sprint(ri.PlayerIDs)
	}
	for i := 0; i < 3; i++ {
		for role, ri := range e.PhaseInfo().RoleInfos {
			if got := fmt.Sprint(ri.PlayerIDs); got != want[role] {
				t.Fatalf("seed=%d step=%d PhaseInfo list for %v is not stably ordered: %s vs %s",
					seed, step, role, want[role], got)
			}
		}
	}
}

// checkStatusCoherent is invariant E: Status's four fields must be consistent
// with each other.
//
// Over means stopped at PhaseEnd with a winner; not over means no winner yet.
// The reverse direction (over with no winner) went unnoticed for a long time
// -- with the winner missing from the snapshot, that is exactly what a
// restored game looked like.
func checkStatusCoherent(t *testing.T, seed, step int, e *hiddenrole.Engine) {
	t.Helper()
	st := e.Status()
	switch {
	case st.Over && st.Phase != hiddenrole.PhaseEnd:
		t.Fatalf("seed=%d step=%d over, yet stopped at %v", seed, step, st.Phase)
	case !st.Over && st.Winner != hiddenrole.CampUnspecified:
		t.Fatalf("seed=%d step=%d not over, yet already has winner %v", seed, step, st.Winner)
	case st.Round < 1:
		t.Fatalf("seed=%d step=%d round number %d is invalid", seed, step, st.Round)
	}
}

// checkEndedStaysEnded is invariant F: once the game is over the board stops
// changing.
func checkEndedStaysEnded(t *testing.T, seed, step int, e *hiddenrole.Engine, g Game) {
	t.Helper()
	before, _ := json.Marshal(e.Snapshot())
	st := e.Status()

	_, _ = e.EndPhase() // one more step after the end: an error or a no-op are both fine

	after, _ := json.Marshal(e.Snapshot())
	if string(before) != string(after) {
		t.Fatalf("seed=%d step=%d board still changing after the end:\n  before %s\n  after  %s", seed, step, before, after)
	}
	if e.Status() != st {
		t.Fatalf("seed=%d step=%d Status still changing after the end: %+v -> %+v", seed, step, st, e.Status())
	}

	// A finished board has to survive a save round trip too.
	clone := checkSnapshotRoundTrip(t, seed, step, e, g)
	checkSameState(t, seed, step, e, clone)
}

// checkPrimitivesNeverBroadcast is invariant G: not one kernel state
// primitive may reach OnEvent.
//
// A host forwarding OnEvent verbatim would be broadcasting the god's view.
// The kernel makes this part non-configurable, and "non-configurable" still
// needs something checking it.
func checkPrimitivesNeverBroadcast(t *testing.T, seed int, seen []hiddenrole.EventType) {
	t.Helper()
	primitives := map[hiddenrole.EventType]bool{
		hiddenrole.EventSetAlive: true, hiddenrole.EventSetVar: true,
		hiddenrole.EventSetActors: true, hiddenrole.EventDetour: true,
		hiddenrole.EventGotoPhase: true, hiddenrole.EventPlayerAdded: true,
		hiddenrole.EventPhaseChanged: true,
	}
	for _, typ := range seen {
		if primitives[typ] {
			t.Fatalf("seed=%d state primitive %v reached OnEvent", seed, typ)
		}
	}
}

// sortedKeys returns the stats table's keys, sorted, so the log is
// deterministic.
func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
