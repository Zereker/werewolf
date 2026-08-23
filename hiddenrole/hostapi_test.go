package hiddenrole

import (
	"errors"
	"testing"
	"time"
)

// hostapi_test.go covers the things a host uses that the kernel itself never
// verified.
//
// What they have in common is that they were only ever exercised downstream: a
// host console reads PhaseInfo to run the phase, a server branches on error
// codes, a timeout reads PhaseTimeout. Now that the kernel is its own library,
// these are the first things a user touches, and they should not depend on a
// rules package to attest for them.

// TestPhaseInfo_TellsTheHostWhatToDo: PhaseInfo is the host console's
// instructions.
//
// Three methods answer three questions: is there an announcement, which one,
// and who else has to act. All three used to have 0% coverage.
func TestPhaseInfo_TellsTheHostWhatToDo(t *testing.T) {
	const phaseTalk = PhaseType("TALK")
	cfg := testConfig()
	cfg.Phases[phaseTalk] = &PhaseConfig{
		Type: phaseTalk,
		Steps: []PhaseStep{
			{Role: RoleSystem, Skill: SkillAnnounce},               // an announcement is due
			{Role: roleVillager, Skill: skillVote, Required: true}, // a player action
		},
		NextPhase: phaseDay,
	}
	cfg.Phases[phaseNightGuard].NextPhase = phaseTalk

	opts := append(withNoopResolvers(), WithResolver(phaseTalk, noopResolver{}))
	e, err := NewEngine(cfg, opts...)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	mustAdd(t, e, "v", roleVillager)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	t.Run("a phase with no announcement", func(t *testing.T) {
		info := e.PhaseInfo()
		if info.NeedsGodAnnouncement() {
			t.Error("the guard phase does not open with an announcement")
		}
		if info.GodAnnouncementStep() != nil {
			t.Error("with no announcement there should be no announcement step")
		}
	})

	if _, err := e.EndPhase(); err != nil { // -> TALK
		t.Fatalf("EndPhase: %v", err)
	}

	t.Run("a phase with an announcement", func(t *testing.T) {
		info := e.PhaseInfo()
		if !info.NeedsGodAnnouncement() {
			t.Fatal("the first step is RoleSystem + ANNOUNCE, so the host should announce")
		}
		step := info.GodAnnouncementStep()
		if step == nil || step.Skill != SkillAnnounce {
			t.Fatalf("the announcement step read back wrong: %+v", step)
		}
	})

	t.Run("player actions are listed apart from the announcement", func(t *testing.T) {
		steps := e.PhaseInfo().PlayerActionSteps()
		for _, s := range steps {
			if s.Role == RoleSystem {
				t.Error("PlayerActionSteps should not include the step no player carries")
			}
		}
		if len(steps) != 1 || steps[0].Skill != skillVote {
			t.Errorf("the player action step read back wrong: %+v", steps)
		}
	})
}

// TestEngine_CheapReaders: the cheap readers read something real.
//
// AlivePlayerIDs and PhaseTimeout used to have 0% coverage.
func TestEngine_CheapReaders(t *testing.T) {
	e := newTestEngine(t, withNoopResolvers()...)
	mustAdd(t, e, "b", roleVillager)
	mustAdd(t, e, "a", roleWerewolf)
	mustAdd(t, e, "c", roleVillager)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	t.Run("the living roster is sorted by ID", func(t *testing.T) {
		got := e.AlivePlayerIDs()
		want := []string{"a", "b", "c"}
		if len(got) != len(want) {
			t.Fatalf("roster = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("the roster is not sorted: %v", got)
			}
		}
	})

	t.Run("the eliminated are not on the roster", func(t *testing.T) {
		e.Apply(NewSetAliveEffect("b", false))
		for _, id := range e.AlivePlayerIDs() {
			if id == "b" {
				t.Error("an eliminated player should not appear on the living roster")
			}
		}
	})

	t.Run("the round context is a copy", func(t *testing.T) {
		rc := e.RoundContext()
		if rc == nil {
			t.Fatal("there should be a round context after the start")
		}
		rc.Vars = map[string]string{"tampered": "1"}
		if got := e.Var(ScopeRound, "tampered"); got != "" {
			t.Error("changing the copy should not change the engine's internals")
		}
	})
}

