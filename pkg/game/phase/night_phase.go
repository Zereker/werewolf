package phase

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/event"
)

var logger = slog.Default()

// EventType 事件类型
type EventType string

const (
	EventSystemPhaseStart  EventType = "system_phase_start"  // 阶段开始
	EventSystemPhaseEnd    EventType = "system_phase_end"    // 阶段结束
	EventSystemSkillResult EventType = "system_skill_result" // 技能使用结果
	EventUserSkill         EventType = "user_skill"          // 玩家使用技能
)

// Event 游戏事件
type Event struct {
	Type      EventType   `json:"type"`      // 事件类型
	Data      interface{} `json:"data"`      // 事件数据
	Receivers []string    `json:"receivers"` // 接收者ID列表
	Timestamp time.Time   `json:"timestamp"` // 事件发生时间
}

// UserSkillData 用户技能数据
type UserSkillData struct {
	TargetID  string         `json:"target_id"`  // 目标ID
	SkillType game.SkillType `json:"skill_type"` // 技能类型
}

// NightPhase 夜晚阶段
type NightPhase struct {
	round   int
	players map[string]game.Player

	actions []game.Action
	results game.SkillResultMap
}

func NewNightPhase(round int, players []game.Player) *NightPhase {
	playerMap := make(map[string]game.Player)
	for _, player := range players {
		playerMap[player.GetID()] = player
	}

	return &NightPhase{
		round:   round,
		players: playerMap,
		actions: make([]game.Action, 0),
		results: make(game.SkillResultMap),
	}
}

func (p *NightPhase) GetName() game.PhaseType {
	return game.PhaseNight
}

func (p *NightPhase) Start() error {
	// 通知所有玩家进入夜晚
	if err := p.broadcastEvent(event.Event[event.PhaseStartData]{
		Type: event.EventSystemPhaseStart,
		Data: event.PhaseStartData{
			Phase:   string(game.PhaseNight),
			Round:   p.round,
			Message: "天黑了，所有玩家请闭眼",
		},
		Receivers: p.getAllPlayerIDs(),
		Timestamp: time.Now(),
	}); err != nil {
		return fmt.Errorf("broadcast night phase start failed: %w", err)
	}

	// 处理狼人行动
	if err := p.handleWerewolfActions(); err != nil {
		return fmt.Errorf("handle werewolf actions failed: %w", err)
	}

	// 处理预言家行动
	if err := p.handleSeerActions(); err != nil {
		return fmt.Errorf("handle seer actions failed: %w", err)
	}

	// 处理守卫行动
	if err := p.handleGuardActions(); err != nil {
		return fmt.Errorf("handle guard actions failed: %w", err)
	}

	// 处理女巫行动
	if err := p.handleWitchActions(); err != nil {
		return fmt.Errorf("handle witch actions failed: %w", err)
	}

	return nil
}

// handleWerewolfActions 处理狼人行动
func (p *NightPhase) handleWerewolfActions() error {
	// 获取所有存活的狼人
	wolves := p.getAlivePlayerIDsByRole(game.RoleTypeWerewolf)
	if len(wolves) == 0 {
		return nil
	}

	// 通知狼人行动
	if err := p.broadcastEvent(event.Event[event.SkillResultData]{
		Type: event.EventSystemSkillResult,
		Data: event.SkillResultData{
			SkillType: string(game.SkillTypeKill),
			Message:   "狼人请睁眼，请选择要击杀的目标",
		},
		Receivers: wolves,
		Timestamp: time.Now(),
	}); err != nil {
		return err
	}

	// 等待狼人行动
	return p.waitForPlayerActions(game.RoleTypeWerewolf, game.SkillTypeKill)
}

// handleSeerActions 处理预言家行动
func (p *NightPhase) handleSeerActions() error {
	// 获取所有存活的预言家
	seers := p.getAlivePlayerIDsByRole(game.RoleTypeSeer)
	if len(seers) == 0 {
		return nil
	}

	// 通知预言家行动
	if err := p.broadcastEvent(event.Event[event.SkillResultData]{
		Type: event.EventSystemSkillResult,
		Data: event.SkillResultData{
			SkillType: string(game.SkillTypeCheck),
			Message:   "预言家请睁眼，请选择要查验的目标",
		},
		Receivers: seers,
		Timestamp: time.Now(),
	}); err != nil {
		return err
	}

	// 等待预言家行动
	return p.waitForPlayerActions(game.RoleTypeSeer, game.SkillTypeCheck)
}

