package hiddenrole

import (
	"sync"
	"testing"
)

// TestStatus_SurvivesSnapshot: Status's four fields stay consistent across a
// save round trip.
//
// This one was added after the fact: Status claims its four fields come from
// one instant, and on the **restore** path they never lined up -- the
// snapshot did not carry the winner, so a finished game restored as
// Over=true with an empty Winner.
//
// TestStatus_IsAtomic only covered the "not over yet has a winner" direction;
// the reverse, "over with no winner", was nobody's job and went unnoticed for
// a long time. Both directions are covered now.
func TestStatus_SurvivesSnapshot(t *testing.T) {
	opts := append(withNoopResolvers(),
		WithVictoryChecker(VictoryFunc(func(view GameView) (bool, Camp) {
			return view.Round() > 1, Camp("PROBE")
		})))
	e := newTestEngine(t, opts...)
	mustAdd(t, e, "p1", roleVillager)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	for i := 0; i < 30 && !e.Status().Over; i++ {
		if _, err := e.EndPhase(); err != nil {
			t.Fatalf("EndPhase: %v", err)
		}
	}

	before := e.Status()
	if !before.Over || before.Winner == CampUnspecified {
		t.Fatalf("this test needs a game that is **over and has a winner**, got %+v", before)
	}

	restored, err := RestoreEngine(testConfig(), e.Snapshot(), opts...)
	if err != nil {
		t.Fatalf("RestoreEngine: %v", err)
	}
	if got := restored.Status(); got != before {
		t.Errorf("Status changed across a save round trip: %+v -> %+v", before, got)
	}
}

// TestStatus_IsAtomic: Status's four fields must come from one instant.
//
// This is the **only** reason Phase / Round / IsGameOver / Winner were merged
// into one method: four methods each took their own read lock, a host
// rendering "the day of round 3" had to ask twice, and if another goroutine
// resolved a phase in between, it read a combination of values that never
// held at the same time.
//
// Here one goroutine keeps advancing phases while others keep reading Status,
// asserting that the combination read is always legal: over means stopped at
// PhaseEnd, and not over means no winner yet.
func TestStatus_IsAtomic(t *testing.T) {
	e := newTestEngine(t, append(withNoopResolvers(),
		WithVictoryChecker(VictoryFunc(func(view GameView) (bool, Camp) {
			// End after a few rounds, so the readers get plenty of chances to
			// hit an intermediate state.
			return view.Round() > 3, Camp("PROBE")
		})))...)
	mustAdd(t, e, "p1", roleVillager)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	done := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(done)
		for i := 0; i < 200; i++ {
			if _, err := e.EndPhase(); err != nil {
				return
			}
		}
	}()

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
				}
				st := e.Status()
				if st.Over && st.Winner == CampUnspecified {
					t.Error("read \"over\" with no winner -- the four fields came from different instants")
					return
				}
				if st.Over && st.Phase != PhaseEnd {
					t.Errorf("read \"over\" but the phase is %v -- the four fields came from different instants", st.Phase)
					return
				}
				if !st.Over && st.Winner != CampUnspecified {
					t.Errorf("read \"not over\" yet a winner %v is already set -- the four fields came from different instants", st.Winner)
					return
				}
				if st.Round < 1 {
					t.Errorf("round number %d is invalid", st.Round)
					return
				}
			}
		}()
	}
	wg.Wait()
}
