package hiddenrole

import (
	"errors"
	"testing"
)

// TestNewPhase: the phase machine the kernel builds carries no resolvers.
//
// This is the split's central assertion: the kernel does not know who
// resolves NIGHT_WITCH. There used to be a default table of nine phases here,
// which is to say the kernel knew werewolf's entire flow.
func TestNewPhase(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	if p.config != config {
		t.Error("expected config to be set")
	}
	if len(p.resolvers) != 0 {
		t.Errorf("the kernel should ship no resolvers, got %d", len(p.resolvers))
	}
}

func TestGetPhaseConfig(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	dayConfig := p.phaseConfig(phaseDay)
	if dayConfig == nil {
		t.Fatal("expected day config")
	}

	voteConfig := p.phaseConfig(phaseVote)
	if voteConfig == nil {
		t.Fatal("expected vote config")
	}

	guardConfig := p.phaseConfig(phaseNightGuard)
	if guardConfig == nil {
		t.Fatal("expected guard phase config")
	}
}

func TestGetPhaseConfig_Invalid(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	startConfig := p.phaseConfig(PhaseStart)
	if startConfig != nil {
		t.Error("expected nil for START phase config")
	}

	endConfig := p.phaseConfig(PhaseEnd)
	if endConfig != nil {
		t.Error("expected nil for END phase config")
	}
}

// TestGetResolver: a registered resolver comes back verbatim.
//
// The kernel recognises no specific resolver -- what comes back here is the
// one the test registered itself.
func TestGetResolver(t *testing.T) {
	marker := noopResolver{}
	e := newTestEngine(t, append(withNoopResolvers(),
		WithResolver(phaseDay, marker))...)

	got := e.phase.resolver(phaseDay)
	if got == nil {
		t.Fatal("a registered phase should yield its resolver")
	}
	if got != Resolver(marker) {
		t.Errorf("what came back is not what was registered: %#v", got)
	}
	if e.phase.resolver(PhaseType("NEVER_REGISTERED")) != nil {
		t.Error("a phase that was never registered should give nil")
	}
}

func TestGetResolver_Nil(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	startResolver := p.resolver(PhaseStart)
	if startResolver != nil {
		t.Error("expected nil for START phase resolver")
	}
}

func TestGetAllowedSkills_Guard(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	skills := p.allowedSkills(phaseNightGuard, roleGuard)

	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}
	if skills[0] != skillProtect {
		t.Errorf("expected PROTECT, got %v", skills[0])
	}
}

func TestGetAllowedSkills_Werewolf(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	skills := p.allowedSkills(phaseNightWolf, roleWerewolf)

	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}
	if skills[0] != skillKill {
		t.Errorf("expected KILL, got %v", skills[0])
	}
}

func TestGetAllowedSkills_Witch(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	skills := p.allowedSkills(phaseNightWitch, roleWitch)

	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(skills))
	}

	hasAntidote := false
	hasPoison := false
	for _, s := range skills {
		if s == skillAntidote {
			hasAntidote = true
		}
		if s == skillPoison {
			hasPoison = true
		}
	}

	if !hasAntidote {
		t.Error("expected ANTIDOTE skill")
	}
	if !hasPoison {
		t.Error("expected POISON skill")
	}
}

func TestGetAllowedSkills_Seer(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	skills := p.allowedSkills(phaseNightSeer, roleSeer)

	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}
	if skills[0] != skillCheck {
		t.Errorf("expected CHECK, got %v", skills[0])
	}
}

func TestGetAllowedSkills_Villager_NightGuard(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	skills := p.allowedSkills(phaseNightGuard, roleVillager)

	// Villager has no skills in guard phase
	if len(skills) != 0 {
		t.Errorf("expected 0 skills for villager in guard phase, got %d", len(skills))
	}
}

func TestGetAllowedSkills_DayHasNoPlayerSkill(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	// The day has no player skills -- speaking is not a skill, it goes
	// through SendMessage. The DAY phase used to declare SPEAK, which could
	// be submitted and resolved to nothing: a dangling concept.
	roles := []RoleType{
		roleWerewolf,
		roleSeer,
		roleWitch,
		roleGuard,
		roleVillager,
	}
	for _, role := range roles {
		if skills := p.allowedSkills(phaseDay, role); len(skills) != 0 {
			t.Errorf("the day should have no submittable skill, %v got %v", role, skills)
		}
	}
}

