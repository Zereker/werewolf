package missions

import (
	"strconv"

	"github.com/Zereker/hiddenrole"
)

// resolver.go is one resolver per phase, four in all.

// proposeResolver: the leader nominates the mission team.
//
// **One submission carries the whole team**: SkillUse.Targets is a slice and
// carries as many IDs as the team has members. Duplicates are dropped in
// submission order, and anything beyond the required size is discarded.
//
// SkillUse used to carry a single target and the leader had to submit N times
// -- which left readiness unable to say how many players were still missing
// (it reported Ready=true after one nomination when two were needed). See
// SCARS.md, scar 5.
type proposeResolver struct{}

func (proposeResolver) Resolve(uses []*hiddenrole.SkillUse, view hiddenrole.GameView) []*hiddenrole.Effect {
	leader := leaderID(view)
	need := MissionSize(len(view.AllPlayers()), mission(view))

	// One submission carries the whole team.
	//
	// SkillUse used to carry a single target and the leader had to submit N
	// times -- which left readiness unable to say how many players were still
	// missing, reporting Ready=true after one nomination when two were needed.
	// One submission is now a complete team, and readiness tells the truth.
	//
	// It does not check that the submitter is the leader: the kernel already
	// blocked a non-leader at SubmitSkillUse (this phase's actors are named by
	// SetActors).
	seen := map[string]bool{}
	var team []string
	for _, u := range uses {
		if u.Skill != SkillPropose {
			continue
		}
		for _, id := range u.Targets {
			if id == "" || seen[id] || len(team) >= need {
				continue
			}
			if _, ok := view.Player(id); !ok {
				continue
			}
			seen[id] = true
			team = append(team, id)
		}
	}

	// The markers left by the previous nomination need no clearing here: the
	// nomination phase declares ClearsRoundVars and the kernel clears them when
	// it resolves. The kernel used to have only the round lifetime while what
	// was wanted here was "one nomination", so it had to be done by hand --
	// that code is now deleted.
	var effects []*hiddenrole.Effect
	for _, id := range team {
		effects = append(effects,
			hiddenrole.NewEffect(EventProposed, leader, id),
			hiddenrole.NewSetVarEffect(hiddenrole.ScopeRound.Of(id), varOnTeam, hiddenrole.VarPresent))
	}
	// Name the mission phase's actors: only these players may vote success or
	// failure.
	//
	// The list is computed here and used in the next phase -- which is exactly
	// why SetActors names a phase rather than applying to the current one.
	return append(effects, hiddenrole.NewSetActorsEffect(PhaseMission, team...))
}

// teamVoteResolver: everyone votes on the team.
//
// The votes are public: in the article the voting cards are revealed together,
// so the whole table sees who voted what. Every vote therefore produces a
// public event -- the exact opposite of the mission phase, where not even who
// voted failure may show.
type teamVoteResolver struct{}

func (teamVoteResolver) Resolve(uses []*hiddenrole.SkillUse, view hiddenrole.GameView) []*hiddenrole.Effect {
	need := MissionSize(len(view.AllPlayers()), mission(view))
	team := teamIDs(view)

	voted := map[string]bool{}
	approve, reject := 0, 0
	var effects []*hiddenrole.Effect
	for _, u := range uses {
		if u.Skill != SkillApprove && u.Skill != SkillReject {
			continue
		}
		if voted[u.PlayerID] {
			continue // one vote each; the first one counts
		}
		voted[u.PlayerID] = true
		if u.Skill == SkillApprove {
			approve++
		} else {
			reject++
		}
		effects = append(effects,
			hiddenrole.NewEffect(EventVote, u.PlayerID, "").WithData("approve", u.Skill == SkillApprove))
	}

	// A team of the wrong size (the leader under-nominated, or did not
	// nominate at all) is treated as a rejection.
	ok := len(team) == need && need > 0 && approve > reject

	// Leadership passes on every round, approved or not.
	next := gameNum(view, varLeader) + 1
	effects = append(effects,
		setGameNum(view, varLeader, next),
		hiddenrole.NewEffect(EventLeaderChanged, "", ""),
		hiddenrole.NewSetActorsEffect(PhasePropose, leaderAt(view, next)))

	if ok {
		return append(effects,
			hiddenrole.NewEffect(EventTeamApproved, "", "").WithData("team", len(team)),
			hiddenrole.NewSetVarEffect(hiddenrole.ScopeRound, varApproved, hiddenrole.VarPresent),
			setGameNum(view, varRejects, 0),
			hiddenrole.NewGotoPhaseEffect(PhaseMission))
	}

	n := rejects(view) + 1
	effects = append(effects,
		hiddenrole.NewEffect(EventTeamRejected, "", "").WithData("consecutive", n),
		setGameNum(view, varRejects, n))
	if n >= HammerRejections {
		// Five consecutive rejections hand the evil side an outright win. The
		// VictoryChecker reads this count to decide.
		effects = append(effects, hiddenrole.NewEffect(EventHammerReached, "", ""))
	}
	// A rejection goes straight back to nomination rather than idling through
	// the mission phase.
	//
	// This is the benefit that arrived the moment the kernel handed "where to
	// go next" to the rules: the branch's outcome is computed by this phase's
	// resolution, and a static NextPhase cannot express it. It fixed the round
	// number along the way -- the mission phase declares EndsRound, so idling
	// through it once advanced a round too many.
	return append(effects, hiddenrole.NewGotoPhaseEffect(PhasePropose))
}

