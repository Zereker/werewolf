package hiddenrole

// phaseManager owns the phase configuration and its resolvers.
type phaseManager struct {
	config    *Config
	resolvers map[PhaseType]Resolver
}

// newPhaseManager builds a phase manager.
//
// It installs no default resolvers: the kernel does not know which phase is
// resolved by whom. Werewolf's are passed in as construction options by
// werewolf.Options.
func newPhaseManager(config *Config) *phaseManager {
	return &phaseManager{
		config:    config,
		resolvers: make(map[PhaseType]Resolver, 8),
	}
}

// registerResolver registers or replaces one phase's resolver.
func (p *phaseManager) registerResolver(phase PhaseType, r Resolver) {
	p.resolvers[phase] = r
}

// validateResolvers checks that every configured phase has a resolver.
//
// A missing resolver raises no error at play time; it just silently drops the
// skills submitted in that phase -- a failure that is nearly impossible to
// locate mid-game, so it has to be caught before the game starts.
func (p *phaseManager) validateResolvers() error {
	for phaseType := range p.config.Phases {
		if p.resolvers[phaseType] == nil {
			return WrapError(CodeInvalidPhase,
				"phase %v has no resolver registered", phaseType)
		}
	}
	return nil
}

// phaseConfig returns a phase's configuration.
func (p *phaseManager) phaseConfig(phase PhaseType) *PhaseConfig {
	return p.config.Phases[phase]
}

// resolver returns a phase's resolver.
func (p *phaseManager) resolver(phase PhaseType) Resolver {
	return p.resolvers[phase]
}

// stepFor finds the step declaration matching "this role, in this phase,
// using this skill".
//
// It shares one rule with allowedSkills: RoleUnspecified means "every role".
// No match means the skill is not allowed right now.
func (p *phaseManager) stepFor(phase PhaseType, role RoleType, skill SkillType) (PhaseStep, bool) {
	pc := p.phaseConfig(phase)
	if pc == nil {
		return PhaseStep{}, false
	}
	// An empty skill cannot be submitted: that is a "wake up and look" step,
	// not an action. Without this guard SkillUnspecified would match the empty
	// step exactly.
	if skill == SkillUnspecified {
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

// allowedSkills returns the skills a role may use in the given phase.
func (p *phaseManager) allowedSkills(phase PhaseType, role RoleType) []SkillType {
	config := p.phaseConfig(phase)
	if config == nil {
		return []SkillType{}
	}

	skills := make([]SkillType, 0)
	for _, step := range config.Steps {
		// An empty step is "wake up and look" and has no submittable skill
		// (see PhaseStep.Skill).
		if step.Skill == SkillUnspecified {
			continue
		}
		// UNSPECIFIED means every role qualifies.
		if step.Role == role || step.Role == RoleUnspecified {
			skills = append(skills, step.Skill)
		}
	}

	return skills
}

// nextSubPhase computes the next phase from the declarative configuration.
func (p *phaseManager) nextSubPhase(current PhaseType) PhaseType {
	// The start phase is special-cased.
	if current == PhaseStart {
		return p.config.startPhase()
	}

	// Take the next phase from the configuration.
	config := p.phaseConfig(current)
	if config != nil && config.NextPhase != PhaseUnspecified {
		return config.NextPhase
	}

	// Not in the configuration: the game ends.
	return PhaseEnd
}

// validateSkillUse checks whether a skill use is legal.
func (p *phaseManager) validateSkillUse(use *SkillUse, state *gameState) error {
	// Does the player exist?
	player, ok := state.getPlayer(use.PlayerID)
	if !ok {
		return ErrPlayerNotFound
	}

	// Who may act: two layers, lined up item for item with actorsForStep --
	// if the two disagree you get the self-contradiction of "the kernel
	// accepted his submission while telling everyone else he should not be
	// acting".
	//
	//	named by the rules   whoever is on the list; aliveness is the rules' business
	//	default              whoever is alive
	//
	// The phase a detour leads to goes through the first layer: on entering
	// that phase the player has already been written onto the list (see
	// gameState.nameDetourActor). That used to be a separate first layer
	// answering the same question as naming, with a nearly word-for-word
	// identical implementation -- one concept, two implementations.
	//
	// Aliveness is therefore the **default** qualification to act, not the
	// law. Only the trigger path used to be able to step over it -- one
	// kernel letting its own mechanism move the dead while forbidding the
	// rules' mechanism from doing the same is the kernel deciding "may the
	// dead act" on the rules' behalf. What that blocks is real play: the dead
	// in Blood on the Clocktower keep a ghost vote, and werewolf has a
	// last-words phase.
	switch named, hasNamed := state.actorsFor(state.Phase); {
	case hasNamed:
		if !contains(named, use.PlayerID) {
			return ErrSkillNotAllowed
		}
	case !player.Alive:
		return ErrPlayerDead
	}

	// Is the skill allowed in this phase, and what is its declaration?
	step, allowed := p.stepFor(state.Phase, player.Role, use.Skill)
	if !allowed {
		return ErrSkillNotAllowed
	}

	// Are the targets valid? A multi-target skill is checked one by one -- if
	// a single invalid target is mixed into one submission, the whole
	// submission should be rejected rather than silently keeping the valid
	// ones.
	for _, id := range use.Targets {
		if id == "" {
			continue
		}
		target, ok := state.getPlayer(id)
		if !ok {
			return ErrTargetNotFound
		}
		// Whether an eliminated player may be targeted is declared by the
		// step; the kernel recognises no specific skill.
		if !target.Alive && !step.AllowDeadTarget {
			return ErrTargetDead
		}
	}

	return nil
}
