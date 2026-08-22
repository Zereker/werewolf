package werewolf

import (
	"testing"
)

// ==================== VoteResolver Tests ====================

func TestVoteResolver_Empty(t *testing.T) {
	resolver := NewVoteResolver()
	state := newState()
	config := DefaultGameConfig()

	effects := resolver.Resolve([]*SkillUse{}, newStateView(state), config)

	if len(effects) != 1 {
		t.Fatalf("expected 1 effect (tied), got %d", len(effects))
	}
	if effects[0].Type != EventVoteTied {
		t.Errorf("expected VOTE_TIED for empty votes, got %v", effects[0].Type)
	}
}

func TestVoteResolver_Single(t *testing.T) {
	resolver := NewVoteResolver()
	state := newState()
	mustAddTo(t, state, "p1", RoleVillager)
	mustAddTo(t, state, "p2", RoleVillager)
	config := DefaultGameConfig()

	uses := []*SkillUse{
		{PlayerID: "p1", Skill: SkillVote, TargetID: "p2"},
	}

	effects := resolver.Resolve(uses, newStateView(state), config)

	// 断言的是结果而不是效果个数：ELIMINATE 是「发生了什么」的说法，
	// 旁边那条 SET_ALIVE 才真正让人出局，两者缺一不可。
	elim := filterEffects(effects, EventEliminate)
	if len(elim) != 1 || elim[0].TargetID != "p2" {
		t.Fatalf("expected one ELIMINATE on p2, got %v", effects)
	}
	for _, e := range effects {
		state.applyEffect(e)
	}
	if p, _ := state.getPlayer("p2"); p.Alive {
		t.Error("被放逐的玩家应当出局")
	}
}

func TestVoteResolver_Clear(t *testing.T) {
	resolver := NewVoteResolver()
	state := newState()
	mustAddTo(t, state, "p1", RoleVillager)
	mustAddTo(t, state, "p2", RoleVillager)
	mustAddTo(t, state, "p3", RoleVillager)
	mustAddTo(t, state, "wolf", RoleWerewolf)
	config := DefaultGameConfig()

	uses := []*SkillUse{
		{PlayerID: "p1", Skill: SkillVote, TargetID: "wolf"},
		{PlayerID: "p2", Skill: SkillVote, TargetID: "wolf"},
		{PlayerID: "p3", Skill: SkillVote, TargetID: "p1"},
	}

	effects := resolver.Resolve(uses, newStateView(state), config)

	elim := filterEffects(effects, EventEliminate)
	if len(elim) != 1 || elim[0].TargetID != "wolf" {
		t.Fatalf("expected one ELIMINATE on wolf (majority), got %v", effects)
	}
	for _, e := range effects {
		state.applyEffect(e)
	}
	if p, _ := state.getPlayer("wolf"); p.Alive {
		t.Error("得票最多的玩家应当出局")
	}
}

func TestVoteResolver_Tie(t *testing.T) {
	resolver := NewVoteResolver()
	state := newState()
	mustAddTo(t, state, "p1", RoleVillager)
	mustAddTo(t, state, "p2", RoleVillager)
	mustAddTo(t, state, "p3", RoleVillager)
	mustAddTo(t, state, "p4", RoleVillager)
	config := DefaultGameConfig()

	uses := []*SkillUse{
		{PlayerID: "p1", Skill: SkillVote, TargetID: "p3"},
		{PlayerID: "p2", Skill: SkillVote, TargetID: "p3"},
		{PlayerID: "p3", Skill: SkillVote, TargetID: "p4"},
		{PlayerID: "p4", Skill: SkillVote, TargetID: "p4"},
	}

	effects := resolver.Resolve(uses, newStateView(state), config)

	if len(effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effects))
	}
	if effects[0].Type != EventVoteTied {
		t.Errorf("expected VOTE_TIED for tie, got %v", effects[0].Type)
	}
	if effects[0].Data["result"] != "tied" {
		t.Errorf("expected result=tied, got %v", effects[0].Data["result"])
	}
}

