package engine

import (
	"testing"
)

// TestNewPhase 内核造出来的阶段机不带任何解析器。
//
// 这是拆分的核心断言：内核不知道 NIGHT_WITCH 该由谁结算。此前这里装着
// 一张九个阶段的默认表，也就是内核认得狼人杀的整套流程。
func TestNewPhase(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	if p.config != config {
		t.Error("expected config to be set")
	}
	if len(p.resolvers) != 0 {
		t.Errorf("内核不该自带解析器，实际 %d 个", len(p.resolvers))
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

// TestGetResolver 注册进去的解析器要能原样取回来。
//
// 内核不认得任何具体解析器——这里取回的是测试自己注册的那个。
func TestGetResolver(t *testing.T) {
	marker := noopResolver{}
	e := newTestEngine(t, append(withNoopResolvers(),
		WithResolver(phaseDay, marker))...)

	got := e.phase.resolver(phaseDay)
	if got == nil {
		t.Fatal("注册过的阶段应当取得到解析器")
	}
	if got != Resolver(marker) {
		t.Errorf("取回的不是注册进去的那个: %#v", got)
	}
	if e.phase.resolver(PhaseType("从未注册")) != nil {
		t.Error("没注册过的阶段应当返回 nil")
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

	// 白天没有玩家技能——发言不是技能，走 SendMessage。
	// 此前 DAY 阶段声明了 SPEAK，提交能通过但结算零效果，是个悬空概念。
	roles := []RoleType{
		roleWerewolf,
		roleSeer,
		roleWitch,
		roleGuard,
		roleVillager,
	}
	for _, role := range roles {
		if skills := p.allowedSkills(phaseDay, role); len(skills) != 0 {
			t.Errorf("白天不应有可提交技能，%v 得到 %v", role, skills)
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
		TargetID: "victim",
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
		TargetID: "victim",
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
		TargetID: "victim",
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
		TargetID: "victim",
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
		TargetID: "nonexistent",
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
		TargetID: "victim",
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
		TargetID: "victim",
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
		TargetID: "",
		Phase:    phaseNightWolf,
	}

	err := p.validateSkillUse(use, state)
	if err != nil {
		t.Errorf("expected no error for empty target, got %v", err)
	}
}
