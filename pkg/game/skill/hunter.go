package skill

import (
	"fmt"

	"github.com/Zereker/werewolf/pkg/game"
)

// Hunter 猎人技能 - This skill is triggered when the Hunter dies.
type Hunter struct {
	name     game.SkillType
	phase    game.PhaseType // Less relevant for a passive on-death skill, might be game.PhaseNone
	priority int            // Priority of the shot if multiple on-death effects occur
	hasUsed  bool           // Has the Hunter used their shot? (Once per game)
}

func NewHunterSkill() *Hunter {
	return &Hunter{
		name:     game.SkillTypeHunter,
		phase:    game.PhaseNone, // Indicates it's not tied to a standard phase action turn
		priority: PriorityHunter, // Still relevant for ordering if multiple deaths/effects
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

// Check determines if the Hunter can use their ability.
// This is called by the runtime when the Hunter dies.
// Caster is the Hunter. Target is who they intend to shoot.
func (h *Hunter) Check(phase game.PhaseType, caster game.Player, target game.Player) error {
	// Note: 'phase' is less relevant here. The trigger is Hunter's death.
	if h.hasUsed {
		return fmt.Errorf("hunter skill has already been used")
	}

	// Caster (Hunter) is confirmed dead by the runtime before this is called.
	// So, caster.IsAlive() would be false. This check is more about skill state.

	if target == nil {
		return fmt.Errorf("hunter skill requires a target to shoot")
	}

	if !target.IsAlive() {
		return fmt.Errorf("cannot shoot an already dead target")
	}
	
	// Hunter cannot shoot themselves (usually a game rule)
	if target.GetID() == caster.GetID() {
		return fmt.Errorf("hunter cannot target themselves")
	}

	// Hunter's shot usually cannot be protected against (game rule dependent)
	// if target.IsProtected() {
	// 	return fmt.Errorf("target is protected (Hunter's shot might ignore this depending on rules)")
	// }

	return nil
}

// Put executes the Hunter's shot.
// Caster is the Hunter (who is dead). Target is the player to be shot.
func (h *Hunter) Put(caster game.Player, target game.Player, result *game.SkillResult) {
	if h.hasUsed {
		result.Success = false
		result.Message = "Hunter skill already used."
		return
	}
	if target == nil || !target.IsAlive() { // Double check target validity
		result.Success = false
		result.Message = "Invalid target for Hunter's shot."
		return
	}

	h.hasUsed = true
	target.SetAlive(false) // The target is shot

	result.Success = true
	result.Message = fmt.Sprintf("玩家 %s 被猎人 %s 临死前射杀", target.GetID(), caster.GetID())
}

// Reset for Hunter skill means nothing, as it's a one-time, per-game ability.
// The hasUsed flag is permanent once set.
func (h *Hunter) Reset() {
	// Hunter skill is once per game, so Reset does nothing to hasUsed.
	// If there were other per-round states for Hunter (unlikely), they'd reset here.
}