// missionResolver: the mission resolves.
//
// Two information constraints specific to this ruleset live here:
//
//   - **Only team members may vote.** The kernel decides actors by role and
//     the team is chosen at runtime, so this step can only have the resolver
//     throw away submissions from non-members. The cost is in SCARS.md,
//     item 1.
//   - **Fail votes cannot be attributed.** The table may know only how many
//     fail votes there were, never who cast them. In implementation terms that
//     means **producing no event per vote**, only one aggregate event carrying
//     the count. A good player's mistaken fail vote counts as a success, and
//     the veto is visible to them alone.
type missionResolver struct{}

func (missionResolver) Resolve(uses []*hiddenrole.SkillUse, view hiddenrole.GameView) []*hiddenrole.Effect {
	if !approved(view) {
		return nil // the previous vote failed, so this phase idles through
	}

	// No longer checks "are they on the team": the kernel already blocked
	// submissions from non-members.
	acted := map[string]bool{}
	fails := 0
	var effects []*hiddenrole.Effect
	for _, u := range uses {
		if u.Skill != SkillMissionSuccess && u.Skill != SkillMissionFail {
			continue
		}
		if acted[u.PlayerID] {
			continue // one vote each; the first one counts
		}
		acted[u.PlayerID] = true
		if u.Skill != SkillMissionFail {
			continue
		}
		p, _ := view.Player(u.PlayerID)
		if !isEvil(p.Role) {
			// A good player may not vote failure. The veto goes to them
			// alone -- nobody else should even learn that someone tried, or
			// it amounts to naming them.
			effects = append(effects, cancel(
				hiddenrole.NewEffect(EventFailRejected, u.PlayerID, ""), "the good side may only vote success"))
			continue
		}
		fails++
	}

	players := len(view.AllPlayers())
	m := mission(view)
	failed := fails >= FailsNeeded(players, m)

	if failed {
		effects = append(effects,
			hiddenrole.NewEffect(EventMissionFailed, "", "").
				WithData("mission", m).WithData("fails", strconv.Itoa(fails)),
			setGameNum(view, varFail, failures(view)+1))
	} else {
		effects = append(effects,
			hiddenrole.NewEffect(EventMissionSucceeded, "", "").WithData("mission", m),
			setGameNum(view, varSuccess, successes(view)+1))
	}
	effects = append(effects, setGameNum(view, varMission, m+1))

	// The good side reaches three successes: queue the assassination.
	//
	// It uses the kernel's "who, and to which phase" detour queue -- built for
	// abilities triggered on elimination, but its meaning is exactly what is
	// wanted here, and it additionally defers the victory check until after
	// the assassination resolves.
	if !failed && successes(view)+1 >= 3 {
		if ids := idsWithRole(view, RoleAssassin); len(ids) > 0 {
			effects = append(effects, hiddenrole.NewDetourEffect(ids[0], PhaseAssassin))
		}
	}
	return effects
}

// assassinResolver: the assassin names Merlin.
type assassinResolver struct{}

func (assassinResolver) Resolve(uses []*hiddenrole.SkillUse, view hiddenrole.GameView) []*hiddenrole.Effect {
	for _, u := range uses {
		if u.Skill != SkillAssassinate || u.Target() == "" {
			continue
		}
		p, ok := view.Player(u.Target())
		if !ok {
			continue
		}
		hit := p.Role == RoleMerlin
		return []*hiddenrole.Effect{
			hiddenrole.NewEffect(EventAssassinated, u.PlayerID, u.Target()).WithData("hit", hit),
			hiddenrole.NewSetVarEffect(hiddenrole.ScopeGame, varAssassinated, boolVar(hit)),
		}
	}
	// Naming nobody counts as a missed assassination.
	return []*hiddenrole.Effect{
		hiddenrole.NewEffect(EventAssassinated, "", "").WithData("hit", false),
		hiddenrole.NewSetVarEffect(hiddenrole.ScopeGame, varAssassinated, "miss"),
	}
}

func boolVar(b bool) string {
	if b {
		return "hit"
	}
	return "miss"
}

func cancel(e *hiddenrole.Effect, reason string) *hiddenrole.Effect {
	e.Cancel(reason)
	return e
}
