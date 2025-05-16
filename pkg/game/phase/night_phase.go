package phase

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

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

func (p *NightPhase) Start(ctx context.Context) error {
	// 通知所有玩家进入夜晚
	if err := p.broadcastPhaseStart(game.PhaseNight, "天黑了，所有玩家请闭眼"); err != nil {
		return fmt.Errorf("broadcast night phase start failed: %w", err)
	}

	// 处理狼人行动
	if err := p.handleWerewolfActions(ctx); err != nil {
		return fmt.Errorf("handle werewolf actions failed: %w", err)
	}

	// 处理预言家行动
	if err := p.handleSeerActions(ctx); err != nil {
		return fmt.Errorf("handle seer actions failed: %w", err)
	}

	// 处理守卫行动
	if err := p.handleGuardActions(ctx); err != nil {
		return fmt.Errorf("handle guard actions failed: %w", err)
	}

	// 处理女巫行动
	if err := p.handleWitchActions(ctx); err != nil {
		return fmt.Errorf("handle witch actions failed: %w", err)
	}

	phaseResult := p.calculatePhaseResult()

	// 通知所有玩家夜晚阶段结束
	message := "天亮了，所有玩家请睁眼"
	if len(phaseResult.Deaths) > 0 {
		deathNames := make([]string, 0, len(phaseResult.Deaths))
		for _, player := range phaseResult.Deaths {
			deathNames = append(deathNames, player.GetID())
		}

		message = fmt.Sprintf("天亮了，所有玩家请睁眼。昨晚死亡的玩家是：%s", strings.Join(deathNames, "、"))
	}

	if err := p.broadcastPhaseEnd(game.PhaseNight, message); err != nil {
		return fmt.Errorf("broadcast night phase end failed: %w", err)
	}

	return nil
}

// handleWerewolfActions 处理狼人行动
func (p *NightPhase) handleWerewolfActions(ctx context.Context) error {
	// 获取所有存活的狼人
	wolves := p.getAlivePlayerIDsByRole(game.RoleTypeWerewolf)
	if len(wolves) == 0 {
		return nil
	}

	// 通知狼人行动
	if err := p.broadcastSkillResult(game.SkillTypeKill, "狼人请睁眼，请选择要击杀的目标", wolves...); err != nil {
		return err
	}

	// 等待狼人行动
	return p.waitForPlayerActions(ctx, game.RoleTypeWerewolf)
}

// handleSeerActions 处理预言家行动
func (p *NightPhase) handleSeerActions(ctx context.Context) error {
	// 获取所有存活的预言家
	seers := p.getAlivePlayerIDsByRole(game.RoleTypeSeer)
	if len(seers) == 0 {
		return nil
	}

	// 通知预言家行动
	if err := p.broadcastSkillResult(game.SkillTypeCheck, "预言家请睁眼，请选择要查验的目标", seers...); err != nil {
		return err
	}

	// 等待预言家行动
	return p.waitForPlayerActions(ctx, game.RoleTypeSeer)
}

// handleGuardActions 处理守卫行动
func (p *NightPhase) handleGuardActions(ctx context.Context) error {
	// 获取所有存活的守卫
	guards := p.getAlivePlayerIDsByRole(game.RoleTypeGuard)
	if len(guards) == 0 {
		return nil
	}

	// 通知守卫行动
	if err := p.broadcastSkillResult(game.SkillTypeProtect, "守卫请睁眼，请选择要守护的目标", guards...); err != nil {
		return err
	}

	// 等待守卫行动
	return p.waitForPlayerActions(ctx, game.RoleTypeGuard)
}

// handleWitchActions 处理女巫行动
func (p *NightPhase) handleWitchActions(ctx context.Context) error {
	// 获取所有存活的女巫
	witches := p.getAlivePlayerIDsByRole(game.RoleTypeWitch)
	if len(witches) == 0 {
		return nil
	}

	// 通知女巫行动
	if err := p.broadcastSkillResult(game.SkillTypeAntidote, "女巫请睁眼，今晚有人被杀了，你要使用解药救他吗？或者使用毒药？", witches...); err != nil {
		return err
	}

	// 等待女巫行动
	return p.waitForPlayerActions(ctx, game.RoleTypeWitch)
}

// waitForPlayerActions 等待指定角色的玩家完成行动
func (p *NightPhase) waitForPlayerActions(ctx context.Context, roleType game.RoleType) error {
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
		evt, err := player.Read(ctx)
		if err != nil {
			p.logger.Warn("玩家行动超时", "player_id", playerID, "role", roleType)
			continue
		}

		// 处理用户事件
		if evt.Type == event.UserSkill {
			skillData := evt.Data.(*event.UserSkillData)
			// 将用户事件转换为玩家行动
			action := game.Action{
				Caster: player,
				Target: p.players[skillData.TargetID],
				Skill:  p.getSkillByType(game.SkillType(skillData.SkillType)),
			}

			// 执行行动
			if err := action.Skill.Check(p.GetName(), action.Caster, action.Target); err != nil {
				p.logger.Error("技能检查失败", "player_id", playerID, "error", err)
				continue
			}

			p.AddAction(&action)
		}
	}

	return nil
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
