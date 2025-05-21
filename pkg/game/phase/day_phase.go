package phase

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/event"
)

// DayPhase 白天阶段
type DayPhase struct {
	*BasePhase

	discussionTime time.Duration
}

func NewDayPhase(players []game.Player) *DayPhase {
	return &DayPhase{
		BasePhase:      NewBasePhase(players),
		discussionTime: 5 * time.Minute, // 默认讨论时间为5分钟
	}
}

func (d *DayPhase) GetName() game.PhaseType {
	return game.PhaseDay
}

// IsComplete checks if the DayPhase is complete.
// This is true if all alive players have submitted a "speak" action.
func (d *DayPhase) IsComplete(runtimeWrapper interface{}) bool {
	// rt, ok := runtimeWrapper.(interface{ GetPlayers() []game.Player })
	// if !ok {
	// 	 d.logger.Error("DayPhase.IsComplete received invalid runtime object")
	// 	 return false // Or panic
	// }
	// allGamePlayers := rt.GetPlayers() // Using p.players which should be consistent

	actedPlayerIDs := make(map[string]bool)
	for _, action := range d.actions {
		if action.Skill.GetName() == game.SkillTypeSpeak {
			actedPlayerIDs[action.Caster.GetID()] = true
		}
	}

	for _, player := range d.players { // d.players are the players at the start of this phase
		if player.IsAlive() {
			// Check if this alive player has a corresponding speak action
			if !actedPlayerIDs[player.GetID()] {
				// d.logger.Debug("DayPhase not complete: player has not spoken", "playerID", player.GetID())
				return false // Not all alive players have spoken
			}
		}
	}
	
	if len(d.getAlivePlayerIDs()) == 0 && len(d.actions) == 0 { // No one alive to speak
		d.logger.Info("DayPhase complete: no alive players to speak.")
		return true
	}
	
	if len(actedPlayerIDs) > 0 && len(actedPlayerIDs) == len(d.getAlivePlayerIDs()) {
		d.logger.Info("DayPhase complete: all alive players have spoken.")
		return true
	}

	// Fallback, should be caught by above conditions if logic is correct
	// d.logger.Debug("DayPhase IsComplete defaulting to false", "acted_count", len(actedPlayerIDs), "alive_count", len(d.getAlivePlayerIDs()))
	return false
}

// Start 开始阶段
// The DayPhase now primarily involves players sending 'speak' actions.
// The actual processing of these speeches and game progression will be managed by the Runtime
// based on collected actions or timeouts.
func (d *DayPhase) Start(ctx context.Context) error {
	d.actions = make([]*game.Action, 0) // Clear actions from previous phase if any lingered.
	d.logger.Info("DayPhase starting", "round", d.round)
	// 通知所有玩家进入白天
	if err := d.broadcastPhaseStart(game.PhaseDay, "现在是白天，请依次发言。"); err != nil {
		return errors.Wrap(err, "broadcast day phase start failed")
	}

	// Day phase no longer blocks here. It will receive 'speak' actions via HandleAction.
	// The Runtime will decide when the DayPhase concludes (e.g., timeout, all players spoken).
	// For now, Start just announces the phase. Speech collection happens in HandleAction.
	// Processing of speeches (GetPhaseResult) and broadcasting results would be called by Runtime.
	
	// The following logic is illustrative of what Runtime might do after collecting actions:
	// phaseResult := d.GetPhaseResult()
	// for caster, result := range phaseResult {
	// 	 d.logger.Info("Player spoke", "playerID", caster.GetID(), "message", result.Message)
	//	 // Broadcasting individual speeches might be too noisy; usually summarized or handled differently.
	// }
	// if err := d.broadcastPhaseEnd(game.PhaseDay, "白天讨论结束"); err != nil {
	//	 return fmt.Errorf("broadcast day phase end failed: %w", err)
	// }

	return nil
}

// waitForSpeeches is removed. Actions are received via HandleAction.

// GetPhaseResult processes collected 'speak' actions.
// This would be called by the Runtime when it determines the speech part of the phase is over.
func (d *DayPhase) GetPhaseResult() game.UserSkillResultMap {
	// 执行所有行为
	speakResults := make(map[game.Player]*game.SkillResult)

	for _, action := range d.actions {
		var result game.SkillResult
		action.Skill.Put(action.Caster, action.Target, &result)
		speakResults[action.Caster] = &result
	}

	return speakResults
}
