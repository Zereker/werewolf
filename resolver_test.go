package werewolf

import (
	"github.com/Zereker/werewolf/engine"
	"testing"
)

// ==================== VoteResolver Tests ====================

func TestVoteResolver_Empty(t *testing.T) {
	resolver := NewVoteResolver()
	b := newBoard()

	effects := resolver.Resolve([]*SkillUse{}, b.View())

	if len(effects) != 1 {
		t.Fatalf("expected 1 effect (tied), got %d", len(effects))
	}
	if effects[0].Type != EventVoteTied {
		t.Errorf("expected VOTE_TIED for empty votes, got %v", effects[0].Type)
	}
}

func TestVoteResolver_Single(t *testing.T) {
	resolver := NewVoteResolver()
	b := newBoard()
	b.Players = append(b.Players, seatOf("p1", RoleVillager))
	b.Players = append(b.Players, seatOf("p2", RoleVillager))

	uses := []*SkillUse{
		{PlayerID: "p1", Skill: SkillVote, Targets: []string{"p2"}},
	}

	effects := resolver.Resolve(uses, b.View())

	// 断言的是结果而不是效果个数：ELIMINATE 是「发生了什么」的说法，
	// 旁边那条 SET_ALIVE 才真正让人出局，两者缺一不可。
	elim := filterEffects(effects, EventEliminate)
	if len(elim) != 1 || elim[0].TargetID != "p2" {
		t.Fatalf("expected one ELIMINATE on p2, got %v", effects)
	}
	b = b.Apply(effects)
	if p, _ := b.Player("p2"); p.Alive {
		t.Error("被放逐的玩家应当出局")
	}
}

func TestVoteResolver_Clear(t *testing.T) {
	resolver := NewVoteResolver()
	b := newBoard()
	b.Players = append(b.Players, seatOf("p1", RoleVillager))
	b.Players = append(b.Players, seatOf("p2", RoleVillager))
	b.Players = append(b.Players, seatOf("p3", RoleVillager))
	b.Players = append(b.Players, seatOf("wolf", RoleWerewolf))

	uses := []*SkillUse{
		{PlayerID: "p1", Skill: SkillVote, Targets: []string{"wolf"}},
		{PlayerID: "p2", Skill: SkillVote, Targets: []string{"wolf"}},
		{PlayerID: "p3", Skill: SkillVote, Targets: []string{"p1"}},
	}

	effects := resolver.Resolve(uses, b.View())

	elim := filterEffects(effects, EventEliminate)
	if len(elim) != 1 || elim[0].TargetID != "wolf" {
		t.Fatalf("expected one ELIMINATE on wolf (majority), got %v", effects)
	}
	b = b.Apply(effects)
	if p, _ := b.Player("wolf"); p.Alive {
		t.Error("得票最多的玩家应当出局")
	}
}

func TestVoteResolver_Tie(t *testing.T) {
	resolver := NewVoteResolver()
	b := newBoard()
	b.Players = append(b.Players, seatOf("p1", RoleVillager))
	b.Players = append(b.Players, seatOf("p2", RoleVillager))
	b.Players = append(b.Players, seatOf("p3", RoleVillager))
	b.Players = append(b.Players, seatOf("p4", RoleVillager))

	uses := []*SkillUse{
		{PlayerID: "p1", Skill: SkillVote, Targets: []string{"p3"}},
		{PlayerID: "p2", Skill: SkillVote, Targets: []string{"p3"}},
		{PlayerID: "p3", Skill: SkillVote, Targets: []string{"p4"}},
		{PlayerID: "p4", Skill: SkillVote, Targets: []string{"p4"}},
	}

	effects := resolver.Resolve(uses, b.View())

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
	b := newBoard()
	b.Players = append(b.Players, seatOf("p1", RoleVillager))
	b.Players = append(b.Players, seatOf("p2", RoleVillager))

	uses := []*SkillUse{
		// Not a vote skill
		{PlayerID: "p1", Skill: SkillKill, Targets: []string{"p2"}},
		// Empty target
		{PlayerID: "p2", Skill: SkillVote, Targets: []string{""}},
	}

	effects := resolver.Resolve(uses, b.View())

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
	b := newBoard()

	uses := []*SkillUse{
		{PlayerID: "p1", Skill: SkillSpeak, Targets: []string{""}},
		{PlayerID: "p2", Skill: SkillSpeak, Targets: []string{""}},
	}

	effects := resolver.Resolve(uses, b.View())

	if len(effects) != 0 {
		t.Errorf("expected 0 effects for day phase, got %d", len(effects))
	}
}

// ==================== WolfResolver Tests (Sub-step mode) ====================

