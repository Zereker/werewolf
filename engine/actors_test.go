package engine

import "testing"

// setActorsOnce 只在第一次结算时点名，之后不再点
type setActorsOnce struct {
	phase PhaseType
	ids   []string
	done  *bool
}

func (r setActorsOnce) Resolve([]*SkillUse, GameView) []*Effect {
	if *r.done {
		return nil
	}
	*r.done = true
	return []*Effect{NewSetActorsEffect(r.phase, r.ids...)}
}

// TestSetActors_IsConsumedAfterThePhaseResolves 名单用过就作废，不会沿用到下一次。
//
// 行动者名单几乎总是「这一轮算出来的」——阿瓦隆的任务队伍是本轮提名选的，
// 队长是本轮轮转到的。沿用上一轮的名单几乎总是错的，而且错得很隐蔽：
// 游戏照常推进，只是换了一批不该行动的人。
//
// 这个测试是变异验证逼出来的：拆掉 consumeActors 之后**整套测试一条都不红**，
// 因为两套规则包每次进那个阶段之前都会重新点名，陈旧名单永远被覆盖。
// 没有测试的规矩只是一句注释。
func TestSetActors_IsConsumedAfterThePhaseResolves(t *testing.T) {
	done := false
	opts := append(withNoopResolvers(),
		WithResolver(phaseNightGuard, setActorsOnce{
			phase: phaseNightWolf, ids: []string{"w1"}, done: &done,
		}))
	e := newTestEngine(t, opts...)
	mustAdd(t, e, "w1", roleWerewolf)
	mustAdd(t, e, "w2", roleWerewolf)
	mustAdd(t, e, "g", roleGuard)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// 第一次进狼人阶段：名单点了 w1，w2 不该能行动
	if _, err := e.EndPhase(); err != nil { // NIGHT_GUARD -> NIGHT_WOLF
		t.Fatalf("EndPhase: %v", err)
	}
	if e.Status().Phase != phaseNightWolf {
		t.Fatalf("阶段 = %v，期望 %v", e.Status().Phase, phaseNightWolf)
	}
	if got := e.AllowedSkills("w2"); len(got) != 0 {
		t.Fatalf("w2 不在点名名单里，AllowedSkills 却给出 %v", got)
	}

	// 绕一整圈回到狼人阶段。这一次没有人点名，应当退回按角色算——
	// 两只狼都能行动。沿用上一轮那份名单的话 w2 会被继续挡在外面。
	for i := 0; i < 20 && e.Status().Phase != phaseNightWolf; i++ {
		if _, err := e.EndPhase(); err != nil {
			t.Fatalf("EndPhase: %v", err)
		}
	}
	for i := 0; i < 20; i++ {
		if _, err := e.EndPhase(); err != nil {
			t.Fatalf("EndPhase: %v", err)
		}
		if e.Status().Phase == phaseNightWolf {
			break
		}
	}
	if e.Status().Phase != phaseNightWolf {
		t.Fatalf("没能绕回狼人阶段，停在 %v", e.Status().Phase)
	}
	if got := e.AllowedSkills("w2"); len(got) == 0 {
		t.Error("这一轮没有人点名，w2 该按角色算能行动——上一轮的名单被沿用了")
	}
}

// TestSetActors_EmptyListIsNotTheSameAsUnset 点名空名单 ≠ 没点名。
//
// 「规则说了，这个阶段没有人能行动」与「规则没说，按角色算」是两件事。
// 用 nil 表示前者会让它退化成后者，全场突然都能行动。
func TestSetActors_EmptyListIsNotTheSameAsUnset(t *testing.T) {
	done := false
	opts := append(withNoopResolvers(),
		WithResolver(phaseNightGuard, setActorsOnce{
			phase: phaseNightWolf, ids: nil, done: &done,
		}))
	e := newTestEngine(t, opts...)
	mustAdd(t, e, "w1", roleWerewolf)
	mustAdd(t, e, "g", roleGuard)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := e.EndPhase(); err != nil {
		t.Fatalf("EndPhase: %v", err)
	}
	if got := e.AllowedSkills("w1"); len(got) != 0 {
		t.Errorf("点名的是空名单，没有人能行动，w1 却拿到 %v", got)
	}
}

// detourTwice 第一次结算时把两名玩家一起排进同一个阶段。
type detourTwice struct {
	phase PhaseType
	a, b  string
	done  *bool
}

func (r detourTwice) Resolve([]*SkillUse, GameView) []*Effect {
	if *r.done {
		return nil
	}
	*r.done = true
	return []*Effect{
		NewDetourEffect(r.a, r.phase),
		NewDetourEffect(r.b, r.phase),
	}
}