func TestGetAllowedSkills_AllVote(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	// All roles should be able to vote
	roles := []RoleType{
		roleWerewolf,
		roleSeer,
		roleWitch,
		roleGuard,
		roleVillager,
	}

	for _, role := range roles {
		skills := p.allowedSkills(phaseVote, role)
		if len(skills) != 1 {
			t.Errorf("expected 1 skill for %v during vote, got %d", role, len(skills))
		}
		if len(skills) > 0 && skills[0] != skillVote {
			t.Errorf("expected VOTE for %v during vote, got %v", role, skills[0])
		}
	}
}

func TestGetAllowedSkills_InvalidPhase(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	skills := p.allowedSkills(PhaseStart, roleWerewolf)
	if skills == nil || len(skills) != 0 {
		t.Errorf("expected empty non-nil slice for invalid phase, got %v", skills)
	}
}

func TestNextSubPhase_StartToGuard(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	next := p.nextSubPhase(PhaseStart)
	if next != phaseNightGuard {
		t.Errorf("expected NIGHT_GUARD, got %v", next)
	}
}

func TestNextSubPhase_NightFlow(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	// Guard -> Wolf -> Witch -> Seer -> Resolve -> Day
	next := p.nextSubPhase(phaseNightGuard)
	if next != phaseNightWolf {
		t.Errorf("expected NIGHT_WOLF, got %v", next)
	}

	next = p.nextSubPhase(phaseNightWolf)
	if next != phaseNightWitch {
		t.Errorf("expected NIGHT_WITCH, got %v", next)
	}

	next = p.nextSubPhase(phaseNightWitch)
	if next != phaseNightSeer {
		t.Errorf("expected NIGHT_SEER, got %v", next)
	}

	next = p.nextSubPhase(phaseNightSeer)
	if next != phaseNightResolve {
		t.Errorf("expected NIGHT_RESOLVE, got %v", next)
	}

	next = p.nextSubPhase(phaseNightResolve)
	if next != phaseDay {
		t.Errorf("expected DAY, got %v", next)
	}
}

func TestNextSubPhase_DayToVote(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	next := p.nextSubPhase(phaseDay)
	if next != phaseVote {
		t.Errorf("expected VOTE, got %v", next)
	}
}

func TestNextSubPhase_VoteToGuard(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	next := p.nextSubPhase(phaseVote)
	if next != phaseNightGuard {
		t.Errorf("expected NIGHT_GUARD, got %v", next)
	}
}

func TestNextSubPhase_EndStays(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	next := p.nextSubPhase(PhaseEnd)
	if next != PhaseEnd {
		t.Errorf("expected END, got %v", next)
	}
}

