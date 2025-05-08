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

// Check 检查技能是否可以使用
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

// Put 使用技能
func (h *Hunter) Put(caster game.Player, target game.Player, result *game.SkillResult) {
	h.hasUsed = true
	target.SetAlive(false)

	result.Success = true
	result.Message = fmt.Sprintf("玩家 %s 被猎人射杀", target.GetID())
}

func (h *Hunter) Reset() {
	h.hasUsed = false
}
