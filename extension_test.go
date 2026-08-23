package werewolf

import (
	"github.com/Zereker/hiddenrole"
	"testing"
)

// 本文件用「加一个狼王」验证扩展契约：
// 全程只用导出 API，不改动库内任何一行代码。
//
// 狼王：狼人阵营，被投票放逐时可以开枪带走一名玩家。
// 它同时用到了三个扩展点——自定义角色、自定义阶段、死亡触发。

const (
	// 自定义取值从 1000 起，避免与后续内置枚举撞号
	roleWolfKing  = RoleType("WOLF_KING")
	skillWolfClaw = SkillType("WOLF_CLAW")
	phaseWolfKing = PhaseType("PHASE_WOLF_KING")

	// 狼王的枪：开局一发，打出去就没了。存在 PlayerVar 里、由
	// RoleSetup 发放，走的是与女巫的药完全相同的两条路。
	varWolfKingGun = "wolfking.gun"
)

// wolfKingSetup 狼王的初始状态：狼人阵营的狼，开局带一发子弹。
//
// 阵营与类别写在角色自己身上，而不是入座时的两个参数——引擎不认识狼王，
// 也就没有办法替它推导；写在这里，每一处入座都不会填错。
func wolfKingSetup(playerID string, role RoleType) map[string]string {
	out := CampVars(CampEvil, RoleCategoryWolf)
	out[varWolfKingGun] = VarPresent
	return out
}

// wolfKingResolver 狼王的开枪结算。第三方实现，只依赖导出的 GameView。
type wolfKingResolver struct{}

func (r *wolfKingResolver) Resolve(uses []*SkillUse, view GameView) []*Effect {
	effects := make([]*Effect, 0)
	for _, use := range uses {
		if use.Skill != skillWolfClaw || use.Target() == "" {
			continue
		}
		// 枪只有一发。子弹是入座时发的（wolfKingSetup），用掉即清空——
		// 与女巫的药同一条路：状态在 PlayerVar 里，改动经 Effect 表达。
		if view.Var(hiddenrole.ScopeGame.Of(use.PlayerID), varWolfKingGun) == "" {
			continue
		}
		// SHOOT 是狼王给「发生了什么」起的名字，内核不认得它；
		// 真正让人出局的是旁边那条 SET_ALIVE。内置角色的狼刀、
		// 投票放逐走的也是同一条路，第三方在这件事上没有额外负担。
		effects = append(effects,
			hiddenrole.NewEffect(EventShoot, use.PlayerID, use.Target()),
			hiddenrole.NewSetAliveEffect(use.Target(), false),
			hiddenrole.NewSetVarEffect(hiddenrole.ScopeGame.Of(use.PlayerID), varWolfKingGun, ""))
		break
	}
	return effects
}

// voteWithWolfKing 包装内置投票解析器：被放逐者若是狼王，追加一个死亡触发。
//
// 这演示了「装饰已有解析器」——扩展不必从零实现整个阶段。
type voteWithWolfKing struct {
	inner hiddenrole.Resolver
}

func (r *voteWithWolfKing) Resolve(uses []*SkillUse, view GameView) []*Effect {
	effects := r.inner.Resolve(uses, view)
	for _, ef := range effects {
		if ef.Type != EventEliminate || ef.Canceled {
			continue
		}
		if p, ok := view.Player(ef.TargetID); ok && p.Role == roleWolfKing {
			effects = append(effects, hiddenrole.NewDetourEffect(ef.TargetID, phaseWolfKing))
		}
	}
	return effects
}

