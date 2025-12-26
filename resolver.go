package werewolf

import (
	pb "github.com/Zereker/werewolf/proto"
)

// Resolver 冲突解析器接口
type Resolver interface {
	Resolve(uses []*SkillUse, state *State, config *GameConfig) []*Effect
}

// VoteResult 投票结果
type VoteResult struct {
	Winner  string              // 得票最多的目标（平票时为空）
	Tied    bool                // 是否平票
	Votes   map[string]int      // 各目标得票数
	Voters  map[string][]string // 各目标的投票者
	MaxVote int                 // 最高票数
}

// countVotes 统计投票结果（公共函数，消除重复逻辑）
func countVotes(uses []*SkillUse, skillType pb.SkillType) VoteResult {
	votes := make(map[string]int)
	voters := make(map[string][]string)
	votedPlayers := make(map[string]bool)

	for _, use := range uses {
		if use.Skill != skillType || use.TargetID == "" {
			continue
		}
		// 防止同一玩家重复投票
		if votedPlayers[use.PlayerID] {
			continue
		}
		votedPlayers[use.PlayerID] = true
		votes[use.TargetID]++
		voters[use.TargetID] = append(voters[use.TargetID], use.PlayerID)
	}

	// 找出最高票数和是否平票
	var winner string
	maxVotes := 0
	tied := false

	for target, count := range votes {
		if count > maxVotes {
			winner = target
			maxVotes = count
			tied = false
		} else if count == maxVotes && maxVotes > 0 {
			tied = true
		}
	}

	if tied {
		winner = ""
	}

	return VoteResult{
		Winner:  winner,
		Tied:    tied,
		Votes:   votes,
		Voters:  voters,
		MaxVote: maxVotes,
	}
}

// VoteResolver 投票阶段解析器
type VoteResolver struct{}

// NewVoteResolver 创建投票解析器
func NewVoteResolver() *VoteResolver {
	return &VoteResolver{}
}

// Resolve 解析投票结果
func (r *VoteResolver) Resolve(uses []*SkillUse, state *State, config *GameConfig) []*Effect {
	effects := make([]*Effect, 0)

	result := countVotes(uses, pb.SkillType_SKILL_TYPE_VOTE)

	// 如果平票或无票，不处决任何人
	if result.Tied || result.Winner == "" {
		effect := NewEffect(pb.EventType_EVENT_TYPE_UNSPECIFIED, "", "").
			WithData("result", "tied").
			WithData("votes", result.Votes)
		effects = append(effects, effect)
		return effects
	}

	// 处决得票最多的玩家
	effect := NewEffect(pb.EventType_EVENT_TYPE_ELIMINATE, "", result.Winner).
		WithData("votes", result.MaxVote).
		WithData("voters", result.Voters[result.Winner]).
		WithData("allVotes", result.Votes)
	effects = append(effects, effect)

	// 检查被处决者是否是猎人
	if target, ok := state.GetPlayerInfo(result.Winner); ok {
		if target.Role == pb.RoleType_ROLE_TYPE_HUNTER {
			hunterTriggerEffect := NewEffect(pb.EventType_EVENT_TYPE_HUNTER_TRIGGERED, result.Winner, "")
			effects = append(effects, hunterTriggerEffect)
		}
	}

	return effects
}

// DayResolver 白天阶段解析器（主要处理发言，无状态变化）
type DayResolver struct{}

// NewDayResolver 创建白天解析器
func NewDayResolver() *DayResolver {
	return &DayResolver{}
}

// Resolve 解析白天行动（发言不产生状态变化）
func (r *DayResolver) Resolve(uses []*SkillUse, state *State, config *GameConfig) []*Effect {
	// 白天发言不产生游戏状态变化
	return []*Effect{}
}

// ==================== 夜晚子阶段 Resolver ====================

// GuardResolver 守卫阶段解析器
type GuardResolver struct{}

func NewGuardResolver() *GuardResolver {
	return &GuardResolver{}
}

func (r *GuardResolver) Resolve(uses []*SkillUse, state *State, config *GameConfig) []*Effect {
	effects := make([]*Effect, 0)
	usedPlayers := make(map[string]bool)

	for _, use := range uses {
		// 防止同一玩家重复提交技能
		if usedPlayers[use.PlayerID] {
			continue
		}

		if use.Skill == pb.SkillType_SKILL_TYPE_PROTECT && use.TargetID != "" {
			usedPlayers[use.PlayerID] = true
			protectEffect := NewEffect(pb.EventType_EVENT_TYPE_PROTECT, use.PlayerID, use.TargetID)

			// 检查是否可以保护目标（连续保护限制）
			if !state.CanProtect(use.PlayerID, use.TargetID, config.GuardCanRepeat) {
				protectEffect.Cancel("cannot protect same target consecutively")
			} else if use.PlayerID == use.TargetID && !config.GuardCanProtectSelf {
				// 检查是否自守
				protectEffect.Cancel("guard cannot protect self")
			} else {
				// 通过 Effect 记录本回合保护的目标
				setLastProtectedEffect := NewEffect(pb.EventType_EVENT_TYPE_SET_LAST_PROTECTED, use.PlayerID, use.TargetID)
				effects = append(effects, setLastProtectedEffect)
			}

			effects = append(effects, protectEffect)
		}
	}
	return effects
}

