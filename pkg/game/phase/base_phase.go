package phase

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/event"
	// "github.com/Zereker/werewolf/pkg/game/skill" // No longer needed for _ import
	"log/slog" // For logging
)

// BasePhase 基础阶段结构体
type BasePhase struct {
	round   int
	players map[string]game.Player // Map playerID to game.Player
	actions []*game.Action         // Collected actions for the phase

	// skillResults game.SkillResultMap // This might be populated by phase-specific logic after processing actions.
	// For now, BasePhase won't directly manage skillResults on action adding.

	logger *slog.Logger
}

// NewBasePhase 创建基础阶段
func NewBasePhase(players []game.Player) *BasePhase {
	playerMap := make(map[string]game.Player)
	for _, p := range players {
		playerMap[p.GetID()] = p
	}

	return &BasePhase{
		players: playerMap,
		actions: make([]*game.Action, 0),
		// skillResults: make(game.SkillResultMap),
		logger: slog.Default().With("component", "BasePhase"),
	}
}

// HandleAction processes a player's action.
// This is the default implementation for BasePhase.
// It converts actionData (expected to be map[string]interface{} for skill use)
// into a game.Action and stores it.
func (p *BasePhase) HandleAction(actingPlayer game.Player, actionData interface{}, results chan<- error) error {
	p.logger.Debug("BasePhase HandleAction received", "playerID", actingPlayer.GetID(), "actionData", actionData)

	if !actingPlayer.IsAlive() {
		err := fmt.Errorf("player %s is not alive and cannot perform actions", actingPlayer.GetID())
		if results != nil {
			results <- err
			close(results)
		}
		return err
	}

	// Type assert actionData to map[string]interface{} which is what json.Unmarshal produces for objects.
	actionMap, ok := actionData.(map[string]interface{})
	if !ok {
		err := fmt.Errorf("unexpected actionData type: expected map[string]interface{}, got %T", actionData)
		if results != nil {
			results <- err
			close(results)
		}
		return err
	}

	skillTypeStr, _ := actionMap["skillType"].(string)
	targetID, _ := actionMap["targetID"].(string) // Optional
	content, _ := actionMap["content"].(string)   // Optional, for speak/last_words

	if skillTypeStr == "" {
		err := fmt.Errorf("actionData missing 'skillType'")
		if results != nil {
			results <- err
			close(results)
		}
		return err
	}

	skillType := game.SkillType(skillTypeStr)
	foundSkill := p.getPlayerSkill(actingPlayer, skillType)
	if foundSkill == nil {
		err := fmt.Errorf("player %s cannot use skill %s or skill not found", actingPlayer.GetID(), skillType)
		if results != nil {
			results <- err
			close(results)
		}
		return err
	}
	
	// Special handling for skills that embed content directly (like Speak)
	// This is a bit of a hack; ideally, skill data would be more structured.
	if speakSkill, ok := foundSkill.(*skill.Speak); ok {
		speakSkill.Content = content
	} else if lwSkill, ok := foundSkill.(*skill.LastWords); ok {
		lwSkill.Content = content
	}


	var targetPlayer game.Player
	if targetID != "" {
		var exists bool
		targetPlayer, exists = p.players[targetID]
		if !exists {
			err := fmt.Errorf("target player %s not found", targetID)
			if results != nil {
				results <- err
				close(results)
			}
			return err
		}
	}

	// Perform skill check (Is this the right place? Or should it be when actions are processed?)
	// For now, let's assume a basic check here. More complex checks might be phase-specific.
	if err := foundSkill.Check(p.GetName(), actingPlayer, targetPlayer); err != nil {
			p.logger.Warn("Skill check failed for player", "playerID", actingPlayer.GetID(), "skill", skillType, "error", err.Error())
			if results != nil {
				results <- err
				close(results)
			}
			return err
	}


	action := &game.Action{
		Caster: actingPlayer,
		Target: targetPlayer,
		Skill:  foundSkill,
	}

	p.AddAction(action)
	p.logger.Info("Player action added to phase queue", "playerID", actingPlayer.GetID(), "skill", skillType, "targetID", targetID)

	if results != nil {
		results <- nil // Indicate action successfully queued
		close(results)
	}
	return nil
}


// GetName needs to be implemented by concrete phases.
// This is a placeholder to satisfy the interface for BasePhase itself,
// though BasePhase shouldn't be directly instantiated as a phase.
func (p *BasePhase) GetName() game.PhaseType {
	return "base" // Should be overridden
}

// IsComplete checks if the phase has completed its action collection or conditions.
// For BasePhase, this is a placeholder and should be overridden by concrete phases.
// The runtime argument is cast to an interface to avoid direct dependency cycles with werewolf.Runtime.
// Specific phase implementations will cast `runtime` to `*werewolf.Runtime`.
func (p *BasePhase) IsComplete(runtimeWrapper interface{}) bool {
	// Base implementation, concrete phases should override.
	// Defaulting to false means phase won't complete unless specific logic is added.
	p.logger.Debug("BasePhase.IsComplete called, returning false by default", "phaseName", p.GetName())
	return false
}

func (p *BasePhase) SetRound(round int) {
	p.round = round
}

// GetRound 获取当前回合数
func (p *BasePhase) GetRound() int {
	return p.round
}

