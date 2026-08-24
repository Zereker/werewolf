package missions

import (
	"time"

	"github.com/Zereker/hiddenrole"
)

// Suggested timeouts per phase. Board data; the engine does not time by
// them.
const (
	ProposeTimeout  = 60 * time.Second
	TeamVoteTimeout = 30 * time.Second
	MissionTimeout  = 30 * time.Second
	AssassinTimeout = 90 * time.Second // the assassination reviews the whole game, so allow time
)

// missionSizes is how many players each of the five missions needs, per table
// size.
//
// Taken from the table in the English Wikipedia article. Columns are player
// counts 5-10, rows are missions 1-5:
//
//	mission 5   6   7   8   9  10
//	 1      2   2   2   3   3   3
//	 2      3   3   3   4   4   4
//	 3      2   4   3   4   4   4
//	 4      3   3   4   5   5   5
//	 5      3   4   4   5   5   5
//
// In a 6-player game mission 3 takes 4 and mission 4 takes only 3. That is not
// a typo; the original table says so.
var missionSizes = map[int][5]int{
	5:  {2, 3, 2, 3, 3},
	6:  {2, 3, 4, 3, 4},
	7:  {2, 3, 3, 4, 4},
	8:  {3, 4, 4, 5, 5},
	9:  {3, 4, 4, 5, 5},
	10: {3, 4, 4, 5, 5},
}

// evilCounts is how many evil players there are per table size. From the
// English Wikipedia article.
var evilCounts = map[int]int{5: 2, 6: 2, 7: 3, 8: 3, 9: 3, 10: 4}

// MissionSize is how many players mission (1-5) needs in a game of players.
// It returns 0 when the count or the mission number is out of range.
func MissionSize(players, mission int) int {
	sizes, ok := missionSizes[players]
	if !ok || mission < 1 || mission > 5 {
		return 0
	}
	return sizes[mission-1]
}

// EvilCount is how many evil players a game of players has. It returns 0 when
// the count is out of range.
func EvilCount(players int) int { return evilCounts[players] }

// FailsNeeded is how many fail votes make mission fail.
//
// The article's wording: "If one (or two in Mission 4 when at least 7 players
// are playing) or more players choose to fail the mission, the mission
// fails." -- two are needed only on mission 4, and only with seven or more.
func FailsNeeded(players, mission int) int {
	if mission == 4 && players >= 7 {
		return 2
	}
	return 1
}

// HammerRejections is how many consecutive team rejections hand the evil side
// an outright win.
//
// The article's wording: "After five successively rejected mission proposals
// in a single mission, the Spies immediately win the game."
// mission, the Spies immediately win the game.」
const HammerRejections = 5

// DefaultConfig is this package's default board.
//
// The phase cycle is a loop of three nodes: propose -> vote -> mission ->
// propose. When a vote fails the mission phase idles through once (nobody is
// marked as being on a team that round), which is one of the costs of the
// workaround; see SCARS.md, item 2.
//
// The assassination phase is not in the loop: the mission phase queues it with
// a detour once the good side reaches three successes. The kernel's "who, and
// to which phase" queue was built for abilities triggered on elimination, and
// it fits here exactly -- and it additionally guarantees that the victory
// check is deferred until after the assassination resolves, which is what the
// rules want.
func DefaultConfig() *hiddenrole.Config {
	return &hiddenrole.Config{
		StartPhase: PhasePropose,
		Phases: map[hiddenrole.PhaseType]*hiddenrole.PhaseConfig{
			PhasePropose: {
				Type: PhasePropose,

				// Team markers live until the next nomination begins, not
				// until the next mission -- one mission may take five
				// nominations. The kernel used to have only one lifetime, the
				// round, and the round number has to track which mission it is
				// (EndsRound is marked on the mission phase); the two do not
				// coincide, so they had to be cleared by hand in the nomination
				// resolver.
				//
				// Lifetime and counting are declared separately now: this says
				// "I begin from a clean board", and the round number is the
				// mission phase's EndsRound business.
				ClearsRoundVars: true,
				// The leader nominates the team.
				//
				// One submission carries the whole team (SkillUse.Targets is a
				// slice). Splitting it across several submissions did work,
				// but readiness could then not say how many players were still
				// missing -- it only knew whether the leader had submitted.
				// See SCARS.md, item 3.
				Steps: []hiddenrole.PhaseStep{
					{Role: hiddenrole.RoleUnspecified, Skill: SkillPropose, Required: true},
				},
				Timeout:   ProposeTimeout,
				NextPhase: PhaseTeamVote,
			},
			PhaseTeamVote: {
				Type: PhaseTeamVote,
				// Everyone votes, one vote each, accept or reject.
				Steps: []hiddenrole.PhaseStep{
					{Role: hiddenrole.RoleUnspecified, Skill: SkillApprove, Required: true, Multiple: true, Group: "vote"},
					{Role: hiddenrole.RoleUnspecified, Skill: SkillReject, Required: true, Multiple: true, Group: "vote"},
				},
				Timeout:   TeamVoteTimeout,
				NextPhase: PhaseMission,
			},
			PhaseMission: {
				Type: PhaseMission,
				// Only the chosen team members should be able to submit, and
				// the kernel decides actors by role alone. This can only be
				// opened to everyone with the resolver throwing away what
				// should not count -- at the cost of AllowedSkills telling a
				// player who is not on the mission "you may vote". The most
				// expensive part of the workaround; see SCARS.md, item 1.
				Steps: []hiddenrole.PhaseStep{
					{Role: hiddenrole.RoleUnspecified, Skill: SkillMissionSuccess, Required: true, Multiple: true, Group: "mission"},
					{Role: hiddenrole.RoleUnspecified, Skill: SkillMissionFail, Required: true, Multiple: true, Group: "mission"},
				},
				Timeout:   MissionTimeout,
				NextPhase: PhasePropose,

				// A resolved mission is a new round.
				//
				// This is the benefit that arrived the moment the round
				// boundary was handed to the rules: this package's Round now
				// equals **which mission it is**, and agrees with what it says
				// about itself. The kernel used to guess "looping back to the
				// start phase counts as a new round", and here every nomination
				// goes round the loop, so Round became a nomination counter,
				// out by as much as a factor of five from "which mission", and
				// it was handed to players verbatim in PlayerView.Round.
				EndsRound: true,
			},
			PhaseAssassin: {
				Type: PhaseAssassin,
				Steps: []hiddenrole.PhaseStep{
					{Role: RoleAssassin, Skill: SkillAssassinate, Required: true},
				},
				Timeout:   AssassinTimeout,
				NextPhase: PhasePropose,
			},
		},
	}
}
