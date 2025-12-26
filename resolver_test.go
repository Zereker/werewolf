package werewolf

import (
	"testing"

	pb "github.com/Zereker/werewolf/proto"
)

// ==================== VoteResolver Tests ====================

func TestVoteResolver_Empty(t *testing.T) {
	resolver := NewVoteResolver()
	state := NewState()
	config := DefaultGameConfig()

	effects := resolver.Resolve([]*SkillUse{}, state, config)

	if len(effects) != 1 {
		t.Fatalf("expected 1 effect (tied), got %d", len(effects))
	}
	if effects[0].Type != pb.EventType_EVENT_TYPE_UNSPECIFIED {
		t.Errorf("expected EVENT_TYPE_UNSPECIFIED for empty votes, got %v", effects[0].Type)
	}
}

func TestVoteResolver_Single(t *testing.T) {
	resolver := NewVoteResolver()
	state := NewState()
	state.AddPlayer("p1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	state.AddPlayer("p2", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	config := DefaultGameConfig()

	uses := []*SkillUse{
		{PlayerID: "p1", Skill: pb.SkillType_SKILL_TYPE_VOTE, TargetID: "p2"},
	}

	effects := resolver.Resolve(uses, state, config)

	if len(effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effects))
	}
	if effects[0].Type != pb.EventType_EVENT_TYPE_ELIMINATE {
		t.Errorf("expected EVENT_TYPE_ELIMINATE, got %v", effects[0].Type)
	}
	if effects[0].TargetID != "p2" {
		t.Errorf("expected target=p2, got %s", effects[0].TargetID)
	}
}

func TestVoteResolver_Clear(t *testing.T) {
	resolver := NewVoteResolver()
	state := NewState()
	state.AddPlayer("p1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	state.AddPlayer("p2", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	state.AddPlayer("p3", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	state.AddPlayer("wolf", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	config := DefaultGameConfig()

	uses := []*SkillUse{
		{PlayerID: "p1", Skill: pb.SkillType_SKILL_TYPE_VOTE, TargetID: "wolf"},
		{PlayerID: "p2", Skill: pb.SkillType_SKILL_TYPE_VOTE, TargetID: "wolf"},
		{PlayerID: "p3", Skill: pb.SkillType_SKILL_TYPE_VOTE, TargetID: "p1"},
	}

	effects := resolver.Resolve(uses, state, config)

	if len(effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effects))
	}
	if effects[0].Type != pb.EventType_EVENT_TYPE_ELIMINATE {
		t.Errorf("expected EVENT_TYPE_ELIMINATE, got %v", effects[0].Type)
	}
	if effects[0].TargetID != "wolf" {
		t.Errorf("expected target=wolf (majority), got %s", effects[0].TargetID)
	}
}

func TestVoteResolver_Tie(t *testing.T) {
	resolver := NewVoteResolver()
	state := NewState()
	state.AddPlayer("p1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	state.AddPlayer("p2", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	state.AddPlayer("p3", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	state.AddPlayer("p4", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	config := DefaultGameConfig()

	uses := []*SkillUse{
		{PlayerID: "p1", Skill: pb.SkillType_SKILL_TYPE_VOTE, TargetID: "p3"},
		{PlayerID: "p2", Skill: pb.SkillType_SKILL_TYPE_VOTE, TargetID: "p3"},
		{PlayerID: "p3", Skill: pb.SkillType_SKILL_TYPE_VOTE, TargetID: "p4"},
		{PlayerID: "p4", Skill: pb.SkillType_SKILL_TYPE_VOTE, TargetID: "p4"},
	}

	effects := resolver.Resolve(uses, state, config)

	if len(effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effects))
	}
	if effects[0].Type != pb.EventType_EVENT_TYPE_UNSPECIFIED {
		t.Errorf("expected EVENT_TYPE_UNSPECIFIED for tie, got %v", effects[0].Type)
	}
	if effects[0].Data["result"] != "tied" {
		t.Errorf("expected result=tied, got %v", effects[0].Data["result"])
	}
}

