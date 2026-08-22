package werewolf

import (
	"testing"
)

// TestPhaseReadiness_WolfConsensus 狼人商刀要求全部存活狼人都提交。
func TestPhaseReadiness_WolfConsensus(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), seer("s"), witch("wi"),
		villagers("v1", "v2", "v3"),
	)...)
	g.end(PhaseNightWolf)

	r := g.e.PhaseReadiness()
	if r.Ready {
		t.Error("无人提交时不应就绪")
	}
	if len(r.Pending) != 2 {
		t.Fatalf("应当还差两名狼人，实际 %d: %+v", len(r.Pending), r.Pending)
	}
	for _, p := range r.Pending {
		if p.Skill != SkillKill {
			t.Errorf("待办技能应为 KILL，实际 %v", p.Skill)
		}
	}

	g.mustUse("w1", SkillKill, "v1")
	r = g.e.PhaseReadiness()
	if r.Ready {
		t.Error("Multiple=true，只有一狼提交时仍不应就绪")
	}
	if len(r.Pending) != 1 || r.Pending[0].PlayerID != "w2" {
		t.Errorf("应当还差 w2，实际 %+v", r.Pending)
	}
	if len(r.Acted) != 1 || r.Acted[0] != "w1" {
		t.Errorf("已行动者应为 [w1]，实际 %v", r.Acted)
	}

	g.mustUse("w2", SkillKill, "v1")
	if r = g.e.PhaseReadiness(); !r.Ready {
		t.Errorf("两狼都提交后应当就绪，仍差 %+v", r.Pending)
	}
}

// TestPhaseReadiness_NoEligibleActor 无人能承担的必需步骤视为自动满足，
// 否则阶段会永远卡住。
func TestPhaseReadiness_NoEligibleActor(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), seer("s"), villagers("v1", "v2"),
	)...)
	g.end(PhaseNightWolf)

	// 唯一的狼出局后，狼人步骤没有合格行动者
	g.setDead("w1")
	if r := g.e.PhaseReadiness(); !r.Ready {
		t.Errorf("没有合格行动者时应视为就绪，实际仍差 %+v", r.Pending)
	}
}

// TestPhaseReadiness_OptionalStepsNeverBlock 非必需步骤不影响就绪。
func TestPhaseReadiness_OptionalStepsNeverBlock(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), guard("g"), seer("s"), witch("wi"),
		villagers("v1", "v2"),
	)...)

	// NIGHT_GUARD：守卫可以选择不守护
	if r := g.e.PhaseReadiness(); !r.Ready {
		t.Errorf("守卫阶段非必需，应当就绪，实际 %+v", r.Pending)
	}

	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "v1")
	g.end(PhaseNightWitch)
	if r := g.e.PhaseReadiness(); !r.Ready {
		t.Errorf("女巫阶段非必需，应当就绪，实际 %+v", r.Pending)
	}
	g.end(PhaseNightSeer)
	if r := g.e.PhaseReadiness(); !r.Ready {
		t.Errorf("预言家阶段非必需，应当就绪，实际 %+v", r.Pending)
	}
}

// TestPhaseReadiness_Vote 投票要求全体存活玩家参与。
func TestPhaseReadiness_Vote(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), seer("s"), villagers("v1", "v2"),
	)...)
	g.walkNight()
	g.end(PhaseVote)

	r := g.e.PhaseReadiness()
	if len(r.Pending) != 4 {
		t.Fatalf("应当还差 4 人投票，实际 %d", len(r.Pending))
	}

	g.vote("v1", "w1", "s", "v1")
	r = g.e.PhaseReadiness()
	if r.Ready {
		t.Error("还有人没投时不应就绪")
	}
	if len(r.Pending) != 1 || r.Pending[0].PlayerID != "v2" {
		t.Errorf("应当还差 v2，实际 %+v", r.Pending)
	}

	g.vote("v1", "v2")
	if r = g.e.PhaseReadiness(); !r.Ready {
		t.Errorf("全员投票后应当就绪，仍差 %+v", r.Pending)
	}
}

