package hiddenrole

import (
	"errors"
	"testing"
)

// provider_test.go: the four information-boundary extension points really
// **are called** once installed.
//
// extpoint_test.go checks that all eight extension points can be installed
// with a plain function -- that is assembly. This batch checks the other
// half: once installed the engine really does ask them, and the answers
// really do reach players. Both are needed, and the latter was missing:
// adapter methods like TeammateFunc.Teammates and RoleInfoFunc.RoleInfo had
// 0% coverage -- installable, and never asked.

// providerGame is one game with the full information boundary installed.
func providerGame(t *testing.T) *Engine {
	t.Helper()

	opts := append(withNoopResolvers(),
		WithTeammates(TeammateFunc(func(playerID string, view GameView) []string {
			if playerID != "w1" {
				return nil
			}
			return []string{"w2"} // only w1 knows w2; the reverse does not hold
		})),
		WithRoleInfo(roleWitch, RoleInfoFunc(func(playerID string, view GameView) map[string]string {
			return map[string]string{"probe.round": string(view.Phase())}
		})),
		WithSpeech(SpeechFunc(func(senderID string, view GameView) []string {
			if senderID == "wi" {
				return nil // nobody hears the witch speak
			}
			return []string{"w1"}
		})),
		WithAudience(AudienceFunc(func(event *Event, view GameView) ([]string, bool) {
			if event.Type == EventType("PROBE") {
				return []string{"w1"}, true
			}
			return nil, false
		})),
	)

	e, err := NewEngine(testConfig(), opts...)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	mustAdd(t, e, "w1", roleWerewolf)
	mustAdd(t, e, "w2", roleWerewolf)
	mustAdd(t, e, "wi", roleWitch)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return e
}

// TestProviders_AreActuallyAsked: the four installed providers really are
// asked, and their answers really reach players.
func TestProviders_AreActuallyAsked(t *testing.T) {
	e := providerGame(t)

	t.Run("teammates: asymmetry is allowed", func(t *testing.T) {
		if got := e.Teammates("w1"); len(got) != 1 || got[0] != "w2" {
			t.Errorf("w1 should know w2, got %v", got)
		}
		if got := e.Teammates("w2"); len(got) != 0 {
			t.Errorf("the reverse should not hold, got %v", got)
		}
		// The same provider has to feed the player view too; two paths must
		// not give two answers.
		if got := e.PlayerView("w1").Teammates; len(got) != 1 || got[0] != "w2" {
			t.Errorf("teammates in PlayerView should agree with Engine.Teammates, got %v", got)
		}
	})

	t.Run("role information: projected into the player view", func(t *testing.T) {
		got := e.PlayerView("wi").RoleInfo
		if got["probe.round"] == "" {
			t.Error("RoleInfoProvider was installed but never asked")
		}
		if len(e.PlayerView("w1").RoleInfo) != 0 {
			t.Error("information registered only for the witch should not appear in a werewolf's view")
		}
	})

	t.Run("speech: the provider decides", func(t *testing.T) {
		if got := e.MessageReceivers("w2"); len(got) != 1 || got[0] != "w1" {
			t.Errorf("the provider says only w1 hears, got %v", got)
		}
		if got := e.MessageReceivers("wi"); len(got) != 0 {
			t.Errorf("the provider says nobody hears the witch, got %v", got)
		}
		// Speech with no receivers is rejected -- the one place the kernel
		// blocks speech at all.
		if err := e.SendMessage("wi", "anyone there?"); !errors.Is(err, ErrMessageNotAllowed) {
			t.Errorf("no receivers should be rejected as %v, got %v", ErrMessageNotAllowed, err)
		}
		if err := e.SendMessage("w2", "here"); err != nil {
			t.Errorf("with receivers it should go through, got %v", err)
		}
		if err := e.SendMessage("ghost", "?"); !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("a player who does not exist should be rejected as %v, got %v", ErrPlayerNotFound, err)
		}
	})

	t.Run("audience: obey the rules where they spoke, answer \"don't know\" where they did not", func(t *testing.T) {
		probe := NewEffect(EventType("PROBE"), "", "").ToEvent()
		got, known := e.AudienceOf(probe)
		if !known || len(got) != 1 || got[0] != "w1" {
			t.Errorf("the provider spoke, so the answer should be [w1], got %v (known=%v)", got, known)
		}
		other := NewEffect(EventType("NOT_DECLARED"), "", "").ToEvent()
		if _, known := e.AudienceOf(other); known {
			t.Error("for an event the provider did not speak on, the answer should be \"don't know\"")
		}
	})
}

