package phase

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/event"
	"github.com/Zereker/werewolf/pkg/game/skill"
)

// DayPhase 白天阶段
type DayPhase struct {
	round          int
	players        map[string]game.Player
	actions        []*game.Action
	skillResults   game.SkillResultMap
	discussionTime time.Duration
}

func NewDayPhase(round int, players []game.Player) *DayPhase {
	playerMap := make(map[string]game.Player)
	for _, player := range players {
		playerMap[player.GetID()] = player
	}

	return &DayPhase{
		round:          round,
		players:        playerMap,
		actions:        make([]*game.Action, 0),
		skillResults:   make(game.SkillResultMap),
		discussionTime: 5 * time.Minute, // 默认讨论时间为5分钟
	}
}

func (d *DayPhase) GetName() game.PhaseType {
	return game.PhaseDay
}

// broadcastEvent 广播事件
func (d *DayPhase) broadcastEvent(evt any) error {
	// 将事件转换为 event.Event[any] 类型
	eventAny, ok := evt.(event.Event[any])
	if !ok {
		return fmt.Errorf("invalid event type: %T", evt)
	}

	for _, receiverID := range eventAny.Receivers {
		if player, exists := d.players[receiverID]; exists {
			if err := player.Write(eventAny); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *DayPhase) Start() error {
	// 通知所有玩家进入白天
	if err := d.broadcastEvent(event.Event[event.PhaseStartData]{
		Type: event.EventSystemPhaseStart,
		Data: event.PhaseStartData{
			Phase:   string(game.PhaseDay),
			Round:   d.round,
			Message: "现在是白天，所有玩家可以自由讨论",
		},
		Receivers: d.getAllPlayerIDs(),
		Timestamp: time.Now(),
	}); err != nil {
		return fmt.Errorf("broadcast day phase start failed: %w", err)
	}

	// 等待玩家发言
	if err := d.waitForSpeeches(); err != nil {
		return err
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
				if err := d.broadcastEvent(event.Event[event.SkillResultData]{
					Type: event.EventSystemSkillResult,
					Data: event.SkillResultData{
						SkillType: string(game.SkillTypeSpeak),
						Message:   message,
					},
					Receivers: d.getAllPlayerIDs(),
					Timestamp: time.Now(),
				}); err != nil {
					return fmt.Errorf("broadcast speech results failed: %w", err)
				}
			}
		}
	}

	// 通知所有玩家白天阶段结束
	if err := d.broadcastEvent(event.Event[event.PhaseStartData]{
		Type: event.EventSystemPhaseEnd,
		Data: event.PhaseStartData{
			Phase:   string(game.PhaseDay),
			Round:   d.round,
			Message: "白天讨论时间结束，进入投票阶段",
		},
		Receivers: d.getAllPlayerIDs(),
		Timestamp: time.Now(),
	}); err != nil {
		return fmt.Errorf("broadcast day phase end failed: %w", err)
	}

	return nil
}

// waitForSpeeches 等待所有玩家发言
func (d *DayPhase) waitForSpeeches() error {
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
				Target: d.players[skillData.TargetID],
				Skill:  d.getSkillByType(game.SkillTypeSpeak),
			}
			// 执行行动
			if err := action.Skill.Check(d.GetName(), action.Caster, action.Target); err != nil {
				continue
			}
			action.Skill.Put(action.Caster, action.Target, game.PutOption{})
			d.actions = append(d.actions, &action)

			// 广播发言给其他玩家
			if err := d.broadcastEvent(event.Event[event.UserSpeakData]{
				Type:      event.EventUserSpeak,
				PlayerID:  playerID,
				Receivers: d.getAlivePlayerIDs(),
				Timestamp: time.Now(),
				Data: event.UserSpeakData{
					Message: skillData.Content,
				},
			}); err != nil {
				return fmt.Errorf("broadcast speech failed: %w", err)
			}
		}
	}

	return nil
}

// GetPhaseResult 获取阶段结果
func (d *DayPhase) GetPhaseResult() *game.PhaseResult[game.SkillResultMap] {
	// 按优先级排序所有行为
	sort.Slice(d.actions, func(i, j int) bool {
		return d.actions[i].Skill.GetPriority() < d.actions[j].Skill.GetPriority()
	})

	// 执行所有行为
	speakResults := make(map[game.Player]string)
	lastWordsResults := make(map[game.Player]string)

	for _, action := range d.actions {
		// 执行技能，传入内容选项
		action.Skill.Put(action.Caster, action.Target, game.PutOption{
			Content: action.Content,
		})

		// 根据技能类型记录结果
		switch action.Skill.GetName() {
		case game.SkillTypeSpeak:
			if speak, ok := action.Skill.(*skill.Speak); ok {
				speakResults[action.Caster] = speak.GetContent()
			}
		case game.SkillTypeLastWords:
			if lastWords, ok := action.Skill.(*skill.LastWords); ok {
				lastWordsResults[action.Caster] = lastWords.GetContent()
			}
		}
	}

	// 记录发言结果
	d.skillResults[game.SkillTypeSpeak] = &game.SkillResult{
		Success: true,
		Message: "发言结果",
		Data: map[string]interface{}{
			"spoken": speakResults,
		},
	}

	// 记录遗言结果
	if len(lastWordsResults) > 0 {
		d.skillResults[game.SkillTypeLastWords] = &game.SkillResult{
			Success: true,
			Message: "遗言结果",
			Data: map[string]interface{}{
				"lastWords": lastWordsResults,
			},
		}
	}

	return &game.PhaseResult[game.SkillResultMap]{
		ExtraData: d.skillResults,
	}
}

// getAlivePlayerIDs 获取所有存活的玩家ID
func (d *DayPhase) getAlivePlayerIDs() []string {
	ids := make([]string, 0)
	for id, player := range d.players {
		if player.IsAlive() {
			ids = append(ids, id)
		}
	}
	return ids
}

// getAllPlayerIDs 获取所有玩家ID
func (d *DayPhase) getAllPlayerIDs() []string {
	ids := make([]string, 0, len(d.players))
	for id := range d.players {
		ids = append(ids, id)
	}
	return ids
}

// getSkillByType 获取指定类型的技能
func (d *DayPhase) getSkillByType(skillType game.SkillType) game.Skill {
	for _, player := range d.players {
		for _, s := range player.GetRole().GetAvailableSkills() {
			if s.GetName() == skillType {
				return s
			}
		}
	}
	return nil
}
