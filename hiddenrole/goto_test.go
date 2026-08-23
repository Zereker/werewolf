package hiddenrole

import "testing"

// gotoAndTrigger emits both a death detour and a next-phase override, so the
// two compete for the same decision.
type gotoAndTrigger struct {
	trigger PhaseType
	goto_   PhaseType
}

func (r gotoAndTrigger) Resolve([]*SkillUse, GameView) []*Effect {
	return []*Effect{
		NewDetourEffect("w1", r.trigger),
		NewGotoPhaseEffect(r.goto_),
	}
}

// TestGotoPhase_TriggerQueueWins: a pending detour outranks a next-phase
// override.
//
// The priority is not arbitrary: the detour queue has to drain, and both the
// victory check and the round boundary wait on it (see advancePhase and
// nextPhase). Jumping away mid-queue with a GOTO would make an unsettled
// death ability vanish -- the exiled hunter's shot disappears, and the rules
// cannot tell.
//
// This test exists because mutation testing found the gap: moving GOTO ahead
// of the detour turned not one test red. A rule written in the documentation
// needs something guarding it, or it is only a sentence.
func TestGotoPhase_TriggerQueueWins(t *testing.T) {
	opts := append(withNoopResolvers(),
		WithResolver(phaseNightGuard, gotoAndTrigger{
			trigger: phaseNightHunter,
			goto_:   phaseDay,
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
	if got := e.Status().Phase; got != phaseNightHunter {
		t.Fatalf("phase = %v, want %v -- the detour queue must outrank GOTO",
			got, phaseNightHunter)
	}

	// Once the detour is settled the GOTO must not linger: the exit goes back
	// to that phase's own NextPhase.
	if _, err := e.EndPhase(); err != nil {
		t.Fatalf("EndPhase: %v", err)
	}
	if got, want := e.Status().Phase, testConfig().Phases[phaseNightHunter].NextPhase; got != want {
		t.Errorf("phase = %v, want %v -- a previous phase's GOTO must not carry over", got, want)
	}
}

// gotoOnly emits only a next-phase override.
type gotoOnly struct{ to PhaseType }

func (r gotoOnly) Resolve([]*SkillUse, GameView) []*Effect {
	return []*Effect{NewGotoPhaseEffect(r.to)}
}

// TestGotoPhase_UnknownTargetFallsBack: a destination absent from the
// configuration falls back to NextPhase.
//
// One malformed effect should not bring the game down, but neither may it
// quietly jump somewhere nobody expected -- the kernel logs an error and
// falls back to the default exit.
func TestGotoPhase_UnknownTargetFallsBack(t *testing.T) {
	rec := &recordingLogger{}
	opts := append(withNoopResolvers(),
		WithResolver(phaseNightGuard, gotoOnly{to: PhaseType("NO_SUCH_PHASE")}),
		WithLogger(rec))
	e := newTestEngine(t, opts...)
	mustAdd(t, e, "w1", roleWerewolf)
	mustAdd(t, e, "g", roleGuard)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := e.EndPhase(); err != nil {
		t.Fatalf("EndPhase: %v", err)
	}

	if got, want := e.Status().Phase, testConfig().Phases[phaseNightGuard].NextPhase; got != want {
		t.Errorf("phase = %v, want a fallback to the default exit %v", got, want)
	}
	if !rec.sawError {
		t.Error("a missing destination should be logged as an error -- falling back silently hides the bug")
	}
}

// TestGotoPhase_CanceledIsIgnored: a vetoed GOTO does not count.
//
// The rules cancelled it themselves, which says that directive should not
// take hold.
func TestGotoPhase_CanceledIsIgnored(t *testing.T) {
	opts := append(withNoopResolvers(),
		WithResolver(phaseNightGuard, canceledGoto{to: phaseVote}))
	e := newTestEngine(t, opts...)
	mustAdd(t, e, "w1", roleWerewolf)
	mustAdd(t, e, "g", roleGuard)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := e.EndPhase(); err != nil {
		t.Fatalf("EndPhase: %v", err)
	}
	if got, want := e.Status().Phase, testConfig().Phases[phaseNightGuard].NextPhase; got != want {
		t.Errorf("phase = %v, want %v -- a vetoed GOTO must not take hold", got, want)
	}
}

type canceledGoto struct{ to PhaseType }

func (r canceledGoto) Resolve([]*SkillUse, GameView) []*Effect {
	ef := NewGotoPhaseEffect(r.to)
	ef.Cancel("the rules withdrew it themselves")
	return []*Effect{ef}
}

// recordingLogger counts how many log lines arrived at each level.
//
// sawError was the original use ("was an error ever logged"); infos verifies
// that an installed logger really is wired up, see
// TestWithLogger_IsActuallyWired.
type recordingLogger struct {
	sawError bool
	infos    int
}

func (l *recordingLogger) Debug(string, ...Field) {}
func (l *recordingLogger) Info(string, ...Field)  { l.infos++ }
func (l *recordingLogger) Warn(string, ...Field)  {}
func (l *recordingLogger) Error(string, ...Field) { l.sawError = true }
