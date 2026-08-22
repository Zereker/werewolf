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

// TestOnlyNamedActorsMayAct 只有被点名的人能行动，内核自己拦。
//
// **这条曾经是疤 1**，而且是六条里最贵的一条——代价直接落在给玩家看的东西上：
// 内核判定行动者只看 (阶段, 角色, 技能)，而角色是入座时定死的，任何运行时
// 选出来的集合都表达不了。阿瓦隆里它咬了两次（队长、任务队伍），
// 狼人杀里咬了一次（猎人开枪，内核为它开了触发队列这个单人特例）。
//
// 后果是内核对没资格的玩家说谎：AllowedSkills 说他能动、PhaseReadiness
// 等着他、SubmitSkillUse 收下他的提交再由解析器丢掉。
//
// 内核补上 NewSetActorsEffect 之后，三个问题（校验、AllowedSkills、
// PhaseReadiness）改从同一处取数。这个测试把三处一起盯住。
func TestOnlyNamedActorsMayAct(t *testing.T) {
	e := fivePlayer(t)

	// 一、提名阶段：只有队长
	leader := leaderID(e.View())
	for _, id := range allPlayerIDs(e) {
		allowed := e.AllowedSkills(id)
		if id == leader {
			if len(allowed) == 0 {
				t.Errorf("队长 %s 该能提名，AllowedSkills 是空的", id)
			}
			continue
		}
		if len(allowed) != 0 {
			t.Errorf("%s 不是队长，AllowedSkills 却给出 %v", id, allowed)
		}
		if err := e.SubmitSkillUse(&engine.SkillUse{
			PlayerID: id, Skill: SkillPropose, Targets: []string{"a"},
		}); err == nil {
			t.Errorf("%s 不是队长，内核却收下了他的提名", id)
		}
	}

	// 二、任务阶段：只有队伍成员
	proposeAndApprove(t, e, "a", "b")
	team := map[string]bool{"a": true, "b": true}
	for _, id := range allPlayerIDs(e) {
		allowed := e.AllowedSkills(id)
		if team[id] {
			if len(allowed) == 0 {
				t.Errorf("队员 %s 该能投票，AllowedSkills 是空的", id)
			}
			continue
		}
		if len(allowed) != 0 {
			t.Errorf("%s 没上任务，AllowedSkills 却给出 %v", id, allowed)
		}
		if err := e.SubmitSkillUse(&engine.SkillUse{
			PlayerID: id, Skill: SkillMissionFail,
		}); err == nil {
			t.Errorf("%s 没上任务，内核却收下了他的失败票", id)
		}
	}

	// 三、就绪判定也只等队伍成员
	for _, p := range e.PhaseReadiness().Pending {
		if !team[p.PlayerID] {
			t.Errorf("PhaseReadiness 在等 %s，可他没上任务", p.PlayerID)
		}
	}
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
	mustSubmit(t, e, &engine.SkillUse{PlayerID: leader, Skill: SkillPropose, Targets: members})
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

// TestReadinessKnowsTheWholeTeamIsProposed 就绪判定说得清「提名齐了没有」。
//
// **这条曾经是疤 5**：`SkillUse` 只能带一个目标，队长得提交 N 次。那个形状是
// 被样本量为一固定下来的——狼人杀九个技能恰好每个都只有一个目标。后果是就绪
// 判定只知道「队长提交过没有」：7 人局第一轮要 2 个人，队长只提名 1 个，
// 它就报 Ready=true。
//
// 那与「AllowedSkills 对没资格的人说他能行动」是同一类问题：内核对玩家说了
// 不实的话。既然疤 1 按这个标准修了，这条也该按同一个标准修。
//
// 现在一次提交带整支队伍，提名与就绪是同一件事。
func TestReadinessKnowsTheWholeTeamIsProposed(t *testing.T) {
	e := fivePlayer(t)
	need := MissionSize(5, 1)
	if need < 2 {
		t.Fatalf("这个测试需要至少 2 人的任务，实际 %d", need)
	}

	if e.PhaseReadiness().Ready {
		t.Fatal("还没提名就报就绪了")
	}
	leader := leaderID(e.View())
	mustSubmit(t, e, &engine.SkillUse{
		PlayerID: leader, Skill: SkillPropose, Targets: []string{"a", "b"},
	})
	if !e.PhaseReadiness().Ready {
		t.Error("整支队伍都提名了，还报没就绪")
	}

	// 提名的确实是整支队伍
	mustEnd(t, e)
	if got := len(teamIDs(e.View())); got != need {
		t.Errorf("队伍人数 = %d，期望 %d", got, need)
	}
}
