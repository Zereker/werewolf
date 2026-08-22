package engine

import ()

// phaseManager 阶段管理器
type phaseManager struct {
	config    *GameConfig
	resolvers map[PhaseType]Resolver
}

// newPhaseManager 创建阶段管理器。
//
// 不装任何默认解析器：内核不知道哪个阶段该由谁结算。狼人杀的那一批
// 由 werewolf.Options 作为构造选项传进来。
func newPhaseManager(config *GameConfig) *phaseManager {
	return &phaseManager{
		config:    config,
		resolvers: make(map[PhaseType]Resolver, 8),
	}
}

// registerResolver 注册或替换某阶段的解析器
func (p *phaseManager) registerResolver(phase PhaseType, r Resolver) {
	p.resolvers[phase] = r
}

// validateResolvers 检查每个已配置的阶段都注册了解析器。
//
// 缺失解析器不会报错，只会让该阶段收到的技能被悄悄丢弃——这种失败
// 在对局中几乎无法定位，必须在开局前拦下。
func (p *phaseManager) validateResolvers() error {
	for phaseType := range p.config.Phases {
		if p.resolvers[phaseType] == nil {
			return WrapError(CodeInvalidPhase,
				"phase %v has no resolver registered", phaseType)
		}
	}
	return nil
}

// phaseConfig 获取阶段配置
func (p *phaseManager) phaseConfig(phase PhaseType) *PhaseConfig {
	return p.config.Phases[phase]
}

// resolver 获取阶段解析器
func (p *phaseManager) resolver(phase PhaseType) Resolver {
	return p.resolvers[phase]
}

// stepFor 找出「这个角色在这个阶段用这个技能」对应的步骤声明。
//
// 与 allowedSkills 共用同一份判定：RoleUnspecified 表示「所有角色」。
// 找不到即这个技能此刻不被允许。
func (p *phaseManager) stepFor(phase PhaseType, role RoleType, skill SkillType) (PhaseStep, bool) {
	pc := p.phaseConfig(phase)
	if pc == nil {
		return PhaseStep{}, false
	}
	for _, step := range pc.Steps {
		if step.Skill != skill {
			continue
		}
		if step.Role == role || step.Role == RoleUnspecified {
			return step, true
		}
	}
	return PhaseStep{}, false
}

// allowedSkills 获取指定角色在当前阶段允许的技能
func (p *phaseManager) allowedSkills(phase PhaseType, role RoleType) []SkillType {
	config := p.phaseConfig(phase)
	if config == nil {
		return []SkillType{}
	}

	skills := make([]SkillType, 0)
	for _, step := range config.Steps {
		// UNSPECIFIED 表示所有角色都可以
		if step.Role == role || step.Role == RoleUnspecified {
			skills = append(skills, step.Skill)
		}
	}

	return skills
}

// nextSubPhase 计算下一阶段（使用声明式配置）
func (p *phaseManager) nextSubPhase(current PhaseType) PhaseType {
	// 游戏开始阶段的特殊处理
	if current == PhaseStart {
		return p.config.startPhase()
	}

	// 从配置中获取下一阶段
	config := p.phaseConfig(current)
	if config != nil && config.NextPhase != PhaseUnspecified {
		return config.NextPhase
	}

	// 配置中未找到，返回 END
	return PhaseEnd
}

// validateSkillUse 验证技能使用是否合法
func (p *phaseManager) validateSkillUse(use *SkillUse, state *gameState) error {
	// 检查玩家是否存在
	player, ok := state.getPlayer(use.PlayerID)
	if !ok {
		return ErrPlayerNotFound
	}

	// 死亡技能阶段：技能的持有者即便已经出局也可以行动，
	// 但仅限「本次触发的那名玩家」——否则任何已出局的同角色玩家
	// 都能在该阶段再用一次技能。
	if t, ok := state.peekTrigger(); ok && t.Phase == state.Phase {
		if t.PlayerID != use.PlayerID {
			return ErrSkillNotAllowed
		}
	} else if !player.Alive {
		return ErrPlayerDead
	}

	// 检查技能是否在当前阶段允许，并取出它的声明
	step, allowed := p.stepFor(state.Phase, player.Role, use.Skill)
	if !allowed {
		return ErrSkillNotAllowed
	}

	// 弃权不需要目标
	if use.Skill == SkillSkip {
		return nil
	}

	// 检查目标是否有效
	if use.TargetID != "" {
		target, ok := state.getPlayer(use.TargetID)
		if !ok {
			return ErrTargetNotFound
		}
		// 能否指向已出局的玩家由步骤声明，内核不认得任何具体技能
		if !target.Alive && !step.AllowDeadTarget {
			return ErrTargetDead
		}
	}

	return nil
}
