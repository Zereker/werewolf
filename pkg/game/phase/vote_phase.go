package phase

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/event"
)

// VotePhase 投票阶段
type VotePhase struct {
	*BasePhase

	deaths       []game.Player
	// skillResults game.SkillResultMap // This was part of BasePhase but commented out, VotePhase might need its own.
	// For now, GetPhaseResult will construct it.
}

func NewVotePhase(players []game.Player) *VotePhase {
	return &VotePhase{
		BasePhase: NewBasePhase(players),
		deaths:    make([]game.Player, 0),
		// skillResults: make(game.SkillResultMap), // If needed
	}
}

func (v *VotePhase) GetName() game.PhaseType {
	return game.PhaseVote
}

// IsComplete checks if the VotePhase is complete.
// This is true if all alive players have submitted a "vote" action.
func (v *VotePhase) IsComplete(runtimeWrapper interface{}) bool {
	// rt, ok := runtimeWrapper.(interface{ GetPlayers() []game.Player })
	// if !ok {
	// 	 v.logger.Error("VotePhase.IsComplete received invalid runtime object")
	// 	 return false
	// }
	// allGamePlayers := rt.GetPlayers()

	actedPlayerIDs := make(map[string]bool)
	for _, action := range v.actions {
		if action.Skill.GetName() == game.SkillTypeVote {
			actedPlayerIDs[action.Caster.GetID()] = true
		}
	}

	for _, player := range v.players {
		if player.IsAlive() {
			if !actedPlayerIDs[player.GetID()] {
				// v.logger.Debug("VotePhase not complete: player has not voted", "playerID", player.GetID())
				return false // Not all alive players have voted
			}
		}
	}
	
	if len(v.getAlivePlayerIDs()) == 0 && len(v.actions) == 0 { // No one alive to vote
		v.logger.Info("VotePhase complete: no alive players to vote.")
		return true
	}
	
	if len(actedPlayerIDs) > 0 && len(actedPlayerIDs) == len(v.getAlivePlayerIDs()) {
		v.logger.Info("VotePhase complete: all alive players have voted.")
		return true
	}
	
	// v.logger.Debug("VotePhase IsComplete defaulting to false", "acted_count", len(actedPlayerIDs), "alive_count", len(v.getAlivePlayerIDs()))
	return false
}

// broadcastEvent is removed, BasePhase.broadcastEvent will be used.

func (v *VotePhase) Start(ctx context.Context) error {
	v.actions = make([]*game.Action, 0) // Clear previous actions
	v.deaths = make([]game.Player, 0)   // Clear previous deaths
	v.logger.Info("VotePhase starting", "round", v.round)

	// 通知所有玩家进入投票阶段
	if err := v.broadcastPhaseStart(game.PhaseVote, "投票阶段开始，请投票。"); err != nil {
		return fmt.Errorf("broadcast vote phase start failed: %w", err)
	}

	// VotePhase no longer blocks. Actions are received via HandleAction.
	// The Runtime will decide when to progress (e.g., after a timeout or all expected actions received).
	return nil
}

// waitForVotes is removed. Actions are received via HandleAction.

// GetPhaseResult processes collected 'vote' actions.
// This would be called by the Runtime when it determines the voting part of the phase is over.
// It also updates player 'alive' status directly.
func (v *VotePhase) GetPhaseResult() *game.PhaseResult[game.SkillResultMap] {
	// 按优先级排序所有投票行为
	sort.Slice(v.actions, func(i, j int) bool {
		return v.actions[i].Skill.GetPriority() < v.actions[j].Skill.GetPriority()
	})

	// 执行所有投票并统计
	// This method now processes the actions stored in v.BasePhase.actions
	v.logger.Info("Calculating VotePhase results", "round", v.round, "action_count", len(v.actions))
	v.deaths = make([]game.Player, 0) // Clear previous deaths for this calculation

	voteCount := make(map[string]int) // playerID voted for -> count
	voteRecord := make(map[string]string) // voterID -> votedID

	for _, action := range v.actions {
		if action.Skill.GetName() != game.SkillTypeVote {
			v.logger.Warn("Non-vote action found in VotePhase actions", "skill_type", action.Skill.GetName())
			continue
		}
		if action.Caster == nil || action.Target == nil {
			v.logger.Warn("Vote action has nil caster or target", "caster", action.Caster, "target", action.Target)
			continue
		}

		var result game.SkillResult // Skill's Put method might populate this
		action.Skill.Put(action.Caster, action.Target, &result)
		// We don't store this result in skillResults map directly for vote, process below

		voteCount[action.Target.GetID()]++
		voteRecord[action.Caster.GetID()] = action.Target.GetID()
	}

	var maxVotes int
	var mostVotedPlayerIDs []string
	for playerID, count := range voteCount {
		if count > maxVotes {
			maxVotes = count
			mostVotedPlayerIDs = []string{playerID}
		} else if count == maxVotes {
			mostVotedPlayerIDs = append(mostVotedPlayerIDs, playerID)
		}
	}
	
	var votedOutPlayers []game.Player
	// Only execute if there's a clear single winner of the vote (not a tie)
	// and there are votes. (Game rule: what if no one votes, or all skip?)
	// For now, assume a simple majority or tie means no one is executed unless further rounds of voting.
	if len(mostVotedPlayerIDs) == 1 && maxVotes > 0 { // Ensure someone was voted and it's not a tie.
		votedPlayerID := mostVotedPlayerIDs[0]
		if votedPlayer, exists := v.players[votedPlayerID]; exists {
			if votedPlayer.IsAlive() { // Ensure they are not already dead
				votedPlayer.SetAlive(false)
				v.deaths = append(v.deaths, votedPlayer) // Add to phase deaths
				votedOutPlayers = append(votedOutPlayers, votedPlayer)
				v.logger.Info("Player voted out", "playerID", votedPlayerID, "votes", maxVotes)
			}
		}
	} else if len(mostVotedPlayerIDs) > 1 {
		v.logger.Info("Vote resulted in a tie", "tied_players", mostVotedPlayerIDs, "votes", maxVotes)
		// Handle tie logic if necessary (e.g. re-vote, or no one executed)
	} else {
		v.logger.Info("No player received enough votes to be executed or no votes cast.")
	}

	// Create skill result data for broadcasting or logging
	// The skillResults map in BasePhase was removed, so we construct data for PhaseResult directly.
	votePhaseExtraData := game.SkillResultMap{
		game.SkillTypeVote: &game.SkillResult{
			Success: true, // Or based on whether someone was actually voted out
			Message: "Vote results",
			Data: event.VoteResultData{ // Using the event struct for data
				Votes:      voteRecord,
				Executed:   "", // Fill if someone was executed
				IsTie:      len(mostVotedPlayerIDs) > 1 && maxVotes > 0,
				TiePlayers: nil, // Fill if tie
			},
		},
	}
	if len(votedOutPlayers) == 1 {
		votePhaseExtraData[game.SkillTypeVote].Data.(event.VoteResultData).Executed = votedOutPlayers[0].GetID()
	}
	if len(mostVotedPlayerIDs) > 1 && maxVotes > 0 {
		votePhaseExtraData[game.SkillTypeVote].Data.(event.VoteResultData).TiePlayers = mostVotedPlayerIDs
	}


	return &game.PhaseResult[game.SkillResultMap]{
		Deaths:    v.deaths, // Players who died in this phase
		ExtraData: votePhaseExtraData,
	}
}