// TestConfig_PhaseTimeout: the suggested timeout is the phase's own where it
// declares one, and the default otherwise.
//
// The engine **does not time anything by it** -- it is advice for the caller.
// That is worth pinning down, so that nobody later assumes the engine advances
// on a timeout by itself.
func TestConfig_PhaseTimeout(t *testing.T) {
	cfg := testConfig()
	cfg.DefaultTimeout = 7 * time.Second
	cfg.Phases[phaseDay].Timeout = 99 * time.Second

	if got := cfg.PhaseTimeout(phaseDay); got != 99*time.Second {
		t.Errorf("a phase that declares a timeout should use it, got %v", got)
	}
	if got := cfg.PhaseTimeout(phaseVote); got != 7*time.Second {
		t.Errorf("with none declared it should fall back to the default, got %v", got)
	}
	if got := cfg.PhaseTimeout(PhaseType("NOT_THERE")); got != 7*time.Second {
		t.Errorf("a phase that does not exist should still give the default, got %v", got)
	}

	empty := &Config{}
	if got := empty.PhaseTimeout(phaseDay); got != DefaultPhaseTimeout {
		t.Errorf("with no default set either it should fall back to DefaultPhaseTimeout, got %v", got)
	}
}

// TestSkillUse_Target covers the single-target reader.
func TestSkillUse_Target(t *testing.T) {
	cases := []struct {
		use  *SkillUse
		want string
	}{
		{&SkillUse{Targets: []string{"a"}}, "a"},
		{&SkillUse{Targets: []string{"a", "b"}}, "a"},
		{&SkillUse{Targets: nil}, ""},
		{&SkillUse{Targets: []string{}}, ""},
	}
	for _, c := range cases {
		if got := c.use.Target(); got != c.want {
			t.Errorf("Target(%v) = %q, want %q", c.use.Targets, got, c.want)
		}
	}
}

// TestAddPlayer_RejectsBadSeats: the four ways seating is rejected, which a
// host branches on by error code.
func TestAddPlayer_RejectsBadSeats(t *testing.T) {
	e := newTestEngine(t, withNoopResolvers()...)
	mustAdd(t, e, "v", roleVillager)

	cases := []struct {
		name string
		id   string
		role RoleType
		want error
	}{
		{"empty ID", "", roleVillager, ErrInvalidPlayerID},
		{"duplicate ID", "v", roleWerewolf, ErrPlayerExists},
		{"the system role cannot be seated", "sys", RoleSystem, ErrInvalidRole},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := e.AddPlayer(c.id, c.role); !errors.Is(err, c.want) {
				t.Errorf("should be rejected as %v, got %v", c.want, err)
			}
		})
	}

	t.Run("no seating after the start", func(t *testing.T) {
		if err := e.Start(); err != nil {
			t.Fatalf("Start: %v", err)
		}
		if err := e.AddPlayer("late", roleVillager); !errors.Is(err, ErrGameAlreadyStarted) {
			t.Errorf("should be rejected as %v, got %v", ErrGameAlreadyStarted, err)
		}
	})
}

// TestRestoreEngine_RejectsBadSnapshots: a bad snapshot must be rejected.
func TestRestoreEngine_RejectsBadSnapshots(t *testing.T) {
	e := newTestEngine(t, withNoopResolvers()...)
	mustAdd(t, e, "v", roleVillager)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	good := e.Snapshot()

	t.Run("a nil snapshot", func(t *testing.T) {
		if _, err := RestoreEngine(testConfig(), nil, withNoopResolvers()...); !errors.Is(err, ErrNilSnapshot) {
			t.Errorf("should be rejected as %v, got %v", ErrNilSnapshot, err)
		}
	})

	t.Run("a version mismatch", func(t *testing.T) {
		bad := *good
		bad.Version = SnapshotVersion + 1
		if _, err := RestoreEngine(testConfig(), &bad, withNoopResolvers()...); !HasCode(err, CodeInvalidSnapshot) {
			t.Errorf("should be rejected as %v, got %v", CodeInvalidSnapshot, CodeOf(err))
		}
	})

	t.Run("a phase absent from the config", func(t *testing.T) {
		bad := *good
		bad.Phase = PhaseType("NOT_IN_CONFIG")
		if _, err := RestoreEngine(testConfig(), &bad, withNoopResolvers()...); !HasCode(err, CodeInvalidSnapshot) {
			t.Errorf("should be rejected as %v, got %v", CodeInvalidSnapshot, CodeOf(err))
		}
	})
}
