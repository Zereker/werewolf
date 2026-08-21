package werewolf

import (
	pb "github.com/Zereker/werewolf/proto"
)

// Resolver 冲突解析器接口。
//
// 实现者只能读 GameView、只能通过返回 Effect 表达状态变更——
// 这是引擎最重要的不变量，由签名保证而非靠约定。
//
// 注意：Resolve 在引擎持锁期间被调用，实现中不要回调 Engine 的任何方法。
type Resolver interface {
	Resolve(uses []*SkillUse, view GameView, config *GameConfig) []*Effect
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

	// 两遍扫描：先求最高票，再数有几个人拿到最高票。
	//
	// 单遍扫描也能得到正确结果，但正确性依赖「tied 会被后来的严格更大值
	// 重置」这个推理，而 map 的遍历顺序又是随机的——要确信它对，读者得
	// 自己把几种顺序都推一遍。两遍扫描一眼就能看出与顺序无关。
	maxVotes := 0
	for _, count := range votes {
		if count > maxVotes {
			maxVotes = count
		}
	}

	var winner string
	winners := 0
	for target, count := range votes {
		if count == maxVotes {
			winners++
			winner = target
		}
	}

	tied := winners > 1
	if tied || maxVotes == 0 {
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

// firstUsePerPlayer 按提交顺序遍历，每位玩家只取其首次提交的该技能。
//
// 三个 Resolver（守卫、预言家、猎人）原本各写了一遍相同的去重样板。
// 「改主意」由调用方在提交前自行处理；到了结算这一步，一人一次。
func firstUsePerPlayer(uses []*SkillUse, fn func(use *SkillUse)) {
	seen := make(map[string]bool, len(uses))
	for _, use := range uses {
		if seen[use.PlayerID] {
			continue
		}
		seen[use.PlayerID] = true
		fn(use)
	}
}

// VoteResolver 投票阶段解析器
type VoteResolver struct{}

// NewVoteResolver 创建投票解析器
func NewVoteResolver() *VoteResolver {
	return &VoteResolver{}
}

// Resolve 解析投票结果
func (r *VoteResolver) Resolve(uses []*SkillUse, view GameView, config *GameConfig) []*Effect {
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
	if target, ok := view.Player(result.Winner); ok {
		if target.Role == pb.RoleType_ROLE_TYPE_HUNTER {
			effects = append(effects,
				NewAbilityTriggerEffect(result.Winner, pb.PhaseType_PHASE_TYPE_DAY_HUNTER))
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
func (r *DayResolver) Resolve(uses []*SkillUse, view GameView, config *GameConfig) []*Effect {
	// 白天发言不产生游戏状态变化
	return []*Effect{}
}

// ==================== 夜晚子阶段 Resolver ====================

// GuardResolver 守卫阶段解析器
type GuardResolver struct{}

func NewGuardResolver() *GuardResolver {
	return &GuardResolver{}
}

func (r *GuardResolver) Resolve(uses []*SkillUse, view GameView, config *GameConfig) []*Effect {
	effects := make([]*Effect, 0)

	firstUsePerPlayer(uses, func(use *SkillUse) {
		if use.Skill != pb.SkillType_SKILL_TYPE_PROTECT || use.TargetID == "" {
			return
		}

		protect := NewEffect(pb.EventType_EVENT_TYPE_PROTECT, use.PlayerID, use.TargetID)

		switch {
		case !config.GuardCanRepeat && view.LastProtectedTarget(use.PlayerID) == use.TargetID:
			// 连守限制：视图只给「上回合守了谁」，是否允许由规则配置决定
			protect.Cancel("cannot protect same target consecutively")
		case use.PlayerID == use.TargetID && !config.GuardCanProtectSelf:
			protect.Cancel("guard cannot protect self")
		default:
			// 守护生效，记下本回合的目标供下回合判断连守
			effects = append(effects,
				NewEffect(pb.EventType_EVENT_TYPE_SET_LAST_PROTECTED, use.PlayerID, use.TargetID))
		}

		effects = append(effects, protect)
	})

	return effects
}

// WolfResolver 狼人阶段解析器
type WolfResolver struct{}

func NewWolfResolver() *WolfResolver {
	return &WolfResolver{}
}

func (r *WolfResolver) Resolve(uses []*SkillUse, view GameView, config *GameConfig) []*Effect {
	effects := make([]*Effect, 0)

	// 使用公共投票统计函数
	result := countVotes(uses, pb.SkillType_SKILL_TYPE_KILL)

	// 无票或平票则空刀（狼人未达成共识）
	if result.Winner == "" {
		return effects
	}

	// 无论目标是否被守卫守护，都记录刀口。
	//
	// 守护能否抵消、解药能否救回，统一由 NightResolveResolver 判定：
	//   - 狼人不知道守卫守了谁，刀是照砍的
	//   - 女巫看到的是「狼刀目标」，她同样不知道守卫的动作
	// 若在此处因守护而不记录刀口，「同守同救」这一局面根本无法构成。
	setKillEffect := NewEffect(pb.EventType_EVENT_TYPE_SET_NIGHT_KILL, "", result.Winner)
	effects = append(effects, setKillEffect)

	return effects
}

// WitchResolver 女巫阶段解析器
type WitchResolver struct{}

func NewWitchResolver() *WitchResolver {
	return &WitchResolver{}
}

func (r *WitchResolver) Resolve(uses []*SkillUse, view GameView, config *GameConfig) []*Effect {
	effects := make([]*Effect, 0)
	killTarget := view.RoundContext().KillTarget

	// 同一玩家的同一技能只取首次提交
	type skillKey struct {
		player string
		skill  pb.SkillType
	}
	used := make(map[skillKey]bool, len(uses))

	// 规则「解藥和毒藥不可以在同一夜使用」：记录本夜已成功用药的女巫。
	// 只有真正生效的用药才计入——解药若因「今晚没人被杀」被取消，
	// 女巫本夜仍可以正常使用毒药。
	potionUsed := make(map[string]bool)

	for _, use := range uses {
		if use.Skill != pb.SkillType_SKILL_TYPE_ANTIDOTE &&
			use.Skill != pb.SkillType_SKILL_TYPE_POISON {
			continue
		}
		if use.TargetID == "" {
			continue
		}

		key := skillKey{player: use.PlayerID, skill: use.Skill}
		if used[key] {
			continue
		}
		used[key] = true

		// 本夜是否已经用过另一瓶药
		blocked := !config.WitchCanUseBothPotions && potionUsed[use.PlayerID]

		var produced []*Effect
		var consumed bool
		if use.Skill == pb.SkillType_SKILL_TYPE_ANTIDOTE {
			produced, consumed = resolveAntidote(use, view, config, killTarget, blocked)
		} else {
			produced, consumed = resolvePoison(use, view, blocked)
		}

		if consumed {
			potionUsed[use.PlayerID] = true
		}
		effects = append(effects, produced...)
	}

	return effects
}

// resolveAntidote 结算一次解药使用。
//
// 无论成败都产出一个 SAVE 效果，被拒时带上原因——调用方需要知道
// 「女巫点了但没成，为什么」，而不是什么都收不到。
//
// 第二个返回值表示解药是否真的被消耗，用于判断本夜能否再用毒药。
// 这个信息必须显式返回：从产出的效果个数去反推既脆弱又难读。
func resolveAntidote(use *SkillUse, view GameView, config *GameConfig, killTarget string, blocked bool) ([]*Effect, bool) {
	save := NewEffect(pb.EventType_EVENT_TYPE_SAVE, use.PlayerID, use.TargetID)

	switch {
	case blocked:
		save.Cancel("cannot use both potions in one night")
	case !witchHas(view, use.PlayerID, potionAntidote):
		save.Cancel("no antidote")
	case use.PlayerID == use.TargetID && !config.WitchCanSaveSelf:
		save.Cancel("witch cannot save self")
	case killTarget == "":
		// 今晚没有人被杀（狼人空刀或平票）
		save.Cancel("no one is dying tonight")
	case use.TargetID != killTarget:
		save.Cancel("target is not dying")
	default:
		// 救的是被杀的人，消耗解药。是否真的救回由 NightResolveResolver
		// 综合「是否同时被守卫守护」判定（同守同救可能依然死亡）
		return []*Effect{
			NewEffect(pb.EventType_EVENT_TYPE_USE_ANTIDOTE, use.PlayerID, ""),
			save,
		}, true
	}

	return []*Effect{save}, false
}

// resolvePoison 结算一次毒药使用。与 resolveAntidote 同构。
func resolvePoison(use *SkillUse, view GameView, blocked bool) ([]*Effect, bool) {
	poison := NewEffect(pb.EventType_EVENT_TYPE_POISON, use.PlayerID, use.TargetID)

	switch {
	case blocked:
		poison.Cancel("cannot use both potions in one night")
	case !witchHas(view, use.PlayerID, potionPoison):
		poison.Cancel("no poison")
	case use.PlayerID == use.TargetID:
		poison.Cancel("witch cannot poison self")
	default:
		// 消耗毒药并标记目标，实际死亡在 NightResolveResolver 结算
		return []*Effect{
			NewEffect(pb.EventType_EVENT_TYPE_USE_POISON, use.PlayerID, use.TargetID),
		}, true
	}

	return []*Effect{poison}, false
}

// SeerResolver 预言家阶段解析器
// 仅处理预言家查验，夜晚结算由 NightResolveResolver 处理
type SeerResolver struct{}

func NewSeerResolver() *SeerResolver {
	return &SeerResolver{}
}

func (r *SeerResolver) Resolve(uses []*SkillUse, view GameView, config *GameConfig) []*Effect {
	effects := make([]*Effect, 0)

	firstUsePerPlayer(uses, func(use *SkillUse) {
		if use.Skill != pb.SkillType_SKILL_TYPE_CHECK || use.TargetID == "" {
			return
		}

		check := NewEffect(pb.EventType_EVENT_TYPE_CHECK, use.PlayerID, use.TargetID)
		// 只报阵营，不报具体角色
		if target, ok := view.Player(use.TargetID); ok {
			check.
				WithData("camp", target.Camp).
				WithData("isGood", target.Camp == pb.Camp_CAMP_GOOD)
		}
		effects = append(effects, check)
	})

	return effects
}

// NightResolveResolver 夜晚结算阶段解析器
// 处理狼人击杀结算、女巫毒杀结算、猎人触发检测等
type NightResolveResolver struct{}

func NewNightResolveResolver() *NightResolveResolver {
	return &NightResolveResolver{}
}

func (r *NightResolveResolver) Resolve(uses []*SkillUse, view GameView, config *GameConfig) []*Effect {
	effects := make([]*Effect, 0)

	// 处理狼人击杀。
	//
	// 刀口的最终结果由「是否被守卫守护」与「女巫是否用了解药」共同决定：
	//
	//	被守 + 被救 -> GuardSaveTogetherDies 决定生死（同守同救，默认死亡）
	//	被守       -> SameGuardKillIsEmpty 决定守护是否生效
	//	被救       -> 救回
	//	都没有      -> 死亡
	rc := view.RoundContext()
	if rc.KillTarget != "" {
		killTarget := rc.KillTarget
		protected := rc.IsProtected(killTarget)
		saved := rc.IsSaved(killTarget)

		var dies bool
		var reason string
		switch {
		case protected && saved:
			// 同守同救
			dies = config.GuardSaveTogetherDies
			reason = "guard and antidote used on the same target"
		case protected:
			dies = !config.SameGuardKillIsEmpty
			reason = "protected by guard"
		case saved:
			dies = false
			reason = "saved by witch antidote"
		default:
			dies = true
		}

		if dies {
			killEffect := NewEffect(pb.EventType_EVENT_TYPE_KILL, "", killTarget)
			if reason != "" {
				killEffect.WithData("reason", reason)
			}
			effects = append(effects, killEffect)

			// 检查被杀者是否是猎人，如果是则触发猎人技能
			if target, ok := view.Player(killTarget); ok {
				if target.Role == pb.RoleType_ROLE_TYPE_HUNTER {
					effects = append(effects,
						NewAbilityTriggerEffect(killTarget, pb.PhaseType_PHASE_TYPE_NIGHT_HUNTER))
				}
			}
		} else {
			// 刀口未生效，清除击杀目标
			clearKillEffect := NewEffect(pb.EventType_EVENT_TYPE_CLEAR_NIGHT_KILL, "", "").
				WithData("reason", reason)
			effects = append(effects, clearKillEffect)
		}
	}

	// 处理女巫毒杀（毒杀的玩家已在 WitchResolver 中标记到 RoundContext）
	//
	// 规则「除殉情或被毒殺外，以任何其他方式被淘汰時可以…開槍」：
	// 被毒死的猎人不触发开枪，这正是毒药相对于狼刀的战术价值所在。
	for playerID := range rc.PoisonedPlayers {
		poisonKillEffect := NewEffect(pb.EventType_EVENT_TYPE_POISON, "", playerID)
		effects = append(effects, poisonKillEffect)
	}

	return effects
}

// HunterResolver 猎人阶段解析器
type HunterResolver struct{}

func NewHunterResolver() *HunterResolver {
	return &HunterResolver{}
}

func (r *HunterResolver) Resolve(uses []*SkillUse, view GameView, config *GameConfig) []*Effect {
	effects := make([]*Effect, 0)

	firstUsePerPlayer(uses, func(use *SkillUse) {
		switch use.Skill {
		case pb.SkillType_SKILL_TYPE_SHOOT:
			if use.TargetID != "" {
				effects = append(effects,
					NewEffect(pb.EventType_EVENT_TYPE_SHOOT, use.PlayerID, use.TargetID))
			}
		case pb.SkillType_SKILL_TYPE_SKIP:
			// 猎人选择不开枪
			effects = append(effects,
				NewEffect(pb.EventType_EVENT_TYPE_SKIP, use.PlayerID, ""))
		}
	})

	return effects
}

// potionKind 女巫的两种药
type potionKind int

const (
	potionAntidote potionKind = iota
	potionPoison
)

// witchHas 该女巫是否还持有指定的药。
// 非女巫一律返回 false。
func witchHas(view GameView, playerID string, kind potionKind) bool {
	p, ok := view.Player(playerID)
	if !ok || p.Role != pb.RoleType_ROLE_TYPE_WITCH {
		return false
	}
	if kind == potionAntidote {
		return p.HasAntidote
	}
	return p.HasPoison
}
