package avalon

import (
	"testing"

	"github.com/Zereker/werewolf/engine"
)

// scars_test.go 把绕法的代价钉成可跑的证据。
//
// 这些测试断言的是**当前实现的不对之处**，不是期望的行为。它们存在的
// 意义是：SCARS.md 里的每一条都能被验证，而不是我说了算。内核补上对应
// 能力之后，它们会变红——那时候就该把它们改写成正面断言。

// TestScar1_AllowedSkillsLiesToNonTeamMembers 没上任务的人，被告知他可以投票。
//
// 内核判定行动者只看 (阶段, 角色, 技能)，而任务队伍是运行时定的，
// 因此任务阶段只能对所有角色开放。后果是 AllowedSkills 与 PlayerView
// 会对没被选上的玩家说「你可以投成功/失败」——而这两个正是这个库
// 拿来给玩家看的东西。
func TestScar1_AllowedSkillsLiesToNonTeamMembers(t *testing.T) {
	e := fivePlayer(t)
	proposeAndApprove(t, e, "a", "b") // 队伍是 a、b

	notOnTeam := "c"
	if onTeamNow(e, notOnTeam) {
		t.Fatalf("%s 不该在队伍里，测试前提坏了", notOnTeam)
	}

	allowed := e.AllowedSkills(notOnTeam)
	if len(allowed) == 0 {
		t.Skip("内核已经能按运行时名单划行动者了——这道疤该改成正面断言")
	}
	t.Logf("疤 1：%s 没上任务，AllowedSkills 却给出 %v", notOnTeam, allowed)

	// 而且他真的提交得进去（只是解析器会丢掉）
	if err := e.SubmitSkillUse(&engine.SkillUse{
		PlayerID: notOnTeam, Skill: SkillMissionFail,
	}); err != nil {
		t.Skipf("内核已经拦住了非队员的提交：%v", err)
	}
	t.Logf("疤 1：%s 的失败票被内核收下了，只能靠解析器丢弃", notOnTeam)

	// 就绪判定同样把他算进「还差谁」
	r := e.PhaseReadiness()
	t.Logf("疤 1：PhaseReadiness 认为还差 %v（队伍其实只有 a、b）", r.Pending)
}

// TestRejectedProposalGoesStraightBackToPropose 提名被否决，直接回提名，不空转任务阶段。
//
// **这条曾经是疤 2**：阶段流转是一张静态图，没有「表决通过就去任务、否则
// 回提名」这种条件分支，于是被否决的提名也要空转一次任务阶段。
//
// 内核把「下一步去哪」交给规则之后（NewGotoPhaseEffect），表决解析器自己
// 说去哪，这条疤关掉了。这个测试从「断言缺陷」翻成了「断言修好」。
func TestRejectedProposalGoesStraightBackToPropose(t *testing.T) {
	e := fivePlayer(t)
	propose(t, e, "a", "b")
	for _, id := range allPlayerIDs(e) {
		mustSubmit(t, e, &engine.SkillUse{PlayerID: id, Skill: SkillReject})
	}
	mustEnd(t, e)

	if got := e.Phase(); got != PhasePropose {
		t.Fatalf("阶段 = %v，期望直接回到 PROPOSE（不再空转任务阶段）", got)
	}
}

// TestApprovedProposalGoesToMission 表决通过，进任务阶段。
//
// 与上面一条成对：同一个解析器算出两个不同的出口，这正是静态图表达不了的。
func TestApprovedProposalGoesToMission(t *testing.T) {
	e := fivePlayer(t)
	propose(t, e, "a", "b")
	for _, id := range allPlayerIDs(e) {
		mustSubmit(t, e, &engine.SkillUse{PlayerID: id, Skill: SkillApprove})
	}
	mustEnd(t, e)

	if got := e.Phase(); got != PhaseMission {
		t.Fatalf("阶段 = %v，期望 MISSION", got)
	}
}

