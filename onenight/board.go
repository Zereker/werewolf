// board.go 一夜狼人的阶段图。

package onenight

import "github.com/Zereker/werewolf/engine"

// CenterCount 中央牌的张数。规则固定为 3——发牌时永远比人数多三张。
const CenterCount = 3

// GameConfig 一夜狼人的阶段图：九个夜晚环节，然后讨论，然后投票。
//
// # 这是一条直线
//
// 前两套规则包的阶段图都是环：狼人杀绕回守卫、阿瓦隆绕回提名。这一套走到
// VOTE 就结束，一个回合都不需要。Round 从头到尾是 1，回合级变量一次都不清。
//
// 内核的 Config.Validate 要求「至少有一个阶段标 EndsRound、至少有一个标
// ClearsRoundVars」——那两条是为了防「回合永远是 1、回合变量永远不清」这类
// 配置错误。而对这一套规则，回合永远是 1 **恰恰是对的**。只好把两个标记都
// 挂在 VOTE 上，虽然它之后没有下一个回合。见 SCARS.md 疤 2。
//
// # 夜晚次序是规则的一部分
//
// 抢劫者在捣蛋鬼之前动，捣蛋鬼因此能把刚被抢走的牌再换掉；失眠者最后动，
// 因此他看到的是所有交换之后的结果。把次序调换，游戏就变成另一个游戏。
// 这份次序取自官方规则书的叫醒顺序。
func GameConfig() *engine.Config {
	// step 一个只有单一动作的步骤。夜晚能力全是可选的（规则允许「你可以…」），
	// 因此 Required 一律为 false——某个角色不动是合法的。
	step := func(role engine.RoleType, skill engine.SkillType) []engine.PhaseStep {
		return []engine.PhaseStep{{Role: role, Skill: skill}}
	}

	// group 一组几选一的动作：提交其中任意一个即算这个角色动过了。
	group := func(role engine.RoleType, name string, skills ...engine.SkillType) []engine.PhaseStep {
		out := make([]engine.PhaseStep, 0, len(skills))
		for _, s := range skills {
			out = append(out, engine.PhaseStep{Role: role, Skill: s, Group: name})
		}
		return out
	}

	return &engine.Config{
		StartPhase: PhaseNightWerewolf,
		Phases: map[engine.PhaseType]*engine.PhaseConfig{
			// 狼人互认是纯信息（走 RoleInfo），只有「场上仅一只狼」时才有
			// 动作可提交——看一张中央牌。
			PhaseNightWerewolf: {
				Type:      PhaseNightWerewolf,
				Steps:     group(RoleWerewolf, "peek", SkillPeekCenter0, SkillPeekCenter1, SkillPeekCenter2),
				NextPhase: PhaseNightMinion,
			},

			// 爪牙、守夜人、失眠者都只接收信息、不做任何动作。内核里
			// 「醒过来看一眼」没有对应物：没有步骤的阶段，PhaseInfo.ActiveRoles
			// 是空的，主持人不知道该叫醒谁。只好挂一个 SKIP 步骤当占位。
			// 见 SCARS.md 疤 3。
			PhaseNightMinion: {
				Type:      PhaseNightMinion,
				Steps:     step(RoleMinion, engine.SkillSkip),
				NextPhase: PhaseNightMason,
			},
			PhaseNightMason: {
				Type:      PhaseNightMason,
				Steps:     step(RoleMason, engine.SkillSkip),
				NextPhase: PhaseNightSeer,
			},

			PhaseNightSeer: {
				Type: PhaseNightSeer,
				Steps: group(RoleSeer, "look",
					SkillSeerPlayer, SkillSeerCenter01, SkillSeerCenter02, SkillSeerCenter12),
				NextPhase: PhaseNightRobber,
			},

			PhaseNightRobber: {
				Type:      PhaseNightRobber,
				Steps:     step(RoleRobber, SkillRob),
				NextPhase: PhaseNightTroublemake,
			},

			PhaseNightTroublemake: {
				Type:      PhaseNightTroublemake,
				Steps:     step(RoleTroublemaker, SkillMeddle),
				NextPhase: PhaseNightDrunk,
			},

			PhaseNightDrunk: {
				Type: PhaseNightDrunk,
				Steps: group(RoleDrunk, "drink",
					SkillDrinkCenter0, SkillDrinkCenter1, SkillDrinkCenter2),
				NextPhase: PhaseNightInsomniac,
			},

			PhaseNightInsomniac: {
				Type:      PhaseNightInsomniac,
				Steps:     step(RoleInsomniac, engine.SkillSkip),
				NextPhase: PhaseDay,
			},

			// 讨论：没有任何提交，主持人看够了就推进。
			PhaseDay: {
				Type:      PhaseDay,
				NextPhase: PhaseVote,
			},

			// 投票：全员必须投，同时揭晓。
			PhaseVote: {
				Type: PhaseVote,
				Steps: []engine.PhaseStep{{
					Role: engine.RoleUnspecified, Skill: SkillVote,
					Required: true, Multiple: true,
					// 投票指向的是活人，这一局里所有人都活着，
					// 但写明白比依赖默认好。
				}},
				NextPhase:       engine.PhaseEnd,
				EndsRound:       true,
				ClearsRoundVars: true,
			},
		},
	}
}
