package werewolf

import (
	"testing"

	pb "github.com/Zereker/werewolf/proto"
)

func TestNewPhase(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	if p.config != config {
		t.Error("expected config to be set")
	}
	// 3 day/vote/hunter resolvers + 6 night phase resolvers = 9
	if len(p.resolvers) != 9 {
		t.Errorf("expected 9 resolvers, got %d", len(p.resolvers))
	}

	// Verify resolvers are registered
	if p.resolvers[pb.PhaseType_PHASE_TYPE_DAY] == nil {
		t.Error("expected DayResolver to be registered")
	}
	if p.resolvers[pb.PhaseType_PHASE_TYPE_VOTE] == nil {
		t.Error("expected VoteResolver to be registered")
	}

	// Verify night sub-phase resolvers are registered
	if p.resolvers[pb.PhaseType_PHASE_TYPE_NIGHT_GUARD] == nil {
		t.Error("expected GuardResolver to be registered")
	}
	if p.resolvers[pb.PhaseType_PHASE_TYPE_NIGHT_WOLF] == nil {
		t.Error("expected WolfResolver to be registered")
	}
	if p.resolvers[pb.PhaseType_PHASE_TYPE_NIGHT_WITCH] == nil {
		t.Error("expected WitchResolver to be registered")
	}
	if p.resolvers[pb.PhaseType_PHASE_TYPE_NIGHT_SEER] == nil {
		t.Error("expected SeerResolver to be registered")
	}
}

func TestGetPhaseConfig(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	dayConfig := p.GetPhaseConfig(pb.PhaseType_PHASE_TYPE_DAY)
	if dayConfig == nil {
		t.Fatal("expected day config")
	}

	voteConfig := p.GetPhaseConfig(pb.PhaseType_PHASE_TYPE_VOTE)
	if voteConfig == nil {
		t.Fatal("expected vote config")
	}

	guardConfig := p.GetPhaseConfig(pb.PhaseType_PHASE_TYPE_NIGHT_GUARD)
	if guardConfig == nil {
		t.Fatal("expected guard phase config")
	}
}

func TestGetPhaseConfig_Invalid(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	startConfig := p.GetPhaseConfig(pb.PhaseType_PHASE_TYPE_START)
	if startConfig != nil {
		t.Error("expected nil for START phase config")
	}

	endConfig := p.GetPhaseConfig(pb.PhaseType_PHASE_TYPE_END)
	if endConfig != nil {
		t.Error("expected nil for END phase config")
	}
}

func TestGetResolver(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	dayResolver := p.GetResolver(pb.PhaseType_PHASE_TYPE_DAY)
	if dayResolver == nil {
		t.Error("expected day resolver")
	}
	if _, ok := dayResolver.(*DayResolver); !ok {
		t.Error("expected DayResolver type")
	}

	voteResolver := p.GetResolver(pb.PhaseType_PHASE_TYPE_VOTE)
	if voteResolver == nil {
		t.Error("expected vote resolver")
	}
	if _, ok := voteResolver.(*VoteResolver); !ok {
		t.Error("expected VoteResolver type")
	}
}

func TestGetResolver_Nil(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	startResolver := p.GetResolver(pb.PhaseType_PHASE_TYPE_START)
	if startResolver != nil {
		t.Error("expected nil for START phase resolver")
	}
}

func TestGetRequiredRoles_NightGuard(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	roles := p.GetRequiredRoles(pb.PhaseType_PHASE_TYPE_NIGHT_GUARD)

	// Should have God and Guard
	if len(roles) != 2 {
		t.Errorf("expected 2 unique roles, got %d", len(roles))
	}

	roleSet := make(map[pb.RoleType]bool)
	for _, r := range roles {
		roleSet[r] = true
	}

	if !roleSet[pb.RoleType_ROLE_TYPE_GOD] {
		t.Error("expected God in required roles")
	}
	if !roleSet[pb.RoleType_ROLE_TYPE_GUARD] {
		t.Error("expected Guard in required roles")
	}
}

