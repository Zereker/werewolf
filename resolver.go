package werewolf

import ()

// Resolver 冲突解析器接口。
//
// 实现者只能读 GameView、只能通过返回 Effect 表达状态变更——
// 这是引擎最重要的不变量，由签名保证而非靠约定。
//
// 注意：Resolve 在引擎持锁期间被调用，实现中不要回调 Engine 的任何方法。
type Resolver interface {
	Resolve(uses []*SkillUse, view GameView, config *GameConfig) []*Effect
}

// voteResult 投票结果（包内使用）
type voteResult struct {
	Winner  string              // 得票最多的目标（平票时为空）
	Tied    bool                // 是否平票
	Votes   map[string]int      // 各目标得票数
	Voters  map[string][]string // 各目标的投票者
	MaxVote int                 // 最高票数
}

// countVotes 统计投票结果（公共函数，消除重复逻辑）
func countVotes(uses []*SkillUse, skillType SkillType) voteResult {
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

	return voteResult{
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

	result := countVotes(uses, SkillVote)

	// 如果平票或无票，不处决任何人。
	//
	// 用专门的 VOTE_TIED 而不是 UNSPECIFIED：平票是最常见的一种投票结局，
	// 全场都得知道「今天没人出局」。挂在 UNSPECIFIED 上的事件既没法分类，
	// 也拿不到受众划分。
	if result.Tied || result.Winner == "" {
		effect := NewEffect(EventVoteTied, "", "").
			WithData("result", "tied").
			WithData("votes", result.Votes)
		effects = append(effects, effect)
		return effects
	}

	// 处决得票最多的玩家
	effect := NewEffect(EventEliminate, "", result.Winner).
		WithData("votes", result.MaxVote).
		WithData("voters", result.Voters[result.Winner]).
		WithData("allVotes", result.Votes)
	effects = append(effects, effect, NewSetAliveEffect(result.Winner, false))

	// 被投票出局的猎人可以开枪
	effects = append(effects,
		hunterTrigger(view, result.Winner, PhaseDayHunter)...)

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

// NewGuardResolver 创建守卫阶段解析器。
//
// 内置解析器都是导出的，扩展可以包装它们复用已有逻辑，
// 再经 WithResolver 换上——参见 extension_test.go。
func NewGuardResolver() *GuardResolver {
	return &GuardResolver{}
}

func (r *GuardResolver) Resolve(uses []*SkillUse, view GameView, config *GameConfig) []*Effect {
	effects := make([]*Effect, 0)

	firstUsePerPlayer(uses, func(use *SkillUse) {
		if use.Skill != SkillProtect || use.TargetID == "" {
			return
		}

		protect := NewEffect(EventProtect, use.PlayerID, use.TargetID)

		switch {
		case !config.GuardCanRepeat && lastProtected(view, use.PlayerID) == use.TargetID:
			// 连守限制：视图只给「上回合守了谁」，是否允许由规则配置决定
			protect.Cancel("cannot protect same target consecutively")
		case use.PlayerID == use.TargetID && !config.GuardCanProtectSelf:
			protect.Cancel("guard cannot protect self")
		default:
			// PROTECT 是「发生了什么」的说法，下面两条才真正改状态：
			// 标记今晚被守的人，并记下本回合的守护供下回合判断连守。
			effects = append(effects,
				NewSetPlayerRoundVarEffect(use.TargetID, PlayerRoundVarProtected, VarPresent))
			effects = append(effects, markProtected(view, use.PlayerID, use.TargetID)...)
		}

		effects = append(effects, protect)
	})

	return effects
}

// WolfResolver 狼人阶段解析器
type WolfResolver struct{}

// NewWolfResolver 创建狼人阶段解析器。
func NewWolfResolver() *WolfResolver {
	return &WolfResolver{}
}

func (r *WolfResolver) Resolve(uses []*SkillUse, view GameView, config *GameConfig) []*Effect {
	effects := make([]*Effect, 0)

	// 使用公共投票统计函数
	result := countVotes(uses, SkillKill)

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
	effects = append(effects,
		NewSetRoundVarEffect(RoundVarKillTarget, result.Winner))

	return effects
}

// WitchResolver 女巫阶段解析器
type WitchResolver struct{}

// NewWitchResolver 创建女巫阶段解析器。
func NewWitchResolver() *WitchResolver {
	return &WitchResolver{}
}

func (r *WitchResolver) Resolve(uses []*SkillUse, view GameView, config *GameConfig) []*Effect {
	effects := make([]*Effect, 0)
	killTarget := nightKillTarget(view)

	// 同一玩家的同一技能只取首次提交
	type skillKey struct {
		player string
		skill  SkillType
	}
	used := make(map[skillKey]bool, len(uses))

	// 规则「解藥和毒藥不可以在同一夜使用」：记录本夜已成功用药的女巫。
	// 只有真正生效的用药才计入——解药若因「今晚没人被杀」被取消，
	// 女巫本夜仍可以正常使用毒药。
	potionUsed := make(map[string]bool)

	for _, use := range uses {
		if use.Skill != SkillAntidote &&
			use.Skill != SkillPoison {
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
		if use.Skill == SkillAntidote {
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
	save := NewEffect(EventSave, use.PlayerID, use.TargetID)

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
		// 综合「是否同时被守卫守护」判定（同守同救可能依然死亡）。
		//
		// SAVE 是说法，下面两条是状态：药少一瓶，目标带上「今晚被救」的标记。
		return []*Effect{
			save,
			NewSetPlayerVarEffect(use.PlayerID, VarWitchAntidote, ""),
			NewSetPlayerRoundVarEffect(use.TargetID, PlayerRoundVarSaved, VarPresent),
		}, true
	}

	return []*Effect{save}, false
}

// resolvePoison 结算一次毒药使用。与 resolveAntidote 同构。
func resolvePoison(use *SkillUse, view GameView, blocked bool) ([]*Effect, bool) {
	poison := NewEffect(EventPoison, use.PlayerID, use.TargetID)

	switch {
	case blocked:
		poison.Cancel("cannot use both potions in one night")
	case !witchHas(view, use.PlayerID, potionPoison):
		poison.Cancel("no poison")
	case use.PlayerID == use.TargetID:
		poison.Cancel("witch cannot poison self")
	default:
		// 消耗毒药并标记目标，实际死亡在 NightResolveResolver 结算。
		// 这里刻意不产出 POISON：中毒要到天亮才公布，此刻发出去
		// 等于当场告诉全场谁被毒了。
		return []*Effect{
			NewSetPlayerVarEffect(use.PlayerID, VarWitchPoison, ""),
			NewSetPlayerRoundVarEffect(use.TargetID, PlayerRoundVarPoisoned, VarPresent),
		}, true
	}

	return []*Effect{poison}, false
}

// SeerResolver 预言家阶段解析器
// 仅处理预言家查验，夜晚结算由 NightResolveResolver 处理
type SeerResolver struct{}

// NewSeerResolver 创建预言家阶段解析器。
func NewSeerResolver() *SeerResolver {
	return &SeerResolver{}
}

func (r *SeerResolver) Resolve(uses []*SkillUse, view GameView, config *GameConfig) []*Effect {
	effects := make([]*Effect, 0)

	firstUsePerPlayer(uses, func(use *SkillUse) {
		if use.Skill != SkillCheck || use.TargetID == "" {
			return
		}

		check := NewEffect(EventCheck, use.PlayerID, use.TargetID)
		// 只报阵营，不报具体角色
		if target, ok := view.Player(use.TargetID); ok {
			check.
				WithData("camp", target.Camp).
				WithData("isGood", target.Camp == CampGood)
		}
		effects = append(effects, check)
	})

	return effects
}

// NightResolveResolver 夜晚结算阶段解析器
// 处理狼人击杀结算、女巫毒杀结算、猎人触发检测等
type NightResolveResolver struct{}

// NewNightResolveResolver 创建夜晚结算解析器。
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
	if killTarget := nightKillTarget(view); killTarget != "" {
		protected := isProtected(view, killTarget)
		saved := isSaved(view, killTarget)

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
			killEffect := NewEffect(EventKill, "", killTarget)
			if reason != "" {
				killEffect.WithData("reason", reason)
			}
			effects = append(effects, killEffect, NewSetAliveEffect(killTarget, false))

			// 「除殉情或被毒殺外」是对死因的排除。同一晚既被刀又被毒的猎人
			// 身上有毒，即便这里走的是刀口这条通道，也不能开枪。
			if !isPoisoned(view, killTarget) {
				effects = append(effects,
					hunterTrigger(view, killTarget, PhaseNightHunter)...)
			}
		} else {
			// 刀口未生效，清除击杀目标
			effects = append(effects,
				NewSetRoundVarEffect(RoundVarKillTarget, "").WithData("reason", reason))
		}
	}

	// 处理女巫毒杀（毒杀的玩家已在 WitchResolver 中标记）
	//
	// 规则「除殉情或被毒殺外，以任何其他方式被淘汰時可以…開槍」：
	// 被毒死的猎人不触发开枪，这正是毒药相对于狼刀的战术价值所在。
	//
	// 走 AllPlayers 而不是遍历一张 map：它按 ID 排序，同一个局面每次
	// 结算产出的效果顺序都一样，效果流的回放与比对才有确定性。
	for _, p := range view.AllPlayers() {
		if !isPoisoned(view, p.ID) {
			continue
		}
		effects = append(effects,
			NewEffect(EventPoison, "", p.ID),
			NewSetAliveEffect(p.ID, false))
	}

	return effects
}

// HunterResolver 猎人阶段解析器
type HunterResolver struct{}

// NewHunterResolver 创建猎人阶段解析器。
func NewHunterResolver() *HunterResolver {
	return &HunterResolver{}
}

func (r *HunterResolver) Resolve(uses []*SkillUse, view GameView, config *GameConfig) []*Effect {
	effects := make([]*Effect, 0)

	firstUsePerPlayer(uses, func(use *SkillUse) {
		switch use.Skill {
		case SkillShoot:
			if use.TargetID != "" {
				effects = append(effects,
					NewEffect(EventShoot, use.PlayerID, use.TargetID),
					NewSetAliveEffect(use.TargetID, false))
				// 枪口下的另一名猎人同样可以回枪：规则排除的只有
				// 殉情与毒杀，被枪打死属于「其他方式」。
				// 死亡触发的入队分散在狼刀、投票、开枪三条通道上，
				// 少一条这类连锁就断在那里。
				effects = append(effects, hunterTrigger(view, use.TargetID, use.Phase)...)
			}
		case SkillSkip:
			// 猎人选择不开枪
			effects = append(effects,
				NewEffect(EventSkip, use.PlayerID, ""))
		}
	})

	return effects
}

// hunterTrigger 目标若是猎人，产出一个把他拉进 phase 结算开枪的触发效果。
//
// 猎人可以死于狼刀、投票、另一名猎人的枪口，三条通道各自都要记得入队。
// 收在一处是为了让「又多了一条死亡通道」时只有一个地方需要改。
func hunterTrigger(view GameView, playerID string, phase PhaseType) []*Effect {
	if playerID == "" {
		return nil
	}
	target, ok := view.Player(playerID)
	if !ok || target.Role != RoleHunter {
		return nil
	}
	return []*Effect{NewAbilityTriggerEffect(playerID, phase)}
}

// potionKind 女巫的两种药
type potionKind int

const (
	potionAntidote potionKind = iota
	potionPoison
)

// witchHas 该女巫是否还持有指定的药。
// 非女巫一律返回 false——「谁有资格用药」是规则，由这里判定，
// 而不是由状态的写入点判定。
func witchHas(view GameView, playerID string, kind potionKind) bool {
	p, ok := view.Player(playerID)
	if !ok || p.Role != RoleWitch {
		return false
	}
	key := VarWitchAntidote
	if kind == potionPoison {
		key = VarWitchPoison
	}
	return view.PlayerVar(playerID, key) != ""
}
