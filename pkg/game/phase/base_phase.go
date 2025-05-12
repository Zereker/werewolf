package phase

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/event"
	"github.com/Zereker/werewolf/pkg/game/skill"
	_ "github.com/Zereker/werewolf/pkg/game/skill"
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
func (p *BasePhase) getSkillByType(skillType game.SkillType) game.Skill {
	var result game.Skill
	for _, player := range p.players {
		for _, skill := range player.GetRole().GetAvailableSkills() {
			if skill.GetName() == skillType {
				result = skill
			}
		}
	}

	return result
}

// getSkillByType 获取指定类型的技能
func (p *BasePhase) getPlayerSkill(player game.Player, skillType game.SkillType) game.Skill {
	for _, s := range player.GetRole().GetAvailableSkills() {
		if s.GetName() == skillType {
			return s
		}
	}

	return nil
}

func (p *BasePhase) waitPlayer(ctx context.Context, player game.Player, timeout time.Duration) (event.Event[any], error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 等待该玩家的发言
	evt, err := player.Read(ctx)
	if err != nil {
		return event.Event[any]{}, err
	}

	return evt, nil
}

// broadcastEvent 广播事件
func (p *BasePhase) broadcastEvent(evt any) error {
	// 使用类型断言获取事件的基本信息
	switch e := evt.(type) {
	case event.Event[event.PhaseStartData]:
		for _, receiverID := range e.Receivers {
			if player, exists := p.players[receiverID]; exists {
				if err := player.Write(event.Event[any]{
					Type:      e.Type,
					PlayerID:  e.PlayerID,
					Receivers: e.Receivers,
					Timestamp: e.Timestamp,
					Data:      e.Data,
				}); err != nil {
					return err
				}
			}
		}
	case event.Event[event.SkillResultData]:
		for _, receiverID := range e.Receivers {
			if player, exists := p.players[receiverID]; exists {
				if err := player.Write(event.Event[any]{
					Type:      e.Type,
					PlayerID:  e.PlayerID,
					Receivers: e.Receivers,
					Timestamp: e.Timestamp,
					Data:      e.Data,
				}); err != nil {
					return err
				}
			}
		}
	case event.Event[event.SystemGameStartData]:
		for _, receiverID := range e.Receivers {
			if player, exists := p.players[receiverID]; exists {
				if err := player.Write(event.Event[any]{
					Type:      e.Type,
					PlayerID:  e.PlayerID,
					Receivers: e.Receivers,
					Timestamp: e.Timestamp,
					Data:      e.Data,
				}); err != nil {
					return err
				}
			}
		}
	case event.Event[event.SystemGameEndData]:
		for _, receiverID := range e.Receivers {
			if player, exists := p.players[receiverID]; exists {
				if err := player.Write(event.Event[any]{
					Type:      e.Type,
					PlayerID:  e.PlayerID,
					Receivers: e.Receivers,
					Timestamp: e.Timestamp,
					Data:      e.Data,
				}); err != nil {
					return err
				}
			}
		}
	default:
		return fmt.Errorf("unsupported event type: %T", evt)
	}
	return nil
}

// broadcastPhaseStart 广播阶段开始
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

// convertActionToSkillEvent 将 Action 转换为 Skill 事件
func (p *BasePhase) convertActionToSkillEvent(action *game.Action) event.Event[any] {
	var targetID string
	if action.Target != nil {
		targetID = action.Target.GetID()
	}

	return event.Event[any]{
		ID:        uuid.NewString(),
		Type:      event.UserSkill,
		PlayerID:  action.Caster.GetID(),
		Receivers: p.getAllPlayerIDs(),
		Timestamp: time.Now(),
		Data: &event.UserSkillData{
			TargetID:  targetID,
			SkillType: string(action.Skill.GetName()),
		},
	}
}

// convertEventToAction 将用户事件转换为 Action
func (p *BasePhase) convertEventToAction(evt event.Event[any]) (*game.Action, error) {
	// 检查事件类型
	if evt.Type != event.UserSkill {
		return nil, fmt.Errorf("invalid event type: %s", evt.Type)
	}

	// 获取施法者
	caster, exists := p.players[evt.PlayerID]
	if !exists {
		return nil, fmt.Errorf("caster not found: %s", evt.PlayerID)
	}

	// 获取技能数据
	skillData, ok := evt.Data.(*event.UserSkillData)
	if !ok {
		return nil, fmt.Errorf("invalid event data type: %T", evt.Data)
	}

	// 获取技能
	s := p.getPlayerSkill(caster, game.SkillType(skillData.SkillType))
	if s == nil {
		return nil, fmt.Errorf("skill not found: %s", skillData.SkillType)
	}

	if speak, ok := s.(*skill.Speak); ok {
		speak.Content = skillData.Content
	}

	// 获取目标（如果有）
	var target game.Player
	if skillData.TargetID != "" {
		target, exists = p.players[skillData.TargetID]
		if !exists {
			return nil, fmt.Errorf("target not found: %s", skillData.TargetID)
		}
	}

	return &game.Action{
		Caster: caster,
		Target: target,
		Skill:  s,
	}, nil
}
