package phase

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time" // Ensure time is imported if used by any event creation

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/event"
	"github.com/Zereker/werewolf/pkg/game/skill" // Required for skill type constants if used
)

// NightPhase 夜晚阶段
type NightPhase struct {
	*BasePhase
	// deaths is already part of BasePhase if we choose to store results there,
	// but NightPhase often has specific death processing.
	// For now, keep local deaths to be filled by CalculatePhaseResult.
	deaths []game.Player
	// logger is already part of BasePhase
}

func NewNightPhase(players []game.Player) *NightPhase {
	return &NightPhase{
		BasePhase: NewBasePhase(players), // This initializes its own logger
		deaths:    make([]game.Player, 0),
		// logger field is inherited from BasePhase, no need to re-declare unless shadowing with different tags
	}
}

func (p *NightPhase) GetName() game.PhaseType {
	return game.PhaseNight
}

// IsComplete checks if the NightPhase is complete.
// This version assumes completion when all roles expected to act have done so.
// (e.g., Werewolves, Seer, Guard, Witch if they have usable skills).
// This is a simplified heuristic; a real game might use timeouts or more complex conditions.
func (p *NightPhase) IsComplete(runtimeWrapper interface{}) bool {
	// Extract player list from runtimeWrapper if needed, or use p.players
	// For simplicity, we'll use p.players which was set at phase creation.
	// Ensure this list is up-to-date if players can disconnect mid-phase.
	// The runtime currently updates player.IsAlive upon disconnection.

	actedPlayerIDs := make(map[string]bool)
	for _, action := range p.actions {
		actedPlayerIDs[action.Caster.GetID()] = true
	}

	// Determine who is expected to act
	for _, player := range p.players {
		if !player.IsAlive() {
			continue
		}

		roleType := player.GetRole().GetName()
		hasActed := actedPlayerIDs[player.GetID()]

		// Check roles that *must* or *typically* act at night
		switch roleType {
		case game.RoleTypeWerewolf:
			// Simplification: if any werewolf hasn't acted, phase isn't complete.
			// TODO: More complex: allow one werewolf to act for the pack.
			if !hasActed {
				// p.logger.Debug("NightPhase not complete: werewolf has not acted", "playerID", player.GetID())
				return false
			}
		case game.RoleTypeSeer:
			if !hasActed {
				// p.logger.Debug("NightPhase not complete: seer has not acted", "playerID", player.GetID())
				return false
			}
		case game.RoleTypeGuard:
			// Guard is optional. If they have skill.Protect and haven't used it, they *could* act.
			// For simplicity, if a Guard *can* act (has skill, hasn't acted), we wait.
			// This needs refinement: Guard might choose not to act.
			// A "skip" action or timeout is better.
			// For now: if they have the Protect skill and haven't acted, assume we wait.
			guardSkill := p.getPlayerSkill(player, game.SkillTypeProtect)
			if guardSkill != nil { // Guard has the skill
				// Type assert to check if skill is usable (e.g. not on cooldown for same target - advanced)
				// For now, just check if they've submitted *any* action this phase.
				if !hasActed {
					// p.logger.Debug("NightPhase not complete: guard has not acted", "playerID", player.GetID())
					return false
				}
			}
		case game.RoleTypeWitch:
			// Witch is optional. Check if they have usable potions.
			hasAntidote := false
			antidoteUsed := true // Assume used if not found or skill says so
			if as := p.getPlayerSkill(player, game.SkillTypeAntidote); as != nil {
				if ant, ok := as.(*skill.Antidote); ok {
					hasAntidote = true
					antidoteUsed = ant.HasUsed()
				}
			}
			hasPoison := false
			poisonUsed := true
			if ps := p.getPlayerSkill(player, game.SkillTypePoison); ps != nil {
				if pzn, ok := ps.(*skill.Poison); ok {
					hasPoison = true
					poisonUsed = pzn.HasUsed()
				}
			}
			if (hasAntidote && !antidoteUsed) || (hasPoison && !poisonUsed) {
				if !hasActed { // If they have usable potions and haven't submitted any action
					// p.logger.Debug("NightPhase not complete: witch has usable potions and has not acted", "playerID", player.GetID())
					return false
				}
			}
		}
	}

	p.logger.Info("NightPhase considered complete: all expected actors seem to have acted or cannot act.")
	return true
}

