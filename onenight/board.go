// board.go is One Night's phase graph.

package onenight

import "github.com/Zereker/hiddenrole"

// CenterCount is how many cards sit in the centre. The rules fix it at 3 --
// dealing always leaves three more cards than there are players.
const CenterCount = 3

// GameConfig is One Night's phase graph: nine night steps, then discussion,
// then the vote.
//
// # This is a straight line
//
// Both earlier rules packages have cyclic graphs: werewolf loops back to the
// guard, the mission-based games back to nomination. This one ends at VOTE and
// needs no rounds at all. Round is 1 from start to finish, and round-scoped
// variables are never cleared.
//
// So this graph **declares no round boundary at all**: no EndsRound, no
// ClearsRoundVars. The kernel's Config.Validate only requires them of a graph
// that **loops** -- in a graph that does not, each phase is visited once and
// there is no second round.
//
// This package is what ran into that (SCARS.md, scar 2): those two checks used
// to be unconditional, so the kernel, guarding against one class of
// misconfiguration, forced this correct configuration to lie -- EndsRound had
// to be hung on VOTE even though no round follows it. Not any more.
//
// # The night order is part of the rules
//
// The robber acts before the troublemaker, so the troublemaker can move the
// card that was just stolen; the insomniac acts last, so what they see is the
// result of every swap. Reorder them and it becomes a different game. This
// order is taken from the wake-up order in the official rulebook.
func GameConfig() *hiddenrole.Config {
	// step is a step with a single action. Night abilities are all optional
	// (the rules say "you may..."), so Required is always false -- a role not
	// acting is legal.
	step := func(role hiddenrole.RoleType, skill hiddenrole.SkillType) []hiddenrole.PhaseStep {
		return []hiddenrole.PhaseStep{{Role: role, Skill: skill}}
	}

	// watch is "this role wakes, but takes no action" -- an empty skill.
	watch := func(role hiddenrole.RoleType) []hiddenrole.PhaseStep {
		return []hiddenrole.PhaseStep{{Role: role}}
	}

	// group is a pick-one-of set: submitting any member counts as this role
	// having acted.
	group := func(role hiddenrole.RoleType, name string, skills ...hiddenrole.SkillType) []hiddenrole.PhaseStep {
		out := make([]hiddenrole.PhaseStep, 0, len(skills))
		for _, s := range skills {
			out = append(out, hiddenrole.PhaseStep{Role: role, Skill: s, Group: name})
		}
		return out
	}

	return &hiddenrole.Config{
		StartPhase: PhaseNightWerewolf,
		Phases: map[hiddenrole.PhaseType]*hiddenrole.PhaseConfig{
			// Wolves recognising each other is pure information (it goes
			// through RoleInfo); there is only something to submit when a
			// single wolf is in play -- peeking at one centre card.
			PhaseNightWerewolf: {
				Type:      PhaseNightWerewolf,
				Steps:     group(RoleWerewolf, "peek", SkillPeekCenter0, SkillPeekCenter1, SkillPeekCenter2),
				NextPhase: PhaseNightMinion,
			},

			// The minion, the masons and the insomniac only receive
			// information and take no action: open your eyes, look, close
			// them. An empty skill means exactly that -- this role wakes, but
			// takes no action (see hiddenrole.PhaseStep.Skill).
			//
			// This used to be inexpressible, so a SKIP was hung on it as a
			// placeholder -- and SKIP means "declining to act", which they are
			// not doing: there was never an action to decline. See SCARS.md,
			// scar 3.
			PhaseNightMinion: {
				Type:      PhaseNightMinion,
				Steps:     watch(RoleMinion),
				NextPhase: PhaseNightMason,
			},
			PhaseNightMason: {
				Type:      PhaseNightMason,
				Steps:     watch(RoleMason),
				NextPhase: PhaseNightSeer,
			},

			PhaseNightSeer: {
				Type: PhaseNightSeer,
				Steps: group(RoleSeer, "look",
					SkillSeerPlayer, SkillSeerCenter01, SkillSeerCenter02, SkillSeerCenter12),
				NextPhase: PhaseNightRobber,
			},

			PhaseNightRobber: {
				Type:      PhaseNightRobber,
				Steps:     step(RoleRobber, SkillRob),
				NextPhase: PhaseNightTroublemake,
			},

			PhaseNightTroublemake: {
				Type:      PhaseNightTroublemake,
				Steps:     step(RoleTroublemaker, SkillMeddle),
				NextPhase: PhaseNightDrunk,
			},

			PhaseNightDrunk: {
				Type: PhaseNightDrunk,
				Steps: group(RoleDrunk, "drink",
					SkillDrinkCenter0, SkillDrinkCenter1, SkillDrinkCenter2),
				NextPhase: PhaseNightInsomniac,
			},

			PhaseNightInsomniac: {
				Type:      PhaseNightInsomniac,
				Steps:     watch(RoleInsomniac),
				NextPhase: PhaseDay,
			},

			// Discussion: nothing is submitted; the host advances when they
			// have seen enough.
			PhaseDay: {
				Type:      PhaseDay,
				NextPhase: PhaseVote,
			},

			// The vote: everyone must vote, and all votes are revealed at once.
			PhaseVote: {
				Type: PhaseVote,
				Steps: []hiddenrole.PhaseStep{{
					Role: hiddenrole.RoleUnspecified, Skill: SkillVote,
					Required: true, Multiple: true,
					// A vote points at a living player, and in this game
					// everyone is alive, but saying so beats relying on the
					// default.
				}},
				// No EndsRound / ClearsRoundVars: this ruleset has no second
				// round.
				NextPhase: hiddenrole.PhaseEnd,
			},
		},
	}
}
