package skill

import (
	"errors"

	"github.com/Zereker/werewolf/pkg/game"
)

// Poison represents witch's poison skill
type Poison struct {
	hasUsed bool // Whether skill has been used
}

// NewPoisonSkill creates new poison skill
func NewPoisonSkill() *Poison {
	return &Poison{
		hasUsed: false,
	}
}

// GetName returns skill name
func (p *Poison) GetName() string {
	return string(game.SkillTypePoison)
}

// Put uses poison skill
func (p *Poison) Put(currentPhase game.Phase, caster game.Player, target game.Player) error {
	if currentPhase != game.PhaseNight {
		return errors.New("poison can only be used at night")
	}

	if p.hasUsed {
		return errors.New("poison has already been used")
	}

	if !target.IsAlive() {
		return errors.New("target is already dead")
	}

	if target.IsProtected() {
		return errors.New("target is protected")
	}

	target.SetAlive(false)
	p.hasUsed = true
	return nil
}

// Reset resets skill state
func (p *Poison) Reset() {
	p.hasUsed = false
}
