package skill

import (
	"errors"

	"github.com/Zereker/werewolf/pkg/game"
)

// Kill represents werewolf's kill skill
type Kill struct {
	hasUsed bool // Whether skill has been used
}

// NewKillSkill creates new kill skill
func NewKillSkill() *Kill {
	return &Kill{
		hasUsed: false,
	}
}

// GetName returns skill name
func (k *Kill) GetName() string {
	return string(game.SkillTypeKill)
}

// Put uses kill skill
func (k *Kill) Put(currentPhase game.Phase, caster game.Player, target game.Player) error {
	if currentPhase != game.PhaseNight {
		return errors.New("kill can only be used at night")
	}

	if k.hasUsed {
		return errors.New("kill has already been used")
	}

	if !target.IsAlive() {
		return errors.New("target is already dead")
	}

	if target.IsProtected() {
		return errors.New("target is protected")
	}

	target.SetAlive(false)
	k.hasUsed = true
	return nil
}

// Reset resets skill state
func (k *Kill) Reset() {
	k.hasUsed = false
}