// newWolfKingGame 组装一局带狼王的游戏，全部通过导出 API
func newWolfKingGame(t *testing.T) *Engine {
	t.Helper()

	cfg := DefaultGameConfig()

	// 1. 声明狼王阶段，结算完回到夜晚
	cfg.Phases[phaseWolfKing] = &PhaseConfig{
		Type: phaseWolfKing,
		Steps: []PhaseStep{
			{Role: roleWolfKing, Skill: skillWolfClaw},
			{Role: roleWolfKing, Skill: SkillSkip},
		},
		NextPhase: PhaseNightGuard,
	}

	// 2. 构造时注册狼王阶段的解析器，并装饰投票解析器
	eng, err := NewWith(cfg, DefaultRules(),
		hiddenrole.WithRoleSetup(roleWolfKing, hiddenrole.RoleSetupFunc(wolfKingSetup)),
		hiddenrole.WithResolver(phaseWolfKing, &wolfKingResolver{}),
		hiddenrole.WithResolver(PhaseVote,
			&voteWithWolfKing{inner: NewVoteResolver()}))
	if err != nil {
		t.Fatalf("配置应当合法: %v", err)
	}

	// 3. 狼王的阵营与类别推导不出来，显式给出
	if err := eng.AddPlayer("wk", roleWolfKing); err != nil {
		t.Fatal(err)
	}
	for id, role := range map[string]RoleType{
		"w1": RoleWerewolf,
		"s":  RoleSeer,
		"g":  RoleGuard,
		"v1": RoleVillager,
		"v2": RoleVillager,
		"v3": RoleVillager,
	} {
		if err := eng.AddPlayer(id, role); err != nil {
			t.Fatal(err)
		}
	}

	if err := eng.Start(); err != nil {
		t.Fatal(err)
	}
	return eng
}