// TestRoundEqualsMissionNumber 引擎的「回合」等于阿瓦隆的「第几轮任务」。
//
// **这条曾经是疤 3**：内核把「回合数加一」焊死在「阶段环绕回起始阶段」上，
// 而阿瓦隆每提名一次就绕一圈，于是 Round 成了提名计数器，与「第几轮任务」
// 最多差五倍，还被 PlayerView.Round 原样发给玩家。
//
// 两处改动合起来才关掉它，缺一不可：
//   - PhaseConfig.EndsRound 让板子自己声明回合边界（阿瓦隆声明在任务阶段）；
//   - NewGotoPhaseEffect 让被否决的提名直接跳回提名阶段，不再空转任务阶段
//     ——只改前者的话，空转那一次照样推进回合，这条疤只关掉一半。
//
// 两条疤耦合这件事本身是个发现：它们同根，都是内核替规则做了只有规则知道
// 答案的决定。
func TestRoundEqualsMissionNumber(t *testing.T) {
	e := fivePlayer(t)

	// 先连续否决两次：一个任务都没打，回合数就不该动
	for i := 0; i < 2; i++ {
		propose(t, e, "a", "b")
		for _, id := range allPlayerIDs(e) {
			mustSubmit(t, e, &engine.SkillUse{PlayerID: id, Skill: SkillReject})
		}
		mustEnd(t, e)
	}
	if got, want := e.Round(), 1; got != want {
		t.Errorf("否决两次之后 Round = %d，期望仍是 %d——一个任务都没打", got, want)
	}

	// 打完两轮任务，回合数跟着任务走
	runMission(t, e, 0, "a", "b")
	runMission(t, e, 0, "a", "b", "c")

	if got, want := e.Round(), missionOf(e); got != want {
		t.Errorf("Round = %d，阿瓦隆的第几轮 = %d，两者该相等", got, want)
	}
	if v := e.PlayerView("a"); v.Round != e.Round() {
		t.Errorf("PlayerView.Round = %d，引擎 = %d", v.Round, e.Round())
	}
}

// TestGameProgressLivesInGameVars 整局进度住在整局作用域里，不挂在任何玩家身上。
//
// **这条曾经是疤 4**：内核的变量作用域是一张 2x2 的表，缺「整局有效 + 无主」
// 这一格。阿瓦隆的五个计数器（第几轮、成功几次、失败几次、连续否决几次、
// 队长是谁）只能挂到 ID 字典序最小那名玩家的 PlayerVar 上当账本——全局事实
// 记在某个人名下，那个玩家的视图里凭空多出五个与他无关的字段。
//
// 内核补上第四格（GameVar）之后账本整个删掉了。这个测试盯住两件事：
// 进度确实在整局作用域里，且**没有任何玩家身上沾着它**。
func TestGameProgressLivesInGameVars(t *testing.T) {
	e := fivePlayer(t)
	proposeAndApprove(t, e, "a", "b")
	for _, id := range []string{"a", "b"} {
		mustSubmit(t, e, &engine.SkillUse{PlayerID: id, Skill: SkillMissionSuccess})
	}
	mustEnd(t, e)

	// 一、进度读得到，而且在整局作用域里
	if got := successes(e.View()); got != 1 {
		t.Fatalf("成功次数 = %d，期望 1", got)
	}
	if e.GameVar(varSuccess) == "" {
		t.Errorf("成功次数该住在 GameVar 里，%q 是空的", varSuccess)
	}

	// 二、没有任何玩家身上沾着阿瓦隆的整局计数
	for _, id := range e.AlivePlayerIDs() {
		p, _ := e.PlayerInfo(id)
		for k := range p.Vars {
			if k != engine.VarCamp {
				t.Errorf("玩家 %s 身上不该有 %q——那是整局状态，不属于任何人", id, k)
			}
		}
	}
}

// ==================== 测试辅助 ====================

func allPlayerIDs(e *engine.Engine) []string { return e.AlivePlayerIDs() }

func onTeamNow(e *engine.Engine, id string) bool {
	v := e.View()
	return onTeam(v, id)
}

func missionOf(e *engine.Engine) int { return mission(e.View()) }

func mustSubmit(t *testing.T, e *engine.Engine, use *engine.SkillUse) {
	t.Helper()
	if err := e.SubmitSkillUse(use); err != nil {
		t.Fatalf("提交 %s/%s: %v", use.PlayerID, use.Skill, err)
	}
}

func mustEnd(t *testing.T, e *engine.Engine) []*engine.Effect {
	t.Helper()
	effects, err := e.EndPhase()
	if err != nil {
		t.Fatalf("EndPhase(%v): %v", e.Phase(), err)
	}
	return effects
}

func propose(t *testing.T, e *engine.Engine, members ...string) {
	t.Helper()
	leader := leaderID(e.View())
	for _, m := range members {
		mustSubmit(t, e, &engine.SkillUse{PlayerID: leader, Skill: SkillPropose, TargetID: m})
	}
	mustEnd(t, e)
}

func proposeAndApprove(t *testing.T, e *engine.Engine, members ...string) {
	t.Helper()
	propose(t, e, members...)
	for _, id := range allPlayerIDs(e) {
		mustSubmit(t, e, &engine.SkillUse{PlayerID: id, Skill: SkillApprove})
	}
	mustEnd(t, e)
}
