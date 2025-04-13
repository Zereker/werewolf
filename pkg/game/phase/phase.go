package phase

import (
	"github.com/Zereker/werewolf/pkg/game"
)

// Phase interface defines game phase behavior
type Phase interface {
	// GetName returns phase name
	GetName() string
	// GetNextPhase returns next phase
	GetNextPhase() Phase
	// GetAvailableActions returns available actions
	GetAvailableActions() []game.SkillType
	// ValidateAction validates if action is legal
	ValidateAction(skill game.Skill) error
}

// BasePhase implements basic phase functionality
type BasePhase struct {
	name             string
	nextPhase        Phase
	availableActions []game.SkillType
}

// NewBasePhase creates a new base phase
func NewBasePhase(name string, nextPhase Phase, actions []game.SkillType) *BasePhase {
	return &BasePhase{
		name:             name,
		nextPhase:        nextPhase,
		availableActions: actions,
	}
}

// GetName returns phase name
func (p *BasePhase) GetName() string {
	return p.name
}

// GetNextPhase returns next phase
func (p *BasePhase) GetNextPhase() Phase {
	return p.nextPhase
}

// GetAvailableActions returns available actions
func (p *BasePhase) GetAvailableActions() []game.SkillType {
	return p.availableActions
}

// ValidateAction validates if action is legal
func (p *BasePhase) ValidateAction(skill game.Skill) error {
	for _, action := range p.availableActions {
		if skill.GetName() == string(action) {
			return nil
		}
	}

	return game.ErrInvalidAction
}
