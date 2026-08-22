package werewolf

import (
	pb "github.com/Zereker/werewolf/proto"
)

// PhaseReadiness 当前阶段的行动就绪情况。
//
// 引擎不计时，也不会替调用方决定阶段何时结束——但它手里握着
// 「谁该行动、谁已经行动了」的全部信息，没有理由让调用方自己去数。
// 此前 PhaseStep.Required / Multiple 声明了这层语义却从不读取，
// 调用方只能自己维护一份「还差谁」的账。
type PhaseReadiness struct {
	Phase pb.PhaseType // 当前阶段
	Round int          // 当前回合

	// Ready 所有 Required 步骤是否都已满足。
	// 为 false 时调用方可以继续等待；是否按超时强行推进由调用方决定，
	// EndPhase 不会因未就绪而拒绝。
	Ready bool

	// Pending 还未完成的必需行动。Ready 为 true 时为空。
	Pending []PendingAction

	// Acted 本阶段已提交过技能的玩家，按 ID 排序。
	Acted []string
}

// PendingAction 一项尚未完成的必需行动
type PendingAction struct {
	PlayerID string       // 该行动的玩家；步骤要求全员行动时逐个列出
	Role     pb.RoleType  // 其角色
	Skill    pb.SkillType // 尚未提交的技能
}

// PhaseReadiness 返回当前阶段还差谁行动。
//
// 判定只针对 Required 步骤；没有合格行动者的步骤（例如守卫已出局）
// 视为自动满足，不会让阶段永远卡住。
func (e *Engine) PhaseReadiness() PhaseReadiness {
	e.mu.RLock()
	defer e.mu.RUnlock()

	out := PhaseReadiness{
		Phase: e.state.Phase,
		Round: e.state.Round,
		Ready: true,
	}

	// 已提交记录：playerID -> 该玩家提交过的技能集合
	submitted := make(map[string]map[pb.SkillType]bool, len(e.pendingUses))
	for _, use := range e.pendingUses {
		if submitted[use.PlayerID] == nil {
			submitted[use.PlayerID] = make(map[pb.SkillType]bool)
		}
		submitted[use.PlayerID][use.Skill] = true
	}
	for _, id := range e.state.allPlayerIDs() {
		if submitted[id] != nil {
			out.Acted = append(out.Acted, id)
		}
	}

	phaseConfig := e.phase.phaseConfig(e.state.Phase)
	if phaseConfig == nil {
		return out
	}

	trigger, hasTrigger := e.state.peekTrigger()
	triggerActive := hasTrigger && trigger.Phase == e.state.Phase

	// 互斥备选组：同组的任一技能都算完成，逐步骤独立判定会把
	// 「猎人已明确表示不开枪」记成「还欠着开枪」
	groupSkills := groupSkillsOf(phaseConfig.Steps)

	for _, step := range phaseConfig.Steps {
		// 上帝是系统角色，没有玩家承担
		if !step.Required || step.Role == pb.RoleType_ROLE_TYPE_GOD {
			continue
		}

		actors := e.actorsForStep(step.Role, triggerActive, trigger)
		if len(actors) == 0 {
			// 无人能承担该步骤，视为自动满足
			continue
		}

		accepted := groupSkills[step.Group]
		if accepted == nil {
			accepted = []pb.SkillType{step.Skill}
		}

		var missing []PendingAction
		for _, id := range actors {
			if !submittedAny(submitted[id], accepted) {
				missing = append(missing, PendingAction{
					PlayerID: id,
					Role:     step.Role,
					Skill:    step.Skill,
				})
			}
		}

		// Multiple=false 时任意一人完成即可
		if !step.Multiple && len(missing) < len(actors) {
			continue
		}
		if len(missing) > 0 {
			out.Ready = false
			out.Pending = append(out.Pending, missing...)
		}
	}

	// 同一个组会被组里每个步骤各报一次，去重后调用方看到的才是
	// 「这个人还欠一次行动」而不是「他欠了开枪又欠了不开枪」
	out.Pending = dedupPending(out.Pending, groupOfStep(phaseConfig.Steps))

	return out
}

// groupSkillsOf 归拢互斥备选组：组名 -> 该组接受的全部技能。
func groupSkillsOf(steps []PhaseStep) map[string][]pb.SkillType {
	out := make(map[string][]pb.SkillType)
	for _, step := range steps {
		if step.Group == "" {
			continue
		}
		out[step.Group] = append(out[step.Group], step.Skill)
	}
	return out
}

// groupOfStep 技能 -> 它所属的互斥备选组名。
func groupOfStep(steps []PhaseStep) map[pb.SkillType]string {
	out := make(map[pb.SkillType]string)
	for _, step := range steps {
		if step.Group != "" {
			out[step.Skill] = step.Group
		}
	}
	return out
}

// submittedAny 该玩家是否提交过其中任意一个技能。
func submittedAny(done map[pb.SkillType]bool, skills []pb.SkillType) bool {
	for _, skill := range skills {
		if done[skill] {
			return true
		}
	}
	return false
}

// dedupPending 同一玩家在同一互斥备选组里只保留一条待办。
func dedupPending(pending []PendingAction, group map[pb.SkillType]string) []PendingAction {
	if len(pending) == 0 {
		return pending
	}

	type key struct {
		player string
		group  string
	}
	seen := make(map[key]bool, len(pending))
	out := pending[:0]
	for _, p := range pending {
		g, inGroup := group[p.Skill]
		if !inGroup {
			out = append(out, p)
			continue
		}
		k := key{player: p.PlayerID, group: g}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, p)
	}
	return out
}

// actorsForStep 该步骤的合格行动者。调用前需持有 e.mu。
func (e *Engine) actorsForStep(role pb.RoleType, triggerActive bool, trigger PendingTrigger) []string {
	if triggerActive {
		// 死亡技能阶段只有触发者能行动，但他只承担与自己角色相符的步骤。
		// 无视 role 一律返回触发者，会让复用了多角色阶段配置的自定义
		// 死亡技能阶段声称「触发者要替所有角色行动」。
		return e.triggerActorFor(role, trigger)
	}
	if role == pb.RoleType_ROLE_TYPE_UNSPECIFIED {
		return sortedStrings(e.state.getAlivePlayerIDs())
	}
	return sortedStrings(e.state.getAlivePlayerIDsByRole(role))
}

// triggerActorFor 触发者是否承担该角色的步骤。调用前需持有 e.mu。
func (e *Engine) triggerActorFor(role pb.RoleType, trigger PendingTrigger) []string {
	if role == pb.RoleType_ROLE_TYPE_UNSPECIFIED {
		return []string{trigger.PlayerID}
	}
	p, ok := e.state.getPlayer(trigger.PlayerID)
	if !ok || p.Role != role {
		return nil
	}
	return []string{trigger.PlayerID}
}
