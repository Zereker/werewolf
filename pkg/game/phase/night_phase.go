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

func (n *NightPhase) IsCompleted() bool {
	// 检查所有夜晚技能是否都已使用
	usedSkills := make(map[game.SkillType]bool)
	for _, action := range n.actions {
		usedSkills[action.Skill.GetName()] = true
	}

	// 检查必须使用的技能是否都已使用
	requiredSkills := []game.SkillType{
		game.SkillTypeKill,  // 狼人必须杀人
		game.SkillTypeCheck, // 预言家必须验人
	}

	for _, s := range requiredSkills {
		if !usedSkills[s] {
			return false
		}
	}

	return true
}

// GetPhaseResult 获取阶段结果
func (n *NightPhase) GetPhaseResult() *game.PhaseResult[game.SkillResultMap] {
	// 按优先级排序所有行为（优先级数字小的先执行）
	sort.Slice(n.actions, func(i, j int) bool {
		return n.actions[i].Skill.GetPriority() < n.actions[j].Skill.GetPriority()
	})

	// 执行所有行为
	for _, action := range n.actions {
		// 执行技能，技能自己负责修改玩家状态
		action.Skill.Put(action.Caster, action.Target, game.PutOption{
			Content: action.Content,
		})

		// 记录技能结果
		n.skillResults[action.Skill.GetName()] = &game.SkillResult{
			Success: true,
			Message: fmt.Sprintf("%s skill result", action.Skill.GetName()),
			Data: map[string]interface{}{
				"target": action.Target.GetRole().GetName(),
			},
		}
	}

	// 所有技能执行完后，收集死亡玩家
	n.deaths = make([]game.Player, 0)
	targetPlayers := make(map[game.Player]struct{})

	// 收集所有涉及到的目标玩家
	for _, action := range n.actions {
		targetPlayers[action.Target] = struct{}{}
	}

	// 检查所有涉及到的玩家的最终状态
	for player := range targetPlayers {
		if !player.IsAlive() {
			n.deaths = append(n.deaths, player)
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
