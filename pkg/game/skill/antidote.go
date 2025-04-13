package skill

import (
	"errors"

	"github.com/Zereker/werewolf/pkg/game"
)

// Antidote represents witch's antidote skill
type Antidote struct {
	hasUsed bool // Whether skill has been used
}

// NewAntidoteSkill creates new antidote skill
func NewAntidoteSkill() *Antidote {
	return &Antidote{
		hasUsed: false,
	}
}

// GetName returns skill name
func (a *Antidote) GetName() string {
	return string(game.SkillTypeAntidote)
}

// Put uses antidote skill
func (a *Antidote) Put(currentPhase game.Phase, caster game.Player, target game.Player) error {
	if currentPhase != game.PhaseNight {
		return errors.New("antidote can only be used at night")
	}

	if a.hasUsed {
		return errors.New("antidote has already been used")
	}

	if !target.IsAlive() {
		return errors.New("target is already dead")
	}

	if target.IsProtected() {
		return errors.New("target is already protected")
	}

	target.SetProtected(true)
	a.hasUsed = true
	return nil
}

// Reset resets skill state
func (a *Antidote) Reset() {
	a.hasUsed = false
}
