package werewolf

import (
	"strconv"
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

	// 通知狼人行动
	r.broadcastEvent(Event{
		Type: EventSystemSkillResult,
		Data: map[string]interface{}{
			"skill_type": game.SkillTypeKill,
			"message":    "狼人请睁眼，请选择要击杀的目标",
		},
		Receivers: r.getAlivePlayerIDsByRole(game.RoleTypeWerewolf),
		Timestamp: time.Now(),
	})
	// 等待狼人回复
	r.waitForPlayerActions(game.RoleTypeWerewolf)

	// 通知预言家行动
	r.broadcastEvent(Event{
		Type: EventSystemSkillResult,
		Data: map[string]interface{}{
			"skill_type": game.SkillTypeCheck,
			"message":    "预言家请睁眼，请选择要查验的目标",
		},
		Receivers: r.getAlivePlayerIDsByRole(game.RoleTypeSeer),
		Timestamp: time.Now(),
	})
	// 等待预言家回复
	r.waitForPlayerActions(game.RoleTypeSeer)

	// 通知守卫行动
	r.broadcastEvent(Event{
		Type: EventSystemSkillResult,
		Data: map[string]interface{}{
			"skill_type": game.SkillTypeProtect,
			"message":    "守卫请睁眼，请选择要守护的目标",
		},
		Receivers: r.getAlivePlayerIDsByRole(game.RoleTypeGuard),
		Timestamp: time.Now(),
	})
	// 等待守卫回复
	r.waitForPlayerActions(game.RoleTypeGuard)

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
	// 等待女巫回复
	r.waitForPlayerActions(game.RoleTypeWitch)
}

// waitForPlayerActions 等待指定角色的玩家完成行动
func (r *Runtime) waitForPlayerActions(roleType game.RoleType) {
	// 获取该角色的所有存活玩家
	players := r.getAlivePlayerIDsByRole(roleType)
	if len(players) == 0 {
		return
	}

	// 创建一个通道来接收所有玩家的行动
	actionChan := make(chan struct{}, len(players))

	// 为每个玩家创建一个等待组
	for _, playerID := range players {
		go func(id string) {
			// 将字符串ID转换为整数
			playerIDInt, err := strconv.Atoi(id)
			if err != nil {
				r.logger.Error("转换玩家ID失败", "player_id", id, "error", err)
				actionChan <- struct{}{}
				return
			}

			// 获取玩家对象
			player := r.players[id]
			if player == nil {
				r.logger.Error("玩家不存在", "player_id", id)
				actionChan <- struct{}{}
				return
			}

			// 等待该玩家的行动
			for {
				select {
				case <-time.After(100 * time.Millisecond): // 定期检查玩家事件
					// 检查玩家是否有新的事件
					if len(player.events) > 0 {
						evt := player.events[0]
						player.events = player.events[1:] // 移除已处理的事件

						// 处理用户事件
						if evt.Type == EventUserSkill {
							skillData := evt.Data.(*UserSkillData)
							// 将用户事件转换为玩家行动
							action := PlayerAction{
								PlayerID:   playerIDInt,
								TargetID:   playerIDInt, // 暂时使用相同的ID，后续需要从 skillData 中获取
								ActionType: string(skillData.SkillType),
								Data:       nil, // 暂时不传递额外数据
							}
							// 发送到行动通道
							r.actionChan <- action
							actionChan <- struct{}{}
							return
						}
					}
				case <-time.After(30 * time.Second): // 设置超时时间
					// 如果超时，记录日志并继续
					r.logger.Warn("玩家行动超时", "player_id", id, "role", roleType)
					actionChan <- struct{}{}
					return
				}
			}
		}(playerID)
	}

	// 等待所有玩家完成行动
	for i := 0; i < len(players); i++ {
		<-actionChan
	}
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
