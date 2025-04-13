package skill

import (
	"errors"

	"github.com/Zereker/werewolf/pkg/game"
)

// Vote 投票技能
type Vote struct {
	hasVoted map[game.Player]bool // 记录每个玩家是否已经投票
}

// NewVoteSkill 创建投票技能
func NewVoteSkill() *Vote {
	return &Vote{
		hasVoted: make(map[game.Player]bool),
	}
}

// GetName 获取技能名称
func (v *Vote) GetName() string {
	return string(game.SkillTypeVote)
}

// Put 使用投票技能
func (v *Vote) Put(currentPhase game.Phase, caster game.Player, target game.Player) error {
	if currentPhase != game.PhaseVote {
		return errors.New("只能在投票阶段投票")
	}

	if !caster.IsAlive() {
		return errors.New("死亡玩家不能投票")
	}

	if v.hasVoted[caster] {
		return errors.New("本轮已经投过票了")
	}

	if !target.IsAlive() {
		return errors.New("不能投票给已死亡的玩家")
	}

	v.hasVoted[caster] = true
	return nil
}

// Reset 重置技能状态
func (v *Vote) Reset() {
	v.hasVoted = make(map[game.Player]bool)
}