func TestGetRequiredRoles_Day(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	roles := p.GetRequiredRoles(pb.PhaseType_PHASE_TYPE_DAY)

	// Day phase has God announce + UNSPECIFIED role for speak
	if len(roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(roles))
	}

	roleSet := make(map[pb.RoleType]bool)
	for _, r := range roles {
		roleSet[r] = true
	}

	if !roleSet[pb.RoleType_ROLE_TYPE_GOD] {
		t.Error("expected God in required roles")
	}
	if !roleSet[pb.RoleType_ROLE_TYPE_UNSPECIFIED] {
		t.Error("expected UNSPECIFIED in required roles")
	}
}

func TestGetRequiredRoles_Invalid(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	roles := p.GetRequiredRoles(pb.PhaseType_PHASE_TYPE_START)
	if roles != nil {
		t.Error("expected nil for invalid phase")
	}
}

func TestGetAllowedSkills_Guard(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	skills := p.GetAllowedSkills(pb.PhaseType_PHASE_TYPE_NIGHT_GUARD, pb.RoleType_ROLE_TYPE_GUARD)

	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}
	if skills[0] != pb.SkillType_SKILL_TYPE_PROTECT {
		t.Errorf("expected PROTECT, got %v", skills[0])
	}
}

func TestGetAllowedSkills_Werewolf(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	skills := p.GetAllowedSkills(pb.PhaseType_PHASE_TYPE_NIGHT_WOLF, pb.RoleType_ROLE_TYPE_WEREWOLF)

	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}
	if skills[0] != pb.SkillType_SKILL_TYPE_KILL {
		t.Errorf("expected KILL, got %v", skills[0])
	}
}

func TestGetAllowedSkills_Witch(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	skills := p.GetAllowedSkills(pb.PhaseType_PHASE_TYPE_NIGHT_WITCH, pb.RoleType_ROLE_TYPE_WITCH)

	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(skills))
	}

	hasAntidote := false
	hasPoison := false
	for _, s := range skills {
		if s == pb.SkillType_SKILL_TYPE_ANTIDOTE {
			hasAntidote = true
		}
		if s == pb.SkillType_SKILL_TYPE_POISON {
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
	p := NewPhase(config)

	skills := p.GetAllowedSkills(pb.PhaseType_PHASE_TYPE_NIGHT_SEER, pb.RoleType_ROLE_TYPE_SEER)

	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}
	if skills[0] != pb.SkillType_SKILL_TYPE_CHECK {
		t.Errorf("expected CHECK, got %v", skills[0])
	}
}

func TestGetAllowedSkills_Villager_NightGuard(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	skills := p.GetAllowedSkills(pb.PhaseType_PHASE_TYPE_NIGHT_GUARD, pb.RoleType_ROLE_TYPE_VILLAGER)

	// Villager has no skills in guard phase
	if len(skills) != 0 {
		t.Errorf("expected 0 skills for villager in guard phase, got %d", len(skills))
	}
}

func TestGetAllowedSkills_AllSpeak(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	// All roles should be able to speak during day (UNSPECIFIED matches all)
	roles := []pb.RoleType{
		pb.RoleType_ROLE_TYPE_WEREWOLF,
		pb.RoleType_ROLE_TYPE_SEER,
		pb.RoleType_ROLE_TYPE_WITCH,
		pb.RoleType_ROLE_TYPE_GUARD,
		pb.RoleType_ROLE_TYPE_VILLAGER,
	}

	for _, role := range roles {
		skills := p.GetAllowedSkills(pb.PhaseType_PHASE_TYPE_DAY, role)
		if len(skills) != 1 {
			t.Errorf("expected 1 skill for %v during day, got %d", role, len(skills))
		}
		if len(skills) > 0 && skills[0] != pb.SkillType_SKILL_TYPE_SPEAK {
			t.Errorf("expected SPEAK for %v during day, got %v", role, skills[0])
		}
	}
}

