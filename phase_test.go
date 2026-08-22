package werewolf

import (
	"testing"
)

func TestNewPhase(t *testing.T) {
	config := DefaultGameConfig()
	p := newPhaseManager(config)

	if p.config != config {
		t.Error("expected config to be set")
	}
	// 3 day/vote/hunter resolvers + 6 night phase resolvers = 9
	if len(p.resolvers) != 9 {
		t.Errorf("expected 9 resolvers, got %d", len(p.resolvers))
	}

	// Verify resolvers are registered
	if p.resolvers[PhaseDay] == nil {
		t.Error("expected DayResolver to be registered")
	}
	if p.resolvers[PhaseVote] == nil {
		t.Error("expected VoteResolver to be registered")
	}

	// Verify night sub-phase resolvers are registered
	if p.resolvers[PhaseNightGuard] == nil {
		t.Error("expected GuardResolver to be registered")
	}
	if p.resolvers[PhaseNightWolf] == nil {
		t.Error("expected WolfResolver to be registered")
	}
	if p.resolvers[PhaseNightWitch] == nil {
		t.Error("expected WitchResolver to be registered")
	}
	if p.resolvers[PhaseNightSeer] == nil {
		t.Error("expected SeerResolver to be registered")
	}
}

func TestGetPhaseConfig(t *testing.T) {
	config := DefaultGameConfig()
	p := newPhaseManager(config)

	dayConfig := p.phaseConfig(PhaseDay)
	if dayConfig == nil {
		t.Fatal("expected day config")
	}

	voteConfig := p.phaseConfig(PhaseVote)
	if voteConfig == nil {
		t.Fatal("expected vote config")
	}

	guardConfig := p.phaseConfig(PhaseNightGuard)
	if guardConfig == nil {
		t.Fatal("expected guard phase config")
	}
}

func TestGetPhaseConfig_Invalid(t *testing.T) {
	config := DefaultGameConfig()
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

func TestGetResolver(t *testing.T) {
	config := DefaultGameConfig()
	p := newPhaseManager(config)

	dayResolver := p.resolver(PhaseDay)
	if dayResolver == nil {
		t.Error("expected day resolver")
	}
	if _, ok := dayResolver.(*DayResolver); !ok {
		t.Error("expected DayResolver type")
	}

	voteResolver := p.resolver(PhaseVote)
	if voteResolver == nil {
		t.Error("expected vote resolver")
	}
	if _, ok := voteResolver.(*VoteResolver); !ok {
		t.Error("expected VoteResolver type")
	}
}

func TestGetResolver_Nil(t *testing.T) {
	config := DefaultGameConfig()
	p := newPhaseManager(config)

	startResolver := p.resolver(PhaseStart)
	if startResolver != nil {
		t.Error("expected nil for START phase resolver")
	}
}

func TestGetAllowedSkills_Guard(t *testing.T) {
	config := DefaultGameConfig()
	p := newPhaseManager(config)

	skills := p.allowedSkills(PhaseNightGuard, RoleGuard)

	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}
	if skills[0] != SkillProtect {
		t.Errorf("expected PROTECT, got %v", skills[0])
	}
}

func TestGetAllowedSkills_Werewolf(t *testing.T) {
	config := DefaultGameConfig()
	p := newPhaseManager(config)

	skills := p.allowedSkills(PhaseNightWolf, RoleWerewolf)

	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}
	if skills[0] != SkillKill {
		t.Errorf("expected KILL, got %v", skills[0])
	}
}

func TestGetAllowedSkills_Witch(t *testing.T) {
	config := DefaultGameConfig()
	p := newPhaseManager(config)

	skills := p.allowedSkills(PhaseNightWitch, RoleWitch)

	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(skills))
	}

	hasAntidote := false
	hasPoison := false
	for _, s := range skills {
		if s == SkillAntidote {
			hasAntidote = true
		}
		if s == SkillPoison {
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
	config := DefaultGameConfig()
	p := newPhaseManager(config)

	skills := p.allowedSkills(PhaseNightSeer, RoleSeer)

	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}
	if skills[0] != SkillCheck {
		t.Errorf("expected CHECK, got %v", skills[0])
	}
}

func TestGetAllowedSkills_Villager_NightGuard(t *testing.T) {
	config := DefaultGameConfig()
	p := newPhaseManager(config)

	skills := p.allowedSkills(PhaseNightGuard, RoleVillager)

	// Villager has no skills in guard phase
	if len(skills) != 0 {
		t.Errorf("expected 0 skills for villager in guard phase, got %d", len(skills))
	}
}

