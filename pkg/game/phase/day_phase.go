package phase

import (
	"sort"

	"github.com/Zereker/werewolf/pkg/game"
)

// DayPhase 白天阶段
type DayPhase struct {
	actions      []*game.Action
	skillResults game.SkillResultMap
}

func NewDayPhase() *DayPhase {
	return &DayPhase{
		actions:      make([]*game.Action, 0),
		skillResults: make(game.SkillResultMap),
	}
}

func (d *DayPhase) GetName() game.PhaseType {
	return game.PhaseDay
}

func (d *DayPhase) Handle(action *game.Action) error {
	// 检查技能
	if err := action.Skill.Check(d.GetName(), action.Caster, action.Target); err != nil {
		return err
	}

	// 记录行为
	d.actions = append(d.actions, action)

	return nil
}

// GetPhaseResult 获取阶段结果
func (d *DayPhase) GetPhaseResult() *game.PhaseResult[game.SkillResultMap] {
	// 按优先级排序所有行为
	sort.Slice(d.actions, func(i, j int) bool {
		return d.actions[i].Skill.GetPriority() < d.actions[j].Skill.GetPriority()
	})

	// 执行所有行为
	speakResults := make(map[game.Player]bool)     // 记录每个玩家的发言状态
	lastWordsResults := make(map[game.Player]bool) // 记录每个玩家的遗言状态

	for _, action := range d.actions {
		// 执行技能
		action.Skill.Put(action.Caster, action.Target)

		// 根据技能类型记录结果
		switch action.Skill.GetName() {
		case game.SkillTypeSpeak:
			speakResults[action.Caster] = true
		case game.SkillTypeLastWords:
			lastWordsResults[action.Caster] = true
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

func (d *DayPhase) Reset() {
	d.skillResults = make(game.SkillResultMap)
	d.actions = make([]*game.Action, 0)
}
