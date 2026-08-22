// phase_info.go 阶段信息：告诉调用方本阶段该让谁行动、能用什么技能。
//
// 全部由阶段配置（PhaseConfig.Steps）派生，因此第三方经 WithResolver
// 加入的自定义角色同样能拿到。

package werewolf

import ()

// PhaseInfo 当前阶段的信息（上帝视角）。
//
// 调用方据此组织本阶段的流程与公告。内容包含狼队名单、女巫可见的刀口
// 等敏感信息，不可整体转发给玩家——面向玩家的内容用 Engine.PlayerView。
type PhaseInfo struct {
	Phase       PhaseType                   // 当前阶段
	Round       int                         // 当前回合
	Steps       []PhaseStep                 // 当前阶段的步骤配置（包含上帝公告和玩家行动）
	ActiveRoles []RoleType                  // 需要行动的玩家角色（不含上帝）
	RoleInfos   map[RoleType]*RolePhaseInfo // 各角色的阶段信息
}

// NeedsGodAnnouncement 判断当前阶段是否需要上帝公告
func (p *PhaseInfo) NeedsGodAnnouncement() bool {
	if len(p.Steps) == 0 {
		return false
	}
	return p.Steps[0].Role == RoleGod &&
		p.Steps[0].Skill == SkillAnnounce
}

// GodAnnouncementStep 获取上帝公告步骤（如果存在）
func (p *PhaseInfo) GodAnnouncementStep() *PhaseStep {
	if p.NeedsGodAnnouncement() {
		return &p.Steps[0]
	}
	return nil
}

// PlayerActionSteps 获取玩家行动步骤（不含上帝公告）
func (p *PhaseInfo) PlayerActionSteps() []PhaseStep {
	if len(p.Steps) == 0 {
		return nil
	}
	if p.NeedsGodAnnouncement() {
		return p.Steps[1:]
	}
	return p.Steps
}

// RolePhaseInfo 角色阶段信息
type RolePhaseInfo struct {
	PlayerIDs     []string            // 该角色的玩家ID列表
	AllowedSkills []SkillType         // 可用技能
	Teammates     map[string][]string // 同阵营队友（玩家ID -> 队友IDs），好人阵营为空

	// RoleInfo 角色专属信息：玩家ID -> 该玩家额外看得到的东西。
	//
	// 由角色自己的 RoleInfoProvider 回答，引擎不认识任何具体角色。
	// 内置女巫的刀口在这里的键是 RoleInfoKillTarget。
	RoleInfo map[string]map[string]string
}

// PhaseInfo 获取当前阶段信息（上帝视角）。
//
// 返回的内容包含狼队名单、女巫可见的刀口等敏感信息，供调用方作为主持人
// 组织本阶段的流程与公告使用，**不可以整体转发给玩家**。
// 要拿到可以直接发给某个玩家的内容，用 PlayerView。
//
// 各角色的信息由阶段配置（PhaseConfig.Steps）派生，因此第三方通过
// WithResolver 加入的自定义角色同样能拿到。
func (e *Engine) PhaseInfo() *PhaseInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()

	info := &PhaseInfo{
		Phase:       e.state.Phase,
		Round:       e.state.Round,
		Steps:       make([]PhaseStep, 0),
		ActiveRoles: make([]RoleType, 0),
		RoleInfos:   make(map[RoleType]*RolePhaseInfo),
	}

	phaseConfig := e.phase.phaseConfig(e.state.Phase)
	if phaseConfig == nil {
		return info
	}

	// 返回副本：Steps 直接暴露会让调用方改到引擎内部的阶段配置
	info.Steps = make([]PhaseStep, len(phaseConfig.Steps))
	copy(info.Steps, phaseConfig.Steps)

	// 本阶段若在结算某个死亡技能，则只有触发者能行动
	trigger, hasTrigger := e.state.peekTrigger()
	triggerActive := hasTrigger && trigger.Phase == e.state.Phase

	seen := make(map[RoleType]bool)
	for _, step := range phaseConfig.Steps {
		// 上帝是系统角色，不是需要行动的玩家
		if step.Role == RoleGod || seen[step.Role] {
			continue
		}
		seen[step.Role] = true

		info.ActiveRoles = append(info.ActiveRoles, step.Role)
		info.RoleInfos[step.Role] = e.buildRolePhaseInfo(step.Role, triggerActive, trigger)
	}

	return info
}

// allowedSkillsFor 返回指定角色在当前阶段可用的技能。
//
// 唯一真相来源是阶段配置（PhaseConfig.Steps），与 ValidateSkillUse 走同一条路径。
func (e *Engine) allowedSkillsFor(role RoleType) []SkillType {
	return e.phase.allowedSkills(e.state.Phase, role)
}

// buildRolePhaseInfo 组装某个角色在当前阶段的信息。
// 调用前需持有 e.mu。
func (e *Engine) buildRolePhaseInfo(role RoleType, triggerActive bool, trigger PendingTrigger) *RolePhaseInfo {
	ri := &RolePhaseInfo{
		AllowedSkills: e.allowedSkillsFor(role),
	}

	// 与 PhaseReadiness 共用同一份「谁该行动」的判定：两处各写一遍的时候，
	// 这里漏了排序，同一个局面每次调用给出的名单顺序都不一样。
	ri.PlayerIDs = e.actorsForStep(role, triggerActive, trigger)

	for _, id := range ri.PlayerIDs {
		// 队友按**阵营**给，不按角色。此前这里是 case RoleWerewolf，
		// 于是经 AddCustomPlayer 加进来的狼王在这份名单里拿不到队友——
		// 而 PlayerView 与 WolfTeammates 那两条路都是对的，只有主持人
		// 照着组织流程的这一份漏了。
		if mates := e.state.getWolfTeammates(id); len(mates) > 0 {
			if ri.Teammates == nil {
				ri.Teammates = make(map[string][]string, len(ri.PlayerIDs))
			}
			ri.Teammates[id] = mates
		}

		// 角色专属信息由角色自己回答，引擎不认识任何具体角色
		if info := e.roleInfoFor(id, role); info != nil {
			if ri.RoleInfo == nil {
				ri.RoleInfo = make(map[string]map[string]string, len(ri.PlayerIDs))
			}
			ri.RoleInfo[id] = info
		}
	}

	return ri
}
