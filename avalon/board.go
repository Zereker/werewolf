package avalon

import (
	"time"

	"github.com/Zereker/werewolf/engine"
)

// 各阶段的建议超时。板子数据，引擎不据此计时。
const (
	ProposeTimeout  = 60 * time.Second
	TeamVoteTimeout = 30 * time.Second
	MissionTimeout  = 30 * time.Second
	AssassinTimeout = 90 * time.Second // 刺杀要复盘整局，给足时间
)

// missionSizes 每种人数下五轮任务各需要几人。
//
// 取自英文维基条目的表格。列是人数 5-10，行是第 1-5 轮：
//
//	轮次    5   6   7   8   9  10
//	 1      2   2   2   3   3   3
//	 2      3   3   3   4   4   4
//	 3      2   4   3   4   4   4
//	 4      3   3   4   5   5   5
//	 5      3   4   4   5   5   5
//
// 6 人局第 3 轮要 4 人、第 4 轮反而只要 3 人，不是笔误——原表就是这样。
var missionSizes = map[int][5]int{
	5:  {2, 3, 2, 3, 3},
	6:  {2, 3, 4, 3, 4},
	7:  {2, 3, 3, 4, 4},
	8:  {3, 4, 4, 5, 5},
	9:  {3, 4, 4, 5, 5},
	10: {3, 4, 4, 5, 5},
}

// evilCounts 每种人数下有几个坏人。取自英文维基条目。
var evilCounts = map[int]int{5: 2, 6: 2, 7: 3, 8: 3, 9: 3, 10: 4}

// MissionSize 第 mission 轮任务（1-5）在 players 人局里需要几人上场。
// 人数或轮次超出范围时返回 0。
func MissionSize(players, mission int) int {
	sizes, ok := missionSizes[players]
	if !ok || mission < 1 || mission > 5 {
		return 0
	}
	return sizes[mission-1]
}

// EvilCount players 人局里有几个坏人。人数超出范围时返回 0。
func EvilCount(players int) int { return evilCounts[players] }

// FailsNeeded 第 mission 轮任务需要几张失败票才算失败。
//
// 条目原文：「If one (or two in Mission 4 when at least 7 players are playing)
// Mission Fail cards were turned in, the Spies win a point for the active
// mission.」——只有第四轮、且七人及以上，才要两张。
func FailsNeeded(players, mission int) int {
	if mission == 4 && players >= 7 {
		return 2
	}
	return 1
}

// HammerRejections 连续这么多次组队被否决，坏人直接获胜。
//
// 条目原文：「After five successively rejected mission proposals in a single
// mission, the Spies immediately win the game.」
const HammerRejections = 5

// DefaultConfig 阿瓦隆的默认板子。
//
// 阶段环是三个节点的循环：提名 -> 表决 -> 任务 -> 提名。表决没通过时
// 任务阶段会空转一次（那一轮没有人被标记上场），这是绕法的代价之一，
// 见 SCARS.md 第 2 条。
//
// 刺杀阶段不在环里：它由任务阶段在好人凑满三次成功时用绕道排进来。
// 内核那套「谁、去哪个阶段」的排队机制原本是为出局技能做的，用在这里
// 严丝合缝——它还顺带保证了胜负判定推迟到刺杀结算之后，正是规则要的。
func DefaultConfig() *engine.Config {
	return &engine.Config{
		StartPhase: PhasePropose,
		Phases: map[engine.PhaseType]*engine.PhaseConfig{
			PhasePropose: {
				Type: PhasePropose,

				// 队伍标记活到「下一次提名开始」，不是「下一轮任务」——
				// 一轮任务里可能提名五次。此前内核只有「回合级」一档寿命，
				// 而回合数要跟着第几轮任务走（EndsRound 标在任务阶段），
				// 两者不重合，只能在提名解析器里手工清一遍。
				//
				// 现在寿命与计数分开声明：这里说「我开始时是干净的」，
				// 回合数由任务阶段的 EndsRound 管。
				ClearsRoundVars: true,
				// 队长提名：一支队伍要几个人，就提交几次 PROPOSE。
				//
				// SkillUse.TargetID 是单个字符串，表达不了「一次提名一支队伍」。
				// 拆成多次提交是能走通的，但就绪判定因此说不清「还差几个人」
				// ——它只知道「队长提交过没有」。见 SCARS.md 第 3 条。
				Steps: []engine.PhaseStep{
					{Role: engine.RoleUnspecified, Skill: SkillPropose, Required: true},
				},
				Timeout:   ProposeTimeout,
				NextPhase: PhaseTeamVote,
			},
			PhaseTeamVote: {
				Type: PhaseTeamVote,
				// 全员表决，一人一票，接受或否决二选一。
				Steps: []engine.PhaseStep{
					{Role: engine.RoleUnspecified, Skill: SkillApprove, Required: true, Multiple: true, Group: "vote"},
					{Role: engine.RoleUnspecified, Skill: SkillReject, Required: true, Multiple: true, Group: "vote"},
				},
				Timeout:   TeamVoteTimeout,
				NextPhase: PhaseMission,
			},
			PhaseMission: {
				Type: PhaseMission,
				// 本该只有被选中的队员能提交，而内核判定行动者只看角色。
				// 这里只能对所有人开放，再由解析器把不该算的丢掉——
				// 代价是 AllowedSkills 会对没上任务的玩家说「你可以投」。
				// 这是绕法最贵的一处，见 SCARS.md 第 1 条。
				Steps: []engine.PhaseStep{
					{Role: engine.RoleUnspecified, Skill: SkillMissionSuccess, Required: true, Multiple: true, Group: "mission"},
					{Role: engine.RoleUnspecified, Skill: SkillMissionFail, Required: true, Multiple: true, Group: "mission"},
				},
				Timeout:   MissionTimeout,
				NextPhase: PhasePropose,

				// 任务结算完是新的一回合。
				//
				// 这是「回合边界交给规则」之后立刻兑现的好处：阿瓦隆的
				// Round 从此等于**第几轮任务**，与它自己说的话对得上。
				// 此前内核猜「绕回起始阶段就算新回合」，而这里每提名一次
				// 就绕一圈，于是 Round 成了提名计数器，跟「第几轮任务」
				// 最多差五倍，还被 PlayerView.Round 原样发给玩家。
				EndsRound: true,
			},
			PhaseAssassin: {
				Type: PhaseAssassin,
				Steps: []engine.PhaseStep{
					{Role: RoleAssassin, Skill: SkillAssassinate, Required: true},
				},
				Timeout:   AssassinTimeout,
				NextPhase: PhasePropose,
			},
		},
	}
}