func TestGetAllowedSkills_AllVote(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	// All roles should be able to vote
	roles := []pb.RoleType{
		pb.RoleType_ROLE_TYPE_WEREWOLF,
		pb.RoleType_ROLE_TYPE_SEER,
		pb.RoleType_ROLE_TYPE_WITCH,
		pb.RoleType_ROLE_TYPE_GUARD,
		pb.RoleType_ROLE_TYPE_VILLAGER,
	}

	for _, role := range roles {
		skills := p.GetAllowedSkills(pb.PhaseType_PHASE_TYPE_VOTE, role)
		if len(skills) != 1 {
			t.Errorf("expected 1 skill for %v during vote, got %d", role, len(skills))
		}
		if len(skills) > 0 && skills[0] != pb.SkillType_SKILL_TYPE_VOTE {
			t.Errorf("expected VOTE for %v during vote, got %v", role, skills[0])
		}
	}
}

func TestGetAllowedSkills_InvalidPhase(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	skills := p.GetAllowedSkills(pb.PhaseType_PHASE_TYPE_START, pb.RoleType_ROLE_TYPE_WEREWOLF)
	if skills != nil {
		t.Error("expected nil for invalid phase")
	}
}

func TestNextSubPhase_StartToGuard(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	next := p.NextSubPhase(pb.PhaseType_PHASE_TYPE_START)
	if next != pb.PhaseType_PHASE_TYPE_NIGHT_GUARD {
		t.Errorf("expected NIGHT_GUARD, got %v", next)
	}
}

func TestNextSubPhase_NightFlow(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	// Guard -> Wolf -> Witch -> Seer -> Resolve -> Day
	next := p.NextSubPhase(pb.PhaseType_PHASE_TYPE_NIGHT_GUARD)
	if next != pb.PhaseType_PHASE_TYPE_NIGHT_WOLF {
		t.Errorf("expected NIGHT_WOLF, got %v", next)
	}

	next = p.NextSubPhase(pb.PhaseType_PHASE_TYPE_NIGHT_WOLF)
	if next != pb.PhaseType_PHASE_TYPE_NIGHT_WITCH {
		t.Errorf("expected NIGHT_WITCH, got %v", next)
	}

	next = p.NextSubPhase(pb.PhaseType_PHASE_TYPE_NIGHT_WITCH)
	if next != pb.PhaseType_PHASE_TYPE_NIGHT_SEER {
		t.Errorf("expected NIGHT_SEER, got %v", next)
	}

	next = p.NextSubPhase(pb.PhaseType_PHASE_TYPE_NIGHT_SEER)
	if next != pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE {
		t.Errorf("expected NIGHT_RESOLVE, got %v", next)
	}

	next = p.NextSubPhase(pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE)
	if next != pb.PhaseType_PHASE_TYPE_DAY {
		t.Errorf("expected DAY, got %v", next)
	}
}

func TestNextSubPhase_DayToVote(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	next := p.NextSubPhase(pb.PhaseType_PHASE_TYPE_DAY)
	if next != pb.PhaseType_PHASE_TYPE_VOTE {
		t.Errorf("expected VOTE, got %v", next)
	}
}

func TestNextSubPhase_VoteToGuard(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	next := p.NextSubPhase(pb.PhaseType_PHASE_TYPE_VOTE)
	if next != pb.PhaseType_PHASE_TYPE_NIGHT_GUARD {
		t.Errorf("expected NIGHT_GUARD, got %v", next)
	}
}

func TestNextSubPhase_EndStays(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	next := p.NextSubPhase(pb.PhaseType_PHASE_TYPE_END)
	if next != pb.PhaseType_PHASE_TYPE_END {
		t.Errorf("expected END, got %v", next)
	}
}

