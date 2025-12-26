package werewolf

import (
	pb "github.com/Zereker/werewolf/proto"
)

// Phase 阶段管理器
type Phase struct {
	config    *GameConfig
	resolvers map[pb.PhaseType]Resolver
}

// NewPhase 创建阶段管理器
func NewPhase(config *GameConfig) *Phase {
	p := &Phase{
		config:    config,
		resolvers: make(map[pb.PhaseType]Resolver),
	}

	// 注册解析器
	p.resolvers[pb.PhaseType_PHASE_TYPE_DAY] = NewDayResolver()
	p.resolvers[pb.PhaseType_PHASE_TYPE_VOTE] = NewVoteResolver()

	// 注册夜晚子阶段解析器
	p.resolvers[pb.PhaseType_PHASE_TYPE_NIGHT_GUARD] = NewGuardResolver()
	p.resolvers[pb.PhaseType_PHASE_TYPE_NIGHT_WOLF] = NewWolfResolver()
	p.resolvers[pb.PhaseType_PHASE_TYPE_NIGHT_WITCH] = NewWitchResolver()
	p.resolvers[pb.PhaseType_PHASE_TYPE_NIGHT_SEER] = NewSeerResolver()
	p.resolvers[pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE] = NewNightResolveResolver()

	// 注册猎人阶段解析器（夜晚和白天共用）
	hunterResolver := NewHunterResolver()
	p.resolvers[pb.PhaseType_PHASE_TYPE_NIGHT_HUNTER] = hunterResolver
	p.resolvers[pb.PhaseType_PHASE_TYPE_DAY_HUNTER] = hunterResolver

	return p
}

// GetPhaseConfig 获取阶段配置
func (p *Phase) GetPhaseConfig(phase pb.PhaseType) *PhaseConfig {
	return p.config.Phases[phase]
}

// GetResolver 获取阶段解析器
func (p *Phase) GetResolver(phase pb.PhaseType) Resolver {
	return p.resolvers[phase]
}

// GetRequiredRoles 获取当前阶段需要行动的角色
func (p *Phase) GetRequiredRoles(phase pb.PhaseType) []pb.RoleType {
	config := p.GetPhaseConfig(phase)
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

// GetAllowedSkills 获取指定角色在当前阶段允许的技能
func (p *Phase) GetAllowedSkills(phase pb.PhaseType, role pb.RoleType) []pb.SkillType {
	config := p.GetPhaseConfig(phase)
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

// GetAllowedSkillsForSubStep 获取指定角色在当前子步骤允许的技能
func (p *Phase) GetAllowedSkillsForSubStep(phase pb.PhaseType, subStep int, role pb.RoleType) []pb.SkillType {
	config := p.GetPhaseConfig(phase)
	if config == nil || subStep >= len(config.Steps) {
		return nil
	}

	step := config.Steps[subStep]
	// UNSPECIFIED 表示所有角色都可以
	if step.Role == role || step.Role == pb.RoleType_ROLE_TYPE_UNSPECIFIED {
		return []pb.SkillType{step.Skill}
	}

	return nil
}

// GetCurrentStepRole 获取当前子步骤需要行动的角色
func (p *Phase) GetCurrentStepRole(phase pb.PhaseType, subStep int) pb.RoleType {
	config := p.GetPhaseConfig(phase)
	if config == nil || subStep >= len(config.Steps) {
		return pb.RoleType_ROLE_TYPE_UNSPECIFIED
	}
	return config.Steps[subStep].Role
}

// NextSubPhase 计算下一阶段（使用声明式配置）
func (p *Phase) NextSubPhase(current pb.PhaseType) pb.PhaseType {
	// 游戏开始阶段的特殊处理
	if current == pb.PhaseType_PHASE_TYPE_START {
		return pb.PhaseType_PHASE_TYPE_NIGHT_GUARD
	}

	// 从配置中获取下一阶段
	config := p.GetPhaseConfig(current)
	if config != nil && config.NextPhase != pb.PhaseType_PHASE_TYPE_UNSPECIFIED {
		return config.NextPhase
	}

	// 配置中未找到，返回 END
	return pb.PhaseType_PHASE_TYPE_END
}

// ValidateSkillUse 验证技能使用是否合法
func (p *Phase) ValidateSkillUse(use *SkillUse, state *State) error {
	// 检查玩家是否存在
	player, ok := state.getPlayer(use.PlayerID)
	if !ok {
		return ErrPlayerNotFound
	}

	// 猎人阶段特殊处理：死亡的猎人可以使用技能
	isHunterPhase := state.Phase == pb.PhaseType_PHASE_TYPE_NIGHT_HUNTER ||
		state.Phase == pb.PhaseType_PHASE_TYPE_DAY_HUNTER
	if !player.Alive && !isHunterPhase {
		return ErrPlayerDead
	}

	// 检查技能是否在当前阶段允许
	allowedSkills := p.GetAllowedSkills(state.Phase, player.Role)
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