func TestGetAllowedSkills_DayHasNoPlayerSkill(t *testing.T) {
	config := DefaultGameConfig()
	p := newPhaseManager(config)

	// 白天没有玩家技能——发言不是技能，走 SendMessage。
	// 此前 DAY 阶段声明了 SPEAK，提交能通过但结算零效果，是个悬空概念。
	roles := []RoleType{
		RoleWerewolf,
		RoleSeer,
		RoleWitch,
		RoleGuard,
		RoleVillager,
	}
	for _, role := range roles {
		if skills := p.allowedSkills(PhaseDay, role); len(skills) != 0 {
			t.Errorf("白天不应有可提交技能，%v 得到 %v", role, skills)
		}
	}
}

func TestGetAllowedSkills_AllVote(t *testing.T) {
	config := DefaultGameConfig()
	p := newPhaseManager(config)

	// All roles should be able to vote
	roles := []RoleType{
		RoleWerewolf,
		RoleSeer,
		RoleWitch,
		RoleGuard,
		RoleVillager,
	}

	for _, role := range roles {
		skills := p.allowedSkills(PhaseVote, role)
		if len(skills) != 1 {
			t.Errorf("expected 1 skill for %v during vote, got %d", role, len(skills))
		}
		if len(skills) > 0 && skills[0] != SkillVote {
			t.Errorf("expected VOTE for %v during vote, got %v", role, skills[0])
		}
	}
}

func TestGetAllowedSkills_InvalidPhase(t *testing.T) {
	config := DefaultGameConfig()
	p := newPhaseManager(config)

	skills := p.allowedSkills(PhaseStart, RoleWerewolf)
	if skills == nil || len(skills) != 0 {
		t.Errorf("expected empty non-nil slice for invalid phase, got %v", skills)
	}
}

func TestNextSubPhase_StartToGuard(t *testing.T) {
	config := DefaultGameConfig()
	p := newPhaseManager(config)

	next := p.nextSubPhase(PhaseStart)
	if next != PhaseNightGuard {
		t.Errorf("expected NIGHT_GUARD, got %v", next)
	}
}

func TestNextSubPhase_NightFlow(t *testing.T) {
	config := DefaultGameConfig()
	p := newPhaseManager(config)

	// Guard -> Wolf -> Witch -> Seer -> Resolve -> Day
	next := p.nextSubPhase(PhaseNightGuard)
	if next != PhaseNightWolf {
		t.Errorf("expected NIGHT_WOLF, got %v", next)
	}

	next = p.nextSubPhase(PhaseNightWolf)
	if next != PhaseNightWitch {
		t.Errorf("expected NIGHT_WITCH, got %v", next)
	}

	next = p.nextSubPhase(PhaseNightWitch)
	if next != PhaseNightSeer {
		t.Errorf("expected NIGHT_SEER, got %v", next)
	}

	next = p.nextSubPhase(PhaseNightSeer)
	if next != PhaseNightResolve {
		t.Errorf("expected NIGHT_RESOLVE, got %v", next)
	}

	next = p.nextSubPhase(PhaseNightResolve)
	if next != PhaseDay {
		t.Errorf("expected DAY, got %v", next)
	}
}

func TestNextSubPhase_DayToVote(t *testing.T) {
	config := DefaultGameConfig()
	p := newPhaseManager(config)

	next := p.nextSubPhase(PhaseDay)
	if next != PhaseVote {
		t.Errorf("expected VOTE, got %v", next)
	}
}

func TestNextSubPhase_VoteToGuard(t *testing.T) {
	config := DefaultGameConfig()
	p := newPhaseManager(config)

	next := p.nextSubPhase(PhaseVote)
	if next != PhaseNightGuard {
		t.Errorf("expected NIGHT_GUARD, got %v", next)
	}
}

func TestNextSubPhase_EndStays(t *testing.T) {
	config := DefaultGameConfig()
	p := newPhaseManager(config)

	next := p.nextSubPhase(PhaseEnd)
	if next != PhaseEnd {
		t.Errorf("expected END, got %v", next)
	}
}

