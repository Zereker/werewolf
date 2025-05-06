package phase

import (
	"fmt"
	"time"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/event"
)

// BasePhase 基础阶段结构体
type BasePhase struct {
	round        int
	players      map[string]game.Player
	actions      []*game.Action
	skillResults game.SkillResultMap
}

// NewBasePhase 创建基础阶段
func NewBasePhase(players []game.Player) *BasePhase {
	playerMap := make(map[string]game.Player)
	for _, player := range players {
		playerMap[player.GetID()] = player
	}

	return &BasePhase{
		players:      playerMap,
		actions:      make([]*game.Action, 0),
		skillResults: make(game.SkillResultMap),
	}
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

// AddSkillResult 添加技能结果
func (p *BasePhase) AddSkillResult(skillType game.SkillType, result *game.SkillResult) {
	p.skillResults[skillType] = result
}

// getAlivePlayerIDs 获取所有存活的玩家ID
func (p *BasePhase) getAlivePlayerIDs() []string {
	ids := make([]string, 0)
	for id, player := range p.players {
		if player.IsAlive() {
			ids = append(ids, id)
		}
	}
	return ids
}

func (p *BasePhase) getAlivePlayerIDsByRole(roleType game.RoleType) []string {
	ids := make([]string, 0)
	for id, player := range p.players {
		if player.IsAlive() && player.GetRole().GetName() == roleType {
			ids = append(ids, id)
		}
	}
	return ids
}

// getAllPlayerIDs 获取所有玩家ID
func (p *BasePhase) getAllPlayerIDs() []string {
	ids := make([]string, 0, len(p.players))
	for id := range p.players {
		ids = append(ids, id)
	}
	return ids
}

// getSkillByType 获取指定类型的技能
func (p *BasePhase) getSkillByType(skillType game.SkillType) game.Skill {
	for _, player := range p.players {
		for _, skill := range player.GetRole().GetAvailableSkills() {
			if skill.GetName() == skillType {
				return skill
			}
		}
	}
	return nil
}

// broadcastEvent 广播事件
func (p *BasePhase) broadcastEvent(evt any) error {
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

// broadcastPhaseStart 广播阶段开始
func (p *BasePhase) broadcastPhaseStart(phase game.PhaseType, message string) error {
	return p.broadcastEvent(event.Event[event.PhaseStartData]{
		Type: event.SystemPhaseStart,
		Data: event.PhaseStartData{
			Phase:   string(phase),
			Round:   p.round,
			Message: message,
		},
		Receivers: p.getAllPlayerIDs(),
		Timestamp: time.Now(),
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
func (p *BasePhase) broadcastSkillResult(skillType game.SkillType, message string) error {
	return p.broadcastEvent(event.Event[event.SkillResultData]{
		Type: event.SystemSkillResult,
		Data: event.SkillResultData{
			SkillType: string(skillType),
			Message:   message,
		},
		Receivers: p.getAllPlayerIDs(),
		Timestamp: time.Now(),
	})
}