// TestPhaseReadiness_MultipleFalse Multiple=false 时任意一人完成即可。
func TestPhaseReadiness_MultipleFalse(t *testing.T) {
	cfg := DefaultGameConfig()
	// 造一个双守卫且必须行动的板子，但只要求其中一人
	guardPhase := cfg.Phases[PhaseNightGuard]
	guardPhase.Steps = []PhaseStep{
		{Role: RoleGod, Skill: SkillAnnounce},
		{Role: RoleGuard, Skill: SkillProtect,
			Required: true, Multiple: false},
	}

	g := newRuleGame(t, cfg, seats(
		wolf("w1"), guard("g1"), guard("g2"), villagers("v1", "v2"),
	)...)

	if r := g.e.PhaseReadiness(); r.Ready {
		t.Error("两名守卫都未行动时不应就绪")
	}
	g.mustUse("g1", SkillProtect, "v1")
	if r := g.e.PhaseReadiness(); !r.Ready {
		t.Errorf("Multiple=false，一人完成即应就绪，仍差 %+v", r.Pending)
	}
}

// TestPhaseReadiness_TriggerPhase 死亡技能阶段只等触发者一人。
func TestPhaseReadiness_TriggerPhase(t *testing.T) {
	cfg := DefaultGameConfig()
	hunterPhase := cfg.Phases[PhaseNightHunter]
	hunterPhase.Steps = []PhaseStep{
		{Role: RoleGod, Skill: SkillAnnounce},
		{Role: RoleHunter, Skill: SkillShoot,
			Required: true},
		{Role: RoleHunter, Skill: SkillSkip},
	}

	g := newRuleGame(t, cfg, seats(
		wolf("w1"), wolf("w2"), hunter("h"), seer("s"),
		villagers("v1", "v2", "v3"),
	)...)
	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "h")
	g.mustUse("w2", SkillKill, "h")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseNightHunter)

	r := g.e.PhaseReadiness()
	if len(r.Pending) != 1 || r.Pending[0].PlayerID != "h" {
		t.Fatalf("应当只等被触发的猎人，实际 %+v", r.Pending)
	}
	g.mustUse("h", SkillShoot, "w1")
	if r = g.e.PhaseReadiness(); !r.Ready {
		t.Errorf("猎人开枪后应当就绪，仍差 %+v", r.Pending)
	}
}

// TestPhaseReadiness_DoesNotBlockEndPhase 就绪与否不影响 EndPhase——
// 引擎不替调用方决定阶段何时结束。
func TestPhaseReadiness_DoesNotBlockEndPhase(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), seer("s"), villagers("v1", "v2", "v3"),
	)...)
	g.end(PhaseNightWolf)

	if g.e.PhaseReadiness().Ready {
		t.Fatal("前置：此刻不应就绪")
	}
	if _, err := g.e.EndPhase(); err != nil {
		t.Errorf("未就绪时 EndPhase 仍应放行（超时推进由调用方决定），实际 %v", err)
	}
}

// TestPhaseReadiness_MutuallyExclusiveGroup 互斥备选组里提交任意一项即算完成。
//
// 猎人的「开枪」与「不开枪」是二选一。逐步骤独立判定会认为明确表示
// 不开枪的猎人仍欠着开枪，一旦按 Required 的文档字面把两步都标上，
// 这个阶段就永远不会就绪。
func TestPhaseReadiness_MutuallyExclusiveGroup(t *testing.T) {
	cfg := DefaultGameConfig()
	for _, phase := range []PhaseType{
		PhaseNightHunter,
		PhaseDayHunter,
	} {
		for i := range cfg.Phases[phase].Steps {
			cfg.Phases[phase].Steps[i].Required = true
		}
	}

	g := newRuleGame(t, cfg, seats(
		wolf("w1"), wolf("w2"), hunter("h"), seer("s"),
		villagers("v1", "v2", "v3", "v4"),
	)...)

	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "h")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseNightHunter)

	// 还没表态：欠一次行动，且只报一条（不是开枪、不开枪各一条）
	before := g.e.PhaseReadiness()
	if before.Ready {
		t.Fatal("猎人尚未表态，不应就绪")
	}
	if len(before.Pending) != 1 {
		t.Errorf("同一组只该报一条待办，实际 %v", before.Pending)
	}

	// 明确表示不开枪之后就该就绪
	g.mustUse("h", SkillSkip, "")
	after := g.e.PhaseReadiness()
	if !after.Ready {
		t.Errorf("猎人已表示不开枪，应当就绪，实际还差 %v", after.Pending)
	}
}

