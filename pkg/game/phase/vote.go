package phase

import (
	"github.com/Zereker/werewolf/pkg/game"
)

// VotePhase represents voting phase in the game
type VotePhase struct {
	*BasePhase
}

// NewVotePhase creates a new vote phase
func NewVotePhase() *VotePhase {
	return &VotePhase{
		BasePhase: NewBasePhase(
			"vote",
			NewNightPhase(),
			[]game.SkillType{
				game.SkillTypeVote,
			},
		),
	}
}

// ValidateAction validates if action is legal in vote phase
func (p *VotePhase) ValidateAction(skill game.Skill) error {
	// Check if skill type is in available actions list
	for _, action := range p.GetAvailableActions() {
		if skill.GetName() == string(action) {
			return nil
		}
	}
	return game.ErrInvalidAction
}