func TestValidateSkillUse_Valid(t *testing.T) {
	config := DefaultGameConfig()
	p := newPhaseManager(config)

	state := newState()
	mustAddTo(t, state, "wolf", RoleWerewolf)
	mustAddTo(t, state, "victim", RoleVillager)
	state.Phase = PhaseNightWolf

	use := &SkillUse{
		PlayerID: "wolf",
		Skill:    SkillKill,
		TargetID: "victim",
		Phase:    PhaseNightWolf,
	}

	err := p.validateSkillUse(use, state)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateSkillUse_PlayerNotFound(t *testing.T) {
	config := DefaultGameConfig()
	p := newPhaseManager(config)

	state := newState()
	state.Phase = PhaseNightWolf

	use := &SkillUse{
		PlayerID: "nonexistent",
		Skill:    SkillKill,
		TargetID: "victim",
		Phase:    PhaseNightWolf,
	}

	err := p.validateSkillUse(use, state)
	if err != ErrPlayerNotFound {
		t.Errorf("expected ErrPlayerNotFound, got %v", err)
	}
}

func TestValidateSkillUse_PlayerDead(t *testing.T) {
	config := DefaultGameConfig()
	p := newPhaseManager(config)

	state := newState()
	mustAddTo(t, state, "wolf", RoleWerewolf)
	mustAddTo(t, state, "victim", RoleVillager)
	state.players["wolf"].Alive = false
	state.Phase = PhaseNightWolf

	use := &SkillUse{
		PlayerID: "wolf",
		Skill:    SkillKill,
		TargetID: "victim",
		Phase:    PhaseNightWolf,
	}

	err := p.validateSkillUse(use, state)
	if err != ErrPlayerDead {
		t.Errorf("expected ErrPlayerDead, got %v", err)
	}
}

func TestValidateSkillUse_SkillNotAllowed(t *testing.T) {
	config := DefaultGameConfig()
	p := newPhaseManager(config)

	state := newState()
	mustAddTo(t, state, "villager", RoleVillager)
	mustAddTo(t, state, "victim", RoleVillager)
	state.Phase = PhaseNightWolf

	// Villager tries to kill at night (not allowed)
	use := &SkillUse{
		PlayerID: "villager",
		Skill:    SkillKill,
		TargetID: "victim",
		Phase:    PhaseNightWolf,
	}

	err := p.validateSkillUse(use, state)
	if err != ErrSkillNotAllowed {
		t.Errorf("expected ErrSkillNotAllowed, got %v", err)
	}
}

func TestValidateSkillUse_TargetNotFound(t *testing.T) {
	config := DefaultGameConfig()
	p := newPhaseManager(config)

	state := newState()
	mustAddTo(t, state, "wolf", RoleWerewolf)
	state.Phase = PhaseNightWolf

	use := &SkillUse{
		PlayerID: "wolf",
		Skill:    SkillKill,
		TargetID: "nonexistent",
		Phase:    PhaseNightWolf,
	}

	err := p.validateSkillUse(use, state)
	if err != ErrTargetNotFound {
		t.Errorf("expected ErrTargetNotFound, got %v", err)
	}
}

func TestValidateSkillUse_TargetDead(t *testing.T) {
	config := DefaultGameConfig()
	p := newPhaseManager(config)

	state := newState()
	mustAddTo(t, state, "wolf", RoleWerewolf)
	mustAddTo(t, state, "victim", RoleVillager)
	state.players["victim"].Alive = false
	state.Phase = PhaseNightWolf

	use := &SkillUse{
		PlayerID: "wolf",
		Skill:    SkillKill,
		TargetID: "victim",
		Phase:    PhaseNightWolf,
	}

	err := p.validateSkillUse(use, state)
	if err != ErrTargetDead {
		t.Errorf("expected ErrTargetDead, got %v", err)
	}
}

func TestValidateSkillUse_AntidoteOnDead(t *testing.T) {
	config := DefaultGameConfig()
	p := newPhaseManager(config)

	state := newState()
	mustAddTo(t, state, "witch", RoleWitch)
	mustAddTo(t, state, "victim", RoleVillager)
	state.players["victim"].Alive = false
	state.Phase = PhaseNightWitch

	// Antidote can be used on dead target
	use := &SkillUse{
		PlayerID: "witch",
		Skill:    SkillAntidote,
		TargetID: "victim",
		Phase:    PhaseNightWitch,
	}

	err := p.validateSkillUse(use, state)
	if err != nil {
		t.Errorf("expected no error for antidote on dead, got %v", err)
	}
}

func TestValidateSkillUse_NoTarget(t *testing.T) {
	config := DefaultGameConfig()
	p := newPhaseManager(config)

	state := newState()
	mustAddTo(t, state, "wolf", RoleWerewolf)
	state.Phase = PhaseNightWolf

	// Empty target (wolf chooses not to kill)
	use := &SkillUse{
		PlayerID: "wolf",
		Skill:    SkillKill,
		TargetID: "",
		Phase:    PhaseNightWolf,
	}

	err := p.validateSkillUse(use, state)
	if err != nil {
		t.Errorf("expected no error for empty target, got %v", err)
	}
}