func TestVoteResolver_Invalid(t *testing.T) {
	resolver := NewVoteResolver()
	state := newState()
	mustAddTo(t, state, "p1", RoleVillager)
	mustAddTo(t, state, "p2", RoleVillager)
	config := DefaultGameConfig()

	uses := []*SkillUse{
		// Not a vote skill
		{PlayerID: "p1", Skill: SkillKill, TargetID: "p2"},
		// Empty target
		{PlayerID: "p2", Skill: SkillVote, TargetID: ""},
	}

	effects := resolver.Resolve(uses, newStateView(state), config)

	// Should be treated as tie (no valid votes)
	if len(effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effects))
	}
	if effects[0].Type != EventVoteTied {
		t.Errorf("expected VOTE_TIED for invalid votes, got %v", effects[0].Type)
	}
}

// ==================== DayResolver Tests ====================

func TestDayResolver(t *testing.T) {
	resolver := NewDayResolver()
	state := newState()
	config := DefaultGameConfig()

	uses := []*SkillUse{
		{PlayerID: "p1", Skill: SkillSpeak, TargetID: ""},
		{PlayerID: "p2", Skill: SkillSpeak, TargetID: ""},
	}

	effects := resolver.Resolve(uses, newStateView(state), config)

	if len(effects) != 0 {
		t.Errorf("expected 0 effects for day phase, got %d", len(effects))
	}
}

// ==================== WolfResolver Tests (Sub-step mode) ====================

func TestWolfResolver_VoteTie_NoKill(t *testing.T) {
	resolver := NewWolfResolver()
	state := newState()
	mustAddTo(t, state, "wolf1", RoleWerewolf)
	mustAddTo(t, state, "wolf2", RoleWerewolf)
	mustAddTo(t, state, "v1", RoleVillager)
	mustAddTo(t, state, "v2", RoleVillager)
	config := DefaultGameConfig()

	// 平票：wolf1 投 v1, wolf2 投 v2
	uses := []*SkillUse{
		{PlayerID: "wolf1", Skill: SkillKill, TargetID: "v1"},
		{PlayerID: "wolf2", Skill: SkillKill, TargetID: "v2"},
	}

	effects := resolver.Resolve(uses, newStateView(state), config)

	// 平票应该不产生击杀
	killEffects := filterEffects(effects, EventKill)
	if len(killEffects) != 0 {
		t.Errorf("expected 0 kill effects for tie vote, got %d", len(killEffects))
	}

	// Night.KillTarget 应该为空
	if killTargetOf(state) != "" {
		t.Errorf("expected empty Night.KillTarget for tie, got %s", killTargetOf(state))
	}
}

func TestWolfResolver_Consensus_Kill(t *testing.T) {
	resolver := NewWolfResolver()
	state := newState()
	mustAddTo(t, state, "wolf1", RoleWerewolf)
	mustAddTo(t, state, "wolf2", RoleWerewolf)
	mustAddTo(t, state, "victim", RoleVillager)
	config := DefaultGameConfig()

	// 达成共识：两个狼人投同一个目标
	uses := []*SkillUse{
		{PlayerID: "wolf1", Skill: SkillKill, TargetID: "victim"},
		{PlayerID: "wolf2", Skill: SkillKill, TargetID: "victim"},
	}

	effects := resolver.Resolve(uses, newStateView(state), config)

	// 狼人阶段只记刀口，实际结算在 NightResolveResolver。
	// 刀口是一个回合变量，不是内核认得的「击杀」事件。
	if len(effects) != 1 {
		t.Errorf("expected 1 effect from WolfResolver, got %d", len(effects))
	}
	if effects[0].Type != EventSetRoundVar {
		t.Errorf("expected SET_ROUND_VAR effect, got %v", effects[0].Type)
	}

	// 应用 Effect 后刀口才会被设置
	for _, e := range effects {
		state.applyEffect(e)
	}
	if killTargetOf(state) != "victim" {
		t.Errorf("expected Night.KillTarget=victim after applying effect, got %s", killTargetOf(state))
	}
}

func TestWolfResolver_Majority_Kill(t *testing.T) {
	resolver := NewWolfResolver()
	state := newState()
	mustAddTo(t, state, "wolf1", RoleWerewolf)
	mustAddTo(t, state, "wolf2", RoleWerewolf)
	mustAddTo(t, state, "wolf3", RoleWerewolf)
	mustAddTo(t, state, "v1", RoleVillager)
	mustAddTo(t, state, "v2", RoleVillager)
	config := DefaultGameConfig()

	// 多数决：2票 v1, 1票 v2
	uses := []*SkillUse{
		{PlayerID: "wolf1", Skill: SkillKill, TargetID: "v1"},
		{PlayerID: "wolf2", Skill: SkillKill, TargetID: "v1"},
		{PlayerID: "wolf3", Skill: SkillKill, TargetID: "v2"},
	}

	effects := resolver.Resolve(uses, newStateView(state), config)

	// 狼人阶段只记刀口，实际结算在 NightResolveResolver
	if len(effects) != 1 {
		t.Errorf("expected 1 effect from WolfResolver, got %d", len(effects))
	}

	// 应用 Effect 后刀口才会被设置
	for _, e := range effects {
		state.applyEffect(e)
	}
	if killTargetOf(state) != "v1" {
		t.Errorf("expected Night.KillTarget=v1 after applying effect, got %s", killTargetOf(state))
	}
}

