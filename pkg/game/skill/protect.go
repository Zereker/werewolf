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
func (p *Protect) Put(currentPhase game.PhaseType, caster game.Player, target game.Player) error {
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

// UseInPhase 技能使用阶段
func (p *Protect) UseInPhase() game.PhaseType {
	return game.PhaseNight
}

// IsUsed 技能是否已使用
func (p *Protect) IsUsed() bool {
	return p.hasUsed
}
