package skill

import (
	"errors"

	"github.com/Zereker/werewolf/pkg/game"
)

// Protect represents guard's protect skill
type Protect struct {
	hasUsed bool // Whether skill has been used
}

// NewProtectSkill creates new protect skill
func NewProtectSkill() *Protect {
	return &Protect{
		hasUsed: false,
	}
}

// GetName returns skill name
func (p *Protect) GetName() string {
	return string(game.SkillTypeProtect)
}

// Put uses protect skill
func (p *Protect) Put(currentPhase game.Phase, caster game.Player, target game.Player) error {
	if currentPhase != game.PhaseNight {
		return errors.New("protect can only be used at night")
	}

	if p.hasUsed {
		return errors.New("protect has already been used")
	}

	if !target.IsAlive() {
		return errors.New("target is already dead")
	}

	if target.IsProtected() {
		return errors.New("target is already protected")
	}

	target.SetProtected(true)
	p.hasUsed = true
	return nil
}

// Reset resets skill state
func (p *Protect) Reset() {
	p.hasUsed = false
}
