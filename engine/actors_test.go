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
	if e.Phase() != phaseNightWolf {
		t.Fatalf("阶段 = %v，期望 %v", e.Phase(), phaseNightWolf)
	}
	if got := e.AllowedSkills("w2"); len(got) != 0 {
		t.Fatalf("w2 不在点名名单里，AllowedSkills 却给出 %v", got)
	}

	// 绕一整圈回到狼人阶段。这一次没有人点名，应当退回按角色算——
	// 两只狼都能行动。沿用上一轮那份名单的话 w2 会被继续挡在外面。
	for i := 0; i < 20 && e.Phase() != phaseNightWolf; i++ {
		if _, err := e.EndPhase(); err != nil {
			t.Fatalf("EndPhase: %v", err)
		}
	}
	for i := 0; i < 20; i++ {
		if _, err := e.EndPhase(); err != nil {
			t.Fatalf("EndPhase: %v", err)
		}
		if e.Phase() == phaseNightWolf {
			break
		}
	}
	if e.Phase() != phaseNightWolf {
		t.Fatalf("没能绕回狼人阶段，停在 %v", e.Phase())
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
