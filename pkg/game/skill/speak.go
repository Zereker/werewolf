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
	content  string
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

// Check 检查技能是否可以使用
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

// Put 使用技能
func (s *Speak) Put(caster game.Player, target game.Player, option game.PutOption) {
	s.hasUsed = true
	s.content = option.Content
}

// Exec 执行技能，包含检查和执行两个步骤
func (s *Speak) Exec(phase game.PhaseType, caster game.Player, target game.Player, option game.PutOption) error {
	if err := s.Check(phase, caster, target); err != nil {
		return err
	}
	s.Put(caster, target, option)
	return nil
}

func (s *Speak) GetPriority() int {
	return s.priority
}

// GetContent 获取发言内容
func (s *Speak) GetContent() string {
	return s.content
}

func (s *Speak) Reset() {
	s.hasUsed = false
}