func TestWolfResolver_VoteTie_NoKill(t *testing.T) {
	resolver := NewWolfResolver()
	b := newBoard()
	b.Players = append(b.Players, seatOf("wolf1", RoleWerewolf))
	b.Players = append(b.Players, seatOf("wolf2", RoleWerewolf))
	b.Players = append(b.Players, seatOf("v1", RoleVillager))
	b.Players = append(b.Players, seatOf("v2", RoleVillager))

	// 平票：wolf1 投 v1, wolf2 投 v2
	uses := []*SkillUse{
		{PlayerID: "wolf1", Skill: SkillKill, Targets: []string{"v1"}},
		{PlayerID: "wolf2", Skill: SkillKill, Targets: []string{"v2"}},
	}

	effects := resolver.Resolve(uses, b.View())

	// 平票应该不产生击杀
	killEffects := filterEffects(effects, EventKill)
	if len(killEffects) != 0 {
		t.Errorf("expected 0 kill effects for tie vote, got %d", len(killEffects))
	}

	// Night.KillTarget 应该为空
	if b.RoundVar(RoundVarKillTarget) != "" {
		t.Errorf("expected empty Night.KillTarget for tie, got %s", b.RoundVar(RoundVarKillTarget))
	}
}

func TestWolfResolver_Consensus_Kill(t *testing.T) {
	resolver := NewWolfResolver()
	b := newBoard()
	b.Players = append(b.Players, seatOf("wolf1", RoleWerewolf))
	b.Players = append(b.Players, seatOf("wolf2", RoleWerewolf))
	b.Players = append(b.Players, seatOf("victim", RoleVillager))

	// 达成共识：两个狼人投同一个目标
	uses := []*SkillUse{
		{PlayerID: "wolf1", Skill: SkillKill, Targets: []string{"victim"}},
		{PlayerID: "wolf2", Skill: SkillKill, Targets: []string{"victim"}},
	}

	effects := resolver.Resolve(uses, b.View())

	// 狼人阶段只记刀口，实际结算在 NightResolveResolver。
	// 刀口是一个回合变量，不是内核认得的「击杀」事件。
	if len(effects) != 1 {
		t.Errorf("expected 1 effect from WolfResolver, got %d", len(effects))
	}
	if effects[0].Type != engine.EventSetRoundVar {
		t.Errorf("expected SET_ROUND_VAR effect, got %v", effects[0].Type)
	}

	// 应用 Effect 后刀口才会被设置
	b = b.Apply(effects)
	if b.RoundVar(RoundVarKillTarget) != "victim" {
		t.Errorf("expected Night.KillTarget=victim after applying effect, got %s", b.RoundVar(RoundVarKillTarget))
	}
}

func TestWolfResolver_Majority_Kill(t *testing.T) {
	resolver := NewWolfResolver()
	b := newBoard()
	b.Players = append(b.Players, seatOf("wolf1", RoleWerewolf))
	b.Players = append(b.Players, seatOf("wolf2", RoleWerewolf))
	b.Players = append(b.Players, seatOf("wolf3", RoleWerewolf))
	b.Players = append(b.Players, seatOf("v1", RoleVillager))
	b.Players = append(b.Players, seatOf("v2", RoleVillager))

	// 多数决：2票 v1, 1票 v2
	uses := []*SkillUse{
		{PlayerID: "wolf1", Skill: SkillKill, Targets: []string{"v1"}},
		{PlayerID: "wolf2", Skill: SkillKill, Targets: []string{"v1"}},
		{PlayerID: "wolf3", Skill: SkillKill, Targets: []string{"v2"}},
	}

	effects := resolver.Resolve(uses, b.View())

	// 狼人阶段只记刀口，实际结算在 NightResolveResolver
	if len(effects) != 1 {
		t.Errorf("expected 1 effect from WolfResolver, got %d", len(effects))
	}

	// 应用 Effect 后刀口才会被设置
	b = b.Apply(effects)
	if b.RoundVar(RoundVarKillTarget) != "v1" {
		t.Errorf("expected Night.KillTarget=v1 after applying effect, got %s", b.RoundVar(RoundVarKillTarget))
	}
}

