package phase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/event"
	"github.com/Zereker/werewolf/pkg/game/skill"
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

	// 计算阶段结果
	phaseResult := d.GetPhaseResult()

	// 广播所有玩家的发言结果
	if skillResult, ok := phaseResult.ExtraData[game.SkillTypeSpeak]; ok {
		if data, ok := skillResult.Data.(map[string]interface{}); ok {
			if spoken, ok := data["spoken"].(map[game.Player]string); ok {
				// 构建发言结果消息
				var messages []string
				for player, content := range spoken {
					messages = append(messages, fmt.Sprintf("%s: %s", player.GetID(), content))
				}
				message := "白天讨论结束，以下是所有玩家的发言：\n" + strings.Join(messages, "\n")

				// 广播发言结果
				if err := d.broadcastSkillResult(game.SkillTypeSpeak, message); err != nil {
					return fmt.Errorf("broadcast speech results failed: %w", err)
				}
			}
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

	// 为每个玩家创建一个等待组
	for _, playerID := range alivePlayers {
		player := d.players[playerID]
		if player == nil {
			continue
		}

		// 等待该玩家的发言
		evt, err := d.waitPlayer(ctx, player, d.discussionTime/time.Duration(len(alivePlayers)))
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

func (d *DayPhase) waitPlayer(ctx context.Context, player game.Player, timeout time.Duration) (event.Event[any], error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 等待该玩家的发言
	evt, err := player.Read(ctx)
	if err != nil {
		return event.Event[any]{}, err
	}

	return evt, nil
}

// GetPhaseResult 获取阶段结果
func (d *DayPhase) GetPhaseResult() *game.PhaseResult[game.SkillResultMap] {
	// 执行所有行为
	speakResults := make(map[game.Player]string)

	for _, action := range d.actions {
		// 执行技能，传入内容选项
		action.Skill.Put(action.Caster, action.Target)
		if speak, ok := action.Skill.(*skill.Speak); ok {
			speakResults[action.Caster] = speak.GetContent()
		}
	}

	// 记录发言结果
	d.AddSkillResult(game.SkillTypeSpeak, &game.SkillResult{
		Success: true,
		Message: "发言结果",
		Data: map[string]interface{}{
			"spoken": speakResults,
		},
	})

	return &game.PhaseResult[game.SkillResultMap]{
		ExtraData: d.skillResults,
	}
}
