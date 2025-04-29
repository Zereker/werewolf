package phase

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Zereker/werewolf/pkg/event"
	"github.com/Zereker/werewolf/pkg/skill"

	"github.com/Zereker/werewolf/pkg/game"
)

// LastWordPhase 遗言阶段
type LastWordPhase struct {
	*BasePhase
	deaths []game.Player
}

func NewLastWordPhase(round int, players []game.Player, deaths []game.Player) *LastWordPhase {
	return &LastWordPhase{
		BasePhase: NewBasePhase(players),
		deaths:    deaths,
	}
}

func (l *LastWordPhase) GetName() game.PhaseType {
	return game.PhaseLastWord
}

func (l *LastWordPhase) Start() error {
	// 通知所有玩家进入遗言阶段
	if err := l.broadcastPhaseStart(game.PhaseLastWord, "现在是遗言阶段，请死亡的玩家发表遗言"); err != nil {
		return fmt.Errorf("broadcast last word phase start failed: %w", err)
	}

	// 等待死亡的玩家发表遗言
	if err := l.waitForLastWords(); err != nil {
		return err
	}

	// 计算阶段结果
	phaseResult := l.GetPhaseResult()

	// 广播所有玩家的遗言结果
	if skillResult, ok := phaseResult.ExtraData[game.SkillTypeLastWords]; ok {
		if data, ok := skillResult.Data.(map[string]interface{}); ok {
			if lastWords, ok := data["lastWords"].(map[game.Player]string); ok {
				// 构建遗言结果消息
				var messages []string
				for player, content := range lastWords {
					messages = append(messages, fmt.Sprintf("%s: %s", player.GetID(), content))
				}
				message := "遗言阶段结束，以下是所有玩家的遗言：\n" + strings.Join(messages, "\n")

				// 广播遗言结果
				if err := l.broadcastSkillResult(game.SkillTypeLastWords, message); err != nil {
					return fmt.Errorf("broadcast last words results failed: %w", err)
				}
			}
		}
	}

	// 通知所有玩家遗言阶段结束
	if err := l.broadcastPhaseEnd(game.PhaseLastWord, "遗言阶段结束，进入下一个阶段"); err != nil {
		return fmt.Errorf("broadcast last word phase end failed: %w", err)
	}

	return nil
}

// waitForLastWords 等待死亡的玩家发表遗言
func (l *LastWordPhase) waitForLastWords() error {
	if len(l.deaths) == 0 {
		return nil
	}

	// 为每个死亡的玩家创建一个等待组
	for _, player := range l.deaths {
		// 等待该玩家的遗言
		evt, err := player.Read(30 * time.Second)
		if err != nil {
			continue
		}

		// 处理用户事件
		if evt.Type == event.EventUserSkill {
			skillData := evt.Data.(*event.UserSkillData)
			// 将用户事件转换为玩家行动
			action := game.Action{
				Caster: player,
				Target: l.players[skillData.TargetID],
				Skill:  l.getSkillByType(game.SkillTypeLastWords),
			}
			// 执行行动
			if err := action.Skill.Check(l.GetName(), action.Caster, action.Target); err != nil {
				continue
			}
			action.Skill.Put(action.Caster, action.Target, game.PutOption{})
			l.AddAction(&action)
		}
	}

	return nil
}

// GetPhaseResult 获取阶段结果
func (l *LastWordPhase) GetPhaseResult() *game.PhaseResult[game.SkillResultMap] {
	// 按优先级排序所有行为
	sort.Slice(l.actions, func(i, j int) bool {
		return l.actions[i].Skill.GetPriority() < l.actions[j].Skill.GetPriority()
	})

	// 执行所有行为
	lastWordsResults := make(map[game.Player]string)

	for _, action := range l.actions {
		// 执行技能，传入内容选项
		action.Skill.Put(action.Caster, action.Target, game.PutOption{
			Content: action.Content,
		})

		// 记录遗言结果
		if lastWords, ok := action.Skill.(*skill.LastWords); ok {
			lastWordsResults[action.Caster] = lastWords.GetContent()
		}
	}

	// 记录遗言结果
	l.AddSkillResult(game.SkillTypeLastWords, &game.SkillResult{
		Success: true,
		Message: "遗言结果",
		Data: map[string]interface{}{
			"lastWords": lastWordsResults,
		},
	})

	return &game.PhaseResult[game.SkillResultMap]{
		ExtraData: l.skillResults,
	}
}