// TestExtension_WolfKing 第三方角色端到端可用
func TestExtension_WolfKing(t *testing.T) {
	eng := newWolfKingGame(t)

	// 走完第一夜（狼人空刀）
	for eng.Status().Phase != PhaseDay {
		if _, err := eng.EndPhase(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := eng.EndPhase(); err != nil { // DAY -> VOTE
		t.Fatal(err)
	}

	// 放逐狼王
	for _, voter := range []string{"s", "g", "v1", "v2"} {
		if err := eng.SubmitSkillUse(&SkillUse{
			PlayerID: voter, Skill: SkillVote, Targets: []string{"wk"},
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := eng.EndPhase(); err != nil {
		t.Fatal(err)
	}

	// 引擎应当自动流转到自定义的狼王阶段
	if got := eng.Status().Phase; got != phaseWolfKing {
		t.Fatalf("期望进入狼王阶段，实际 %v", got)
	}

	// 别的玩家不能冒用狼王的技能
	if err := eng.SubmitSkillUse(&SkillUse{
		PlayerID: "w1", Skill: skillWolfClaw, Targets: []string{"s"},
	}); err == nil {
		t.Error("非触发者不应能使用狼王技能")
	}

	// 狼王开枪带走预言家
	if err := eng.SubmitSkillUse(&SkillUse{
		PlayerID: "wk", Skill: skillWolfClaw, Targets: []string{"s"},
	}); err != nil {
		t.Fatalf("被触发的狼王应当可以开枪: %v", err)
	}
	if _, err := eng.EndPhase(); err != nil {
		t.Fatal(err)
	}

	if alive := mustInfo(t, eng, "s").Alive; alive {
		t.Error("狼王开枪带走的预言家应当出局")
	}
	if got := eng.Status().Phase; got != PhaseNightGuard {
		t.Errorf("狼王阶段结束后应进入下一夜，实际 %v", got)
	}
}

// TestExtension_WolfKingCountsAsWolfForVictory 自定义类别参与屠边判定
func TestExtension_WolfKingCountsAsWolfForVictory(t *testing.T) {
	eng := newWolfKingGame(t)

	// 内置狼人出局，狼王还在 —— 狼人阵营未灭，游戏继续
	eng.Apply(hiddenrole.NewSetAliveEffect("w1", false))
	if over, _ := checkVictory(eng); over {
		t.Error("狼王仍在场，狼人阵营不应判为全灭")
	}

	// 狼王也出局 —— 好人获胜
	eng.Apply(hiddenrole.NewSetAliveEffect("wk", false))
	over, winner := checkVictory(eng)
	if !over || winner != CampGood {
		t.Errorf("狼人阵营全灭应判好人胜利，实际 over=%v winner=%v", over, winner)
	}
}

// TestExtension_WithResolverRejectsNil 注册入口的边界。
//
// 想让某阶段不产生任何效果，注册一个返回空切片的解析器；
// 传 nil 只可能是漏了，必须在构造时就报出来。
func TestExtension_WithResolverRejectsNil(t *testing.T) {
	if _, err := hiddenrole.NewEngine(nil, hiddenrole.WithResolver(PhaseDay, nil)); err == nil {
		t.Error("nil 解析器应当被拒绝")
	}
	if !panicsOnNilResolver(t) {
		t.Error("hiddenrole.MustNewEngine 遇到非法选项应当 panic")
	}
}

func panicsOnNilResolver(t *testing.T) (panicked bool) {
	t.Helper()
	defer func() { panicked = recover() != nil }()
	MustNew(DefaultRules(), hiddenrole.WithResolver(PhaseDay, nil))
	return false
}

func mustInfo(t *testing.T, e *Engine, id string) hiddenrole.PlayerInfo {
	t.Helper()
	p, ok := e.PlayerInfo(id)
	if !ok {
		t.Fatalf("玩家不存在: %s", id)
	}
	return p
}

// TestExtension_CustomPhaseGetsPhaseInfo 自定义阶段也能拿到阶段信息。
//
// 此前 PhaseInfo 是一个写死内置阶段的 switch，自定义阶段返回空，
// 调用方无从得知该让谁行动。
func TestExtension_CustomPhaseGetsPhaseInfo(t *testing.T) {
	eng := newWolfKingGame(t)

	for eng.Status().Phase != PhaseDay {
		if _, err := eng.EndPhase(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := eng.EndPhase(); err != nil { // DAY -> VOTE
		t.Fatal(err)
	}
	for _, voter := range []string{"s", "g", "v1", "v2"} {
		if err := eng.SubmitSkillUse(&SkillUse{
			PlayerID: voter, Skill: SkillVote, Targets: []string{"wk"},
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := eng.EndPhase(); err != nil {
		t.Fatal(err)
	}
	if eng.Status().Phase != phaseWolfKing {
		t.Fatalf("期望进入狼王阶段，实际 %v", eng.Status().Phase)
	}

	info := eng.PhaseInfo()
	ri := info.RoleInfos[roleWolfKing]
	if ri == nil {
		t.Fatal("自定义角色应当出现在 RoleInfos 中")
	}
	if len(ri.PlayerIDs) != 1 || ri.PlayerIDs[0] != "wk" {
		t.Errorf("行动者: 期望 [wk]，实际 %v", ri.PlayerIDs)
	}
	if len(ri.AllowedSkills) != 2 {
		t.Errorf("可用技能: 期望 2 个（开枪与跳过），实际 %v", ri.AllowedSkills)
	}
	if len(info.ActiveRoles) != 1 || info.ActiveRoles[0] != roleWolfKing {
		t.Errorf("ActiveRoles: 期望 [狼王]，实际 %v", info.ActiveRoles)
	}

	// 视角同样对自定义角色生效
	v := eng.PlayerView("wk")
	if len(v.AllowedSkills) != 2 {
		t.Errorf("狼王视角应当看到自己的两个技能，实际 %v", v.AllowedSkills)
	}
	if got := eng.PlayerView("v3").AllowedSkills; len(got) != 0 {
		t.Errorf("非触发者不应有可用技能，实际 %v", got)
	}
}

// TestExtension_CustomWolfCampRoleIsPartOfTheTeam 自定义的狼队角色要真的算进狼队。
//
// 狼队的判定此前写死 WEREWOLF，而狼王、白狼王、狼美人经
// 它的 Camp 是 EVIL 而 Role 不是 WEREWOLF：
// 他们看不到队友、不被真狼看到、夜里也发不出话——自定义狼队角色实际不可用。
func TestExtension_CustomWolfCampRoleIsPartOfTheTeam(t *testing.T) {
	const roleWolfKing = RoleType("WOLF_KING")

	eng := MustNew(DefaultRules(), hiddenrole.WithRoleSetup(roleWolfKing, hiddenrole.RoleSetupFunc(
		func(string, RoleType) map[string]string {
			return CampVars(CampEvil, RoleCategoryWolf)
		})))
	mustAdd(t, eng, "w1", RoleWerewolf)
	if err := eng.AddPlayer("wk", roleWolfKing); err != nil {
		t.Fatalf("AddPlayer 失败: %v", err)
	}
	mustAdd(t, eng, "s", RoleSeer)
	mustAdd(t, eng, "v1", RoleVillager)
	mustAdd(t, eng, "v2", RoleVillager)
	if err := eng.Start(); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}

	// 互为队友
	if got := eng.Teammates("wk"); len(got) != 1 || got[0] != "w1" {
		t.Errorf("狼王应当看到队友 w1，实际 %v", got)
	}
	if got := eng.Teammates("w1"); len(got) != 1 || got[0] != "wk" {
		t.Errorf("真狼应当看到队友 wk，实际 %v", got)
	}
	if got := eng.Teammates("s"); got != nil {
		t.Errorf("好人不该有狼队友，实际 %v", got)
	}

	// 视图里互相翻牌
	view := eng.PlayerView("wk")
	roles := make(map[string]RoleType, len(view.Players))
	for _, p := range view.Players {
		roles[p.ID] = p.Role
	}
	if roles["w1"] != RoleWerewolf {
		t.Errorf("狼王的视图里应能看到 w1 的身份，实际 %v", roles["w1"])
	}
	if roles["s"] != hiddenrole.RoleUnspecified {
		t.Errorf("狼王不该看到预言家的身份，实际 %v", roles["s"])
	}

	// 夜里能和狼队互通
	for eng.Status().Phase != PhaseNightWolf {
		if _, err := eng.EndPhase(); err != nil {
			t.Fatalf("推进失败: %v", err)
		}
	}
	if got := eng.MessageReceivers("wk"); len(got) != 2 {
		t.Errorf("狼王夜里应能与整个狼队互通，实际 %v", got)
	}
	if err := eng.SendMessage("wk", "刀预言家"); err != nil {
		t.Errorf("狼王夜里应当能发言，实际 %v", err)
	}
	if err := eng.SendMessage("s", "?"); err == nil {
		t.Error("好人夜里不该能发言")
	}
}

// TestExtension_TriggerToUnconfiguredPhaseIsRejected 指向未配置阶段的死亡触发要被就地否决。
//
// 这条边是运行期才成形的，GameConfig.Validate 看不见它。放任不管的话，
// 引擎会流转到一个没有配置、没有解析器的阶段，玩家提交什么都不允许，
// 下一次推进直接进 END——游戏在第一夜无声收场，连 GAME_ENDED 都没有。
func TestExtension_TriggerToUnconfiguredPhaseIsRejected(t *testing.T) {
	cfg := DefaultGameConfig()
	delete(cfg.Phases, PhaseNightHunter)
	// 原本指向猎人阶段的静态边也要改掉，否则配置本身就不自洽
	cfg.Phases[PhaseNightResolve].NextPhase = PhaseDay

	g := newRuleGame(t, cfg, seats(
		wolf("w1"), wolf("w2"), hunter("h"), seer("s"),
		villagers("v1", "v2", "v3", "v4"),
	)...)

	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "h")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)

	// 猎人死了，触发指向已被删掉的阶段：应当照常进白天，且触发被否决
	effects := g.end(PhaseDay)
	trigger := findEffect(effects, hiddenrole.EventDetour)
	if trigger == nil {
		t.Fatal("期望产生 ABILITY_TRIGGERED 效果（即便被否决）")
	}
	if !trigger.Canceled {
		t.Error("目标阶段不在配置里，触发应当被否决")
	}
	if got := g.e.RoundContext().Detours; len(got) != 0 {
		t.Errorf("被否决的触发不该入队，实际 %v", got)
	}
	g.assertAlive("h", false, "猎人被刀")
}
