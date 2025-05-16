package phase

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/event"
)

// NightPhase 夜晚阶段
type NightPhase struct {
	*BasePhase

	deaths []game.Player
	logger *slog.Logger
}

func NewNightPhase(players []game.Player) *NightPhase {
	return &NightPhase{
		BasePhase: NewBasePhase(players),
		deaths:    make([]game.Player, 0),
		logger:    slog.Default(),
	}
}

func (p *NightPhase) GetName() game.PhaseType {
	return game.PhaseNight
}

// Start 开始夜晚阶段
func (p *NightPhase) Start(ctx context.Context) error {
	// 1. 通知所有玩家进入夜晚
	if err := p.broadcastPhaseStart(game.PhaseNight, "天黑了，所有玩家请闭眼"); err != nil {
		return fmt.Errorf("notify phase start: %w", err)
	}

	// 2. 按优先级处理角色行动
	if err := p.handleRoleActions(ctx); err != nil {
		return fmt.Errorf("handle role actions: %w", err)
	}

	// 3. 处理行动结果
	p.calculatePhaseResult()

	// 4. 通知所有玩家夜晚结束
	if err := p.notifyPhaseEnd(); err != nil {
		return fmt.Errorf("notify phase end: %w", err)
	}

	return nil
}

// handleRoleActions 按优先级处理角色行动
func (p *NightPhase) handleRoleActions(ctx context.Context) error {
	// 1. 获取所有存活玩家
	alivePlayers := make([]game.Player, 0)
	for _, player := range p.players {
		if player.IsAlive() {
			alivePlayers = append(alivePlayers, player)
		}
	}

	if len(alivePlayers) == 0 {
		return fmt.Errorf("no alive players")
	}

	// 2. 按优先级排序玩家
	sort.Slice(alivePlayers, func(i, j int) bool {
		priorityI := alivePlayers[i].GetRole().GetPriority(game.PhaseNight)
		priorityJ := alivePlayers[j].GetRole().GetPriority(game.PhaseNight)
		return priorityI < priorityJ
	})

	// 3. 按优先级处理每个玩家的行动
	for _, player := range alivePlayers {
		// 3.1 获取玩家可用技能
		skills := player.GetRole().GetAvailableSkills(p.GetName())
		if len(skills) == 0 {
			continue
		}

		// 3.2 等待玩家行动
		action, err := p.waitForPlayerAction(ctx, player, skills)
		if err != nil {
			return fmt.Errorf("wait for player %s action: %w", player.GetID(), err)
		}

		// 3.3 记录行动
		if action != nil {
			p.AddAction(action)
		}
	}

	return nil
}

// waitForPlayerAction 等待玩家行动
func (p *NightPhase) waitForPlayerAction(ctx context.Context, player game.Player, skills []game.Skill) (*game.Action, error) {
	// 1. 发送可用技能列表给玩家
	if err := p.broadcastSkillResult(skills[0].GetName(), "请选择要使用的技能", player.GetID()); err != nil {
		return nil, fmt.Errorf("send skill list: %w", err)
	}

	// 2. 等待玩家选择技能
	evt, err := p.waitPlayer(ctx, player)
	if err != nil {
		return nil, fmt.Errorf("read player action: %w", err)
	}

	// 3. 解析玩家行动
	if evt.Type != event.UserSkill {
		return nil, fmt.Errorf("invalid event type: %s", evt.Type)
	}

	// 4. 获取技能和目标
	skillData, ok := evt.Data.(*event.UserSkillData)
	if !ok {
		return nil, fmt.Errorf("invalid skill use data")
	}

	// 5. 查找技能
	targetSkill := p.getPlayerSkill(player, game.SkillType(skillData.SkillType))
	if targetSkill == nil {
		return nil, fmt.Errorf("skill not found: %s", skillData.SkillType)
	}

	// 6. 获取目标玩家
	target, exists := p.players[skillData.TargetID]
	if !exists {
		return nil, fmt.Errorf("target player not found: %s", skillData.TargetID)
	}

	// 7. 检查技能条件
	if err := targetSkill.Check(game.PhaseNight, player, target); err != nil {
		return nil, fmt.Errorf("check skill: %w", err)
	}

	// 8. 使用技能
	result := &game.SkillResult{}
	targetSkill.Put(player, target, result)

	// 9. 返回行动
	return &game.Action{
		Caster: player,
		Target: target,
		Skill:  targetSkill,
	}, nil
}

// handleActionResults 处理行动结果
func (p *NightPhase) handleActionResults() error {
	// 1. 按优先级排序行动
	sort.Slice(p.actions, func(i, j int) bool {
		priorityI := p.actions[i].Skill.GetPriority()
		priorityJ := p.actions[j].Skill.GetPriority()
		return priorityI < priorityJ
	})

	// 2. 处理每个行动
	for _, action := range p.actions {
		// 2.1 检查目标是否存活
		if !action.Target.IsAlive() {
			continue
		}

		// 2.2 检查目标是否被保护
		if action.Target.IsProtected() {
			continue
		}

		// 2.3 处理技能结果
		switch action.Skill.GetName() {
		case game.SkillTypeKill:
			action.Target.SetAlive(false)
		case game.SkillTypePoison:
			action.Target.SetAlive(false)
		}
	}

	return nil
}

// notifyPhaseEnd 通知阶段结束
func (p *NightPhase) notifyPhaseEnd() error {
	// 1. 获取死亡玩家列表
	deaths := make([]string, 0)
	for _, player := range p.players {
		if !player.IsAlive() {
			deaths = append(deaths, player.GetID())
		}
	}

	// 2. 构建结束消息
	message := "天亮了，所有玩家请睁眼。"
	if len(deaths) > 0 {
		message = fmt.Sprintf("昨晚死亡的玩家是：%s", deaths)
	} else {
		message = fmt.Sprintf("昨晚是一个平安夜")
	}

	// 3. 通知所有玩家夜晚结束
	return p.broadcastPhaseEnd(game.PhaseNight, message)
}

// calculatePhaseResult 计算阶段结果
func (p *NightPhase) calculatePhaseResult() *game.PhaseResult[game.UserSkillResultMap] {
	var (
		deaths      = make([]game.Player, 0)
		skillResult = make(game.UserSkillResultMap)
	)

	for _, action := range p.actions {
		var result game.SkillResult
		action.Skill.Put(action.Caster, action.Target, &result)

		skillResult[action.Caster] = &result
	}

	for _, action := range p.actions {
		isAlive := action.Target.IsAlive()
		if !isAlive {
			deaths = append(deaths, action.Target)
		}
	}

	return &game.PhaseResult[game.UserSkillResultMap]{
		Deaths:    deaths,
		ExtraData: skillResult,
	}
}