func TestWolfResolver_SetsKillTargetEvenIfProtected(t *testing.T) {
	// 狼人不知道守卫守了谁，刀是照砍的：无论目标是否被守护都记录刀口。
	// 守护能否抵消由 NightResolveResolver 判定。
	//
	// 若此处因守护而不记录刀口，女巫就看不到刀口，
	// 「同守同救」（守卫守护 + 女巫解药 -> 依然死亡）这一局面将无法构成。
	resolver := NewWolfResolver()
	b := newBoard()
	b.Players = append(b.Players, seatOf("wolf", RoleWerewolf))
	b.Players = append(b.Players, seatOf("victim", RoleVillager))
	// 使用 NightContext 设置保护状态
	b = markSeat(b, "victim", PlayerRoundVarProtected)
	rules := DefaultRules()
	rules.SameGuardKillIsEmpty = true

	uses := []*SkillUse{
		{PlayerID: "wolf", Skill: SkillKill, Targets: []string{"victim"}},
	}

	effects := resolver.Resolve(uses, b.View())

	// 目标被守护，但刀口仍应被记录
	if len(effects) != 1 {
		t.Fatalf("expected 1 effect (SET_NIGHT_KILL) even when protected, got %d", len(effects))
	}
	if effects[0].Type != engine.EventSetRoundVar {
		t.Errorf("expected SET_NIGHT_KILL, got %v", effects[0].Type)
	}

	b = b.Apply(effects)
	if b.RoundVar(RoundVarKillTarget) != "victim" {
		t.Errorf("expected Night.KillTarget=victim, got %s", b.RoundVar(RoundVarKillTarget))
	}
}

func TestWolfResolver_Protected_NotEmpty(t *testing.T) {
	// 当 SameGuardKillIsEmpty=false 时，即使目标被保护也设置击杀目标
	resolver := NewWolfResolver()
	b := newBoard()
	b.Players = append(b.Players, seatOf("wolf", RoleWerewolf))
	b.Players = append(b.Players, seatOf("victim", RoleVillager))
	// 使用 NightContext 设置保护状态
	b = markSeat(b, "victim", PlayerRoundVarProtected)
	rules := DefaultRules()
	rules.SameGuardKillIsEmpty = false // 不是空刀

	uses := []*SkillUse{
		{PlayerID: "wolf", Skill: SkillKill, Targets: []string{"victim"}},
	}

	effects := resolver.Resolve(uses, b.View())

	// 应该返回 SET_NIGHT_KILL effect
	if len(effects) != 1 {
		t.Errorf("expected 1 effect, got %d", len(effects))
	}

	// 应用 Effect
	b = b.Apply(effects)

	// Night.KillTarget 应该被设置
	if b.RoundVar(RoundVarKillTarget) != "victim" {
		t.Errorf("expected Night.KillTarget=victim, got %s", b.RoundVar(RoundVarKillTarget))
	}
}

// ==================== WitchResolver Tests (Sub-step mode) ====================

func TestWitchResolver_QueryKillTarget(t *testing.T) {
	rules := DefaultRules()
	resolver := NewWitchResolver(rules)
	b := newBoard()
	b.Players = append(b.Players, seatOf("witch", RoleWitch))
	b.Players = append(b.Players, seatOf("victim", RoleVillager))
	// 使用 NightContext 设置击杀目标
	b = withKill(b, "victim")

	// 女巫使用解药救人
	uses := []*SkillUse{
		{PlayerID: "witch", Skill: SkillAntidote, Targets: []string{"victim"}},
	}

	effects := resolver.Resolve(uses, b.View())

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
	b = b.Apply(effects)

	// 刀口保留到结算阶段，但目标已被标记为「已救」
	if b.RoundVar(RoundVarKillTarget) != "victim" {
		t.Errorf("expected Night.KillTarget kept until resolve, got %s", b.RoundVar(RoundVarKillTarget))
	}
	if roundVarOfBoard(b, "victim", PlayerRoundVarSaved) == "" {
		t.Error("expected victim to be marked as saved")
	}

	// 解药应该被消耗
	witch, _ := b.Player("witch")
	if witch.Vars[VarWitchAntidote] != "" {
		t.Errorf("expected witch to have used antidote")
	}
}

func TestWitchResolver_Poison(t *testing.T) {
	rules := DefaultRules()
	resolver := NewWitchResolver(rules)
	b := newBoard()
	b.Players = append(b.Players, seatOf("witch", RoleWitch))
	b.Players = append(b.Players, seatOf("wolf", RoleWerewolf))

	uses := []*SkillUse{
		{PlayerID: "witch", Skill: SkillPoison, Targets: []string{"wolf"}},
	}

	effects := resolver.Resolve(uses, b.View())

	// WitchResolver 只产生 USE_POISON 效果，实际死亡由 NightResolveResolver 处理
	usePoisonEffects := filterEffects(effects, engine.EventSetPlayerRoundVar)
	if len(usePoisonEffects) != 1 {
		t.Fatalf("expected 1 USE_POISON effect, got %d", len(usePoisonEffects))
	}
	if usePoisonEffects[0].TargetID != "wolf" {
		t.Errorf("expected target=wolf, got %s", usePoisonEffects[0].TargetID)
	}

	// 应用效果后，目标应该被标记为中毒
	b = b.Apply(effects)
	if roundVarOfBoard(b, "wolf", PlayerRoundVarPoisoned) == "" {
		t.Error("expected wolf to be marked as poisoned after applying USE_POISON")
	}
}

