// phase_info.go is phase information: it tells the caller who should act in
// this phase and which skills they may use.
//
// It is all derived from the phase configuration (PhaseConfig.Steps), so a
// custom role added by a third party through WithResolver gets the same
// treatment.

package hiddenrole

// PhaseInfo is information about the current phase, from the god's point of
// view.
//
// The caller uses it to run this phase and make its announcements. It
// contains sensitive information -- the wolf roster, the kill the witch can
// see -- and must not be forwarded to players wholesale; for player-facing
// content use Engine.PlayerView.
type PhaseInfo struct {
	Phase       PhaseType                   // the current phase
	Round       int                         // the current round
	Steps       []PhaseStep                 // this phase's steps (both announcements and player actions)
	ActiveRoles []RoleType                  // the roles that act, excluding the system role
	RoleInfos   map[RoleType]*RolePhaseInfo // per-role information for this phase
}

// NeedsGodAnnouncement reports whether this phase opens with an announcement.
func (p *PhaseInfo) NeedsGodAnnouncement() bool {
	if len(p.Steps) == 0 {
		return false
	}
	return p.Steps[0].Role == RoleSystem &&
		p.Steps[0].Skill == SkillAnnounce
}

// GodAnnouncementStep returns the announcement step, if there is one.
func (p *PhaseInfo) GodAnnouncementStep() *PhaseStep {
	if p.NeedsGodAnnouncement() {
		return &p.Steps[0]
	}
	return nil
}

// PlayerActionSteps returns the player action steps, excluding the
// announcement.
func (p *PhaseInfo) PlayerActionSteps() []PhaseStep {
	if len(p.Steps) == 0 {
		return nil
	}
	if p.NeedsGodAnnouncement() {
		return p.Steps[1:]
	}
	return p.Steps
}

// RolePhaseInfo is one role's information for this phase.
type RolePhaseInfo struct {
	PlayerIDs     []string            // the players holding this role
	AllowedSkills []SkillType         // the skills they may use
	Teammates     map[string][]string // teammates, player ID -> teammate IDs; empty when they know of none

	// RoleInfo is role-specific information: player ID -> what that player
	// gets to see beyond the common facts.
	//
	// Answered by the role's own RoleInfoProvider; the engine recognises no
	// specific role. The built-in witch's kill target lives here under the
	// key RoleInfoKillTarget.
	RoleInfo map[string]map[string]string
}

// PhaseInfo returns information about the current phase, from the god's point
// of view.
//
// What it returns contains sensitive information -- the wolf roster, the kill
// the witch can see -- for the caller to run this phase as the host, and
// **must not be forwarded to players wholesale**. For content that can be
// sent straight to one player, use PlayerView.
//
// Each role's information is derived from the phase configuration
// (PhaseConfig.Steps), so a custom role added by a third party through
// WithResolver gets the same treatment.
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

	// Return a copy: exposing Steps directly would let a caller mutate the
	// engine's own phase configuration.
	info.Steps = make([]PhaseStep, len(phaseConfig.Steps))
	copy(info.Steps, phaseConfig.Steps)

	seen := make(map[RoleType]bool)
	for _, step := range phaseConfig.Steps {
		// The system role is not a player who acts.
		if step.Role == RoleSystem || seen[step.Role] {
			continue
		}
		seen[step.Role] = true

		info.ActiveRoles = append(info.ActiveRoles, step.Role)
		info.RoleInfos[step.Role] = e.buildRolePhaseInfo(step.Role)
	}

	return info
}

// allowedSkillsFor returns the skills a role may use in the current phase.
//
// The single source of truth is the phase configuration (PhaseConfig.Steps),
// the same path skill validation takes.
func (e *Engine) allowedSkillsFor(role RoleType) []SkillType {
	return e.phase.allowedSkills(e.state.Phase, role)
}

// buildRolePhaseInfo assembles one role's information for the current phase.
// The caller must hold e.mu.
func (e *Engine) buildRolePhaseInfo(role RoleType) *RolePhaseInfo {
	ri := &RolePhaseInfo{
		AllowedSkills: e.allowedSkillsFor(role),
	}

	// This shares one "who should act" decision with PhaseReadiness. When the
	// two were written separately, this one forgot to sort, and the same
	// board produced a differently ordered list on every call.
	ri.PlayerIDs = e.actorsForStep(role)

	for _, id := range ri.PlayerIDs {
		// Teammates are answered by the TeammateProvider, the same path
		// PlayerView takes. This used to branch on the role directly, so a
		// custom same-camp role got no teammates in this particular list --
		// while the other two paths were correct, and only the one the host
		// runs the phase from was wrong. Three call sites sharing one
		// decision cannot drift like that again.
		if mates := e.teammatesOf(id); len(mates) > 0 {
			if ri.Teammates == nil {
				ri.Teammates = make(map[string][]string, len(ri.PlayerIDs))
			}
			ri.Teammates[id] = mates
		}

		// Role-specific information is answered by the role itself; the engine
		// recognises no specific role.
		if info := e.roleInfoFor(id, role); info != nil {
			if ri.RoleInfo == nil {
				ri.RoleInfo = make(map[string]map[string]string, len(ri.PlayerIDs))
			}
			ri.RoleInfo[id] = info
		}
	}

	return ri
}