// WolfResolver 狼人阶段解析器
type WolfResolver struct{}

func NewWolfResolver() *WolfResolver {
	return &WolfResolver{}
}

func (r *WolfResolver) Resolve(uses []*SkillUse, state *State, config *GameConfig) []*Effect {
	effects := make([]*Effect, 0)

	// 使用公共投票统计函数
	result := countVotes(uses, pb.SkillType_SKILL_TYPE_KILL)

	// 无票或平票则空刀（狼人未达成共识）
	if result.Winner == "" {
		return effects
	}

	// 检查同守同杀：使用 RoundContext 检查保护状态
	// Guard 的 PROTECT Effect 已经在上一阶段应用到 RoundContext
	if state.RoundCtx.IsProtected(result.Winner) && config.SameGuardKillIsEmpty {
		// 同守同杀空刀 - 不设置击杀目标
		// 女巫不知道有人被攻击
		return effects
	}

	// 通过 Effect 设置狼人击杀目标（供女巫查询）
	// 不直接修改 state，由 ApplyEffect 统一处理
	setKillEffect := NewEffect(pb.EventType_EVENT_TYPE_SET_NIGHT_KILL, "", result.Winner)
	effects = append(effects, setKillEffect)

	return effects
}

// WitchResolver 女巫阶段解析器
type WitchResolver struct{}

func NewWitchResolver() *WitchResolver {
	return &WitchResolver{}
}

func (r *WitchResolver) Resolve(uses []*SkillUse, state *State, config *GameConfig) []*Effect {
	effects := make([]*Effect, 0)

	// 获取击杀目标（RoundContext 保证非 nil）
	killTarget := state.RoundCtx.KillTarget

	// 防止同一玩家重复使用同一技能（女巫可以同时使用解药和毒药，但不能重复）
	usedSkills := make(map[string]bool) // key: "playerID:skillType"

	for _, use := range uses {
		skillKey := use.PlayerID + ":" + use.Skill.String()
		if usedSkills[skillKey] {
			continue
		}

		switch use.Skill {
		case pb.SkillType_SKILL_TYPE_ANTIDOTE:
			if use.TargetID != "" {
				usedSkills[skillKey] = true
				saveEffect := NewEffect(pb.EventType_EVENT_TYPE_SAVE, use.PlayerID, use.TargetID)

				// 检查是否有解药
				if !state.CanUseAntidote(use.PlayerID) {
					saveEffect.Cancel("no antidote")
				} else if use.PlayerID == use.TargetID && !config.WitchCanSaveSelf {
					// 检查是否自救
					saveEffect.Cancel("witch cannot save self")
				} else if killTarget == "" {
					// 今晚没有人被杀（狼人空刀或平票）
					saveEffect.Cancel("no one is dying tonight")
				} else if use.TargetID != killTarget {
					// 只能救被杀的人
					saveEffect.Cancel("target is not dying")
				} else {
					// 救的是被杀的人，通过 Effect 消耗解药并清除击杀目标
					useAntidoteEffect := NewEffect(pb.EventType_EVENT_TYPE_USE_ANTIDOTE, use.PlayerID, "")
					effects = append(effects, useAntidoteEffect)

					clearKillEffect := NewEffect(pb.EventType_EVENT_TYPE_CLEAR_NIGHT_KILL, use.PlayerID, "")
					effects = append(effects, clearKillEffect)
				}

				effects = append(effects, saveEffect)
			}
		case pb.SkillType_SKILL_TYPE_POISON:
			if use.TargetID != "" {
				usedSkills[skillKey] = true

				// 检查是否有毒药
				if !state.CanUsePoison(use.PlayerID) {
					// 无毒药，产生一个被取消的效果用于通知
					canceledEffect := NewEffect(pb.EventType_EVENT_TYPE_POISON, use.PlayerID, use.TargetID)
					canceledEffect.Cancel("no poison")
					effects = append(effects, canceledEffect)
				} else if use.PlayerID == use.TargetID {
					// 检查是否自毒
					canceledEffect := NewEffect(pb.EventType_EVENT_TYPE_POISON, use.PlayerID, use.TargetID)
					canceledEffect.Cancel("witch cannot poison self")
					effects = append(effects, canceledEffect)
				} else {
					// 通过 Effect 消耗毒药并标记目标（实际死亡在 NightResolveResolver 处理）
					usePoisonEffect := NewEffect(pb.EventType_EVENT_TYPE_USE_POISON, use.PlayerID, use.TargetID)
					effects = append(effects, usePoisonEffect)
				}
			}
		}
	}

	return effects
}

