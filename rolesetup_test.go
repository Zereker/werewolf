package werewolf

import (
	"github.com/Zereker/werewolf/engine"
	"testing"
)

// 本文件验证第五个扩展点：角色的初始状态。
//
// 标准还是那一条——加一个角色，不该要求改引擎里任何一行。此前引擎里
// 写着 `if role == RoleWitch { HasAntidote = true; HasPoison = true }`，
// 于是「骑士开局带一次决斗」这类事第三方根本没有办法表达。

const (
	// 骑士：好人，开局带一次决斗，用掉就没了。只为测初始状态而存在，
	// 不实现决斗的结算——那是 Resolver 的事，另有测试覆盖。
	roleKnight             = RoleType("KNIGHT")
	varKnightDuel          = "knight.duel"
	roleInfoKnightDuelLeft = "duel"
)

// knightSetup 骑士的初始状态。第三方实现，只依赖导出 API。
func knightSetup(playerID string, role RoleType) map[string]string {
	out := CampVars(CampGood, RoleCategoryGod)
	out[varKnightDuel] = VarPresent
	return out
}

// newKnightGame 组装一局带骑士的游戏，全部通过导出 API。
func newKnightGame(t *testing.T, extra ...EngineOption) *Engine {
	t.Helper()

	opts := append([]EngineOption{
		engine.WithRoleSetup(roleKnight, engine.RoleSetupFunc(knightSetup)),
	}, extra...)

	e := MustNew(DefaultRules(), opts...)
	seats := []struct {
		id   string
		role RoleType
	}{
		{"w1", RoleWerewolf}, {"w2", RoleWerewolf},
		{"wi", RoleWitch}, {"se", RoleSeer},
		{"v1", RoleVillager}, {"v2", RoleVillager},
	}
	for _, s := range seats {
		if err := e.AddPlayer(s.id, s.role); err != nil {
			t.Fatalf("AddPlayer(%s): %v", s.id, err)
		}
	}
	// 骑士与内置角色走同一个入座入口：阵营与类别写在它自己的 setup 里
	if err := e.AddPlayer("kn", roleKnight); err != nil {
		t.Fatalf("AddPlayer(kn): %v", err)
	}
	if err := e.Start(); err != nil {
		t.Fatalf("Start(): %v", err)
	}
	return e
}

// TestRoleSetup_CustomRoleGetsInitialState 第三方角色能给自己发初始状态。
//
// 这是整个扩展点存在的理由：在此之前，引擎里那个 if 只认女巫，
// 骑士开局带什么无处表达。
func TestRoleSetup_CustomRoleGetsInitialState(t *testing.T) {
	e := newKnightGame(t)

	kn, ok := e.PlayerInfo("kn")
	if !ok {
		t.Fatal("骑士不在场上")
	}
	if got := kn.Vars[varKnightDuel]; got != VarPresent {
		t.Fatalf("骑士开局应带着决斗，实际 %q", got)
	}

	// 别的角色不该被波及
	for _, id := range []string{"w1", "se", "v1"} {
		p, _ := e.PlayerInfo(id)
		if p.Vars[varKnightDuel] != "" {
			t.Errorf("%s 不是骑士，不该带着决斗", id)
		}
	}
}

// TestRoleSetup_BuiltinWitchWalksTheSamePath 内置女巫走的是同一张表。
//
// 「内置角色没有特权」不是口号：把 RoleWitch 的那一项换掉，女巫就真的
// 空着手上桌——说明引擎里再没有第二条给她发药的暗道。
func TestRoleSetup_BuiltinWitchWalksTheSamePath(t *testing.T) {
	t.Run("默认两瓶药", func(t *testing.T) {
		e := newKnightGame(t)
		wi, _ := e.PlayerInfo("wi")
		if wi.Vars[VarWitchAntidote] != VarPresent || wi.Vars[VarWitchPoison] != VarPresent {
			t.Fatalf("女巫开局应有两瓶药，实际 %v", wi.Vars)
		}
	})

	t.Run("换掉之后空手上桌", func(t *testing.T) {
		// 只留阵营，不发药——不留阵营的话开局就判好人少一个，
		// 与这条测试想验的事无关
		e := newKnightGame(t, engine.WithRoleSetup(RoleWitch,
			sideSetup(CampGood, RoleCategoryGod)))

		wi, _ := e.PlayerInfo("wi")
		if wi.Vars[VarWitchAntidote] != "" || wi.Vars[VarWitchPoison] != "" {
			t.Fatalf("换掉初始状态后女巫不该有药，实际 %v", wi.Vars)
		}

		// 没有药就用不出来：这一步确认状态真的参与了规则判定，
		// 而不只是视图上少显示了一行
		endTo(t, e, PhaseNightWitch)
		if err := e.SubmitSkillUse(&SkillUse{
			PlayerID: "wi", Skill: SkillAntidote, Targets: []string{"v1"},
		}); err != nil {
			t.Fatalf("提交解药: %v", err)
		}
		for _, ef := range mustEnd(t, e) {
			if ef.Type == EventSave && !ef.Canceled {
				t.Fatal("女巫没有解药，救人不该生效")
			}
		}
	})
}

