package phase

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/event"
)

// DayPhase 白天阶段
type DayPhase struct {
	*BasePhase

	discussionTime time.Duration
}

func NewDayPhase(players []game.Player) *DayPhase {
	return &DayPhase{
		BasePhase:      NewBasePhase(players),
		discussionTime: 5 * time.Minute, // 默认讨论时间为5分钟
	}
}

func (d *DayPhase) GetName() game.PhaseType {
	return game.PhaseDay
}

// Start 开始阶段
func (d *DayPhase) Start(ctx context.Context) error {
	// 通知所有玩家进入白天
	if err := d.broadcastPhaseStart(game.PhaseDay, "现在是白天，所有玩家可以自由讨论"); err != nil {
		return errors.WithMessage(err, "broadcast day phase start failed")
	}

	// 等待玩家发言
	if err := d.waitForSpeeches(ctx); err != nil {
		return errors.WithMessage(err, "wait user speech failed")
	}

	// 广播所有玩家的发言结果
	phaseResult := d.GetPhaseResult()
	for _, result := range phaseResult {
		if err := d.broadcastSkillResult(game.SkillTypeSpeak, result.Message); err != nil {
			return fmt.Errorf("broadcast speech results failed: %w", err)
		}
	}

	// 通知所有玩家白天阶段结束
	if err := d.broadcastPhaseEnd(game.PhaseDay, "白天讨论结束"); err != nil {
		return fmt.Errorf("broadcast day phase end failed: %w", err)
	}

	return nil
}

// waitForSpeeches 等待所有玩家发言
func (d *DayPhase) waitForSpeeches(ctx context.Context) error {
	// 获取所有存活的玩家
	alivePlayers := d.getAlivePlayerIDs()
	if len(alivePlayers) == 0 {
		return nil
	}

	for _, playerID := range alivePlayers {
		player := d.players[playerID]
		if player == nil {
			continue
		}

		// 等待该玩家的发言
		timeout := d.discussionTime / time.Duration(len(alivePlayers))
		evt, err := d.waitPlayer(ctx, player, timeout)
		if err != nil {
			continue
		}

		// 处理用户事件
		if evt.Type != event.UserSkill {
			continue
		}

		action, err := d.convertEventToAction(evt)
		if err != nil {
			continue
		}

		// 执行行动
		if err := action.Skill.Check(d.GetName(), action.Caster, action.Target); err != nil {
			continue
		}

		d.AddAction(action)
	}

	return nil
}

// GetPhaseResult 获取阶段结果
func (d *DayPhase) GetPhaseResult() game.UserSkillResultMap {
	// 执行所有行为
	speakResults := make(map[game.Player]*game.SkillResult)

	for _, action := range d.actions {
		var result game.SkillResult
		action.Skill.Put(action.Caster, action.Target, &result)
		speakResults[action.Caster] = &result
	}

	return speakResults
}