// Start resets actions, deaths, and player protections for the new night.
func (p *NightPhase) Start(ctx context.Context) error {
	p.BasePhase.actions = make([]*game.Action, 0) // Clear actions from previous phase/round
	p.deaths = make([]game.Player, 0)             // Clear deaths from previous night
	
	// Reset protection status for all players at the start of the night
	for _, player := range p.players {
		player.SetProtected(false)
	}
	// Reset per-night skills for all alive players
	for _, player := range p.players {
		if player.IsAlive() {
			for _, sk := range player.GetRole().GetAvailableSkills(game.PhaseNight) {
				if resettableSkill, ok := sk.(interface{ Reset() }); ok {
					// Check if it's not a per-game skill like Witch potions
					isWitchPotion := sk.GetName() == game.SkillTypeAntidote || sk.GetName() == game.SkillTypePoison
					if !isWitchPotion { // Only reset if not a witch potion
						resettableSkill.Reset()
						p.logger.Debug("Reset night skill for player", "playerID", player.GetID(), "skill", sk.GetName())
					}
				}
			}
		}
	}


	p.logger.Info("NightPhase starting", "round", p.round)
	if err := p.broadcastPhaseStart(game.PhaseNight, "天黑了，请行动。"); err != nil {
		return fmt.Errorf("notify phase start: %w", err)
	}
	return nil
}

// HandleAction delegates to BasePhase.HandleAction for now.
// Specific night phase action validation (e.g., only werewolves can use kill)
// is handled by BasePhase.HandleAction's skill check.
func (p *NightPhase) HandleAction(actingPlayer game.Player, actionData interface{}, results chan<- error) error {
	return p.BasePhase.HandleAction(actingPlayer, actionData, results)
}


// NotifyPhaseEnd broadcasts the results of the night (deaths).
func (p *NightPhase) NotifyPhaseEnd() error {
	deathNames := make([]string, 0, len(p.deaths))
	for _, player := range p.deaths {
		deathNames = append(deathNames, player.GetID())
	}

	message := "天亮了，所有玩家请睁眼。"
	if len(deathNames) > 0 {
		message = fmt.Sprintf("昨晚死亡的玩家是：%s。", strings.Join(deathNames, "、"))
	} else {
		message = "昨晚是一个平安夜。"
	}

	return p.broadcastPhaseEnd(game.PhaseNight, message)
}