// GetPlayers 获取玩家映射
func (p *BasePhase) GetPlayers() map[string]game.Player {
	return p.players
}

// GetActions 获取行动列表
func (p *BasePhase) GetActions() []*game.Action {
	return p.actions
}

// GetSkillResults 获取技能结果
func (p *BasePhase) GetSkillResults() game.SkillResultMap {
	return p.skillResults
}

// AddAction 添加行动
func (p *BasePhase) AddAction(action *game.Action) {
	p.actions = append(p.actions, action)
}

// AddSkillResult 添加技能结果 - This might be deprecated if phases manage results more locally
// func (p *BasePhase) AddSkillResult(skillType game.SkillType, result *game.SkillResult) {
// 	p.skillResults[skillType] = result
// }

// getAlivePlayerIDs 获取所有存活的玩家ID
func (p *BasePhase) getAlivePlayerIDs() []string {
	ids := make([]string, 0)
	for id, player := range p.players {
		if player.IsAlive() {
			ids = append(ids, id)
		}
	}

	sort.Strings(ids)
	return ids
}

func (p *BasePhase) getAlivePlayerIDsByRole(roleType game.RoleType) []string {
	ids := make([]string, 0)
	for id, player := range p.players {
		if player.IsAlive() && player.GetRole().GetName() == roleType {
			ids = append(ids, id)
		}
	}

	sort.Strings(ids)
	return ids
}

// getAllPlayerIDs 获取所有玩家ID
func (p *BasePhase) getAllPlayerIDs() []string {
	ids := make([]string, 0, len(p.players))
	for id := range p.players {
		ids = append(ids, id)
	}

	sort.Strings(ids)
	return ids
}

// getSkillByType 获取指定类型的技能
// getSkillByType is unused now, getPlayerSkill is more specific
// func (p *BasePhase) getSkillByType(skillType game.SkillType) game.Skill { ... }

// getPlayerSkill finds a specific skill for a player.
func (p *BasePhase) getPlayerSkill(player game.Player, skillType game.SkillType) game.Skill {
	for _, s := range player.GetRole().GetAvailableSkills(p.GetName()) { // Check skills available in current phase
		if s.GetName() == skillType {
			return s
		}
	}
	return nil
}

// waitPlayer is removed as Player.Read() is removed.

// broadcastEvent sends an event to specified receivers or all players if receivers are nil.
// It now directly takes event.Event[any]
func (p *BasePhase) broadcastEvent(evt event.Event[any]) error {
	p.logger.Debug("Broadcasting event", "type", evt.Type, "playerID", evt.PlayerID, "receivers", evt.Receivers)
	receivers := evt.Receivers
	if len(receivers) == 0 { // If no specific receivers, broadcast to all.
		receivers = p.getAllPlayerIDs()
	}

	for _, receiverID := range receivers {
		if player, exists := p.players[receiverID]; exists {
			if player.IsAlive() || evt.Type == event.SystemGameEnd { // Dead players might only get game end messages
				// Create a new event for each player to avoid data races if Data is a pointer and modified later.
				// Though, for typical event data structs, this might not be an issue.
				// For safety or if specific per-player modifications were needed:
				// individualEvt := evt
				// individualEvt.Receivers = []string{receiverID} // Or keep original receivers for context
				if err := player.Write(evt); err != nil {
					p.logger.Error("Failed to write event to player", "playerID", receiverID, "error", err)
					// Decide if we continue broadcasting or return error. For now, continue.
				}
			}
		} else {
			p.logger.Warn("Player not found for broadcasting event", "receiverID", receiverID)
		}
	}
	return nil
}

// broadcastPhaseStart 广播阶段开始
// Note: The original broadcastEvent had a type switch for event.Event[event.PhaseStartData] etc.
// Now we directly pass event.Event[any].
func (p *BasePhase) broadcastPhaseStart(phase game.PhaseType, message string) error {
	return p.broadcastEvent(event.Event[event.PhaseStartData]{
		ID:        uuid.NewString(),
		Type:      event.SystemPhaseStart,
		PlayerID:  game.SystemPlayerID,
		Receivers: p.getAllPlayerIDs(),
		Timestamp: time.Now(),
		Data: event.PhaseStartData{
			Phase:   string(phase),
			Round:   p.round,
			Message: message,
		},
	})
}

// broadcastPhaseEnd 广播阶段结束
func (p *BasePhase) broadcastPhaseEnd(phase game.PhaseType, message string) error {
	return p.broadcastEvent(event.Event[event.PhaseStartData]{
		Type: event.SystemPhaseEnd,
		Data: event.PhaseStartData{
			Phase:   string(phase),
			Round:   p.round,
			Message: message,
		},
		Receivers: p.getAllPlayerIDs(),
		Timestamp: time.Now(),
	})
}

// broadcastSkillResult 广播技能结果
func (p *BasePhase) broadcastSkillResult(skillType game.SkillType, message string, players ...string) error {
	return p.broadcastEvent(event.Event[event.SkillResultData]{
		Type: event.SystemSkillResult,
		Data: event.SkillResultData{
			SkillType: string(skillType),
			Message:   message,
		},
		Receivers: players,
		Timestamp: time.Now(),
	})
}

// convertActionToSkillEvent is removed as actions are now directly created in HandleAction.
// convertEventToAction is removed; its logic is adapted into HandleAction.