func TestWolfResolver_SetsKillTargetEvenIfProtected(t *testing.T) {
	// 狼人不知道守卫守了谁，刀是照砍的：无论目标是否被守护都记录刀口。
	// 守护能否抵消由 NightResolveResolver 判定。
	//
	// 若此处因守护而不记录刀口，女巫就看不到刀口，
	// 「同守同救」（守卫守护 + 女巫解药 -> 依然死亡）这一局面将无法构成。
	resolver := NewWolfResolver()
	state := newState()
	mustAddTo(t, state, "wolf", RoleWerewolf)
	mustAddTo(t, state, "victim", RoleVillager)
	// 使用 NightContext 设置保护状态
	markRound(state, "victim", PlayerRoundVarProtected)
	config := DefaultGameConfig()
	config.SameGuardKillIsEmpty = true

	uses := []*SkillUse{
		{PlayerID: "wolf", Skill: SkillKill, TargetID: "victim"},
	}

	effects := resolver.Resolve(uses, newStateView(state), config)

	// 目标被守护，但刀口仍应被记录
	if len(effects) != 1 {
		t.Fatalf("expected 1 effect (SET_NIGHT_KILL) even when protected, got %d", len(effects))
	}
	if effects[0].Type != EventSetRoundVar {
		t.Errorf("expected SET_NIGHT_KILL, got %v", effects[0].Type)
	}

	for _, e := range effects {
		state.applyEffect(e)
	}
	if killTargetOf(state) != "victim" {
		t.Errorf("expected Night.KillTarget=victim, got %s", killTargetOf(state))
	}
}

func TestWolfResolver_Protected_NotEmpty(t *testing.T) {
	// 当 SameGuardKillIsEmpty=false 时，即使目标被保护也设置击杀目标
	resolver := NewWolfResolver()
	state := newState()
	mustAddTo(t, state, "wolf", RoleWerewolf)
	mustAddTo(t, state, "victim", RoleVillager)
	// 使用 NightContext 设置保护状态
	markRound(state, "victim", PlayerRoundVarProtected)
	config := DefaultGameConfig()
	config.SameGuardKillIsEmpty = false // 不是空刀

	uses := []*SkillUse{
		{PlayerID: "wolf", Skill: SkillKill, TargetID: "victim"},
	}

	effects := resolver.Resolve(uses, newStateView(state), config)

	// 应该返回 SET_NIGHT_KILL effect
	if len(effects) != 1 {
		t.Errorf("expected 1 effect, got %d", len(effects))
	}

	// 应用 Effect
	for _, e := range effects {
		state.applyEffect(e)
	}

	// Night.KillTarget 应该被设置
	if killTargetOf(state) != "victim" {
		t.Errorf("expected Night.KillTarget=victim, got %s", killTargetOf(state))
	}
}

// ==================== WitchResolver Tests (Sub-step mode) ====================

func TestWitchResolver_QueryKillTarget(t *testing.T) {
	resolver := NewWitchResolver()
	state := newState()
	mustAddTo(t, state, "witch", RoleWitch)
	mustAddTo(t, state, "victim", RoleVillager)
	// 使用 NightContext 设置击杀目标
	setKill(state, "victim")
	config := DefaultGameConfig()

	// 女巫使用解药救人
	uses := []*SkillUse{
		{PlayerID: "witch", Skill: SkillAntidote, TargetID: "victim"},
	}

	effects := resolver.Resolve(uses, newStateView(state), config)

	// 三个效果：SAVE（说法）、解药少一瓶、目标带上「今晚被救」的标记。
	// 解药不再直接清除刀口——是否真的救回由 NightResolveResolver
	// 综合「是否同时被守卫守护」判定
	if len(effects) != 3 {
		t.Fatalf("expected 3 effects, got %d: %v", len(effects), effects)
	}

	saveEffects := filterEffects(effects, EventSave)
	if len(saveEffects) != 1 {
		t.Fatalf("expected 1 save effect, got %d", len(saveEffects))
	}

	// 应用所有 Effect
	for _, e := range effects {
		state.applyEffect(e)
	}

	// 刀口保留到结算阶段，但目标已被标记为「已救」
	if killTargetOf(state) != "victim" {
		t.Errorf("expected Night.KillTarget kept until resolve, got %s", killTargetOf(state))
	}
	if !savedIn(state, "victim") {
		t.Error("expected victim to be marked as saved")
	}

	// 解药应该被消耗
	witch, _ := state.getPlayer("witch")
	if witch.Vars[VarWitchAntidote] != "" {
		t.Errorf("expected witch to have used antidote")
	}
}

