package phase

import (
	"errors"

	"github.com/Zereker/werewolf/pkg/game"
)

var (
	// ErrInvalidAction is returned when action is invalid
	ErrInvalidAction = errors.New("invalid action")
)

// Phase interface defines game phase behavior
type Phase interface {
	// GetName returns phase name
	GetName() string
	// GetNextPhase returns next phase
	GetNextPhase() Phase
	// ValidateAction validates if action is legal
	ValidateAction(skill game.Skill) error
}

// phase implements basic phase functionality
type phase struct {
	name             string
	nextPhase        Phase
	availableActions []game.Skill
}

// NewPhase creates a new base phase
func NewPhase(name string, actions []game.Skill, nextPhase Phase) Phase {
	return &phase{
		name:             name,
		nextPhase:        nextPhase,
		availableActions: actions,
	}
}

// GetName returns phase name
func (p *phase) GetName() string {
	return p.name
}

// GetNextPhase returns next phase
func (p *phase) GetNextPhase() Phase {
	return p.nextPhase
}

func (p *phase) ValidateAction(skill game.Skill) error {
	for _, action := range p.availableActions {
		if skill.GetName() == action.GetName() {
			return nil
		}
	}

	return ErrInvalidAction
}
