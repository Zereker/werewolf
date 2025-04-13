package skill

import (
	"errors"

	"github.com/Zereker/werewolf/pkg/game"
)

// Vote represents vote skill
type Vote struct {
	hasUsed bool // Whether skill has been used
}

// NewVoteSkill creates new vote skill
func NewVoteSkill() *Vote {
	return &Vote{
		hasUsed: false,
	}
}

// GetName returns skill name
func (v *Vote) GetName() string {
	return string(game.SkillTypeVote)
}

// Put uses vote skill
func (v *Vote) Put(currentPhase game.Phase, caster game.Player, target game.Player) error {
	if currentPhase != game.PhaseVote {
		return errors.New("vote can only be used during vote phase")
	}

	if !caster.IsAlive() {
		return errors.New("caster is dead")
	}

	if !target.IsAlive() {
		return errors.New("target is dead")
	}

	if v.hasUsed {
		return errors.New("vote has already been used")
	}

	target.AddVote()
	v.hasUsed = true
	return nil
}

// Reset resets skill state
func (v *Vote) Reset() {
	v.hasUsed = false
}
