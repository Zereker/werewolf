package phase

import (
	"context"
	"fmt"
	"time"

	"log/slog" // For logging

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/event"
	"github.com/Zereker/werewolf/pkg/game/skill" // For skill.LastWords
)

// LastWordsPhase 遗言阶段
type LastWordsPhase struct {
	round        int
	players      map[string]game.Player // All players, to find alive ones for broadcasting
	deadPlayers  []game.Player          // Players who should give last words
	actions      []*game.Action         // Store last words actions
	logger       *slog.Logger
	currentPlayerIdx int // Index for current player giving last words
}

// NewLastWordsPhase creates a new LastWordsPhase.
// players: all players in the game.
// deaths: players who died and should give last words.
func NewLastWordsPhase(round int, players []game.Player, deaths []game.Player) *LastWordsPhase {
	playerMap := make(map[string]game.Player)
	for _, pl := range players { // Use different variable name to avoid conflict
		playerMap[pl.GetID()] = pl
	}

	return &LastWordsPhase{
		round:        round,
		players:      playerMap,
		deadPlayers:  deaths,
		actions:      make([]*game.Action, 0),
		logger:       slog.Default().With("component", "LastWordsPhase"),
		currentPlayerIdx: 0,
	}
}

func (p *LastWordsPhase) GetName() game.PhaseType {
	return "last_words"
}

func (p *LastWordsPhase) GetRound() int {
	return p.round
}

// Start prepares the LastWordsPhase.
// It announces the phase and which player should speak first (if any).
func (p *LastWordsPhase) Start(ctx context.Context) error {
	p.logger.Info("LastWordsPhase starting", "round", p.round, "dead_player_count", len(p.deadPlayers))
	p.actions = make([]*game.Action, 0)
	p.currentPlayerIdx = 0

	// The rule "if p.round != 1" is removed. Last words can happen any round.
	// This phase should only be initiated by the runtime if there are dead players eligible for last words.
	if len(p.deadPlayers) == 0 {
		p.logger.Info("No dead players to give last words.")
		// This phase effectively ends immediately if there are no speakers.
		// Runtime should handle this by not spending time in this phase or by checking a status.
		return nil
	}

	// Notify all alive players that last words phase has started.
	// The actual content of last words will be broadcast as they are received.
	if err := p.broadcastToAlivePlayers(event.Event[any]{
		Type: event.SystemPhaseStart,
		Data: event.PhaseStartData{
			Phase:   string(p.GetName()),
			Round:   p.round,
			Message: fmt.Sprintf("遗言阶段开始。等待 %s 发言。", p.deadPlayers[p.currentPlayerIdx].GetID()),
		},
		Timestamp: time.Now(),
		PlayerID:  game.SystemPlayerID, // System event
	}); err != nil {
		return fmt.Errorf("broadcast last words phase start failed: %w", err)
	}
	
	// Notify the first dead player that it's their turn.
	firstSpeaker := p.deadPlayers[p.currentPlayerIdx]
	if err := p.notifyPlayerTurn(firstSpeaker); err != nil {
		return fmt.Errorf("failed to notify first speaker %s: %w", firstSpeaker.GetID(), err)
	}

	return nil
}

func (p *LastWordsPhase) notifyPlayerTurn(player game.Player) error {
	return player.Write(event.Event[any]{
		Type:      event.SystemSkillResult, // Re-using for prompting
		PlayerID:  game.SystemPlayerID,
		Receivers: []string{player.GetID()},
		Timestamp: time.Now(),
		Data: event.SkillResultData{
			SkillType: string(game.SkillTypeLastWords),
			Message:   fmt.Sprintf("请你发表遗言。"),
		},
	})
}


