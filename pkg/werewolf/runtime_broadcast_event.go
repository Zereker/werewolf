package werewolf

import (
	"fmt"
	"time"

	"github.com/Zereker/werewolf/pkg/game"
)

func (r *Runtime) broadcastGameStart() {
	players := make([]PlayerInfo, 0, len(r.players))
	for _, p := range r.players {
		players = append(players, PlayerInfo{
			ID:      p.GetID(),
			Role:    p.GetRole(),
			IsAlive: p.IsAlive(),
		})
	}

	currentPhase := r.getCurrentPhase()
	phaseInfo := PhaseInfo{
		Type:  currentPhase.GetName(),
		Round: r.round,
	}

	for playerID, p := range r.players {
		personalStartData := &SystemGameStartData{
			Players: players,
			Phase:   phaseInfo,
			Role:    p.GetRole().GetName().String(),
		}

		r.broadcastEvent(Event{
			Type:      EventSystemGameStart,
			Data:      personalStartData,
			PlayerID:  playerID,
			Receivers: []string{playerID},
			Timestamp: time.Now(),
		})
	}
}

func (r *Runtime) broadcastWerewolfTeammates() {
	r.RLock()
	defer r.RUnlock()

	// 找出所有狼人
	var werewolves []string
	for _, p := range r.players {
		if p.GetRole().GetName() == game.RoleTypeWerewolf {
			werewolves = append(werewolves, p.GetID())
		}
	}

	// 给每个狼人推送队友信息
	for _, id := range werewolves {
		// 发送事件
		r.broadcastEvent(Event{
			Type: EventSystemPhaseStart,
			Data: map[string]interface{}{
				"message": fmt.Sprintf("你的狼人队友有：%v", werewolves),
			},
			Receivers: []string{id},
			Timestamp: time.Now(),
		})
	}
}

func (r *Runtime) broadcastVoteResult(voterID, targetID string, err error) {
	if err != nil {
		r.broadcastEvent(Event{
			Type: EventSystemVoteResult,
			Data: &SystemVoteResultData{
				Round:   r.round,
				Success: false,
				Message: err.Error(),
				VoterID: voterID, // 添加 VoterID
			},
			PlayerID:  voterID,
			Receivers: []string{voterID},
			Timestamp: time.Now(),
		})
		return
	}

	r.broadcastEvent(Event{
		Type: EventSystemVoteResult,
		Data: &SystemVoteResultData{
			Round:    r.round,
			Success:  true,
			Message:  "投票成功",
			VoterID:  voterID,  // 添加 VoterID
			TargetID: targetID, // 添加 TargetID
		},
		PlayerID:  voterID,
		Receivers: r.getAlivePlayerIDs(),
		Timestamp: time.Now(),
	})
}

func (r *Runtime) broadcastSkillResult(playerID string, skillType game.SkillType, err error) {
	var receivers []string
	var success bool
	var message string

	if err != nil {
		success = false
		message = err.Error()
		receivers = []string{playerID}
	} else {
		success = true
		message = "技能使用成功"
		// 修复这里的逻辑错误
		if skillType == game.SkillTypeVote {
			receivers = r.getAlivePlayerIDs()
		} else {
			receivers = []string{playerID}
		}
	}

	r.broadcastEvent(Event{
		Type: EventSystemSkillResult,
		Data: &SystemSkillResultData{
			SkillType: skillType,
			Success:   success,
			Message:   message,
		},
		PlayerID:  playerID,
		Receivers: receivers,
		Timestamp: time.Now(),
	})
}