func TestWitchResolver_CannotSaveSelf(t *testing.T) {
	rules := DefaultRules()
	resolver := NewWitchResolver(rules)
	b := newBoard()
	b.Players = append(b.Players, seatOf("witch", RoleWitch))
	b = withKill(b, "witch") // 狼人杀女巫
	rules.WitchCanSaveSelf = false

	uses := []*SkillUse{
		{PlayerID: "witch", Skill: SkillAntidote, Targets: []string{"witch"}},
	}

	effects := resolver.Resolve(uses, b.View())

	saveEffects := filterEffects(effects, EventSave)
	if len(saveEffects) != 1 {
		t.Fatalf("expected 1 save effect, got %d", len(saveEffects))
	}
	if !saveEffects[0].Canceled {
		t.Error("expected save to be canceled when witch tries to save self")
	}

	// Night.KillTarget 应该保持不变
	if b.RoundVar(RoundVarKillTarget) != "witch" {
		t.Errorf("expected Night.KillTarget=witch, got %s", b.RoundVar(RoundVarKillTarget))
	}
}

// ==================== GuardResolver Tests (Sub-step mode) ====================

func TestGuardResolver_Protect(t *testing.T) {
	rules := DefaultRules()
	resolver := NewGuardResolver(rules)
	b := newBoard()
	b.Players = append(b.Players, seatOf("guard", RoleGuard))
	b.Players = append(b.Players, seatOf("target", RoleVillager))

	uses := []*SkillUse{
		{PlayerID: "guard", Skill: SkillProtect, Targets: []string{"target"}},
	}

	effects := resolver.Resolve(uses, b.View())

	// PROTECT 是说法，另外三条是状态：今晚谁被守了，以及守卫这一回合
	// 守的是谁（供下回合判断连守）。
	if got := len(filterEffects(effects, EventProtect)); got != 1 {
		t.Fatalf("expected one PROTECT, got %d in %v", got, effects)
	}
	if got := len(filterEffects(effects, engine.EventSetPlayerRoundVar)); got != 1 {
		t.Fatalf("expected one round mark, got %d in %v", got, effects)
	}
	if got := len(filterEffects(effects, engine.EventSetPlayerVar)); got != 2 {
		t.Fatalf("expected two guard records, got %d in %v", got, effects)
	}

	// 应用所有效果
	b = b.Apply(effects)

	// 目标应该被标记为受保护（使用 NightContext）
	if roundVarOfBoard(b, "target", PlayerRoundVarProtected) == "" {
		t.Error("expected target to be protected after applying effect")
	}

	// 守护记录应该被写下，供下回合判断连守
	guard := mustSeat(t, b, "guard")
	if got := guard.Vars[PlayerVarLastProtectedTarget]; got != "target" {
		t.Errorf("expected guard last protected target=target, got %s", got)
	}
}

// ==================== SeerResolver Tests (Sub-step mode) ====================

func TestSeerResolver_CheckWolf(t *testing.T) {
	resolver := NewSeerResolver()
	b := newBoard()
	b.Players = append(b.Players, seatOf("seer", RoleSeer))
	b.Players = append(b.Players, seatOf("wolf", RoleWerewolf))

	uses := []*SkillUse{
		{PlayerID: "seer", Skill: SkillCheck, Targets: []string{"wolf"}},
	}

	effects := resolver.Resolve(uses, b.View())

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
	b := newBoard()
	b.Players = append(b.Players, seatOf("seer", RoleSeer))
	b.Players = append(b.Players, seatOf("villager", RoleVillager))

	uses := []*SkillUse{
		{PlayerID: "seer", Skill: SkillCheck, Targets: []string{"villager"}},
	}

	effects := resolver.Resolve(uses, b.View())

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
	b := newBoard()
	b.Players = append(b.Players, seatOf("wolf1", RoleWerewolf))
	b.Players = append(b.Players, seatOf("wolf2", RoleWerewolf))
	b.Players = append(b.Players, seatOf("wolf3", RoleWerewolf))
	b.Players = append(b.Players, seatOf("villager", RoleVillager))

	teammates := wolfTeammates("wolf1", b.View())

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
	b := newBoard()
	b.Players = append(b.Players, seatOf("wolf1", RoleWerewolf))
	b.Players = append(b.Players, seatOf("villager", RoleVillager))

	// 非狼人查询应该返回空
	teammates := wolfTeammates("villager", b.View())
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
	b := newBoard()
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		b.Players = append(b.Players, seatOf(id, RoleVillager))
	}
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		b = markSeat(b, id, PlayerRoundVarPoisoned)
	}

	r := NewNightResolveResolver(DefaultRules())
	want := targetsOf(r.Resolve(nil, b.View()))
	for i := 0; i < 20; i++ {
		got := targetsOf(r.Resolve(nil, b.View()))
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
