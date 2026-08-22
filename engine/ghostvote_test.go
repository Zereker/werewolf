package engine

import "testing"

// ghostVote 让一名已出局的玩家在投票阶段行动——血染钟楼的「幽灵票」。
type ghostVote struct {
	dead  string
	alive string
	done  *bool
}

func (r ghostVote) Resolve(_ []*SkillUse, _ GameView) []*Effect {
	if *r.done {
		return nil
	}
	*r.done = true
	return []*Effect{
		NewSetAliveEffect(r.dead, false),
		NewSetActorsEffect(phaseVote, r.dead, r.alive),
	}
}

// TestNamedActorsMayBeDead 规则点名谁，谁就能行动——包括已出局的人。
//
// **存活是默认的行动资格，不是法律。** 此前只有内核自己的触发队列能越过它
// （猎人被刀之后开枪），规则的点名不能——同一个内核允许自己的机制让死人行动、
// 不允许规则的机制这么做，是内核在替规则判断「死了还能不能动」。
//
// 挡掉的是真实存在的玩法：血染钟楼的死人保留一张「幽灵票」，狼人杀有遗言阶段。
// 这个测试就是那张幽灵票。
func TestNamedActorsMayBeDead(t *testing.T) {
	done := false
	opts := append(withNoopResolvers(),
		WithResolver(phaseNightGuard, ghostVote{dead: "w1", alive: "g", done: &done}))
	e := newTestEngine(t, opts...)
	mustAdd(t, e, "w1", roleWerewolf)
	mustAdd(t, e, "w2", roleWerewolf)
	mustAdd(t, e, "g", roleGuard)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := e.EndPhase(); err != nil { // 结算：w1 出局，并点名投票阶段
		t.Fatalf("EndPhase: %v", err)
	}
	if p, _ := e.PlayerInfo("w1"); p.Alive {
		t.Fatal("前提坏了：w1 该已出局")
	}

	// 走到投票阶段
	for i := 0; i < 20 && e.Phase() != phaseVote; i++ {
		if _, err := e.EndPhase(); err != nil {
			t.Fatalf("EndPhase: %v", err)
		}
	}
	if e.Phase() != phaseVote {
		t.Fatalf("没走到投票阶段，停在 %v", e.Phase())
	}

	// 出局的 w1 被点名了，他能投票
	if got := e.AllowedSkills("w1"); len(got) == 0 {
		t.Error("w1 已出局但被规则点名，该能投票——AllowedSkills 是空的")
	}

	// 就绪判定也把他算进「还差谁」——**在他投票之前**查。
	//
	// 这一句的位置是变异验证逼出来的：原本写在投票之后，那时 w1 已经行动，
	// 「按存活过滤掉 w1」与「正确地等 w1」两种实现看起来一模一样，
	// 变异从测试底下溜过去了。
	pendingBefore := map[string]bool{}
	for _, p := range e.PhaseReadiness().Pending {
		pendingBefore[p.PlayerID] = true
	}
	if !pendingBefore["w1"] {
		t.Errorf("就绪判定该在等已出局但被点名的 w1，实际在等 %v", pendingBefore)
	}

	if err := e.SubmitSkillUse(&SkillUse{
		PlayerID: "w1", Skill: skillVote, Targets: []string{"g"},
	}); err != nil {
		t.Errorf("w1 的幽灵票被拒了：%v", err)
	}

	// 活着但没被点名的 w2 不能投票——点名是白名单，不是「额外加人」
	if got := e.AllowedSkills("w2"); len(got) != 0 {
		t.Errorf("w2 没被点名，不该能投票，实际 %v", got)
	}

	// 就绪判定也认这份名单
	for _, p := range e.PhaseReadiness().Pending {
		if p.PlayerID != "g" {
			t.Errorf("就绪判定在等 %s，可名单里只有 w1 与 g（w1 已投）", p.PlayerID)
		}
	}
}