func TestWitchResolver_Poison(t *testing.T) {
	resolver := NewWitchResolver()
	state := newState()
	mustAddTo(t, state, "witch", RoleWitch)
	mustAddTo(t, state, "wolf", RoleWerewolf)
	config := DefaultGameConfig()

	uses := []*SkillUse{
		{PlayerID: "witch", Skill: SkillPoison, TargetID: "wolf"},
	}

	effects := resolver.Resolve(uses, newStateView(state), config)

	// WitchResolver 只产生 USE_POISON 效果，实际死亡由 NightResolveResolver 处理
	usePoisonEffects := filterEffects(effects, EventSetPlayerRoundVar)
	if len(usePoisonEffects) != 1 {
		t.Fatalf("expected 1 USE_POISON effect, got %d", len(usePoisonEffects))
	}
	if usePoisonEffects[0].TargetID != "wolf" {
		t.Errorf("expected target=wolf, got %s", usePoisonEffects[0].TargetID)
	}

	// 应用效果后，目标应该被标记为中毒
	for _, e := range effects {
		state.applyEffect(e)
	}
	if !poisonedIn(state, "wolf") {
		t.Error("expected wolf to be marked as poisoned after applying USE_POISON")
	}
}

func TestWitchResolver_CannotSaveSelf(t *testing.T) {
	resolver := NewWitchResolver()
	state := newState()
	mustAddTo(t, state, "witch", RoleWitch)
	setKill(state, "witch") // 狼人杀女巫
	config := DefaultGameConfig()
	config.WitchCanSaveSelf = false

	uses := []*SkillUse{
		{PlayerID: "witch", Skill: SkillAntidote, TargetID: "witch"},
	}

	effects := resolver.Resolve(uses, newStateView(state), config)

	saveEffects := filterEffects(effects, EventSave)
	if len(saveEffects) != 1 {
		t.Fatalf("expected 1 save effect, got %d", len(saveEffects))
	}
	if !saveEffects[0].Canceled {
		t.Error("expected save to be canceled when witch tries to save self")
	}

	// Night.KillTarget 应该保持不变
	if killTargetOf(state) != "witch" {
		t.Errorf("expected Night.KillTarget=witch, got %s", killTargetOf(state))
	}
}

// ==================== GuardResolver Tests (Sub-step mode) ====================

func TestGuardResolver_Protect(t *testing.T) {
	resolver := NewGuardResolver()
	state := newState()
	mustAddTo(t, state, "guard", RoleGuard)
	mustAddTo(t, state, "target", RoleVillager)
	config := DefaultGameConfig()

	uses := []*SkillUse{
		{PlayerID: "guard", Skill: SkillProtect, TargetID: "target"},
	}

	effects := resolver.Resolve(uses, newStateView(state), config)

	// PROTECT 是说法，另外三条是状态：今晚谁被守了，以及守卫这一回合
	// 守的是谁（供下回合判断连守）。
	if got := len(filterEffects(effects, EventProtect)); got != 1 {
		t.Fatalf("expected one PROTECT, got %d in %v", got, effects)
	}
	if got := len(filterEffects(effects, EventSetPlayerRoundVar)); got != 1 {
		t.Fatalf("expected one round mark, got %d in %v", got, effects)
	}
	if got := len(filterEffects(effects, EventSetPlayerVar)); got != 2 {
		t.Fatalf("expected two guard records, got %d in %v", got, effects)
	}

	// 应用所有效果
	for _, e := range effects {
		state.applyEffect(e)
	}

	// 目标应该被标记为受保护（使用 NightContext）
	if !protectedIn(state, "target") {
		t.Error("expected target to be protected after applying effect")
	}

	// 守护记录应该被写下，供下回合判断连守
	guard := state.players["guard"]
	if got := guard.Vars[PlayerVarLastProtectedTarget]; got != "target" {
		t.Errorf("expected guard last protected target=target, got %s", got)
	}
}

