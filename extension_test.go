package werewolf

import (
	"testing"

	pb "github.com/Zereker/werewolf/proto"
)

// 本文件用「加一个狼王」验证扩展契约：
// 全程只用导出 API，不改动库内任何一行代码。
//
// 狼王：狼人阵营，被投票放逐时可以开枪带走一名玩家。
// 它同时用到了三个扩展点——自定义角色、自定义阶段、死亡触发。

const (
	// 自定义取值从 1000 起，避免与后续内置枚举撞号
	roleWolfKing  = pb.RoleType(1000)
	skillWolfClaw = pb.SkillType(1000)
	phaseWolfKing = pb.PhaseType(1000)
)

// wolfKingResolver 狼王的开枪结算。第三方实现，只依赖导出的 GameView。
type wolfKingResolver struct{}

func (r *wolfKingResolver) Resolve(uses []*SkillUse, view GameView, config *GameConfig) []*Effect {
	effects := make([]*Effect, 0)
	for _, use := range uses {
		if use.Skill == skillWolfClaw && use.TargetID != "" {
			effects = append(effects,
				NewEffect(pb.EventType_EVENT_TYPE_SHOOT, use.PlayerID, use.TargetID))
			break // 一枪
		}
	}
	return effects
}

// voteWithWolfKing 包装内置投票解析器：被放逐者若是狼王，追加一个死亡触发。
//
// 这演示了「装饰已有解析器」——扩展不必从零实现整个阶段。
type voteWithWolfKing struct {
	inner Resolver
}

func (r *voteWithWolfKing) Resolve(uses []*SkillUse, view GameView, config *GameConfig) []*Effect {
	effects := r.inner.Resolve(uses, view, config)
	for _, ef := range effects {
		if ef.Type != pb.EventType_EVENT_TYPE_ELIMINATE || ef.Canceled {
			continue
		}
		if p, ok := view.Player(ef.TargetID); ok && p.Role == roleWolfKing {
			effects = append(effects, NewAbilityTriggerEffect(ef.TargetID, phaseWolfKing))
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
			{Role: roleWolfKing, Skill: pb.SkillType_SKILL_TYPE_SKIP},
		},
		NextPhase: pb.PhaseType_PHASE_TYPE_NIGHT_GUARD,
	}

	engine, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("配置应当合法: %v", err)
	}

	// 2. 注册狼王阶段的解析器，并装饰投票解析器
	if err := engine.RegisterResolver(phaseWolfKing, &wolfKingResolver{}); err != nil {
		t.Fatal(err)
	}
	if err := engine.RegisterResolver(pb.PhaseType_PHASE_TYPE_VOTE,
		&voteWithWolfKing{inner: NewVoteResolver()}); err != nil {
		t.Fatal(err)
	}

	// 3. 狼王的阵营与类别推导不出来，显式给出
	if err := engine.AddCustomPlayer("wk", roleWolfKing,
		pb.Camp_CAMP_EVIL, RoleCategoryWolf); err != nil {
		t.Fatal(err)
	}
	for id, role := range map[string]pb.RoleType{
		"w1": pb.RoleType_ROLE_TYPE_WEREWOLF,
		"s":  pb.RoleType_ROLE_TYPE_SEER,
		"g":  pb.RoleType_ROLE_TYPE_GUARD,
		"v1": pb.RoleType_ROLE_TYPE_VILLAGER,
		"v2": pb.RoleType_ROLE_TYPE_VILLAGER,
		"v3": pb.RoleType_ROLE_TYPE_VILLAGER,
	} {
		if err := engine.AddPlayer(id, role); err != nil {
			t.Fatal(err)
		}
	}

	if err := engine.Start(); err != nil {
		t.Fatal(err)
	}
	return engine
}

