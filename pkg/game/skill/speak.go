package skill

import (
	"errors"

	"github.com/Zereker/werewolf/pkg/game"
)

// Speak represents speak skill
type Speak struct {
	hasUsed bool // Whether skill has been used
}

// NewSpeakSkill creates new speak skill
func NewSpeakSkill() *Speak {
	return &Speak{
		hasUsed: false,
	}
}

// GetName returns skill name
func (s *Speak) GetName() string {
	return string(game.SkillTypeSpeak)
}

// Put uses speak skill
func (s *Speak) Put(currentPhase game.Phase, caster game.Player, target game.Player) error {
	if currentPhase != game.PhaseDay {
		return errors.New("speak can only be used during day")
	}

	if !caster.IsAlive() {
		return errors.New("caster is dead")
	}

	if s.hasUsed {
		return errors.New("speak has already been used")
	}

	s.hasUsed = true
	return nil
}

// Reset resets skill state
func (s *Speak) Reset() {
	s.hasUsed = false
}
