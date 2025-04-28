package phase

import (
	"sort"

	"github.com/Zereker/werewolf/pkg/game"
)

// VotePhase 投票阶段
type VotePhase struct {
	actions []*game.Action

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

func (v *VotePhase) Handle(action game.Action) error {
	// 检查技能
	if err := action.Skill.Check(v.GetName(), action.Caster, action.Target); err != nil {
		return err
	}

	// 记录行为
	v.actions = append(v.actions, action)

	return nil
}

func (v *VotePhase) IsCompleted() bool {
	return true
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

	// 执行投票并记录
	for _, action := range v.actions {
		// 执行技能
		action.Skill.Put(action.Caster, action.Target, game.PutOption{
			Content: action.Content,
		})

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

	// 只有在以下条件都满足时才处理出局：
	// 1. 有最高票的玩家
	// 2. 不是平票
	// 3. 总投票数超过最小阈值（这里设置为2票）
	const minVotesRequired = 2
	if len(votedOut) == 1 && maxVotes > minVotesRequired {
		player := votedOut[0]
		player.SetAlive(false)
		v.deaths = append(v.deaths, player)
	} else {
		// 如果条件不满足，清空出局列表
		votedOut = nil
	}

	// 记录投票结果
	v.skillResults[game.SkillTypeVote] = &game.SkillResult{
		Success: true,
		Message: "投票结果",
		Data: map[string]interface{}{
			"votes":     voteRecord, // 投票记录
			"voteCount": voteCount,  // 票数统计
			"votedOut":  votedOut,   // 被投出的玩家
		},
	}

	return &game.PhaseResult[game.SkillResultMap]{
		Deaths:    v.deaths,
		ExtraData: v.skillResults,
	}
}

func (v *VotePhase) Start() {
	v.deaths = make([]game.Player, 0)
	v.skillResults = make(game.SkillResultMap)
	v.actions = make([]*game.Action, 0)
}
