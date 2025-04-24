package skill

import (
	"fmt"

	"github.com/Zereker/werewolf/pkg/game"
)

// Speak 发言技能
type Speak struct {
	name     game.SkillType
	phase    game.PhaseType
	priority int
	hasUsed  bool
}

// NewSpeakSkill creates new speak skill
func NewSpeakSkill() *Speak {
	return &Speak{
		name:     game.SkillTypeSpeak,
		phase:    game.PhaseDay,
		priority: PrioritySpeak,
	}
}

// GetName returns skill name
func (s *Speak) GetName() game.SkillType {
	return s.name
}

// GetPhase returns skill phase
func (s *Speak) GetPhase() game.PhaseType {
	return s.phase
}

// Check checks the skill's conditions
func (s *Speak) Check(phase game.PhaseType, caster game.Player, target game.Player) error {
	if phase != s.phase {
		return fmt.Errorf("speak skill cannot be used in %s phase", phase)
	}

	if s.hasUsed {
		return fmt.Errorf("speak skill has already been used")
	}

	if !caster.IsAlive() {
		return fmt.Errorf("dead player cannot speak")
	}

	if target != nil {
		return fmt.Errorf("speak skill does not require a target")
	}

	return nil
}

// Put uses speak skill
func (s *Speak) Put(caster game.Player, target game.Player) {
	s.hasUsed = true
}

// Reset resets skill state
func (s *Speak) Reset() {
	s.hasUsed = false
}

func (s *Speak) GetPriority() int {
	return s.priority
}
