package phase

import (
	"fmt"
	"sort"

	"github.com/Zereker/werewolf/pkg/game"
)

// NightPhase 夜晚阶段
type NightPhase struct {
	deaths       []game.Player
	skillResults game.SkillResultMap
	actions      []*game.Action
}

func NewNightPhase() *NightPhase {
	return &NightPhase{
		deaths:       make([]game.Player, 0),
		skillResults: make(game.SkillResultMap),
		actions:      make([]*game.Action, 0),
	}
}

func (n *NightPhase) GetName() game.PhaseType {
	return game.PhaseNight
}

func (n *NightPhase) Handle(action *game.Action) error {
	// 检查技能
	if err := action.Skill.Check(n.GetName(), action.Caster, action.Target); err != nil {
		return err
	}

	// 记录行为
	n.actions = append(n.actions, action)

	return nil
}

func (n *NightPhase) GetPhaseResult() *game.PhaseResult[game.SkillResultMap] {
	// 按优先级排序所有行为
	sort.Slice(n.actions, func(i, j int) bool {
		return n.actions[i].Skill.GetPriority() < n.actions[j].Skill.GetPriority()
	})

	// 执行所有行为
	for _, action := range n.actions {
		// 执行技能
		action.Skill.Put(action.Caster, action.Target, game.PutOption{
			Content: action.Content,
		})

		// 记录死亡玩家
		if !action.Target.IsAlive() {
			n.deaths = append(n.deaths, action.Target)
		}

		// 记录技能结果
		n.skillResults[action.Skill.GetName()] = &game.SkillResult{
			Success: true,
			Message: fmt.Sprintf("%s skill result", action.Skill.GetName()),
			Data: map[string]interface{}{
				"target": action.Target.GetRole().GetName(),
			},
		}
	}

	return &game.PhaseResult[game.SkillResultMap]{
		Deaths:    n.deaths,
		ExtraData: n.skillResults,
	}
}

func (n *NightPhase) Reset() {
	n.deaths = make([]game.Player, 0)
	n.skillResults = make(game.SkillResultMap)
	n.actions = make([]*game.Action, 0)
}