// CalculatePhaseResult processes all collected night actions and determines outcomes.
func (p *NightPhase) CalculatePhaseResult() *game.PhaseResult[game.UserSkillResultMap] {
	p.logger.Info("Calculating NightPhase results", "round", p.round, "action_count", len(p.actions))
	p.deaths = make([]game.Player, 0) // Reset deaths for this calculation pass

	sort.SliceStable(p.actions, func(i, j int) bool {
		return p.actions[i].Skill.GetPriority() < p.actions[j].Skill.GetPriority()
	})

	skillExecutionResults := make(game.UserSkillResultMap)
	werewolfTargets := make(map[string]game.Player) // playerID -> player object

	// Pass 1: Protections and information gathering
	for _, action := range p.actions {
		if !action.Caster.IsAlive() { continue }
		
		var res game.SkillResult
		switch action.Skill.GetName() {
		case game.SkillTypeProtect:
			action.Skill.Put(action.Caster, action.Target, &res)
			p.logger.Info("Guard protected", "caster", action.Caster.GetID(), "target", action.Target.GetID())
		case game.SkillTypeCheck:
			action.Skill.Put(action.Caster, action.Target, &res) // Seer receives result via player.Write
			p.logger.Info("Seer checked", "caster", action.Caster.GetID(), "target", action.Target.GetID())
		}
		if res.Message != "" { skillExecutionResults[action.Caster] = &res }
	}

	// Pass 2: Identify werewolf targets
	for _, action := range p.actions {
		if !action.Caster.IsAlive() { continue }
		if action.Skill.GetName() == game.SkillTypeKill && action.Target != nil && action.Target.IsAlive() {
			if !action.Target.IsProtected() {
				werewolfTargets[action.Target.GetID()] = action.Target
				p.logger.Info("Werewolf targeted", "caster", action.Caster.GetID(), "target", action.Target.GetID())
			} else {
				p.logger.Info("Werewolf attack on protected player", "caster", action.Caster.GetID(), "target", action.Target.GetID())
				// Notify werewolves their target was protected (optional game rule)
				// action.Caster.Write(...)
			}
		}
	}

	// Pass 3: Witch actions (Antidote on werewolf targets, Poison)
	for _, action := range p.actions {
		if !action.Caster.IsAlive() { continue }
		if action.Caster.GetRole().GetName() != game.RoleTypeWitch { continue }

		var res game.SkillResult
		switch action.Skill.GetName() {
		case game.SkillTypeAntidote:
			if action.Target != nil && action.Target.IsAlive() { // Witch must target someone alive
				// Check if target was a werewolf target
				if _, targetedByWW := werewolfTargets[action.Target.GetID()]; targetedByWW {
					action.Skill.Put(action.Caster, action.Target, &res) // Sets target.Alive = true
					delete(werewolfTargets, action.Target.GetID()) // Saved!
					res.Message = fmt.Sprintf("女巫 %s 使用了解药，救活了 %s。", action.Caster.GetID(), action.Target.GetID())
					p.logger.Info("Witch used Antidote successfully on werewolf target", "witch", action.Caster.GetID(), "target", action.Target.GetID())
				} else {
					// Witch used antidote on someone not targeted by werewolves (or already protected)
					// Standard rule: potion is consumed.
					action.Skill.Put(action.Caster, action.Target, &res) // Still consumes potion
					res.Message = fmt.Sprintf("女巫 %s 对 %s 使用了解药，但该玩家未被狼人攻击或已被守护。", action.Caster.GetID(), action.Target.GetID())
					p.logger.Info("Witch used Antidote, but target was not a (successful) werewolf target", "witch", action.Caster.GetID(), "target", action.Target.GetID())
				}
			}
		case game.SkillTypePoison:
			if action.Target != nil && action.Target.IsAlive() && !action.Target.IsProtected() {
				// If target was saved by antidote by another witch (if multiple witches), this could be complex.
				// Assuming one witch or antidote has priority if witch targets same player with both.
				// For now, poison adds to death list if target is not protected.
				action.Skill.Put(action.Caster, action.Target, &res) // Sets target.Alive = false
				if !action.Target.IsAlive() { // Check if poison was successful
					p.deaths = append(p.deaths, action.Target) // Add to deaths if not already dead from WW
					p.logger.Info("Witch poisoned player", "witch", action.Caster.GetID(), "target", action.Target.GetID())
				}
			} else if action.Target != nil && action.Target.IsProtected() {
                 res.Success = false
                 res.Message = fmt.Sprintf("女巫 %s 试图毒杀 %s, 但目标被守护。", action.Caster.GetID(), action.Target.GetID())
				p.logger.Info("Witch poison on protected player", "witch", action.Caster.GetID(), "target", action.Target.GetID())
			}
		}
		if res.Message != "" { skillExecutionResults[action.Caster] = &res }
	}

	// Apply werewolf deaths
	for _, targetPlayer := range werewolfTargets {
		targetPlayer.SetAlive(false)
		p.deaths = append(p.deaths, targetPlayer)
		p.logger.Info("Player died from werewolf attack", "playerID", targetPlayer.GetID())
	}
	
	// Consolidate unique deaths
	uniqueDeaths := make(map[string]game.Player)
	for _, deadPlayer := range p.deaths {
		uniqueDeaths[deadPlayer.GetID()] = deadPlayer
	}
	p.deaths = make([]game.Player, 0, len(uniqueDeaths))
	for _, deadPlayer := range uniqueDeaths {
		p.deaths = append(p.deaths, deadPlayer)
	}


	// Per-night skills were already reset in Start() for this new night.
	// Skills that are single-use-per-game (Witch potions, Hunter shot) manage their 'hasUsed' internally.

	return &game.PhaseResult[game.UserSkillResultMap]{
		Deaths:    p.deaths,
		ExtraData: skillExecutionResults,
	}
}
