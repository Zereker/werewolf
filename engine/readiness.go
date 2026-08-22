package engine

// PhaseReadiness 当前阶段的行动就绪情况。
//
// 引擎不计时，也不会替调用方决定阶段何时结束——但它手里握着
// 「谁该行动、谁已经行动了」的全部信息，没有理由让调用方自己去数。
//
// # 两个问题，分别由 Pending 与 Optional 回答
//
// 「还差谁**必须**动」和「本阶段谁**可以**动」是两回事，而默认配置里
// 只有狼刀与投票是 Required——守卫、女巫、预言家、猎人全都可以不动。
// 只看 Pending 来驱动游戏，这几个角色一整局都不会被叫到。
//
// 所以 Ready / Pending 只管必需行动（超时就该推进的依据），
// Optional 列出「可以动但还没动」的人（主持人该催一催的依据）。
type PhaseReadiness struct {
	Phase PhaseType // 当前阶段
	Round int       // 当前回合

	// Ready 所有 Required 步骤是否都已满足。
	//
	// 注意它**不**表示「所有人都行动过了」：可选技能不计入。
	// 为 false 时调用方可以继续等待；是否按超时强行推进由调用方决定，
	// EndPhase 不会因未就绪而拒绝。
	Ready bool

	// Pending 还未完成的必需行动。Ready 为 true 时为空。
	Pending []PendingAction

	// Optional 本阶段可以行动、但尚未提交的人。
	//
	// 不影响 Ready——他们不动也是合法的（规则里「或不進行守護」
	// 这类表述说的就是这个）。主持人据此决定要不要多等一会儿。
	Optional []PendingAction

	// Acted 本阶段已提交过技能的玩家，按 ID 排序。
	Acted []string
}

// PendingAction 一项尚未完成的行动
type PendingAction struct {
	PlayerID string    // 该行动的玩家；步骤要求全员行动时逐个列出
	Role     RoleType  // 其角色
	Skill    SkillType // 尚未提交的技能
}

// PhaseReadiness 返回当前阶段还差谁行动。
//
// 没有合格行动者的步骤（例如守卫已出局）视为自动满足，
// 不会让阶段永远卡住。
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

	for _, req := range requirementsOf(phaseConfig.Steps) {
		actors := e.actorsForStep(req.role)
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
		if len(missing) == 0 {
			continue
		}

		if !req.required {
			out.Optional = append(out.Optional, missing...)
			continue
		}

		// Multiple=false 时任意一人完成即可
		if !req.multiple && len(missing) < len(actors) {
			continue
		}
		out.Ready = false
		out.Pending = append(out.Pending, missing...)
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
	required bool        // 不满足就不就绪，还是只是「可以动」
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

// requirementsOf 把步骤列表折成行动列表，保持步骤的先后顺序。
//
// 上帝是系统角色、没有玩家承担，跳过。非 Required 的步骤照样收进来——
// 它们不影响就绪，但要出现在 Optional 里。
func requirementsOf(steps []PhaseStep) []requirement {
	out := make([]requirement, 0, len(steps))
	byGroup := make(map[string]int, len(steps)) // 组名 -> out 中的下标

	for _, step := range steps {
		if step.Role == RoleGod {
			continue
		}
		if i, ok := byGroup[step.Group]; ok && step.Group != "" {
			out[i].accepts = append(out[i].accepts, step.Skill)
			// 组里只要有一个步骤是必需的，整组就是必需的
			out[i].required = out[i].required || step.Required
			continue
		}
		if step.Group != "" {
			byGroup[step.Group] = len(out)
		}
		out = append(out, requirement{
			role:     step.Role,
			skill:    step.Skill,
			accepts:  []SkillType{step.Skill},
			required: step.Required,
			multiple: step.Multiple,
		})
	}
	return out
}

// actorsForStep 该步骤的合格行动者。调用前需持有 e.mu。
//
// 这是「谁可以行动」的**唯一**取数点——技能校验、AllowedSkills、
// PhaseReadiness 三处都从这里取。三个问题一个来源，才不会出现
// 「内核收下了他的提交，却告诉别人他不该行动」这种自相矛盾。
//
// 两层，优先级从高到低：
//
//	点到名的人      NewSetActorsEffect，或者死亡触发在进入阶段时写下的那一份
//	PhaseStep.Role  默认：按角色算，角色是入座时定死的
//
// 此前是三层，最上面还有一层「待结算的触发」。那一层与点名回答的是同一个
// 问题，实现也几乎逐字相同——一个概念两份实现，两处都要记得对齐。现在
// 触发队列不再回答「谁能行动」，它在进入阶段时产出一份名单
// （见 gameState.namePendingTriggerActor），之后一切照点名走。
func (e *Engine) actorsForStep(role RoleType) []string {
	if ids, ok := e.state.actorsFor(e.state.Phase); ok {
		return e.namedActorsFor(role, ids)
	}
	if role == RoleUnspecified {
		return sortedStrings(e.state.getAlivePlayerIDs())
	}
	return sortedStrings(e.state.getAlivePlayerIDsByRole(role))
}

// namedActorsFor 规则点名的那些人里，谁承担这个角色的步骤。调用前需持有 e.mu。
//
// 与 triggerActorFor 同一个道理：点名不等于「他要替所有角色行动」。
// 步骤声明了具体角色的，只有角色相符的人算数；声明 RoleUnspecified 的，
// 点到的人都算。
//
// **不按存活过滤**：规则点名谁，谁就能行动。此前这里会把已出局的人剔掉，
// 而那是内核在替规则做判断——同一个内核，却允许**自己的**触发队列让死人
// 行动（猎人被刀之后开枪），不允许**规则的**点名这么做，自相矛盾。
//
// 挡掉的是真实存在的玩法：血染钟楼的死人保留一张「幽灵票」，狼人杀的
// 遗言阶段同理。存活与否是规则的判断，点名就是规则在判断。
func (e *Engine) namedActorsFor(role RoleType, ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		p, ok := e.state.getPlayer(id)
		if !ok {
			continue
		}
		if role != RoleUnspecified && p.Role != role {
			continue
		}
		out = append(out, id)
	}
	return sortedStrings(out)
}