// HandleAction processes a player's "last_words" action.
func (p *LastWordsPhase) HandleAction(actingPlayer game.Player, actionData interface{}, results chan<- error) error {
	p.logger.Debug("LastWordsPhase HandleAction received", "playerID", actingPlayer.GetID(), "actionData", actionData)

	// Check if the acting player is the one whose turn it is.
	if p.currentPlayerIdx >= len(p.deadPlayers) || actingPlayer.GetID() != p.deadPlayers[p.currentPlayerIdx].GetID() {
		err := fmt.Errorf("it is not player %s's turn to give last words or all last words given", actingPlayer.GetID())
		if results != nil {
			results <- err
			close(results)
		}
		return err
	}

	actionMap, ok := actionData.(map[string]interface{})
	if !ok {
		err := fmt.Errorf("unexpected actionData type: expected map[string]interface{}, got %T", actionData)
		if results != nil { results <- err; close(results); }
		return err
	}

	skillTypeStr, _ := actionMap["skillType"].(string)
	content, _ := actionMap["content"].(string)

	if game.SkillType(skillTypeStr) != game.SkillTypeLastWords {
		err := fmt.Errorf("invalid skillType for LastWordsPhase: expected %s, got %s", game.SkillTypeLastWords, skillTypeStr)
		if results != nil { results <- err; close(results); }
		return err
	}

	// Create and store the last words action
	// The LastWords skill itself might need a way to store content.
	// For now, we'll assume the content is directly in actionData.
	lwSkill := skill.NewLastWordsSkill() // Create a new instance, or get from player (but player is dead)
	lwSkill.Content = content // Set content

	action := &game.Action{
		Caster: actingPlayer,
		Target: nil, // No target for last words
		Skill:  lwSkill,
	}
	p.actions = append(p.actions, action)

	// Broadcast the last words to all alive players
	p.broadcastToAlivePlayers(event.Event[any]{
		Type: event.SystemSkillResult, // Or a dedicated "PlayerGaveLastWords" event type
		PlayerID: actingPlayer.GetID(),
		Timestamp: time.Now(),
		Data: event.SkillResultData{ // Could be a more specific data struct
			SkillType: string(game.SkillTypeLastWords),
			Message:   fmt.Sprintf("玩家 %s 的遗言: %s", actingPlayer.GetID(), content),
		},
	})
	
	p.logger.Info("Player gave last words", "playerID", actingPlayer.GetID(), "content", content)

	p.currentPlayerIdx++
	if p.currentPlayerIdx < len(p.deadPlayers) {
		// Notify next player
		nextSpeaker := p.deadPlayers[p.currentPlayerIdx]
		if err := p.notifyPlayerTurn(nextSpeaker); err != nil {
			p.logger.Error("Failed to notify next speaker", "playerID", nextSpeaker.GetID(), "error", err)
			// Continue, but log error
		}
		p.broadcastToAlivePlayers(event.Event[any]{
			Type: event.SystemPhaseStart, // Generic message update
			Data: event.PhaseStartData{
			Phase:   string(p.GetName()),
			Message: fmt.Sprintf("等待 %s 发言。", nextSpeaker.GetID()),
			},
			Timestamp: time.Now(), PlayerID: game.SystemPlayerID,
		})
	} else {
		// All last words given
		p.logger.Info("All last words have been given.")
		p.broadcastToAlivePlayers(event.Event[any]{
			Type: event.SystemPhaseEnd,
			Data: event.PhaseStartData{ Phase: string(p.GetName()), Message: "遗言阶段结束。" },
			Timestamp: time.Now(), PlayerID: game.SystemPlayerID,
		})
		// This phase is now complete. Runtime needs to know this.
	}


	if results != nil {
		results <- nil
		close(results)
	}
	return nil
}

// IsComplete checks if the LastWordsPhase is complete.
// This is true if all dead players who are supposed to speak have done so.
func (p *LastWordsPhase) IsComplete(runtimeWrapper interface{}) bool {
	complete := p.currentPlayerIdx >= len(p.deadPlayers)
	if complete {
		p.logger.Info("LastWordsPhase complete.", "currentPlayerIdx", p.currentPlayerIdx, "deadPlayerCount", len(p.deadPlayers))
	} else {
		// p.logger.Debug("LastWordsPhase not complete.", "currentPlayerIdx", p.currentPlayerIdx, "deadPlayerCount", len(p.deadPlayers))
	}
	return complete
}


// broadcastToAlivePlayers is a helper to broadcast to all ALIVE players in the game.
// It uses p.players (which should contain all players) and filters by IsAlive.
func (p *LastWordsPhase) broadcastToAlivePlayers(evt event.Event[any]) error {
	p.logger.Debug("Broadcasting to alive players", "type", evt.Type, "from_playerID", evt.PlayerID)
	
	aliveReceivers := make([]string, 0)
	for _, player := range p.players { // Iterate over all players passed at construction
		if player.IsAlive() {
			aliveReceivers = append(aliveReceivers, player.GetID())
		}
	}
	
	// If no specific receivers in evt, set to all alive players.
	// If evt.Receivers is already set, this respects that (though for phase-wide typically it's not).
	if len(evt.Receivers) == 0 {
		evt.Receivers = aliveReceivers
	}


	for _, receiverID := range evt.Receivers {
		if player, exists := p.players[receiverID]; exists && player.IsAlive() {
			if err := player.Write(evt); err != nil {
				p.logger.Error("Failed to write event to alive player", "playerID", receiverID, "error", err)
			}
		}
	}
	return nil
}


// broadcastEvent 广播事件 (Old one, to be removed or adapted if still needed for specific targets)
func (p *LastWordsPhase) broadcastEvent(evt any) error {
	// 将事件转换为 event.Event[any] 类型
	eventAny, ok := evt.(event.Event[any])
	if !ok {
		return fmt.Errorf("invalid event type: %T", evt)
	}

	for _, receiverID := range eventAny.Receivers {
		if player, exists := p.players[receiverID]; exists {
			if err := player.Write(eventAny); err != nil {
				return err
			}
		}
	}
	return nil
}

// getAlivePlayerIDs is removed, using broadcastToAlivePlayers helper.