// TestPendingTriggers_QueuedForTheSamePhaseEachGetTheirTurn
// 同一夜排进同一个阶段的两条触发，必须一人一次，不能只剩最后一个。
//
// 绕道队列现在不再自己回答「谁能行动」，它在**进入阶段时**按队首写一份
// 行动者名单（gameState.nameDetourActor）。写在进入阶段而不是写在
// ABILITY_TRIGGERED 的写入点，理由就是这个测试：两名猎人同一夜出局时队列
// 里有两条指向同一个阶段的触发，在写入点各写一次会互相覆盖，只剩后一个人
// 开得了枪，前一个人的那一枪凭空消失。
func TestPendingTriggers_QueuedForTheSamePhaseEachGetTheirTurn(t *testing.T) {
	done := false
	opts := append(withNoopResolvers(),
		WithResolver(phaseNightResolve, detourTwice{
			phase: phaseNightHunter, a: "h1", b: "h2", done: &done,
		}))
	e := newTestEngine(t, opts...)
	mustAdd(t, e, "h1", roleHunter)
	mustAdd(t, e, "h2", roleHunter)
	mustAdd(t, e, "v1", roleVillager)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// 推到结算阶段结束，两条触发一起入队
	for i := 0; i < 20 && e.Status().Phase != phaseNightResolve; i++ {
		if _, err := e.EndPhase(); err != nil {
			t.Fatalf("EndPhase: %v", err)
		}
	}
	if _, err := e.EndPhase(); err != nil { // NIGHT_RESOLVE -> NIGHT_HUNTER
		t.Fatalf("EndPhase: %v", err)
	}

	// 第一趟：队首是 h1，只有他能行动
	if e.Status().Phase != phaseNightHunter {
		t.Fatalf("第一条触发没把阶段引到猎人阶段，实际 %v", e.Status().Phase)
	}
	assertOnlyActor(t, e, "h1", "第一趟")

	if _, err := e.EndPhase(); err != nil { // 消费 h1，队列还剩 h2
		t.Fatalf("EndPhase: %v", err)
	}

	// 第二趟：必须再回到同一个阶段，且这次轮到 h2
	if e.Status().Phase != phaseNightHunter {
		t.Fatalf("第二条触发没把阶段再引回猎人阶段，实际 %v——"+
			"两条触发指向同一个阶段时，后一条覆盖了前一条", e.Status().Phase)
	}
	assertOnlyActor(t, e, "h2", "第二趟")

	if _, err := e.EndPhase(); err != nil { // 消费 h2，队列排空
		t.Fatalf("EndPhase: %v", err)
	}
	if e.Status().Phase == phaseNightHunter {
		t.Error("队列已排空，不该再回到猎人阶段")
	}
}

// assertOnlyActor 这个阶段有且只有 want 一个人能行动，三条路答案一致。
//
// 三条路是 AllowedSkills、PhaseInfo 与 SubmitSkillUse 的校验。它们此前
// 各有一份三层判断，现在共用 actorsForStep 这一个取数点——但共用与否要靠
// 测试说话，光看代码看不出来。
func assertOnlyActor(t *testing.T, e *Engine, want, when string) {
	t.Helper()

	if got := e.AllowedSkills(want); len(got) == 0 {
		t.Errorf("%s：%s 应当能行动，AllowedSkills 却是空的", when, want)
	}
	ids := e.PhaseInfo().RoleInfos[roleHunter].PlayerIDs
	if len(ids) != 1 || ids[0] != want {
		t.Errorf("%s：本阶段应由 %s 行动，PhaseInfo 给出 %v", when, want, ids)
	}
	for _, other := range []string{"h1", "h2", "v1"} {
		if other == want {
			continue
		}
		if got := e.AllowedSkills(other); len(got) != 0 {
			t.Errorf("%s：%s 不该能行动，AllowedSkills 却给出 %v", when, other, got)
		}
		err := e.SubmitSkillUse(&SkillUse{PlayerID: other, Skill: skillShoot, Targets: []string{"v1"}})
		if err == nil {
			t.Errorf("%s：%s 不该能行动，提交却被收下了", when, other)
		}
	}
}

// TestNamePendingTriggerActor_OnlyNamesItsOwnPhase 触发只在它自己那个阶段点名。
//
// 正常推进时这条越不过去：队列非空时 calculateNextPhase 永远把下一站定成
// 队首那个阶段，所以走不到「带着待结算触发进了别的阶段」。但这一条正是
// nameDetourActor 成立的前提，前提写在代码里就该有测试钉住——
// 否则日后有人改了流转顺序（比如让 GOTO_PHASE 越过队列），触发者会在
// 一个毫不相干的阶段被点名，而所有集成测试照样是绿的。
//
// 因此这里绕开流转，直接对状态调用，验的是这个函数自己的契约。
func TestNamePendingTriggerActor_OnlyNamesItsOwnPhase(t *testing.T) {
	s := newState()
	mustAddTo(t, s, "h1", roleHunter)
	s.startAt(phaseNight)
	s.applyEffect(NewDetourEffect("h1", phaseNightHunter))

	// 进一个与触发无关的阶段：不该点任何人
	s.nextPhase(phaseDay, false, false)
	if ids, ok := s.actorsFor(phaseDay); ok {
		t.Errorf("触发指向 %v，进 %v 时却点了名：%v", phaseNightHunter, phaseDay, ids)
	}

	// 进触发要去的那个阶段：点触发者
	s.nextPhase(phaseNightHunter, false, false)
	ids, ok := s.actorsFor(phaseNightHunter)
	if !ok || len(ids) != 1 || ids[0] != "h1" {
		t.Errorf("进 %v 时应当点名 h1，实际 %v（存在=%v）", phaseNightHunter, ids, ok)
	}
}
