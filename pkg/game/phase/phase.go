package phase

import (
	"errors"

	"github.com/Zereker/werewolf/pkg/game"
)

var (
	// ErrInvalidAction is returned when action is invalid
	ErrInvalidAction = errors.New("invalid action")
)

// phase implements basic phase functionality
type phase struct {
	name             string       // 阶段名称
	availableActions []game.Skill // 可用技能列表
	next             game.Phase   // 下一个阶段
}

// NewPhase creates a new base phase
func NewPhase(name string, actions []game.Skill) game.Phase {
	return &phase{
		name:             name,
		availableActions: actions,
	}
}

// GetName returns phase name
func (p *phase) GetName() game.PhaseType {
	return game.PhaseType(p.name)
}

// GetNextPhase returns next phase
func (p *phase) GetNextPhase() game.Phase {
	return p.next
}

// SetNextPhase sets next phase
func (p *phase) SetNextPhase(next game.Phase) {
	p.next = next
}

// ValidateAction validates if action is legal
func (p *phase) ValidateAction(skill game.Skill) error {
	for _, action := range p.availableActions {
		if skill.GetName() == action.GetName() {
			return nil
		}
	}

	return ErrInvalidAction
}
