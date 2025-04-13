package skill

import (
	"errors"

	"github.com/Zereker/werewolf/pkg/game"
)

// LastWords represents last words skill
type LastWords struct {
	hasUsed bool // Whether skill has been used
}

// NewLastWordsSkill creates new last words skill
func NewLastWordsSkill() *LastWords {
	return &LastWords{
		hasUsed: false,
	}
}

// GetName returns skill name
func (l *LastWords) GetName() string {
	return string(game.SkillTypeLastWords)
}

// Put uses last words skill
func (l *LastWords) Put(currentPhase game.Phase, caster game.Player, target game.Player) error {
	if !caster.IsAlive() {
		return errors.New("caster is dead")
	}

	if l.hasUsed {
		return errors.New("last words has already been used")
	}

	l.hasUsed = true
	return nil
}

// Reset resets skill state
func (l *LastWords) Reset() {
	l.hasUsed = false
}