// handleGuardActions 处理守卫行动
func (p *NightPhase) handleGuardActions() error {
	// 获取所有存活的守卫
	guards := p.getAlivePlayerIDsByRole(game.RoleTypeGuard)
	if len(guards) == 0 {
		return nil
	}

	// 通知守卫行动
	if err := p.broadcastEvent(event.Event[event.SkillResultData]{
		Type: event.EventSystemSkillResult,
		Data: event.SkillResultData{
			SkillType: string(game.SkillTypeProtect),
			Message:   "守卫请睁眼，请选择要守护的目标",
		},
		Receivers: guards,
		Timestamp: time.Now(),
	}); err != nil {
		return err
	}

	// 等待守卫行动
	return p.waitForPlayerActions(game.RoleTypeGuard, game.SkillTypeProtect)
}

// handleWitchActions 处理女巫行动
func (p *NightPhase) handleWitchActions() error {
	// 获取所有存活的女巫
	witches := p.getAlivePlayerIDsByRole(game.RoleTypeWitch)
	if len(witches) == 0 {
		return nil
	}

	// 通知女巫行动
	if err := p.broadcastEvent(event.Event[event.SkillResultData]{
		Type: event.EventSystemSkillResult,
		Data: event.SkillResultData{
			SkillType: "witch_choice",
			Message:   "女巫请睁眼，今晚有人被杀了，你要使用解药救他吗？或者使用毒药？",
			Options: map[string]interface{}{
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
		Receivers: witches,
		Timestamp: time.Now(),
	}); err != nil {
		return err
	}

	// 等待女巫行动
	return p.waitForPlayerActions(game.RoleTypeWitch, "witch_choice")
}

// waitForPlayerActions 等待指定角色的玩家完成行动
func (p *NightPhase) waitForPlayerActions(roleType game.RoleType, skillType game.SkillType) error {
	// 获取该角色的所有存活玩家
	players := p.getAlivePlayerIDsByRole(roleType)
	if len(players) == 0 {
		return nil
	}

	// 为每个玩家创建一个等待组
	for _, playerID := range players {
		player := p.players[playerID]
		if player == nil {
			continue
		}

		// 等待该玩家的行动
		evt, err := player.Read(30 * time.Second)
		if err != nil {
			logger.Warn("玩家行动超时", "player_id", playerID, "role", roleType)
			continue
		}

		// 处理用户事件
		if evt.Type == event.EventUserSkill {
			skillData := evt.Data.(*event.UserSkillData)
			// 将用户事件转换为玩家行动
			action := game.Action{
				Caster: player,
				Target: p.players[skillData.TargetID],
				Skill:  p.getSkillByType(skillType),
			}
			// 执行行动
			if err := action.Skill.Check(p.GetName(), action.Caster, action.Target); err != nil {
				logger.Error("技能检查失败", "player_id", playerID, "error", err)
				continue
			}
			action.Skill.Put(action.Caster, action.Target, game.PutOption{})
			p.actions = append(p.actions, action)
		}
	}

	return nil
}

// calculatePhaseResult 计算阶段结果
func (p *NightPhase) calculatePhaseResult() *game.PhaseResult[game.SkillResultMap] {
	// 执行所有行动
	deaths := make([]game.Player, 0)
	for _, action := range p.actions {
		// 检查目标是否被保护
		if action.Target.IsProtected() {
			continue
		}

		// 检查技能类型
		switch action.Skill.GetName() {
		case game.SkillTypeKill:
			// 狼人杀人
			action.Target.SetAlive(false)
			deaths = append(deaths, action.Target)
		case game.SkillTypeCheck:
			// 预言家查验
			// TODO: 实现查验逻辑
		case game.SkillTypeProtect:
			// 守卫保护
			// TODO: 实现保护逻辑
		}
	}

	return &game.PhaseResult[game.SkillResultMap]{
		Deaths:    deaths,
		ExtraData: p.results,
	}
}

// getAlivePlayerIDsByRole 获取指定角色的所有存活玩家ID
func (p *NightPhase) getAlivePlayerIDsByRole(roleType game.RoleType) []string {
	ids := make([]string, 0)
	for id, player := range p.players {
		if player.IsAlive() && player.GetRole().GetName() == roleType {
			ids = append(ids, id)
		}
	}

	return ids
}

// getAllPlayerIDs 获取所有玩家ID
func (p *NightPhase) getAllPlayerIDs() []string {
	ids := make([]string, 0, len(p.players))
	for id := range p.players {
		ids = append(ids, id)
	}

	return ids
}

// getSkillByType 获取指定类型的技能
func (p *NightPhase) getSkillByType(skillType game.SkillType) game.Skill {
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
func (p *NightPhase) broadcastEvent(evt any) error {
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