// ==================== SeerResolver Tests (Sub-step mode) ====================

func TestSeerResolver_CheckWolf(t *testing.T) {
	resolver := NewSeerResolver()
	state := newState()
	mustAddTo(t, state, "seer", RoleSeer)
	mustAddTo(t, state, "wolf", RoleWerewolf)
	config := DefaultGameConfig()

	uses := []*SkillUse{
		{PlayerID: "seer", Skill: SkillCheck, TargetID: "wolf"},
	}

	effects := resolver.Resolve(uses, newStateView(state), config)

	if len(effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effects))
	}
	if effects[0].Data["isGood"] != false {
		t.Error("expected isGood=false for wolf")
	}
	if effects[0].Data["camp"] != CampEvil {
		t.Errorf("expected camp=EVIL, got %v", effects[0].Data["camp"])
	}
}

func TestSeerResolver_CheckGood(t *testing.T) {
	resolver := NewSeerResolver()
	state := newState()
	mustAddTo(t, state, "seer", RoleSeer)
	mustAddTo(t, state, "villager", RoleVillager)
	config := DefaultGameConfig()

	uses := []*SkillUse{
		{PlayerID: "seer", Skill: SkillCheck, TargetID: "villager"},
	}

	effects := resolver.Resolve(uses, newStateView(state), config)

	if len(effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(effects))
	}
	if effects[0].Data["isGood"] != true {
		t.Error("expected isGood=true for villager")
	}
	if effects[0].Data["camp"] != CampGood {
		t.Errorf("expected camp=GOOD, got %v", effects[0].Data["camp"])
	}
}

// ==================== State.WolfTeammates Tests ====================

func TestState_GetWolfTeammates(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "wolf1", RoleWerewolf)
	mustAddTo(t, state, "wolf2", RoleWerewolf)
	mustAddTo(t, state, "wolf3", RoleWerewolf)
	mustAddTo(t, state, "villager", RoleVillager)

	teammates := state.getWolfTeammates("wolf1")

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
	state := newState()
	mustAddTo(t, state, "wolf1", RoleWerewolf)
	mustAddTo(t, state, "villager", RoleVillager)

	// 非狼人查询应该返回空
	teammates := state.getWolfTeammates("villager")
	if len(teammates) != 0 {
		t.Errorf("expected 0 teammates for non-wolf, got %d", len(teammates))
	}
}

// ==================== Helper Functions ====================

func filterEffects(effects []*Effect, eventType EventType) []*Effect {
	result := make([]*Effect, 0)
	for _, e := range effects {
		if e.Type == eventType {
			result = append(result, e)
		}
	}
	return result
}

// TestNightResolveResolver_PoisonOrderIsDeterministic 同一个局面结算出的效果顺序必须稳定。
//
// 毒杀名单是一个 map，直接遍历产出效果的话，同一个局面每次结算的顺序
// 都不一样，效果流的回放与比对就没了确定性。
func TestNightResolveResolver_PoisonOrderIsDeterministic(t *testing.T) {
	st := newState()
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		if err := st.addPlayer(id, RoleVillager); err != nil {
			t.Fatal(err)
		}
		markRound(st, id, PlayerRoundVarPoisoned)
	}

	r := NewNightResolveResolver()
	want := targetsOf(r.Resolve(nil, newStateView(st), DefaultGameConfig()))
	for i := 0; i < 20; i++ {
		got := targetsOf(r.Resolve(nil, newStateView(st), DefaultGameConfig()))
		if len(got) != len(want) {
			t.Fatalf("效果数不稳定: %v vs %v", want, got)
		}
		for j := range got {
			if got[j] != want[j] {
				t.Fatalf("第 %d 次结算顺序不同: %v vs %v", i, want, got)
			}
		}
	}
}

// targetsOf 取出效果的目标列表，用于比对顺序。
func targetsOf(effects []*Effect) []string {
	out := make([]string, 0, len(effects))
	for _, e := range effects {
		out = append(out, e.TargetID)
	}
	return out
}