func TestVoteResolver_Invalid(t *testing.T) {
	resolver := NewVoteResolver()
	state := NewState()
	state.AddPlayer("p1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	state.AddPlayer("p2", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	config := DefaultGameConfig()

	uses := []*SkillUse{
		// Not a vote skill
		{PlayerID: "p1", Skill: pb.SkillType_SKILL_TYPE_KILL, TargetID: "p2"},
		// Empty target
		{PlayerID: "p2", Skill: pb.SkillType_SKILL_TYPE_VOTE, TargetID: ""},
	}

	effects := resolver.Resolve(uses, state, config)

	// Should be treated as tie (no valid votes)
	if len(effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effects))
	}
	if effects[0].Type != pb.EventType_EVENT_TYPE_UNSPECIFIED {
		t.Errorf("expected EVENT_TYPE_UNSPECIFIED for invalid votes, got %v", effects[0].Type)
	}
}

// ==================== DayResolver Tests ====================

func TestDayResolver(t *testing.T) {
	resolver := NewDayResolver()
	state := NewState()
	config := DefaultGameConfig()

	uses := []*SkillUse{
		{PlayerID: "p1", Skill: pb.SkillType_SKILL_TYPE_SPEAK, TargetID: ""},
		{PlayerID: "p2", Skill: pb.SkillType_SKILL_TYPE_SPEAK, TargetID: ""},
	}

	effects := resolver.Resolve(uses, state, config)

	if len(effects) != 0 {
		t.Errorf("expected 0 effects for day phase, got %d", len(effects))
	}
}

// ==================== WolfResolver Tests (Sub-step mode) ====================

