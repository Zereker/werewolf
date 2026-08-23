package hiddenrole

// PhaseReadiness describes how far along the current phase's actions are.
//
// The engine keeps no clock and will not decide for the caller when a phase
// ends -- but it holds every fact about who should act and who already has,
// so there is no reason to make the caller count for themselves.
//
// # Two questions, answered by Pending and Optional respectively
//
// "Who still **must** act" and "who **may** act in this phase" are different
// questions, and in the default configuration only the wolf kill and the vote
// are Required -- the guard, the witch, the seer and the hunter may all
// decline. Drive the game off Pending alone and those roles are never called
// on for a whole game.
//
// So Ready and Pending cover the required actions (the basis for advancing on
// a timeout), and Optional lists whoever may act but has not (the basis for a
// host nudging them along).
type PhaseReadiness struct {
	Phase PhaseType // the current phase
	Round int       // the current round

	// Ready reports whether every Required step is satisfied.
	//
	// Note that it does **not** mean "everyone has acted": optional skills do
	// not count. While it is false the caller may keep waiting; whether to
	// force the phase forward on a timeout is the caller's call, and EndPhase
	// never refuses on the grounds of not being ready.
	Ready bool

	// Pending lists the required actions still outstanding. Empty when Ready
	// is true.
	Pending []PendingAction

	// Optional lists the players who may act in this phase but have not
	// submitted anything.
	//
	// It does not affect Ready -- declining is legal for them (this is what
	// rules phrasings like "or choose not to guard" mean). A host uses it to
	// decide whether to wait a little longer.
	Optional []PendingAction

	// Acted lists the players who have submitted a skill this phase, sorted
	// by ID.
	Acted []string
}

// PendingAction is one outstanding action.
type PendingAction struct {
	PlayerID string    // the player who should act; listed one by one when the step requires everyone
	Role     RoleType  // their role
	Skill    SkillType // the skill they have not submitted
}

// PhaseReadiness reports who has yet to act in the current phase.
//
// A step with no eligible actor (the guard is dead, say) counts as
// automatically satisfied, so it can never wedge the phase open forever.
func (e *Engine) PhaseReadiness() PhaseReadiness {
	e.mu.RLock()
	defer e.mu.RUnlock()

	out := PhaseReadiness{
		Phase: e.state.Phase,
		Round: e.state.Round,
		Ready: true,
	}

	// What has been submitted: playerID -> the set of skills they submitted.
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
			// Nobody can carry this step; count it as satisfied.
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

		// With Multiple=false, any one of them completing it is enough.
		if !req.multiple && len(missing) < len(actors) {
			continue
		}
		out.Ready = false
		out.Pending = append(out.Pending, missing...)
	}

	return out
}

// requirement is one required action: some role, satisfied by submitting any
// one of the skills in accepts.
//
// The unit of judgement is "one action", not "one step". A mutually exclusive
// group -- the hunter shooting or declining -- is two steps in the
// configuration and one requirement here. Judged step by step, a hunter who
// explicitly declined would still be recorded as owing a shot, and a
// deduplication pass would be needed afterwards to merge one player's two
// outstanding items back together.
type requirement struct {
	role     RoleType
	skill    SkillType   // the representative skill reported to the caller: the group's first
	accepts  []SkillType // every skill in the group
	required bool        // unsatisfied means not ready, or merely "may act"
	multiple bool
}

// satisfiedBy reports whether any skill this player submitted is one this
// requirement accepts.
func (r requirement) satisfiedBy(done map[SkillType]bool) bool {
	for _, skill := range r.accepts {
		if done[skill] {
			return true
		}
	}
	return false
}

// requirementsOf folds a step list into an action list, preserving the steps'
// order.
//
// The system role has no player carrying it and is skipped. Non-required
// steps are collected all the same -- they do not affect readiness, but they
// have to show up in Optional.
func requirementsOf(steps []PhaseStep) []requirement {
	out := make([]requirement, 0, len(steps))
	byGroup := make(map[string]int, len(steps)) // group name -> index in out

	for _, step := range steps {
		// Either no player carries this step, or one does but takes no action
		// -- neither enters the readiness decision.
		if step.Role == RoleSystem || step.Skill == SkillUnspecified {
			continue
		}
		if i, ok := byGroup[step.Group]; ok && step.Group != "" {
			out[i].accepts = append(out[i].accepts, step.Skill)
			// One required step in the group makes the whole group required.
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

// actorsForStep returns the eligible actors for a step. The caller must hold
// e.mu.
//
// This is the **single** place "who may act" is read from -- skill
// validation, AllowedSkills and PhaseReadiness all take it from here. Three
// questions with one source is what keeps the self-contradiction of "the
// kernel accepted his submission while telling everyone else he should not be
// acting" from arising.
//
// Two layers, highest priority first:
//
//	named actors    NewSetActorsEffect, or the list a detour writes on entering the phase
//	PhaseStep.Role  the default: by role, which is fixed at seating time
//
// There used to be three layers, with "a pending detour" on top. That layer
// answered the same question as naming, with a nearly word-for-word identical
// implementation -- one concept, two implementations, both of which had to be
// kept in step. The detour queue no longer answers "who may act": it produces
// a list on entering the phase (see gameState.nameDetourActor), and
// everything after that follows the naming path.
func (e *Engine) actorsForStep(role RoleType) []string {
	if ids, ok := e.state.actorsFor(e.state.Phase); ok {
		return e.namedActorsFor(role, ids)
	}
	if role == RoleUnspecified {
		return sortedStrings(e.state.getAlivePlayerIDs())
	}
	return sortedStrings(e.state.getAlivePlayerIDsByRole(role))
}

// namedActorsFor picks, out of the players the rules named, those who carry
// this role's step. The caller must hold e.mu.
//
// Being named does not mean "he acts on behalf of every role". A step that
// declares a specific role counts only the players whose role matches; a step
// declaring RoleUnspecified counts everyone named.
//
// **It does not filter on aliveness**: whoever the rules named may act. This
// used to strip out eliminated players, which is the kernel making a
// judgement on the rules' behalf -- and a self-contradictory one, since the
// same kernel let **its own** detour queue move the dead (the hunter shooting
// after being killed) while forbidding the **rules'** naming from doing the
// same.
//
// What that blocks is real play: the dead in Blood on the Clocktower keep a
// ghost vote, and werewolf's last-words phase is the same idea. Aliveness is
// the rules' judgement, and naming is the rules judging.
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