// TestRoleSetup_SurvivesReplayWithoutTheOption 回放不需要再传一遍初始状态。
//
// 这是刻意设计的：初始状态记在效果流的入座那一条上，不是回放时重新问
// RoleSetup。否则回放方少传一个 WithRoleSetup，重建出来的角色就悄悄空着手，
// 而分叉要到他第一次用那个状态时才暴露——解析器漏传有 validateResolvers
// 拦得住，这里拦不住，因为「这个角色本来就没有初始状态」与「你忘了传」
// 在签名上无法区分。
func TestRoleSetup_SurvivesReplayWithoutTheOption(t *testing.T) {
	e := newKnightGame(t)

	// 回放时只带规则包的默认选项，**不带**骑士的 WithRoleSetup
	replayed, err := Replay(nil, DefaultRules(), e.EffectLog())
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}

	for id, want := range map[string]map[string]string{
		"kn": {VarCamp: string(CampGood), VarCategory: string(RoleCategoryGod), varKnightDuel: VarPresent},
		"wi": {VarCamp: string(CampGood), VarCategory: string(RoleCategoryGod),
			VarWitchAntidote: VarPresent, VarWitchPoison: VarPresent},
		"v1": {VarCamp: string(CampGood), VarCategory: string(RoleCategoryVillager)},
	} {
		got, ok := replayed.PlayerInfo(id)
		if !ok {
			t.Fatalf("回放后 %s 不在场上", id)
		}
		if !sameVars(got.Vars, want) {
			t.Errorf("回放后 %s 的初始状态不同: 期望 %v，实际 %v", id, want, got.Vars)
		}
	}
}

// TestRoleSetup_SurvivesSnapshot 初始状态随快照往返。
//
// 恢复走的是 restorePlayer 而不是入座，因此不会再发一次药——
// 这里同时确认「用掉的药不会因为存档读档回来」。
func TestRoleSetup_SurvivesSnapshot(t *testing.T) {
	e := newKnightGame(t)

	// 女巫先用掉解药，留下一个「初始状态已被改动」的局面
	endTo(t, e, PhaseNightWolf)
	mustSubmit(t, e, &SkillUse{PlayerID: "w1", Skill: SkillKill, Targets: []string{"v1"}})
	endTo(t, e, PhaseNightWitch)
	mustSubmit(t, e, &SkillUse{PlayerID: "wi", Skill: SkillAntidote, Targets: []string{"v1"}})
	mustEnd(t, e)

	before, _ := e.PlayerInfo("wi")
	if before.Vars[VarWitchAntidote] != "" {
		t.Fatalf("前置条件：解药应已用掉，实际 %v", before.Vars)
	}

	restored, err := Restore(nil, DefaultRules(), e.Snapshot())
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}

	after, _ := restored.PlayerInfo("wi")
	if !sameVars(after.Vars, before.Vars) {
		t.Fatalf("恢复后女巫的状态不同: 期望 %v，实际 %v", before.Vars, after.Vars)
	}
	kn, _ := restored.PlayerInfo("kn")
	if kn.Vars[varKnightDuel] != VarPresent {
		t.Errorf("恢复后骑士的决斗没带上: %v", kn.Vars)
	}
}

