package onenight

import (
	"testing"

	"github.com/Zereker/werewolf/engine"
)

// TestSeer_LooksAtTwoCenterCards 预言家看两张中央牌。
//
// 这一条走的是「下标编进技能名」那条绕法——中央牌不是玩家，内核的目标校验
// 只认玩家 ID。见 SCARS.md 疤 1。
func TestSeer_LooksAtTwoCenterCards(t *testing.T) {
	g := newGame(t,
		[CenterCount]engine.RoleType{RoleWerewolf, RoleTanner, RoleVillager},
		at("s", RoleSeer), at("v1", RoleVillager), at("v2", RoleVillager))

	g.advance(PhaseNightSeer)
	g.use("s", SkillSeerCenter02)
	g.advance(PhaseNightRobber)

	info := g.info("s")
	if got := info["learn.center.0"]; got != string(RoleWerewolf) {
		t.Errorf("中央第 0 张是狼人牌，实际读到 %q", got)
	}
	if got := info["learn.center.2"]; got != string(RoleVillager) {
		t.Errorf("中央第 2 张是村民牌，实际读到 %q", got)
	}
	if _, ok := info["learn.center.1"]; ok {
		t.Error("他只看了 0 和 2，不该知道第 1 张")
	}
}

// TestLoneWolf_MayPeekOnlyWhenAlone 只有场上仅一只狼时才能看中央牌。
//
// 「场上有几只狼」是规则的判断，内核拦不住——它只知道这个阶段允许 PEEK。
// 于是两只狼时的提交会被收下，但结算时被丢掉。
func TestLoneWolf_MayPeekOnlyWhenAlone(t *testing.T) {
	t.Run("独狼可以看", func(t *testing.T) {
		g := newGame(t,
			[CenterCount]engine.RoleType{RoleWerewolf, RoleVillager, RoleVillager},
			at("w", RoleWerewolf), at("v1", RoleVillager), at("v2", RoleVillager))

		g.use("w", SkillPeekCenter0)
		g.advance(PhaseNightMinion)

		if got := g.info("w")["learn.center.0"]; got != string(RoleWerewolf) {
			t.Errorf("独狼应当看到中央第 0 张，实际 %q", got)
		}
	})

	t.Run("两只狼看不了", func(t *testing.T) {
		g := newGame(t,
			[CenterCount]engine.RoleType{RoleVillager, RoleVillager, RoleVillager},
			at("w1", RoleWerewolf), at("w2", RoleWerewolf), at("v", RoleVillager))

		g.use("w1", SkillPeekCenter0)
		g.advance(PhaseNightMinion)

		if got := g.info("w1")["learn.center.0"]; got != "" {
			t.Errorf("场上有两只狼，看牌这个选项不存在，实际看到 %q", got)
		}
	})
}

// TestRobber_LearnsWhatHeTook 抢劫者知道自己抢到了什么，被抢的人不知道。
func TestRobber_LearnsWhatHeTook(t *testing.T) {
	g := newGame(t,
		[CenterCount]engine.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("r", RoleRobber), at("w", RoleWerewolf), at("v", RoleVillager))

	g.advance(PhaseNightRobber)
	g.use("r", SkillRob, "w")
	g.advance(PhaseNightTroublemake)

	if got := g.info("r")["learn.self"]; got != string(RoleWerewolf) {
		t.Errorf("抢劫者应当知道自己抢到了狼人牌，实际 %q", got)
	}
	if got := g.info("w")["learn.self"]; got != "" {
		t.Errorf("被抢的人不该知道自己现在拿的是什么，实际 %q", got)
	}
}

// TestNightActions_AreAllOptional 夜晚能力全是可选的：不提交也能推进。
//
// 规则里这些能力的措辞都是「你**可以**…」。内核据此不把它们标成 Required，
// 因此 PhaseReadiness 会把它们列进 Optional 而不是 Pending。
func TestNightActions_AreAllOptional(t *testing.T) {
	g := newGame(t,
		[CenterCount]engine.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("s", RoleSeer), at("r", RoleRobber), at("v", RoleVillager))

	g.advance(PhaseNightSeer)
	rd := g.e.PhaseReadiness()
	if !rd.Ready {
		t.Errorf("夜晚能力可选，阶段应当一开始就就绪，Pending=%v", rd.Pending)
	}
	if len(rd.Optional) == 0 {
		t.Error("预言家可以行动却还没动，应当出现在 Optional 里")
	}
}

