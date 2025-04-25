package skill

import (
	"fmt"

	"github.com/Zereker/werewolf/pkg/game"
)

// Hunter 猎人技能
type Hunter struct {
	name     game.SkillType
	phase    game.PhaseType
	priority int
	hasUsed  bool
}

func NewHunterSkill() *Hunter {
	return &Hunter{
		name:     game.SkillTypeHunter,
		phase:    game.PhaseNight,
		priority: PriorityHunter,
	}
}

func (h *Hunter) GetName() game.SkillType {
	return h.name
}

func (h *Hunter) GetPhase() game.PhaseType {
	return h.phase
}

func (h *Hunter) GetPriority() int {
	return h.priority
}

func (h *Hunter) Check(phase game.PhaseType, caster game.Player, target game.Player) error {
	if phase != h.phase {
		return fmt.Errorf("hunter skill cannot be used in %s phase", phase)
	}

	if h.hasUsed {
		return fmt.Errorf("hunter skill has already been used")
	}

	if !caster.IsAlive() {
		return fmt.Errorf("dead hunter cannot use skill")
	}

	if target == nil {
		return fmt.Errorf("hunter skill requires a target")
	}

	if !target.IsAlive() {
		return fmt.Errorf("cannot shoot dead target")
	}

	if target.IsProtected() {
		return fmt.Errorf("target is protected")
	}

	return nil
}

func (h *Hunter) Put(caster game.Player, target game.Player, option game.PutOption) {
	h.hasUsed = true
	target.SetAlive(false)
}

func (h *Hunter) Reset() {
	h.hasUsed = false
}
