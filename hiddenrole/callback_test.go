package hiddenrole

import (
	"testing"
	"time"
)

// TestCallbacks_MayCallBackIntoTheEngine: an OnEvent / OnMessage handler may
// call back into the Engine.
//
// This is **supported**, and it is the only way to wire the engine into a
// server: receive an event, ask who it should go to, write it to those
// connections. example/netserver's entire push path rests on this property.
//
// It holds because events and messages are both published **outside the
// lock** (see endPhaseInternal and SendMessage). That used to live only in a
// code comment: anyone moving dispatchEvent back inside the lock would
// deadlock netserver on the spot, and not one test would go red.
//
// Contrast the eight extension points (Resolver, VictoryChecker, the three
// information-boundary providers, RoleInfoProvider, RoleSetup) -- they are
// called while the lock is held, and calling back into the Engine **hangs,
// it does not error**. That prohibition is documented on each interface.
//
// The timeout is necessary: when this really breaks the test hangs rather
// than fails, and a wedged CI job is harder to diagnose than a red line.
func TestCallbacks_MayCallBackIntoTheEngine(t *testing.T) {
	var events, messages int

	done := make(chan struct{})
	go func() {
		defer close(done)

		opts := append(withNoopResolvers(),
			WithResolver(phaseNightGuard, effectProducer{tag: "callback"}),
			WithAudience(AudienceFunc(func(*Event, GameView) ([]string, bool) {
				return []string{"w1"}, true
			})),
			WithSpeech(SpeechFunc(func(string, GameView) []string {
				return []string{"w1", "g"}
			})))
		e := newTestEngine(t, opts...)
		mustAdd(t, e, "w1", roleWerewolf)
		mustAdd(t, e, "g", roleGuard)

		// Call every public reader from inside the handlers.
		e.OnEvent(func(ev *Event) {
			events++
			_, _ = e.AudienceOf(ev)
			_ = e.Status().Phase
			_ = e.Status().Round
			_ = e.PlayerView("w1")
			_, _ = e.PlayerInfo("w1")
			_ = e.PhaseReadiness()
			_ = e.EffectLog()
			_ = e.View()
			_ = e.Teammates("w1")
			_ = e.Snapshot()
		})
		e.OnMessage(func(*Message, []string) {
			messages++
			_ = e.MessageReceivers("w1")
			_ = e.AllowedSkills("w1")
			_ = e.PhaseInfo()
		})

		if err := e.Start(); err != nil {
			t.Errorf("Start: %v", err)
			return
		}
		if _, err := e.EndPhase(); err != nil {
			t.Errorf("EndPhase: %v", err)
			return
		}
		if err := e.SendMessage("w1", "anyone still alive?"); err != nil {
			t.Errorf("SendMessage: %v", err)
			return
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock when a handler called back into the Engine -- events and " +
			"messages must be published outside the lock; check whether the " +
			"publish points in endPhaseInternal and SendMessage moved inside it")
	}

	if events == 0 {
		t.Error("the OnEvent handler was never called -- this test verified nothing")
	}
	if messages == 0 {
		t.Error("the OnMessage handler was never called -- this test verified nothing")
	}
}
