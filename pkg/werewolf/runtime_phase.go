package werewolf

import (
	"time"

	"github.com/Zereker/werewolf/pkg/game"
)

// handlePhase 处理阶段开始事件
func (r *Runtime) handlePhase() {
	phaseType := r.getCurrentPhase().GetName()

	switch phaseType {
	case game.PhaseNight:
		r.handleNightPhase()
	case game.PhaseDay:
		r.handleDayPhase()
	case game.PhaseVote:
		r.handleVotePhase()
	}
}

// handleNightPhase 处理夜晚阶段开始
func (r *Runtime) handleNightPhase() {
	// 广播夜晚阶段开始事件
	r.broadcastNightPhaseStart()

	// 重置当前阶段
	currentPhase := r.getCurrentPhase()
	currentPhase.Start()

	// 通知所有玩家进入夜晚
	r.broadcastEvent(Event{
		Type: EventSystemPhaseStart,
		Data: map[string]interface{}{
			"phase":   game.PhaseNight,
			"round":   r.round,
			"message": "天黑了，所有玩家请闭眼",
		},
		Receivers: r.getAllPlayerIDs(),
		Timestamp: time.Now(),
	})

	// 等待狼人行动
	r.broadcastEvent(Event{
		Type: EventSystemSkillResult,
		Data: map[string]interface{}{
			"skill_type": game.SkillTypeKill,
			"message":    "狼人请睁眼，请选择要击杀的目标",
		},
		Receivers: r.getAlivePlayerIDsByRole(game.RoleTypeWerewolf),
		Timestamp: time.Now(),
	})

	// 等待预言家行动
	r.broadcastEvent(Event{
		Type: EventSystemSkillResult,
		Data: map[string]interface{}{
			"skill_type": game.SkillTypeCheck,
			"message":    "预言家请睁眼，请选择要查验的目标",
		},
		Receivers: r.getAlivePlayerIDsByRole(game.RoleTypeSeer),
		Timestamp: time.Now(),
	})

	// 等待守卫行动
	r.broadcastEvent(Event{
		Type: EventSystemSkillResult,
		Data: map[string]interface{}{
			"skill_type": game.SkillTypeProtect,
			"message":    "守卫请睁眼，请选择要守护的目标",
		},
		Receivers: r.getAlivePlayerIDsByRole(game.RoleTypeGuard),
		Timestamp: time.Now(),
	})

	// 等待女巫行动 - 选择使用解药或毒药
	r.broadcastEvent(Event{
		Type: EventSystemSkillResult,
		Data: map[string]interface{}{
			"skill_type": "witch_choice",
			"message":    "女巫请睁眼，今晚有人被杀了，你要使用解药救他吗？或者使用毒药？",
			"options": map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"type": game.SkillTypeAntidote,
						"desc": "使用解药救人",
					},
					{
						"type": game.SkillTypePoison,
						"desc": "使用毒药杀人",
					},
					{
						"type": "none",
						"desc": "不使用任何药",
					},
				},
			},
		},
		Receivers: r.getAlivePlayerIDsByRole(game.RoleTypeWitch),
		Timestamp: time.Now(),
	})
}

// handleDayPhase 处理白天阶段开始
func (r *Runtime) handleDayPhase() {
	// 广播白天阶段开始事件
	r.broadcastPhaseStart(game.PhaseDay)

	// 广播夜晚死亡信息
	r.broadcastNightDeaths()

	// 重置当前阶段
	currentPhase := r.getCurrentPhase()
	currentPhase.Start()

	// 通知所有玩家进入白天
	r.broadcastEvent(Event{
		Type: EventSystemPhaseStart,
		Data: map[string]interface{}{
			"phase":   game.PhaseDay,
			"round":   r.round,
			"message": "天亮了，所有玩家请睁眼",
		},
		Receivers: r.getAllPlayerIDs(),
		Timestamp: time.Now(),
	})

	// 等待所有存活玩家发言
	r.broadcastEvent(Event{
		Type: EventSystemSkillResult,
		Data: map[string]interface{}{
			"skill_type": game.SkillTypeSpeak,
			"message":    "请开始发言讨论",
		},
		Receivers: r.getAlivePlayerIDs(),
		Timestamp: time.Now(),
	})

	// 如果有玩家死亡，等待遗言
	if len(r.getCurrentPhase().GetPhaseResult().Deaths) > 0 {
		r.broadcastEvent(Event{
			Type: EventSystemSkillResult,
			Data: map[string]interface{}{
				"skill_type": game.SkillTypeLastWords,
				"message":    "请留下遗言",
			},
			Receivers: r.getDeadPlayerIDs(),
			Timestamp: time.Now(),
		})
	}
}

// handleVotePhase 处理投票阶段开始
func (r *Runtime) handleVotePhase() {
	// 广播投票阶段开始事件
	r.broadcastVotePhaseStart()

	// 重置当前阶段
	currentPhase := r.getCurrentPhase()
	currentPhase.Start()

	// 通知所有玩家进入投票阶段
	r.broadcastEvent(Event{
		Type: EventSystemPhaseStart,
		Data: map[string]interface{}{
			"phase":   game.PhaseVote,
			"round":   r.round,
			"message": "开始投票阶段",
		},
		Receivers: r.getAllPlayerIDs(),
		Timestamp: time.Now(),
	})

	// 等待所有存活玩家投票
	r.broadcastEvent(Event{
		Type: EventSystemSkillResult,
		Data: map[string]interface{}{
			"skill_type": game.SkillTypeVote,
			"message":    "请开始投票",
		},
		Receivers: r.getAlivePlayerIDs(),
		Timestamp: time.Now(),
	})
}

// getAlivePlayerIDsByRole 获取指定角色的存活玩家ID
func (r *Runtime) getAlivePlayerIDsByRole(roleType game.RoleType) []string {
	r.RLock()
	defer r.RUnlock()

	ids := make([]string, 0)
	for id, p := range r.players {
		if p.IsAlive() && p.GetRole().GetName() == roleType {
			ids = append(ids, id)
		}
	}
	return ids
}

// getDeadPlayerIDs 获取死亡玩家ID
func (r *Runtime) getDeadPlayerIDs() []string {
	r.RLock()
	defer r.RUnlock()

	ids := make([]string, 0)
	for id, p := range r.players {
		if !p.IsAlive() {
			ids = append(ids, id)
		}
	}
	return ids
}