// TestExtension_WolfKing 第三方角色端到端可用
func TestExtension_WolfKing(t *testing.T) {
	engine := newWolfKingGame(t)

	// 走完第一夜（狼人空刀）
	for engine.GetCurrentPhase() != pb.PhaseType_PHASE_TYPE_DAY {
		if _, err := engine.EndPhase(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := engine.EndPhase(); err != nil { // DAY -> VOTE
		t.Fatal(err)
	}

	// 放逐狼王
	for _, voter := range []string{"s", "g", "v1", "v2"} {
		if err := engine.SubmitSkillUse(&SkillUse{
			PlayerID: voter, Skill: pb.SkillType_SKILL_TYPE_VOTE, TargetID: "wk",
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := engine.EndPhase(); err != nil {
		t.Fatal(err)
	}

	// 引擎应当自动流转到自定义的狼王阶段
	if got := engine.GetCurrentPhase(); got != phaseWolfKing {
		t.Fatalf("期望进入狼王阶段，实际 %v", got)
	}

	// 别的玩家不能冒用狼王的技能
	if err := engine.SubmitSkillUse(&SkillUse{
		PlayerID: "w1", Skill: skillWolfClaw, TargetID: "s",
	}); err == nil {
		t.Error("非触发者不应能使用狼王技能")
	}

	// 狼王开枪带走预言家
	if err := engine.SubmitSkillUse(&SkillUse{
		PlayerID: "wk", Skill: skillWolfClaw, TargetID: "s",
	}); err != nil {
		t.Fatalf("被触发的狼王应当可以开枪: %v", err)
	}
	if _, err := engine.EndPhase(); err != nil {
		t.Fatal(err)
	}

	if alive := mustInfo(t, engine, "s").Alive; alive {
		t.Error("狼王开枪带走的预言家应当出局")
	}
	if got := engine.GetCurrentPhase(); got != pb.PhaseType_PHASE_TYPE_NIGHT_GUARD {
		t.Errorf("狼王阶段结束后应进入下一夜，实际 %v", got)
	}
}

// TestExtension_WolfKingCountsAsWolfForVictory 自定义类别参与屠边判定
func TestExtension_WolfKingCountsAsWolfForVictory(t *testing.T) {
	engine := newWolfKingGame(t)

	// 内置狼人出局，狼王还在 —— 狼人阵营未灭，游戏继续
	engine.state.applyEffect(NewEffect(pb.EventType_EVENT_TYPE_KILL, "", "w1"))
	if over, _ := engine.state.checkVictory(VictoryModeSideWipe); over {
		t.Error("狼王仍在场，狼人阵营不应判为全灭")
	}

	// 狼王也出局 —— 好人获胜
	engine.state.applyEffect(NewEffect(pb.EventType_EVENT_TYPE_KILL, "", "wk"))
	over, winner := engine.state.checkVictory(VictoryModeSideWipe)
	if !over || winner != pb.Camp_CAMP_GOOD {
		t.Errorf("狼人阵营全灭应判好人胜利，实际 over=%v winner=%v", over, winner)
	}
}

// TestExtension_RegisterResolverRejects 注册入口的边界
func TestExtension_RegisterResolverRejects(t *testing.T) {
	engine := MustNewEngine(nil)

	if err := engine.RegisterResolver(pb.PhaseType_PHASE_TYPE_DAY, nil); err == nil {
		t.Error("nil 解析器应当被拒绝")
	}

	mustAdd(t, engine, "w1", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	if err := engine.Start(); err != nil {
		t.Fatal(err)
	}
	if err := engine.RegisterResolver(pb.PhaseType_PHASE_TYPE_DAY, NewDayResolver()); err != ErrGameAlreadyStarted {
		t.Errorf("开局后注册应返回 ErrGameAlreadyStarted，实际 %v", err)
	}
}

func mustInfo(t *testing.T, e *Engine, id string) PlayerInfo {
	t.Helper()
	p, ok := e.GetPlayerInfo(id)
	if !ok {
		t.Fatalf("玩家不存在: %s", id)
	}
	return p
}

// TestExtension_CustomPhaseGetsPhaseInfo 自定义阶段也能拿到阶段信息。
//
// 此前 GetPhaseInfo 是一个写死内置阶段的 switch，自定义阶段返回空，
// 调用方无从得知该让谁行动。
func TestExtension_CustomPhaseGetsPhaseInfo(t *testing.T) {
	engine := newWolfKingGame(t)

	for engine.GetCurrentPhase() != pb.PhaseType_PHASE_TYPE_DAY {
		if _, err := engine.EndPhase(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := engine.EndPhase(); err != nil { // DAY -> VOTE
		t.Fatal(err)
	}
	for _, voter := range []string{"s", "g", "v1", "v2"} {
		if err := engine.SubmitSkillUse(&SkillUse{
			PlayerID: voter, Skill: pb.SkillType_SKILL_TYPE_VOTE, TargetID: "wk",
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := engine.EndPhase(); err != nil {
		t.Fatal(err)
	}
	if engine.GetCurrentPhase() != phaseWolfKing {
		t.Fatalf("期望进入狼王阶段，实际 %v", engine.GetCurrentPhase())
	}

	info := engine.GetPhaseInfo()
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
	v := engine.GetPlayerView("wk")
	if len(v.AllowedSkills) != 2 {
		t.Errorf("狼王视角应当看到自己的两个技能，实际 %v", v.AllowedSkills)
	}
	if got := engine.GetPlayerView("v3").AllowedSkills; len(got) != 0 {
		t.Errorf("非触发者不应有可用技能，实际 %v", got)
	}
}
