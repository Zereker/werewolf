package skill

import (
	"fmt"

	"github.com/Zereker/werewolf/pkg/game"
)

// Vote 投票技能
type Vote struct {
	name     game.SkillType
	phase    game.PhaseType
	priority int
	hasUsed  bool
}

// NewVoteSkill creates new vote skill
func NewVoteSkill() *Vote {
	return &Vote{
		name:     game.SkillTypeVote,
		phase:    game.PhaseVote,
		priority: PriorityVote,
	}
}

// GetName returns skill name
func (v *Vote) GetName() game.SkillType {
	return v.name
}

// GetPhase returns skill phase
func (v *Vote) GetPhase() game.PhaseType {
	return v.phase
}

// Check checks the skill's conditions
func (v *Vote) Check(phase game.PhaseType, caster game.Player, target game.Player) error {
	if phase != v.phase {
		return fmt.Errorf("vote skill cannot be used in %s phase", phase)
	}

	if v.hasUsed {
		return fmt.Errorf("vote skill has already been used")
	}

	if !caster.IsAlive() {
		return fmt.Errorf("dead player cannot vote")
	}

	if target == nil {
		return fmt.Errorf("vote skill requires a target")
	}

	if !target.IsAlive() {
		return fmt.Errorf("cannot vote for dead target")
	}

	return nil
}

// Put uses vote skill
func (v *Vote) Put(caster game.Player, target game.Player, option game.PutOption) {
	v.hasUsed = true
}

// Reset resets skill state
func (v *Vote) Reset() {
	v.hasUsed = false
}

func (v *Vote) GetPriority() int {
	return v.priority
}
