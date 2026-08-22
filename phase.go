package werewolf

import ()

// phaseManager 阶段管理器
type phaseManager struct {
	config    *GameConfig
	resolvers map[PhaseType]Resolver
}

// builtinResolvers 内置阶段的解析器。
//
// 做成表而不是一串赋值：加内置阶段时只需要在这里加一行，
// 也让「哪些阶段有解析器」一眼可见。
// 第三方阶段通过 WithResolver 注册，不进这张表。
var builtinResolvers = map[PhaseType]func() Resolver{
	PhaseDay:          func() Resolver { return NewDayResolver() },
	PhaseVote:         func() Resolver { return NewVoteResolver() },
	PhaseNightGuard:   func() Resolver { return NewGuardResolver() },
	PhaseNightWolf:    func() Resolver { return NewWolfResolver() },
	PhaseNightWitch:   func() Resolver { return NewWitchResolver() },
	PhaseNightSeer:    func() Resolver { return NewSeerResolver() },
	PhaseNightResolve: func() Resolver { return NewNightResolveResolver() },
	PhaseNightHunter:  func() Resolver { return NewHunterResolver() },
	PhaseDayHunter:    func() Resolver { return NewHunterResolver() },
}

// newPhaseManager 创建阶段管理器
func newPhaseManager(config *GameConfig) *phaseManager {
	p := &phaseManager{
		config:    config,
		resolvers: make(map[PhaseType]Resolver, len(builtinResolvers)),
	}
	for phase, make := range builtinResolvers {
		p.resolvers[phase] = make()
	}
	return p
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

	// 检查技能是否在当前阶段允许
	allowedSkills := p.allowedSkills(state.Phase, player.Role)
	allowed := false
	for _, skill := range allowedSkills {
		if skill == use.Skill {
			allowed = true
			break
		}
	}
	if !allowed {
		return ErrSkillNotAllowed
	}

	// SKIP 技能不需要目标
	if use.Skill == SkillSkip {
		return nil
	}

	// 检查目标是否有效
	if use.TargetID != "" {
		target, ok := state.getPlayer(use.TargetID)
		if !ok {
			return ErrTargetNotFound
		}
		// 某些技能需要目标存活
		if !target.Alive && use.Skill != SkillAntidote {
			return ErrTargetDead
		}
	}

	return nil
}
