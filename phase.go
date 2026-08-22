package werewolf

import (
	pb "github.com/Zereker/werewolf/proto"
)

// phaseManager 阶段管理器
type phaseManager struct {
	config    *GameConfig
	resolvers map[pb.PhaseType]Resolver
}

// builtinResolvers 内置阶段的解析器。
//
// 做成表而不是一串赋值：加内置阶段时只需要在这里加一行，
// 也让「哪些阶段有解析器」一眼可见。
// 第三方阶段通过 Engine.RegisterResolver 注册，不进这张表。
var builtinResolvers = map[pb.PhaseType]func() Resolver{
	pb.PhaseType_PHASE_TYPE_DAY:           func() Resolver { return NewDayResolver() },
	pb.PhaseType_PHASE_TYPE_VOTE:          func() Resolver { return NewVoteResolver() },
	pb.PhaseType_PHASE_TYPE_NIGHT_GUARD:   func() Resolver { return NewGuardResolver() },
	pb.PhaseType_PHASE_TYPE_NIGHT_WOLF:    func() Resolver { return NewWolfResolver() },
	pb.PhaseType_PHASE_TYPE_NIGHT_WITCH:   func() Resolver { return NewWitchResolver() },
	pb.PhaseType_PHASE_TYPE_NIGHT_SEER:    func() Resolver { return NewSeerResolver() },
	pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE: func() Resolver { return NewNightResolveResolver() },
	pb.PhaseType_PHASE_TYPE_NIGHT_HUNTER:  func() Resolver { return NewHunterResolver() },
	pb.PhaseType_PHASE_TYPE_DAY_HUNTER:    func() Resolver { return NewHunterResolver() },
}

// newPhaseManager 创建阶段管理器
func newPhaseManager(config *GameConfig) *phaseManager {
	p := &phaseManager{
		config:    config,
		resolvers: make(map[pb.PhaseType]Resolver, len(builtinResolvers)),
	}
	for phase, make := range builtinResolvers {
		p.resolvers[phase] = make()
	}
	return p
}

// registerResolver 注册或替换某阶段的解析器
func (p *phaseManager) registerResolver(phase pb.PhaseType, r Resolver) {
	p.resolvers[phase] = r
}

// validateResolvers 检查每个已配置的阶段都注册了解析器。
//
// 缺失解析器不会报错，只会让该阶段收到的技能被悄悄丢弃——这种失败
// 在对局中几乎无法定位，必须在开局前拦下。
func (p *phaseManager) validateResolvers() error {
	for phaseType := range p.config.Phases {
		if p.resolvers[phaseType] == nil {
			return WrapError(pb.ErrorCode_ERROR_CODE_INVALID_PHASE,
				"phase %v has no resolver registered", phaseType)
		}
	}
	return nil
}

// phaseConfig 获取阶段配置
func (p *phaseManager) phaseConfig(phase pb.PhaseType) *PhaseConfig {
	return p.config.Phases[phase]
}

// resolver 获取阶段解析器
func (p *phaseManager) resolver(phase pb.PhaseType) Resolver {
	return p.resolvers[phase]
}

// requiredRoles 获取当前阶段需要行动的角色
func (p *phaseManager) requiredRoles(phase pb.PhaseType) []pb.RoleType {
	config := p.phaseConfig(phase)
	if config == nil {
		return nil
	}

	roles := make([]pb.RoleType, 0)
	seen := make(map[pb.RoleType]bool)

	for _, step := range config.Steps {
		if !seen[step.Role] {
			roles = append(roles, step.Role)
			seen[step.Role] = true
		}
	}

	return roles
}

// allowedSkills 获取指定角色在当前阶段允许的技能
func (p *phaseManager) allowedSkills(phase pb.PhaseType, role pb.RoleType) []pb.SkillType {
	config := p.phaseConfig(phase)
	if config == nil {
		return nil
	}

	skills := make([]pb.SkillType, 0)
	for _, step := range config.Steps {
		// UNSPECIFIED 表示所有角色都可以
		if step.Role == role || step.Role == pb.RoleType_ROLE_TYPE_UNSPECIFIED {
			skills = append(skills, step.Skill)
		}
	}

	return skills
}

// nextSubPhase 计算下一阶段（使用声明式配置）
func (p *phaseManager) nextSubPhase(current pb.PhaseType) pb.PhaseType {
	// 游戏开始阶段的特殊处理
	if current == pb.PhaseType_PHASE_TYPE_START {
		return p.config.startPhase()
	}

	// 从配置中获取下一阶段
	config := p.phaseConfig(current)
	if config != nil && config.NextPhase != pb.PhaseType_PHASE_TYPE_UNSPECIFIED {
		return config.NextPhase
	}

	// 配置中未找到，返回 END
	return pb.PhaseType_PHASE_TYPE_END
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
	if use.Skill == pb.SkillType_SKILL_TYPE_SKIP {
		return nil
	}

	// 检查目标是否有效
	if use.TargetID != "" {
		target, ok := state.getPlayer(use.TargetID)
		if !ok {
			return ErrTargetNotFound
		}
		// 某些技能需要目标存活
		if !target.Alive && use.Skill != pb.SkillType_SKILL_TYPE_ANTIDOTE {
			return ErrTargetDead
		}
	}

	return nil
}