// SeerResolver 预言家阶段解析器
// 仅处理预言家查验，夜晚结算由 NightResolveResolver 处理
type SeerResolver struct{}

func NewSeerResolver() *SeerResolver {
	return &SeerResolver{}
}

func (r *SeerResolver) Resolve(uses []*SkillUse, state *State, config *GameConfig) []*Effect {
	effects := make([]*Effect, 0)
	usedPlayers := make(map[string]bool)

	for _, use := range uses {
		// 防止同一玩家重复提交技能
		if usedPlayers[use.PlayerID] {
			continue
		}

		if use.Skill == pb.SkillType_SKILL_TYPE_CHECK && use.TargetID != "" {
			usedPlayers[use.PlayerID] = true
			checkEffect := NewEffect(pb.EventType_EVENT_TYPE_CHECK, use.PlayerID, use.TargetID)
			// 使用只读副本避免竞态风险
			if target, ok := state.GetPlayerInfo(use.TargetID); ok {
				checkEffect.
					WithData("camp", target.Camp).
					WithData("isGood", target.Camp == pb.Camp_CAMP_GOOD)
			}
			effects = append(effects, checkEffect)
		}
	}

	return effects
}

// NightResolveResolver 夜晚结算阶段解析器
// 处理狼人击杀结算、女巫毒杀结算、猎人触发检测等
type NightResolveResolver struct{}

func NewNightResolveResolver() *NightResolveResolver {
	return &NightResolveResolver{}
}

func (r *NightResolveResolver) Resolve(uses []*SkillUse, state *State, config *GameConfig) []*Effect {
	effects := make([]*Effect, 0)

	// 处理狼人击杀
	if state.RoundCtx.KillTarget != "" {
		killTarget := state.RoundCtx.KillTarget

		// 检查同守同杀：目标被保护 + 配置为空刀
		if state.RoundCtx.IsProtected(killTarget) && config.SameGuardKillIsEmpty {
			// 同守同杀空刀 - 不产生击杀效果
			clearKillEffect := NewEffect(pb.EventType_EVENT_TYPE_CLEAR_NIGHT_KILL, "", "")
			effects = append(effects, clearKillEffect)
		} else {
			// 正常击杀
			killEffect := NewEffect(pb.EventType_EVENT_TYPE_KILL, "", killTarget)
			effects = append(effects, killEffect)

			// 检查被杀者是否是猎人，如果是则触发猎人技能
			if target, ok := state.GetPlayerInfo(killTarget); ok {
				if target.Role == pb.RoleType_ROLE_TYPE_HUNTER {
					hunterTriggerEffect := NewEffect(pb.EventType_EVENT_TYPE_HUNTER_TRIGGERED, killTarget, "")
					effects = append(effects, hunterTriggerEffect)
				}
			}
		}
	}

	// 处理女巫毒杀（毒杀的玩家已在 WitchResolver 中标记到 RoundContext）
	for playerID := range state.RoundCtx.PoisonedPlayers {
		poisonKillEffect := NewEffect(pb.EventType_EVENT_TYPE_POISON, "", playerID)
		effects = append(effects, poisonKillEffect)

		// 检查被毒者是否是猎人
		if target, ok := state.GetPlayerInfo(playerID); ok {
			if target.Role == pb.RoleType_ROLE_TYPE_HUNTER {
				hunterTriggerEffect := NewEffect(pb.EventType_EVENT_TYPE_HUNTER_TRIGGERED, playerID, "")
				effects = append(effects, hunterTriggerEffect)
			}
		}
	}

	return effects
}

// HunterResolver 猎人阶段解析器
type HunterResolver struct{}

func NewHunterResolver() *HunterResolver {
	return &HunterResolver{}
}

func (r *HunterResolver) Resolve(uses []*SkillUse, state *State, config *GameConfig) []*Effect {
	effects := make([]*Effect, 0)
	usedPlayers := make(map[string]bool)

	for _, use := range uses {
		// 防止同一玩家重复提交技能
		if usedPlayers[use.PlayerID] {
			continue
		}

		switch use.Skill {
		case pb.SkillType_SKILL_TYPE_SHOOT:
			if use.TargetID != "" {
				usedPlayers[use.PlayerID] = true
				shootEffect := NewEffect(pb.EventType_EVENT_TYPE_SHOOT, use.PlayerID, use.TargetID)
				effects = append(effects, shootEffect)
			}
		case pb.SkillType_SKILL_TYPE_SKIP:
			// 猎人选择不开枪
			usedPlayers[use.PlayerID] = true
			skipEffect := NewEffect(pb.EventType_EVENT_TYPE_SKIP, use.PlayerID, "")
			effects = append(effects, skipEffect)
		}
	}

	return effects
}
