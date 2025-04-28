package werewolf

import (
	"fmt"
	"time"

	"github.com/Zereker/werewolf/pkg/game"
)

func (r *Runtime) broadcastEvent(evt Event) {
	r.RLock()
	defer r.RUnlock()

	if len(evt.Receivers) == 0 {
		return
	}

	select {
	case r.systemEventChan <- evt:
	default:
	}
}

// broadcastGameStart 广播游戏开始事件
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

// broadcastPhaseStart 广播阶段开始事件
func (r *Runtime) broadcastPhaseStart(phaseType game.PhaseType) {
	r.broadcastEvent(Event{
		Type: EventSystemPhaseStart,
		Data: map[string]interface{}{
			"phase": phaseType,
			"round": r.round,
		},
		Receivers: r.getAllPlayerIDs(),
		Timestamp: time.Now(),
	})
}

// broadcastNightPhaseStart 广播夜晚阶段开始事件
func (r *Runtime) broadcastNightPhaseStart() {
	// 1. 通知所有玩家进入夜晚
	r.broadcastPhaseStart(game.PhaseNight)

	// 2. 通知狼人队友信息
	r.broadcastWerewolfTeammates()

	// 3. 通知女巫
	r.broadcastWitchNotification()
}

// broadcastWerewolfTeammates 广播狼人队友信息
func (r *Runtime) broadcastWerewolfTeammates() {
	r.RLock()
	defer r.RUnlock()

	// 找出所有狼人
	var werewolves []string
	for _, p := range r.players {
		if !p.IsAlive() {
			continue
		}

		if p.GetRole().GetName() == game.RoleTypeWerewolf {
			werewolves = append(werewolves, p.GetID())
		}
	}

	// 给每个狼人推送队友信息
	for _, id := range werewolves {
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

// broadcastWitchNotification 广播女巫通知
func (r *Runtime) broadcastWitchNotification() {
	r.RLock()
	defer r.RUnlock()

	for _, p := range r.players {
		if p.GetRole().GetName() == game.RoleTypeWitch {
			r.broadcastEvent(Event{
				Type: EventSystemSkillResult,
				Data: map[string]interface{}{
					"message": "女巫请睁眼，今晚有人被杀了，你要救他吗？",
				},
				Receivers: []string{p.GetID()},
				Timestamp: time.Now(),
			})
		}
	}
}

// broadcastDayPhaseStart 广播白天阶段开始事件
func (r *Runtime) broadcastDayPhaseStart() {
	// 1. 通知所有玩家进入白天
	r.broadcastPhaseStart(game.PhaseDay)

	// 2. 广播昨晚的死亡信息
	r.broadcastNightDeaths()

	// 3. 通知可以开始发言
	r.broadcastEvent(Event{
		Type: EventSystemSkillResult,
		Data: map[string]interface{}{
			"message": "请开始发言讨论",
		},
		Receivers: r.getAlivePlayerIDs(),
		Timestamp: time.Now(),
	})
}

// broadcastNightDeaths 广播昨晚的死亡信息
func (r *Runtime) broadcastNightDeaths() {
	r.RLock()
	defer r.RUnlock()

	lastNightResult := r.phaseResults[r.round][game.PhaseNight]
	deaths := make([]string, 0, len(lastNightResult.Deaths))
	for _, p := range lastNightResult.Deaths {
		for id, player := range r.players {
			if player == p {
				deaths = append(deaths, id)
				break
			}
		}
	}

	r.broadcastEvent(Event{
		Type: EventSystemPhaseStart,
		Data: map[string]interface{}{
			"deaths": deaths,
		},
		Receivers: r.getAllPlayerIDs(),
		Timestamp: time.Now(),
	})
}

// broadcastVotePhaseStart 广播投票阶段开始事件
func (r *Runtime) broadcastVotePhaseStart() {
	// 1. 通知所有玩家进入投票阶段
	r.broadcastPhaseStart(game.PhaseVote)

	// 2. 通知可以开始投票
	r.broadcastEvent(Event{
		Type: EventSystemSkillResult,
		Data: map[string]interface{}{
			"message": "请开始投票",
		},
		Receivers: r.getAlivePlayerIDs(),
		Timestamp: time.Now(),
	})
}

// broadcastVoteResult 广播投票结果
func (r *Runtime) broadcastVoteResult(voterID, targetID string, err error) {
	if err != nil {
		r.broadcastEvent(Event{
			Type: EventSystemVoteResult,
			Data: &SystemVoteResultData{
				Round:   r.round,
				Success: false,
				Message: err.Error(),
				VoterID: voterID,
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
			VoterID:  voterID,
			TargetID: targetID,
		},
		PlayerID:  voterID,
		Receivers: r.getAlivePlayerIDs(),
		Timestamp: time.Now(),
	})
}

// broadcastSkillResult 广播技能使用结果
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