func TestValidateSkillUse_Valid(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	state := NewState()
	state.AddPlayer("wolf", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.AddPlayer("victim", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	state.Phase = pb.PhaseType_PHASE_TYPE_NIGHT_WOLF

	use := &SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "victim",
		Phase:    pb.PhaseType_PHASE_TYPE_NIGHT_WOLF,
	}

	err := p.ValidateSkillUse(use, state)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateSkillUse_PlayerNotFound(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	state := NewState()
	state.Phase = pb.PhaseType_PHASE_TYPE_NIGHT_WOLF

	use := &SkillUse{
		PlayerID: "nonexistent",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "victim",
		Phase:    pb.PhaseType_PHASE_TYPE_NIGHT_WOLF,
	}

	err := p.ValidateSkillUse(use, state)
	if err != ErrPlayerNotFound {
		t.Errorf("expected ErrPlayerNotFound, got %v", err)
	}
}

func TestValidateSkillUse_PlayerDead(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	state := NewState()
	state.AddPlayer("wolf", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.AddPlayer("victim", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	state.players["wolf"].Alive = false
	state.Phase = pb.PhaseType_PHASE_TYPE_NIGHT_WOLF

	use := &SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "victim",
		Phase:    pb.PhaseType_PHASE_TYPE_NIGHT_WOLF,
	}

	err := p.ValidateSkillUse(use, state)
	if err != ErrPlayerDead {
		t.Errorf("expected ErrPlayerDead, got %v", err)
	}
}

func TestValidateSkillUse_SkillNotAllowed(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	state := NewState()
	state.AddPlayer("villager", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	state.AddPlayer("victim", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	state.Phase = pb.PhaseType_PHASE_TYPE_NIGHT_WOLF

	// Villager tries to kill at night (not allowed)
	use := &SkillUse{
		PlayerID: "villager",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "victim",
		Phase:    pb.PhaseType_PHASE_TYPE_NIGHT_WOLF,
	}

	err := p.ValidateSkillUse(use, state)
	if err != ErrSkillNotAllowed {
		t.Errorf("expected ErrSkillNotAllowed, got %v", err)
	}
}

func TestValidateSkillUse_TargetNotFound(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	state := NewState()
	state.AddPlayer("wolf", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.Phase = pb.PhaseType_PHASE_TYPE_NIGHT_WOLF

	use := &SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "nonexistent",
		Phase:    pb.PhaseType_PHASE_TYPE_NIGHT_WOLF,
	}

	err := p.ValidateSkillUse(use, state)
	if err != ErrTargetNotFound {
		t.Errorf("expected ErrTargetNotFound, got %v", err)
	}
}

func TestValidateSkillUse_TargetDead(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	state := NewState()
	state.AddPlayer("wolf", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.AddPlayer("victim", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	state.players["victim"].Alive = false
	state.Phase = pb.PhaseType_PHASE_TYPE_NIGHT_WOLF

	use := &SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "victim",
		Phase:    pb.PhaseType_PHASE_TYPE_NIGHT_WOLF,
	}

	err := p.ValidateSkillUse(use, state)
	if err != ErrTargetDead {
		t.Errorf("expected ErrTargetDead, got %v", err)
	}
}

func TestValidateSkillUse_AntidoteOnDead(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	state := NewState()
	state.AddPlayer("witch", pb.RoleType_ROLE_TYPE_WITCH, pb.Camp_CAMP_GOOD)
	state.AddPlayer("victim", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	state.players["victim"].Alive = false
	state.Phase = pb.PhaseType_PHASE_TYPE_NIGHT_WITCH

	// Antidote can be used on dead target
	use := &SkillUse{
		PlayerID: "witch",
		Skill:    pb.SkillType_SKILL_TYPE_ANTIDOTE,
		TargetID: "victim",
		Phase:    pb.PhaseType_PHASE_TYPE_NIGHT_WITCH,
	}

	err := p.ValidateSkillUse(use, state)
	if err != nil {
		t.Errorf("expected no error for antidote on dead, got %v", err)
	}
}

func TestValidateSkillUse_NoTarget(t *testing.T) {
	config := DefaultGameConfig()
	p := NewPhase(config)

	state := NewState()
	state.AddPlayer("wolf", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.Phase = pb.PhaseType_PHASE_TYPE_NIGHT_WOLF

	// Empty target (wolf chooses not to kill)
	use := &SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "",
		Phase:    pb.PhaseType_PHASE_TYPE_NIGHT_WOLF,
	}

	err := p.ValidateSkillUse(use, state)
	if err != nil {
		t.Errorf("expected no error for empty target, got %v", err)
	}
}
