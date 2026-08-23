package hiddenrole

import "testing"

// effectProducer emits one effect carrying Data, so that EndPhase has
// something to return.
type effectProducer struct{ tag string }

func (r effectProducer) Resolve([]*SkillUse, GameView) []*Effect {
	return []*Effect{
		NewEffect(EventType("PROBE"), "src", "dst").WithData("tag", r.tag),
	}
}

// TestEffectLog_HistoryIsNotWritableFromOutside: the effect log is history,
// and history cannot be rewritten from outside.
//
// This invariant used to live only in the documentation: what EndPhase
// returned and what EffectLog returned were the very same pointers as the
// engine's own history. A caller changing one field -- or calling Cancel,
// which is an exported method -- rewrote the engine's history, and a replay
// would rebuild a different game from it. Replayability and auditability were
// both forfeit.
//
// Copies now go into the log and copies come out. This test watches both
// sides.
func TestEffectLog_HistoryIsNotWritableFromOutside(t *testing.T) {
	opts := append(withNoopResolvers(), WithResolver(phaseNightGuard, effectProducer{tag: "original"}))
	e := newTestEngine(t, opts...)
	mustAdd(t, e, "w1", roleWerewolf)
	mustAdd(t, e, "g", roleGuard)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	effects, err := e.EndPhase()
	if err != nil {
		t.Fatalf("EndPhase: %v", err)
	}
	probe := findProbe(t, effects)
	before := len(e.EffectLog())

	// 1. Modify the batch EndPhase handed out.
	probe.TargetID = "modified"
	probe.Cancel("modified from outside")
	probe.Data["tag"] = "modified"
	assertHistoryIntact(t, e, before, "EndPhase's return value")

	// 2. Modify the batch EffectLog handed out.
	log := e.EffectLog()
	logged := findProbe(t, log)
	logged.TargetID = "modified"
	logged.Cancel("modified from outside")
	logged.Data["tag"] = "modified"
	assertHistoryIntact(t, e, before, "EffectLog's return value")
}

func findProbe(t *testing.T, effects []*Effect) *Effect {
	t.Helper()
	for _, ef := range effects {
		if ef.Type == EventType("PROBE") {
			return ef
		}
	}
	t.Fatalf("probe effect not found among %d effects", len(effects))
	return nil
}

func assertHistoryIntact(t *testing.T, e *Engine, wantLen int, via string) {
	t.Helper()
	log := e.EffectLog()
	if len(log) != wantLen {
		t.Fatalf("via %s: history length went from %d to %d", via, wantLen, len(log))
	}
	ef := findProbe(t, log)
	switch {
	case ef.TargetID != "dst":
		t.Errorf("via %s: TargetID in the history was changed to %q", via, ef.TargetID)
	case ef.Canceled:
		t.Errorf("via %s: the effect in the history was cancelled, Reason=%q", via, ef.Reason)
	case ef.Data["tag"] != "original":
		t.Errorf("via %s: Data in the history was changed to %v", via, ef.Data["tag"])
	}
}