// TestVote_IsRequiredOfEveryone 投票是全员必须的。
func TestVote_IsRequiredOfEveryone(t *testing.T) {
	g := newGame(t,
		[CenterCount]engine.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("w", RoleWerewolf), at("v1", RoleVillager), at("v2", RoleVillager))

	g.advance(PhaseVote)
	if rd := g.e.PhaseReadiness(); rd.Ready || len(rd.Pending) != 3 {
		t.Errorf("三个人都还没投，应当欠三票，实际 Ready=%v Pending=%v", rd.Ready, rd.Pending)
	}

	g.use("w", SkillVote, "v1")
	if rd := g.e.PhaseReadiness(); len(rd.Pending) != 2 {
		t.Errorf("投过一票，应当还欠两票，实际 %v", rd.Pending)
	}
}

// TestVote_CannotVoteForSelf 不能投自己。
//
// 内核不管这条——「能不能投自己」是规则的判断。提交会被收下，结算时丢掉。
func TestVote_CannotVoteForSelf(t *testing.T) {
	g := newGame(t,
		[CenterCount]engine.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("w", RoleWerewolf), at("v1", RoleVillager), at("v2", RoleVillager))

	g.advance(PhaseVote)
	g.use("w", SkillVote, "w")
	g.use("v1", SkillVote, "w")
	g.use("v2", SkillVote, "w")
	g.end(engine.PhaseEnd)

	if p, _ := g.e.PlayerInfo("w"); p.Alive {
		t.Error("两票投狼，他该出局")
	}
}

// TestCenterIndexes 从技能名读中央牌下标。
func TestCenterIndexes(t *testing.T) {
	cases := []struct {
		skill engine.SkillType
		want  []int
	}{
		{SkillPeekCenter0, []int{0}},
		{SkillDrinkCenter2, []int{2}},
		{SkillSeerCenter01, []int{0, 1}},
		{SkillSeerCenter12, []int{1, 2}},
		{SkillSeerPlayer, nil},  // 不带下标
		{SkillRob, nil},         // 同上
		{engine.SkillSkip, nil}, // 没有下划线
	}
	for _, c := range cases {
		got := centerIndexes(c.skill)
		if len(got) != len(c.want) {
			t.Errorf("centerIndexes(%v) = %v，期望 %v", c.skill, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("centerIndexes(%v) = %v，期望 %v", c.skill, got, c.want)
				break
			}
		}
	}
}

// TestEffectOrderIsDeterminedByTheBoard 同一个局面结算出的效果顺序必须稳定。
//
// 与前两套规则包同一条：效果流的回放与比对全靠它。投票的死亡名单是一张 map，
// 直接遍历产出效果的话，同一个局面每次结算的顺序都不一样。
func TestEffectOrderIsDeterminedByTheBoard(t *testing.T) {
	build := func() []*engine.Effect {
		g := newGame(t,
			[CenterCount]engine.RoleType{RoleVillager, RoleVillager, RoleVillager},
			at("a", RoleHunter), at("b", RoleWerewolf),
			at("c", RoleVillager), at("d", RoleVillager))
		g.advance(PhaseVote)
		g.use("c", SkillVote, "a")
		g.use("d", SkillVote, "a")
		g.use("b", SkillVote, "a")
		g.use("a", SkillVote, "b")
		out, err := g.e.EndPhase()
		if err != nil {
			t.Fatalf("EndPhase: %v", err)
		}
		return out
	}

	first := build()
	for i := 0; i < 20; i++ {
		again := build()
		if len(again) != len(first) {
			t.Fatalf("第 %d 次结算出 %d 条效果，第一次是 %d 条", i, len(again), len(first))
		}
		for j := range first {
			if again[j].Type != first[j].Type || again[j].TargetID != first[j].TargetID {
				t.Fatalf("第 %d 次的第 %d 条效果与第一次不同：%v/%v vs %v/%v",
					i, j, again[j].Type, again[j].TargetID, first[j].Type, first[j].TargetID)
			}
		}
	}
}