func TestWolfResolver_VoteTie_NoKill(t *testing.T) {
	resolver := NewWolfResolver()
	state := NewState()
	state.AddPlayer("wolf1", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.AddPlayer("wolf2", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.AddPlayer("v1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	state.AddPlayer("v2", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	config := DefaultGameConfig()

	// 平票：wolf1 投 v1, wolf2 投 v2
	uses := []*SkillUse{
		{PlayerID: "wolf1", Skill: pb.SkillType_SKILL_TYPE_KILL, TargetID: "v1"},
		{PlayerID: "wolf2", Skill: pb.SkillType_SKILL_TYPE_KILL, TargetID: "v2"},
	}

	effects := resolver.Resolve(uses, state, config)

	// 平票应该不产生击杀
	killEffects := filterEffects(effects, pb.EventType_EVENT_TYPE_KILL)
	if len(killEffects) != 0 {
		t.Errorf("expected 0 kill effects for tie vote, got %d", len(killEffects))
	}

	// Night.KillTarget 应该为空
	if state.RoundCtx.KillTarget != "" {
		t.Errorf("expected empty Night.KillTarget for tie, got %s", state.RoundCtx.KillTarget)
	}
}

func TestWolfResolver_Consensus_Kill(t *testing.T) {
	resolver := NewWolfResolver()
	state := NewState()
	state.AddPlayer("wolf1", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.AddPlayer("wolf2", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.AddPlayer("victim", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	config := DefaultGameConfig()

	// 达成共识：两个狼人投同一个目标
	uses := []*SkillUse{
		{PlayerID: "wolf1", Skill: pb.SkillType_SKILL_TYPE_KILL, TargetID: "victim"},
		{PlayerID: "wolf2", Skill: pb.SkillType_SKILL_TYPE_KILL, TargetID: "victim"},
	}

	effects := resolver.Resolve(uses, state, config)

	// 子阶段模式下，WolfResolver 返回 SET_NIGHT_KILL effect
	// 实际击杀在 SeerResolver 中处理
	if len(effects) != 1 {
		t.Errorf("expected 1 effect from WolfResolver, got %d", len(effects))
	}

	if effects[0].Type != pb.EventType_EVENT_TYPE_SET_NIGHT_KILL {
		t.Errorf("expected SET_NIGHT_KILL effect, got %v", effects[0].Type)
	}

	if effects[0].TargetID != "victim" {
		t.Errorf("expected target=victim, got %s", effects[0].TargetID)
	}

	// 应用 Effect 后 Night.KillTarget 才会被设置
	for _, e := range effects {
		state.ApplyEffect(e)
	}
	if state.RoundCtx.KillTarget != "victim" {
		t.Errorf("expected Night.KillTarget=victim after applying effect, got %s", state.RoundCtx.KillTarget)
	}
}

func TestWolfResolver_Majority_Kill(t *testing.T) {
	resolver := NewWolfResolver()
	state := NewState()
	state.AddPlayer("wolf1", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.AddPlayer("wolf2", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.AddPlayer("wolf3", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.AddPlayer("v1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	state.AddPlayer("v2", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	config := DefaultGameConfig()

	// 多数决：2票 v1, 1票 v2
	uses := []*SkillUse{
		{PlayerID: "wolf1", Skill: pb.SkillType_SKILL_TYPE_KILL, TargetID: "v1"},
		{PlayerID: "wolf2", Skill: pb.SkillType_SKILL_TYPE_KILL, TargetID: "v1"},
		{PlayerID: "wolf3", Skill: pb.SkillType_SKILL_TYPE_KILL, TargetID: "v2"},
	}

	effects := resolver.Resolve(uses, state, config)

	// 子阶段模式下，WolfResolver 返回 SET_NIGHT_KILL effect
	if len(effects) != 1 {
		t.Errorf("expected 1 effect from WolfResolver, got %d", len(effects))
	}

	if effects[0].TargetID != "v1" {
		t.Errorf("expected target=v1, got %s", effects[0].TargetID)
	}

	// 应用 Effect 后 Night.KillTarget 才会被设置
	for _, e := range effects {
		state.ApplyEffect(e)
	}
	if state.RoundCtx.KillTarget != "v1" {
		t.Errorf("expected Night.KillTarget=v1 after applying effect, got %s", state.RoundCtx.KillTarget)
	}
}

func TestWolfResolver_SameGuardKill_Empty(t *testing.T) {
	// 同守同杀空刀：当守卫保护的目标被狼人攻击时，不设置击杀目标
	// 这样女巫不知道有人被攻击
	resolver := NewWolfResolver()
	state := NewState()
	state.AddPlayer("wolf", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.AddPlayer("victim", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	// 使用 NightContext 设置保护状态
	state.RoundCtx.ProtectedPlayers["victim"] = true
	config := DefaultGameConfig()
	config.SameGuardKillIsEmpty = true

	uses := []*SkillUse{
		{PlayerID: "wolf", Skill: pb.SkillType_SKILL_TYPE_KILL, TargetID: "victim"},
	}

	effects := resolver.Resolve(uses, state, config)

	// 同守同杀时不返回任何 effect
	if len(effects) != 0 {
		t.Errorf("expected 0 effects when same guard kill, got %d", len(effects))
	}

	// Night.KillTarget 应该为空
	if state.RoundCtx.KillTarget != "" {
		t.Errorf("expected empty Night.KillTarget, got %s", state.RoundCtx.KillTarget)
	}
}

func TestWolfResolver_Protected_NotEmpty(t *testing.T) {
	// 当 SameGuardKillIsEmpty=false 时，即使目标被保护也设置击杀目标
	resolver := NewWolfResolver()
	state := NewState()
	state.AddPlayer("wolf", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.AddPlayer("victim", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	// 使用 NightContext 设置保护状态
	state.RoundCtx.ProtectedPlayers["victim"] = true
	config := DefaultGameConfig()
	config.SameGuardKillIsEmpty = false // 不是空刀

	uses := []*SkillUse{
		{PlayerID: "wolf", Skill: pb.SkillType_SKILL_TYPE_KILL, TargetID: "victim"},
	}

	effects := resolver.Resolve(uses, state, config)

	// 应该返回 SET_NIGHT_KILL effect
	if len(effects) != 1 {
		t.Errorf("expected 1 effect, got %d", len(effects))
	}

	// 应用 Effect
	for _, e := range effects {
		state.ApplyEffect(e)
	}

	// Night.KillTarget 应该被设置
	if state.RoundCtx.KillTarget != "victim" {
		t.Errorf("expected Night.KillTarget=victim, got %s", state.RoundCtx.KillTarget)
	}
}

// ==================== WitchResolver Tests (Sub-step mode) ====================

func TestWitchResolver_QueryKillTarget(t *testing.T) {
	resolver := NewWitchResolver()
	state := NewState()
	state.AddPlayer("witch", pb.RoleType_ROLE_TYPE_WITCH, pb.Camp_CAMP_GOOD)
	state.AddPlayer("victim", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	// 使用 NightContext 设置击杀目标
	state.RoundCtx.KillTarget = "victim"
	config := DefaultGameConfig()

	// 女巫使用解药救人
	uses := []*SkillUse{
		{PlayerID: "witch", Skill: pb.SkillType_SKILL_TYPE_ANTIDOTE, TargetID: "victim"},
	}

	effects := resolver.Resolve(uses, state, config)

	// 应该有3个effect: USE_ANTIDOTE, CLEAR_NIGHT_KILL, SAVE
	if len(effects) != 3 {
		t.Fatalf("expected 3 effects, got %d", len(effects))
	}

	saveEffects := filterEffects(effects, pb.EventType_EVENT_TYPE_SAVE)
	if len(saveEffects) != 1 {
		t.Fatalf("expected 1 save effect, got %d", len(saveEffects))
	}

	// 应用所有 Effect
	for _, e := range effects {
		state.ApplyEffect(e)
	}

	// Night.KillTarget 应该被清除
	if state.RoundCtx.KillTarget != "" {
		t.Errorf("expected Night.KillTarget cleared after applying effects, got %s", state.RoundCtx.KillTarget)
	}

	// 解药应该被消耗
	witch, _ := state.getPlayer("witch")
	if witch.HasAntidote {
		t.Errorf("expected witch to have used antidote")
	}
}

func TestWitchResolver_Poison(t *testing.T) {
	resolver := NewWitchResolver()
	state := NewState()
	state.AddPlayer("witch", pb.RoleType_ROLE_TYPE_WITCH, pb.Camp_CAMP_GOOD)
	state.AddPlayer("wolf", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	config := DefaultGameConfig()

	uses := []*SkillUse{
		{PlayerID: "witch", Skill: pb.SkillType_SKILL_TYPE_POISON, TargetID: "wolf"},
	}

	effects := resolver.Resolve(uses, state, config)

	// WitchResolver 只产生 USE_POISON 效果，实际死亡由 NightResolveResolver 处理
	usePoisonEffects := filterEffects(effects, pb.EventType_EVENT_TYPE_USE_POISON)
	if len(usePoisonEffects) != 1 {
		t.Fatalf("expected 1 USE_POISON effect, got %d", len(usePoisonEffects))
	}
	if usePoisonEffects[0].TargetID != "wolf" {
		t.Errorf("expected target=wolf, got %s", usePoisonEffects[0].TargetID)
	}

	// 应用效果后，目标应该被标记为中毒
	for _, e := range effects {
		state.ApplyEffect(e)
	}
	if !state.RoundCtx.IsPoisoned("wolf") {
		t.Error("expected wolf to be marked as poisoned after applying USE_POISON")
	}
}

func TestWitchResolver_CannotSaveSelf(t *testing.T) {
	resolver := NewWitchResolver()
	state := NewState()
	state.AddPlayer("witch", pb.RoleType_ROLE_TYPE_WITCH, pb.Camp_CAMP_GOOD)
	state.RoundCtx.KillTarget = "witch" // 狼人杀女巫
	config := DefaultGameConfig()
	config.WitchCanSaveSelf = false

	uses := []*SkillUse{
		{PlayerID: "witch", Skill: pb.SkillType_SKILL_TYPE_ANTIDOTE, TargetID: "witch"},
	}

	effects := resolver.Resolve(uses, state, config)

	saveEffects := filterEffects(effects, pb.EventType_EVENT_TYPE_SAVE)
	if len(saveEffects) != 1 {
		t.Fatalf("expected 1 save effect, got %d", len(saveEffects))
	}
	if !saveEffects[0].Canceled {
		t.Error("expected save to be canceled when witch tries to save self")
	}

	// Night.KillTarget 应该保持不变
	if state.RoundCtx.KillTarget != "witch" {
		t.Errorf("expected Night.KillTarget=witch, got %s", state.RoundCtx.KillTarget)
	}
}

// ==================== GuardResolver Tests (Sub-step mode) ====================

func TestGuardResolver_Protect(t *testing.T) {
	resolver := NewGuardResolver()
	state := NewState()
	state.AddPlayer("guard", pb.RoleType_ROLE_TYPE_GUARD, pb.Camp_CAMP_GOOD)
	state.AddPlayer("target", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	config := DefaultGameConfig()

	uses := []*SkillUse{
		{PlayerID: "guard", Skill: pb.SkillType_SKILL_TYPE_PROTECT, TargetID: "target"},
	}

	effects := resolver.Resolve(uses, state, config)

	// 现在返回2个effect: SET_LAST_PROTECTED + PROTECT
	if len(effects) != 2 {
		t.Fatalf("expected 2 effects, got %d", len(effects))
	}

	// 检查 SET_LAST_PROTECTED effect
	if effects[0].Type != pb.EventType_EVENT_TYPE_SET_LAST_PROTECTED {
		t.Errorf("expected EVENT_TYPE_SET_LAST_PROTECTED, got %v", effects[0].Type)
	}

	// 检查 PROTECT effect
	if effects[1].Type != pb.EventType_EVENT_TYPE_PROTECT {
		t.Errorf("expected EVENT_TYPE_PROTECT, got %v", effects[1].Type)
	}

	// 应用所有效果
	for _, e := range effects {
		state.ApplyEffect(e)
	}

	// 目标应该被标记为受保护（使用 NightContext）
	if !state.RoundCtx.IsProtected("target") {
		t.Error("expected target to be protected after applying effect")
	}

	// LastProtectedTarget 应该被设置
	guard := state.players["guard"]
	if guard.LastProtectedTarget != "target" {
		t.Errorf("expected guard.LastProtectedTarget=target, got %s", guard.LastProtectedTarget)
	}
}

// ==================== SeerResolver Tests (Sub-step mode) ====================

func TestSeerResolver_CheckWolf(t *testing.T) {
	resolver := NewSeerResolver()
	state := NewState()
	state.AddPlayer("seer", pb.RoleType_ROLE_TYPE_SEER, pb.Camp_CAMP_GOOD)
	state.AddPlayer("wolf", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	config := DefaultGameConfig()

	uses := []*SkillUse{
		{PlayerID: "seer", Skill: pb.SkillType_SKILL_TYPE_CHECK, TargetID: "wolf"},
	}

	effects := resolver.Resolve(uses, state, config)

	if len(effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effects))
	}
	if effects[0].Data["isGood"] != false {
		t.Error("expected isGood=false for wolf")
	}
	if effects[0].Data["camp"] != pb.Camp_CAMP_EVIL {
		t.Errorf("expected camp=EVIL, got %v", effects[0].Data["camp"])
	}
}

func TestSeerResolver_CheckGood(t *testing.T) {
	resolver := NewSeerResolver()
	state := NewState()
	state.AddPlayer("seer", pb.RoleType_ROLE_TYPE_SEER, pb.Camp_CAMP_GOOD)
	state.AddPlayer("villager", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	config := DefaultGameConfig()

	uses := []*SkillUse{
		{PlayerID: "seer", Skill: pb.SkillType_SKILL_TYPE_CHECK, TargetID: "villager"},
	}

	effects := resolver.Resolve(uses, state, config)

	if len(effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effects))
	}
	if effects[0].Data["isGood"] != true {
		t.Error("expected isGood=true for villager")
	}
	if effects[0].Data["camp"] != pb.Camp_CAMP_GOOD {
		t.Errorf("expected camp=GOOD, got %v", effects[0].Data["camp"])
	}
}

// ==================== State.GetWolfTeammates Tests ====================

func TestState_GetWolfTeammates(t *testing.T) {
	state := NewState()
	state.AddPlayer("wolf1", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.AddPlayer("wolf2", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.AddPlayer("wolf3", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.AddPlayer("villager", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	teammates := state.GetWolfTeammates("wolf1")

	// wolf1 的队友应该是 wolf2 和 wolf3（不包括自己）
	if len(teammates) != 2 {
		t.Fatalf("expected 2 teammates, got %d", len(teammates))
	}

	hasWolf2 := false
	hasWolf3 := false
	for _, id := range teammates {
		if id == "wolf2" {
			hasWolf2 = true
		}
		if id == "wolf3" {
			hasWolf3 = true
		}
		if id == "wolf1" {
			t.Error("should not include self in teammates")
		}
	}

	if !hasWolf2 || !hasWolf3 {
		t.Errorf("expected wolf2 and wolf3 as teammates, got %v", teammates)
	}
}

func TestState_GetWolfTeammates_NonWolf(t *testing.T) {
	state := NewState()
	state.AddPlayer("wolf1", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.AddPlayer("villager", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	// 非狼人查询应该返回空
	teammates := state.GetWolfTeammates("villager")
	if len(teammates) != 0 {
		t.Errorf("expected 0 teammates for non-wolf, got %d", len(teammates))
	}
}

// ==================== Helper Functions ====================

func filterEffects(effects []*Effect, eventType pb.EventType) []*Effect {
	result := make([]*Effect, 0)
	for _, e := range effects {
		if e.Type == eventType {
			result = append(result, e)
		}
	}
	return result
}