// TestRoleSetup_InitialStateIsNotHandedToThePlayer 初始状态不自动进玩家视图。
//
// 与 PlayerInfo.Vars 的分工一致：往里放什么由角色决定，默认交给玩家
// 等于让每个角色自己去想「这一项能不能给他看」。要给，经 RoleInfoProvider
// 显式投射——骑士这里就投了，而且投的是自己定的键名。
func TestRoleSetup_InitialStateIsNotHandedToThePlayer(t *testing.T) {
	e := newKnightGame(t)

	v := e.PlayerView("kn")
	if v == nil {
		t.Fatal("eng.PlayerView(kn) 不应为 nil")
	}
	if len(v.RoleInfo) != 0 {
		t.Fatalf("没有注册 engine.RoleInfoProvider 时不该凭空出现专属信息: %v", v.RoleInfo)
	}

	// 注册之后才出现，键名由角色自己定
	e2 := newKnightGame(t, engine.WithRoleInfo(roleKnight, engine.RoleInfoFunc(
		func(id string, view GameView) map[string]string {
			if view.Var(engine.ScopeGame.Of(id), varKnightDuel) == "" {
				return nil
			}
			return map[string]string{roleInfoKnightDuelLeft: VarPresent}
		})))

	v2 := e2.PlayerView("kn")
	if v2.RoleInfo[roleInfoKnightDuelLeft] != VarPresent {
		t.Fatalf("骑士应看到自己还有决斗，实际 %v", v2.RoleInfo)
	}
}

// TestRoleSetup_WitchSeesHerOwnPotions 女巫的药剂存量经 RoleInfo 投射。
//
// 存储（Vars）与投射（RoleInfo）分开：存储只有一种，谁都能写；
// 要给玩家看成什么样由角色自己决定。药剂存量此前是 SelfInfo 上两个具名
// bool 字段，等于内置女巫在面向玩家的视图上比第三方角色多一等公民的待遇。
func TestRoleSetup_WitchSeesHerOwnPotions(t *testing.T) {
	e := newKnightGame(t)

	v := e.PlayerView("wi")
	if v.RoleInfo[RoleInfoAntidote] != VarPresent || v.RoleInfo[RoleInfoPoison] != VarPresent {
		t.Fatalf("女巫应看到自己的两瓶药，实际 %v", v.RoleInfo)
	}

	// 别人看不到
	if other := e.PlayerView("se"); len(other.RoleInfo) != 0 {
		t.Errorf("预言家不该看到女巫的药: %v", other.RoleInfo)
	}

	// 用掉解药之后只剩毒药，刀口也随之看不到了
	endTo(t, e, PhaseNightWolf)
	mustSubmit(t, e, &SkillUse{PlayerID: "w1", Skill: SkillKill, Targets: []string{"v1"}})
	endTo(t, e, PhaseNightWitch)
	mustSubmit(t, e, &SkillUse{PlayerID: "wi", Skill: SkillAntidote, Targets: []string{"v1"}})
	mustEnd(t, e)

	v = e.PlayerView("wi")
	if v.RoleInfo[RoleInfoAntidote] != "" {
		t.Errorf("解药已用掉，不该还在: %v", v.RoleInfo)
	}
	if v.RoleInfo[RoleInfoPoison] != VarPresent {
		t.Errorf("毒药还在手里: %v", v.RoleInfo)
	}
}

// TestRoleSetup_NilRejected 与 WithResolver、WithRoleInfo 一致，拒绝 nil。
func TestRoleSetup_NilRejected(t *testing.T) {
	_, err := New(DefaultRules(), engine.WithRoleSetup(roleKnight, nil))
	if err == nil {
		t.Fatal("注册 nil 的初始状态应当报错")
	}
	if code := engine.CodeOf(err); code != engine.CodeInvalidConfig {
		t.Errorf("期望 engine.CodeInvalidConfig，实际 %v", code)
	}
}

// ==================== 局部辅助 ====================

func mustEnd(t *testing.T, e *Engine) []*Effect {
	t.Helper()
	effects, err := e.EndPhase()
	if err != nil {
		t.Fatalf("EndPhase(): %v", err)
	}
	return effects
}

// endTo 一路推进到目标阶段。防死循环：最多走一整圈。
func endTo(t *testing.T, e *Engine, want PhaseType) {
	t.Helper()
	for i := 0; e.Status().Phase != want; i++ {
		if i > 32 {
			t.Fatalf("推进到 %v 超过一圈仍未到达，当前 %v", want, e.Status().Phase)
		}
		mustEnd(t, e)
	}
}