func TestValidateSkillUse_Valid(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	state := newState()
	mustAddTo(t, state, "wolf", roleWerewolf)
	mustAddTo(t, state, "victim", roleVillager)
	state.Phase = phaseNightWolf

	use := &SkillUse{
		PlayerID: "wolf",
		Skill:    skillKill,
		Targets:  []string{"victim"},
		Phase:    phaseNightWolf,
	}

	err := p.validateSkillUse(use, state)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateSkillUse_PlayerNotFound(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	state := newState()
	state.Phase = phaseNightWolf

	use := &SkillUse{
		PlayerID: "nonexistent",
		Skill:    skillKill,
		Targets:  []string{"victim"},
		Phase:    phaseNightWolf,
	}

	err := p.validateSkillUse(use, state)
	if err != ErrPlayerNotFound {
		t.Errorf("expected ErrPlayerNotFound, got %v", err)
	}
}

func TestValidateSkillUse_PlayerDead(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	state := newState()
	mustAddTo(t, state, "wolf", roleWerewolf)
	mustAddTo(t, state, "victim", roleVillager)
	state.players["wolf"].Alive = false
	state.Phase = phaseNightWolf

	use := &SkillUse{
		PlayerID: "wolf",
		Skill:    skillKill,
		Targets:  []string{"victim"},
		Phase:    phaseNightWolf,
	}

	err := p.validateSkillUse(use, state)
	if err != ErrPlayerDead {
		t.Errorf("expected ErrPlayerDead, got %v", err)
	}
}

func TestValidateSkillUse_SkillNotAllowed(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	state := newState()
	mustAddTo(t, state, "villager", roleVillager)
	mustAddTo(t, state, "victim", roleVillager)
	state.Phase = phaseNightWolf

	// Villager tries to kill at night (not allowed)
	use := &SkillUse{
		PlayerID: "villager",
		Skill:    skillKill,
		Targets:  []string{"victim"},
		Phase:    phaseNightWolf,
	}

	err := p.validateSkillUse(use, state)
	if err != ErrSkillNotAllowed {
		t.Errorf("expected ErrSkillNotAllowed, got %v", err)
	}
}

func TestValidateSkillUse_TargetNotFound(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	state := newState()
	mustAddTo(t, state, "wolf", roleWerewolf)
	state.Phase = phaseNightWolf

	use := &SkillUse{
		PlayerID: "wolf",
		Skill:    skillKill,
		Targets:  []string{"nonexistent"},
		Phase:    phaseNightWolf,
	}

	err := p.validateSkillUse(use, state)
	if err != ErrTargetNotFound {
		t.Errorf("expected ErrTargetNotFound, got %v", err)
	}
}

func TestValidateSkillUse_TargetDead(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	state := newState()
	mustAddTo(t, state, "wolf", roleWerewolf)
	mustAddTo(t, state, "victim", roleVillager)
	state.players["victim"].Alive = false
	state.Phase = phaseNightWolf

	use := &SkillUse{
		PlayerID: "wolf",
		Skill:    skillKill,
		Targets:  []string{"victim"},
		Phase:    phaseNightWolf,
	}

	err := p.validateSkillUse(use, state)
	if err != ErrTargetDead {
		t.Errorf("expected ErrTargetDead, got %v", err)
	}
}

func TestValidateSkillUse_AntidoteOnDead(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	state := newState()
	mustAddTo(t, state, "witch", roleWitch)
	mustAddTo(t, state, "victim", roleVillager)
	state.players["victim"].Alive = false
	state.Phase = phaseNightWitch

	// Antidote can be used on dead target
	use := &SkillUse{
		PlayerID: "witch",
		Skill:    skillAntidote,
		Targets:  []string{"victim"},
		Phase:    phaseNightWitch,
	}

	err := p.validateSkillUse(use, state)
	if err != nil {
		t.Errorf("expected no error for antidote on dead, got %v", err)
	}
}

func TestValidateSkillUse_NoTarget(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	state := newState()
	mustAddTo(t, state, "wolf", roleWerewolf)
	state.Phase = phaseNightWolf

	// Empty target (wolf chooses not to kill)
	use := &SkillUse{
		PlayerID: "wolf",
		Skill:    skillKill,
		Targets:  []string{""},
		Phase:    phaseNightWolf,
	}

	err := p.validateSkillUse(use, state)
	if err != nil {
		t.Errorf("expected no error for empty target, got %v", err)
	}
}

// TestWatchOnlyStep covers a step with an empty skill: this role wakes, but
// takes no action.
//
// This one came out of the third rules package (One Night Ultimate
// Werewolf): the minion opening their eyes to see the wolves, the masons
// recognising each other, the insomniac looking at their own card -- they
// only receive information and submit nothing. It used to be inexpressible,
// so a rules package had to hang a SKIP on it as a placeholder, and SKIP
// means "declining to act", which they are not doing.
//
// Three things have to hold together, and the feature is useless without any
// one of them:
//
//	absent from AllowedSkills     there is nothing they can submit
//	out of the readiness check    there is nothing to satisfy, or the phase is never ready
//	**present in ActiveRoles**    the host has to know who to wake -- the whole point
func TestWatchOnlyStep(t *testing.T) {
	const phaseWatch = PhaseType("WATCH")
	cfg := testConfig()
	cfg.Phases[phaseWatch] = &PhaseConfig{
		Type: phaseWatch,
		Steps: []PhaseStep{
			{Role: roleSeer}, // empty: wake up and look
			{Role: roleWitch, Skill: skillPoison, Required: true}, // control
		},
		NextPhase: phaseDay,
	}
	cfg.Phases[phaseNightGuard].NextPhase = phaseWatch

	opts := append(withNoopResolvers(), WithResolver(phaseWatch, noopResolver{}))
	e, err := NewEngine(cfg, opts...)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	mustAdd(t, e, "s", roleSeer)
	mustAdd(t, e, "wi", roleWitch)
	mustAdd(t, e, "v", roleVillager)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := e.EndPhase(); err != nil { // NIGHT_GUARD -> WATCH
		t.Fatalf("EndPhase: %v", err)
	}
	if got := e.Status().Phase; got != phaseWatch {
		t.Fatalf("phase = %v, want %v", got, phaseWatch)
	}

	t.Run("absent from AllowedSkills", func(t *testing.T) {
		if got := e.AllowedSkills("s"); len(got) != 0 {
			t.Errorf("there is nothing they can submit, yet AllowedSkills gave %v", got)
		}
		if got := e.PlayerView("s").AllowedSkills; len(got) != 0 {
			t.Errorf("PlayerView should not give one either, got %v", got)
		}
	})

	t.Run("cannot be submitted", func(t *testing.T) {
		err := e.SubmitSkillUse(&SkillUse{PlayerID: "s", Skill: SkillUnspecified})
		if err == nil {
			t.Error("an empty skill is not an action, so the submission should be rejected")
		}
	})

	t.Run("out of the readiness check", func(t *testing.T) {
		rd := e.PhaseReadiness()
		for _, p := range append(append([]PendingAction{}, rd.Pending...), rd.Optional...) {
			if p.Role == roleSeer {
				t.Errorf("someone who only wakes and looks should not appear in the readiness check: %+v", p)
			}
		}
		// Control: the witch's step is required, so the phase is not ready.
		if rd.Ready {
			t.Error("the witch's step is required and nobody submitted, so the phase should not be ready")
		}
	})

	t.Run("present in ActiveRoles", func(t *testing.T) {
		var found bool
		for _, role := range e.PhaseInfo().ActiveRoles {
			if role == roleSeer {
				found = true
			}
		}
		if !found {
			t.Error("the host has to know who to wake -- the entire reason an empty step exists")
		}
	})

	t.Run("the phase can advance", func(t *testing.T) {
		// An empty step must not wedge the phase: it should be ready once the
		// witch submits.
		if err := e.SubmitSkillUse(&SkillUse{
			PlayerID: "wi", Skill: skillPoison, Targets: []string{"v"},
		}); err != nil {
			t.Fatalf("witch submission: %v", err)
		}
		if !e.PhaseReadiness().Ready {
			t.Error("the required step is satisfied, so the phase should be ready")
		}
	})
}

// TestSkip_HasNoKernelPrivilege: to the kernel, declining is not a special
// skill.
//
// validateSkillUse used to have a branch reading "skipping needs no target,
// let it through". That branch was empty: a submission with no target already
// passes target validation (the loop never runs), and a submission that does
// carry a target **should** be validated. Its only real effect was to make
// the kernel recognise one specific skill -- and "the kernel recognises no
// value" is one of this library's five invariants.
//
// With it gone, two things have to hold: a submission with no target still
// goes through, and one with a bad target is rejected.
func TestSkip_HasNoKernelPrivilege(t *testing.T) {
	const phasePass = PhaseType("PASS")
	cfg := testConfig()
	cfg.Phases[phasePass] = &PhaseConfig{
		Type: phasePass,
		Steps: []PhaseStep{
			{Role: roleVillager, Skill: skillVote, Group: "act"},
			{Role: roleVillager, Skill: SkillSkip, Group: "act"},
		},
		NextPhase: phaseDay,
	}
	cfg.Phases[phaseNightGuard].NextPhase = phasePass

	opts := append(withNoopResolvers(), WithResolver(phasePass, noopResolver{}))
	e, err := NewEngine(cfg, opts...)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	mustAdd(t, e, "v", roleVillager)
	mustAdd(t, e, "w", roleWerewolf)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := e.EndPhase(); err != nil { // NIGHT_GUARD -> PASS
		t.Fatalf("EndPhase: %v", err)
	}
	if got := e.Status().Phase; got != phasePass {
		t.Fatalf("phase = %v, want %v", got, phasePass)
	}

	t.Run("a submission with no target goes through", func(t *testing.T) {
		if err := e.SubmitSkillUse(&SkillUse{PlayerID: "v", Skill: SkillSkip}); err != nil {
			t.Errorf("declining needs no target, yet the submission was rejected: %v", err)
		}
	})

	t.Run("a target that does not exist is rejected", func(t *testing.T) {
		err := e.SubmitSkillUse(&SkillUse{
			PlayerID: "v", Skill: SkillSkip, Targets: []string{"ghost"},
		})
		if !errors.Is(err, ErrTargetNotFound) {
			t.Errorf("a missing target should be rejected as %v, got %v -- "+
				"to the kernel, declining is not a special skill", ErrTargetNotFound, err)
		}
	})
}
