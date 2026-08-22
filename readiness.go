package werewolf

import ()

// PhaseReadiness 当前阶段的行动就绪情况。
//
// 引擎不计时，也不会替调用方决定阶段何时结束——但它手里握着
// 「谁该行动、谁已经行动了」的全部信息，没有理由让调用方自己去数。
// 此前 PhaseStep.Required / Multiple 声明了这层语义却从不读取，
// 调用方只能自己维护一份「还差谁」的账。
type PhaseReadiness struct {
	Phase PhaseType // 当前阶段
	Round int       // 当前回合

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
	PlayerID string    // 该行动的玩家；步骤要求全员行动时逐个列出
	Role     RoleType  // 其角色
	Skill    SkillType // 尚未提交的技能
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
	submitted := make(map[string]map[SkillType]bool, len(e.pendingUses))
	for _, use := range e.pendingUses {
		if submitted[use.PlayerID] == nil {
			submitted[use.PlayerID] = make(map[SkillType]bool)
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

	for _, req := range requirementsOf(phaseConfig.Steps) {
		actors := e.actorsForStep(req.role, triggerActive, trigger)
		if len(actors) == 0 {
			// 无人能承担该步骤，视为自动满足
			continue
		}

		var missing []PendingAction
		for _, id := range actors {
			if req.satisfiedBy(submitted[id]) {
				continue
			}
			missing = append(missing, PendingAction{
				PlayerID: id,
				Role:     req.role,
				Skill:    req.skill,
			})
		}

		// Multiple=false 时任意一人完成即可
		if !req.multiple && len(missing) < len(actors) {
			continue
		}
		if len(missing) > 0 {
			out.Ready = false
			out.Pending = append(out.Pending, missing...)
		}
	}

	return out
}

// requirement 一项必需行动：某个角色，提交 accepts 里任意一个技能即算完成。
//
// 判定的单位是「一次行动」而不是「一个步骤」。互斥备选组（猎人的开枪与
// 不开枪）在配置里是两个步骤、在这里是一项要求——按步骤逐个判定的话，
// 猎人明确表示不开枪之后仍会被记成欠着开枪，还得再跑一遍去重把同一个人
// 的两条待办合并回去。
type requirement struct {
	role     RoleType
	skill    SkillType   // 报给调用方的代表技能，取组里第一个
	accepts  []SkillType // 组里的全部技能
	multiple bool
}

// satisfiedBy 该玩家提交过的技能里是否有本项要求接受的。
func (r requirement) satisfiedBy(done map[SkillType]bool) bool {
	for _, skill := range r.accepts {
		if done[skill] {
			return true
		}
	}
	return false
}

// requirementsOf 把步骤列表折成必需行动列表，保持步骤的先后顺序。
//
// 上帝是系统角色、没有玩家承担，非 Required 的步骤不参与就绪判定，两者都跳过。
func requirementsOf(steps []PhaseStep) []requirement {
	out := make([]requirement, 0, len(steps))
	byGroup := make(map[string]int, len(steps)) // 组名 -> out 中的下标

	for _, step := range steps {
		if !step.Required || step.Role == RoleGod {
			continue
		}
		if i, ok := byGroup[step.Group]; ok && step.Group != "" {
			out[i].accepts = append(out[i].accepts, step.Skill)
			continue
		}
		if step.Group != "" {
			byGroup[step.Group] = len(out)
		}
		out = append(out, requirement{
			role:     step.Role,
			skill:    step.Skill,
			accepts:  []SkillType{step.Skill},
			multiple: step.Multiple,
		})
	}
	return out
}

// actorsForStep 该步骤的合格行动者。调用前需持有 e.mu。
func (e *Engine) actorsForStep(role RoleType, triggerActive bool, trigger PendingTrigger) []string {
	if triggerActive {
		// 死亡技能阶段只有触发者能行动，但他只承担与自己角色相符的步骤。
		// 无视 role 一律返回触发者，会让复用了多角色阶段配置的自定义
		// 死亡技能阶段声称「触发者要替所有角色行动」。
		return e.triggerActorFor(role, trigger)
	}
	if role == RoleUnspecified {
		return sortedStrings(e.state.getAlivePlayerIDs())
	}
	return sortedStrings(e.state.getAlivePlayerIDsByRole(role))
}

// triggerActorFor 触发者是否承担该角色的步骤。调用前需持有 e.mu。
func (e *Engine) triggerActorFor(role RoleType, trigger PendingTrigger) []string {
	if role == RoleUnspecified {
		return []string{trigger.PlayerID}
	}
	p, ok := e.state.getPlayer(trigger.PlayerID)
	if !ok || p.Role != role {
		return nil
	}
	return []string{trigger.PlayerID}
}