// TestGameView_ReadsThroughEverything: every reader on the read-only view
// reads something real.
//
// The view is a Resolver's **only** window onto the world -- one reader
// reading wrongly and the rules are left guessing.
func TestGameView_ReadsThroughEverything(t *testing.T) {
	e := providerGame(t)
	e.Apply(NewSetAliveEffect("w2", false))
	view := e.View()

	if got := view.Phase(); got != e.Status().Phase {
		t.Errorf("Phase = %v, the engine says %v", got, e.Status().Phase)
	}
	if got := view.Round(); got != e.Status().Round {
		t.Errorf("Round = %d, the engine says %d", got, e.Status().Round)
	}
	if got := len(view.AllPlayers()); got != 3 {
		t.Errorf("AllPlayers must include the eliminated, got %d", got)
	}
	if got := len(view.AlivePlayers()); got != 2 {
		t.Errorf("AlivePlayers counts only the living, got %d", got)
	}
	if got := view.AlivePlayerIDsByRole(roleWerewolf); len(got) != 1 || got[0] != "w1" {
		t.Errorf("living players by role: want [w1], got %v", got)
	}
	if p, ok := view.Player("w1"); !ok || p.Role != roleWerewolf {
		t.Errorf("Player read back wrong: %+v", p)
	}
	if _, ok := view.Player("ghost"); ok {
		t.Error("a player who does not exist should not be readable")
	}
	if rc := view.RoundContext(); rc.Vars == nil && len(rc.Detours) != 0 {
		t.Errorf("the round context read back wrong: %+v", rc)
	}

	t.Run("a view is a snapshot: later state changes do not affect it", func(t *testing.T) {
		before := len(view.AlivePlayers())
		e.Apply(NewSetAliveEffect("w1", false))
		if got := len(view.AlivePlayers()); got != before {
			t.Errorf("a view already taken should not follow along: %d -> %d", before, got)
		}
	})
}

// TestWithLogger_IsActuallyWired: an installed logger really does receive
// something.
func TestWithLogger_IsActuallyWired(t *testing.T) {
	rec := &recordingLogger{}
	e := newTestEngine(t, append(withNoopResolvers(), WithLogger(rec))...)
	mustAdd(t, e, "v", roleVillager)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if rec.infos == 0 {
		t.Error("starting the game should leave an Info-level log line")
	}

	t.Run("nil must not fail construction", func(t *testing.T) {
		if _, err := NewEngine(testConfig(), append(withNoopResolvers(), WithLogger(nil))...); err != nil {
			t.Errorf("a nil logger should be ignored, got %v", err)
		}
	})
}

// TestOptions_RejectNil: an extension point does not accept nil -- installing
// an empty one is worse than installing none.
func TestOptions_RejectNil(t *testing.T) {
	cases := []struct {
		name string
		opt  EngineOption
	}{
		{"Resolver", WithResolver(phaseDay, nil)},
		{"VictoryChecker", WithVictoryChecker(nil)},
		{"Audience", WithAudience(nil)},
		{"Teammates", WithTeammates(nil)},
		{"Speech", WithSpeech(nil)},
		{"RoleInfo", WithRoleInfo(roleWitch, nil)},
		{"RoleSetup", WithRoleSetup(roleWitch, nil)},
		{"GameSetup", WithGameSetup(nil)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := NewEngine(testConfig(), append(withNoopResolvers(), c.opt)...); err == nil {
				t.Error("a nil extension point should be rejected -- installing an empty one is worse than installing none")
			}
		})
	}
}