// TestPhaseReadiness_TriggerActorMatchesRole 死亡技能阶段的触发者只承担自己角色的步骤。
func TestPhaseReadiness_TriggerActorMatchesRole(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), hunter("h"), seer("s"),
		villagers("v1", "v2", "v3", "v4"),
	)...)

	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "h")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseNightHunter)

	info := g.e.PhaseInfo()
	if ri := info.RoleInfos[RoleHunter]; ri == nil ||
		len(ri.PlayerIDs) != 1 || ri.PlayerIDs[0] != "h" {
		t.Fatalf("猎人步骤的行动者应当是 h，实际 %+v", info.RoleInfos)
	}
	// 触发者不是预言家，预言家的步骤（本阶段没有）不该落到他头上
	if ri := info.RoleInfos[RoleSeer]; ri != nil && len(ri.PlayerIDs) > 0 {
		t.Errorf("本阶段不该有预言家的行动者，实际 %v", ri.PlayerIDs)
	}
}

// TestPhaseReadiness_OptionalActions 可选技能不影响就绪，但要报出来。
//
// 默认配置里只有狼刀与投票是 Required，守卫、女巫、预言家、猎人全都
// 可以不动。只看 Pending 来驱动游戏的话，这几个角色一整局都不会被叫到
// ——example/cli 最初就踩了这个坑。「还差谁必须动」和「本阶段谁可以动」
// 是两个问题，得分别有答案。
func TestPhaseReadiness_OptionalActions(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), guard("g"), witch("wi"), seer("s"),
		villagers("v1", "v2", "v3", "v4"),
	)...)

	// 守卫阶段：守护是可选的，所以「就绪」但并非「没人可动」
	r := g.e.PhaseReadiness()
	if !r.Ready {
		t.Errorf("守护是可选技能，不该让阶段不就绪，实际还差 %v", r.Pending)
	}
	if len(r.Pending) != 0 {
		t.Errorf("Pending 只装必需行动，实际 %v", r.Pending)
	}
	if len(r.Optional) != 1 || r.Optional[0].PlayerID != "g" ||
		r.Optional[0].Skill != SkillProtect {
		t.Fatalf("Optional 应当报出守卫还没守，实际 %v", r.Optional)
	}

	// 守卫动过之后，Optional 也就空了
	g.mustUse("g", SkillProtect, "s")
	if got := g.e.PhaseReadiness().Optional; len(got) != 0 {
		t.Errorf("守卫已行动，Optional 应当为空，实际 %v", got)
	}

	// 狼人阶段：刀是必需的，两只狼都得投
	g.end(PhaseNightWolf)
	r = g.e.PhaseReadiness()
	if r.Ready {
		t.Error("狼刀是必需行动，未提交时不该就绪")
	}
	if len(r.Pending) != 2 {
		t.Errorf("两只狼都该出现在 Pending 里，实际 %v", r.Pending)
	}
	if len(r.Optional) != 0 {
		t.Errorf("本阶段没有可选技能，实际 %v", r.Optional)
	}
}

// TestPhaseReadiness_OptionalGroupCountsOnce 互斥备选组在 Optional 里也只报一条。
func TestPhaseReadiness_OptionalGroupCountsOnce(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), hunter("h"), seer("s"),
		villagers("v1", "v2", "v3", "v4"),
	)...)

	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "h")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseNightHunter)

	r := g.e.PhaseReadiness()
	if !r.Ready {
		t.Errorf("开枪是可选的，不该让阶段不就绪，实际 %v", r.Pending)
	}
	if len(r.Optional) != 1 || r.Optional[0].PlayerID != "h" {
		t.Fatalf("开枪/不开枪是一组，只该报一条，实际 %v", r.Optional)
	}

	// 明确表示不开枪之后，这一组就完成了
	g.mustUse("h", SkillSkip, "")
	if got := g.e.PhaseReadiness().Optional; len(got) != 0 {
		t.Errorf("猎人已表态，Optional 应当为空，实际 %v", got)
	}
}
