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

// TestScar2_RejectedProposalSpinsThroughAnEmptyMissionPhase 表决没通过，仍然要空转一次任务阶段。
//
// 阶段流转是静态的 NextPhase，没有「表决通过就去任务、否则回提名」这种
// 条件分支。绕法是让任务阶段在没通过时什么都不做。
func TestScar2_RejectedProposalSpinsThroughAnEmptyMissionPhase(t *testing.T) {
	e := fivePlayer(t)
	propose(t, e, "a", "b")
	// 全员否决
	for _, id := range allPlayerIDs(e) {
		mustSubmit(t, e, &engine.SkillUse{PlayerID: id, Skill: SkillReject})
	}
	mustEnd(t, e)

	if got := e.Phase(); got != PhaseMission {
		t.Fatalf("阶段 = %v，期望仍然被推到 MISSION（这正是疤）", got)
	}
	t.Log("疤 2：队伍被否决了，引擎还是把大家带进了任务阶段")

	effects := mustEnd(t, e)
	if len(effects) != 0 {
		t.Logf("空转的任务阶段产出了 %v", typesOf(effects))
	}
	if got := e.Phase(); got != PhasePropose {
		t.Fatalf("空转之后该回到 PROPOSE，实际 %v", got)
	}
}

// TestScar3_RoundIsAProposalCounterNotAMission 引擎的「回合」对阿瓦隆玩家没有意义。
//
// 内核把「回合数加一」与「阶段环绕回起始阶段」焊成同一件事。阿瓦隆每提名
// 一次就绕一圈，于是 Engine.Round() 数的是提名次数——而阿瓦隆自己说的
// 「第几轮」指的是第几个任务，两者可以差五倍。PlayerView.Round 直接把这个
// 没意义的数发给玩家。
func TestScar3_RoundIsAProposalCounterNotAMission(t *testing.T) {
	e := fivePlayer(t)

	// 连续否决两次，一个任务都没打
	for i := 0; i < 2; i++ {
		propose(t, e, "a", "b")
		for _, id := range allPlayerIDs(e) {
			mustSubmit(t, e, &engine.SkillUse{PlayerID: id, Skill: SkillReject})
		}
		mustEnd(t, e) // -> MISSION
		mustEnd(t, e) // -> PROPOSE，回合数 +1
	}

	engineRound := e.Round()
	avalonMission := missionOf(e)
	t.Logf("疤 3：引擎说第 %d 回合，阿瓦隆说还在第 %d 轮任务", engineRound, avalonMission)
	if engineRound == avalonMission {
		t.Skip("两者恰好相等——换个用例再看")
	}
	if v := e.PlayerView("a"); v.Round != engineRound {
		t.Fatalf("PlayerView.Round = %d，引擎 = %d", v.Round, engineRound)
	}
	t.Logf("疤 3：而 PlayerView.Round 把 %d 这个数原样发给了玩家", engineRound)
}

// TestScar4_GameProgressLivesOnSomeonesPrivateState 整局进度记在某个玩家身上。
//
// 内核的三种变量作用域里没有「整局有效 + 无主」这一格，阿瓦隆的五个计数器
// 只能挂到 ID 字典序最小的那名玩家的 PlayerVar 上。后果是那个玩家的私有
// 状态里凭空多出五个与他无关的字段。
func TestScar4_GameProgressLivesOnSomeonesPrivateState(t *testing.T) {
	e := fivePlayer(t)
	proposeAndApprove(t, e, "a", "b")
	for _, id := range []string{"a", "b"} {
		mustSubmit(t, e, &engine.SkillUse{PlayerID: id, Skill: SkillMissionSuccess})
	}
	mustEnd(t, e)

	holder, _ := e.PlayerInfo("a") // AllPlayers 排序后的第一位
	var leaked []string
	for k := range holder.Vars {
		if k != engine.VarCamp {
			leaked = append(leaked, k)
		}
	}
	if len(leaked) == 0 {
		t.Skip("内核已经有整局作用域了——这道疤该改成正面断言")
	}
	t.Logf("疤 4：玩家 a 的私有状态里多出 %d 个与他无关的字段：%v", len(leaked), leaked)
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
