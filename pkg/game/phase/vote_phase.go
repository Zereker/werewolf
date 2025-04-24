package phase

import (
	"sort"

	"github.com/Zereker/werewolf/pkg/game"
)

// VotePhase 投票阶段
type VotePhase struct {
	actions      []*game.Action
	deaths       []game.Player
	skillResults game.SkillResultMap
}

func NewVotePhase() *VotePhase {
	return &VotePhase{
		actions:      make([]*game.Action, 0),
		deaths:       make([]game.Player, 0),
		skillResults: make(game.SkillResultMap),
	}
}

func (v *VotePhase) GetName() game.PhaseType {
	return game.PhaseVote
}

func (v *VotePhase) Handle(action *game.Action) error {
	// 检查技能
	if err := action.Skill.Check(v.GetName(), action.Caster, action.Target); err != nil {
		return err
	}

	// 记录行为
	v.actions = append(v.actions, action)

	return nil
}

// GetPhaseResult 获取阶段结果
func (v *VotePhase) GetPhaseResult() *game.PhaseResult[game.SkillResultMap] {
	// 按优先级排序所有投票行为
	sort.Slice(v.actions, func(i, j int) bool {
		return v.actions[i].Skill.GetPriority() < v.actions[j].Skill.GetPriority()
	})

	// 执行所有投票并统计
	voteCount := make(map[game.Player]int)
	voteRecord := make(map[game.Player]game.Player)

	for _, action := range v.actions {
		// 执行技能
		action.Skill.Put(action.Caster, action.Target)

		// 记录投票
		voteCount[action.Target]++
		voteRecord[action.Caster] = action.Target
	}

	// 找出票数最多的玩家
	var maxVotes int
	var votedOut []game.Player
	for player, count := range voteCount {
		if count > maxVotes {
			maxVotes = count
			votedOut = []game.Player{player}
		} else if count == maxVotes {
			votedOut = append(votedOut, player)
		}
	}

	// 如果只有一个最高票，则该玩家死亡
	if len(votedOut) == 1 {
		player := votedOut[0]
		player.SetAlive(false)
		v.deaths = append(v.deaths, player)
	}

	// 记录投票结果
	v.skillResults[game.SkillTypeVote] = &game.SkillResult{
		Success: true,
		Message: "投票结果",
		Data: map[string]interface{}{
			"votes":     voteRecord, // 每个玩家的投票记录
			"voteCount": voteCount,  // 每个玩家获得的票数
			"votedOut":  votedOut,   // 被投出的玩家列表
		},
	}

	return &game.PhaseResult[game.SkillResultMap]{
		Deaths:    v.deaths,
		ExtraData: v.skillResults,
	}
}

func (v *VotePhase) Reset() {
	v.deaths = make([]game.Player, 0)
	v.skillResults = make(game.SkillResultMap)
	v.actions = make([]*game.Action, 0)
}
