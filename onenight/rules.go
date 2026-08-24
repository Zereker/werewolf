// rules.go plugs One Night into the kernel.
//
// This file is the direct test of the standard "adding a ruleset must not
// require changing the engine": every entry point below is a public
// construction option of the kernel, the same doors a third party registering
// a custom role goes through. The kernel needed **not one line changed** for
// this ruleset.

package onenight

import (
	"github.com/Zereker/hiddenrole"
)

// MinPlayers is the smallest table. The rules deal three cards more than
// there are players, and three is the minimum.
const MinPlayers = 3

// Options is One Night's full assembly.
//
// center is the three cards left in the middle -- decided by the caller when
// dealing. Dealing happens **before the game is created**, as in the first two
// rules packages: there is no randomness in the kernel, and none is needed.
//
//	e := hiddenrole.MustNewEngine(onenight.GameConfig(),
//		onenight.Options([3]hiddenrole.RoleType{...})...)
func Options(center [CenterCount]hiddenrole.RoleType) []hiddenrole.EngineOption {
	opts := []hiddenrole.EngineOption{
		// One resolver per phase, ten in all.
		hiddenrole.WithResolver(PhaseNightWerewolf, werewolfResolver{}),
		hiddenrole.WithResolver(PhaseNightMinion, noopResolver{}),
		hiddenrole.WithResolver(PhaseNightMason, noopResolver{}),
		hiddenrole.WithResolver(PhaseNightSeer, seerResolver{}),
		hiddenrole.WithResolver(PhaseNightRobber, robberResolver{}),
		hiddenrole.WithResolver(PhaseNightTroublemake, troublemakerResolver{}),
		hiddenrole.WithResolver(PhaseNightDrunk, drunkResolver{}),
		hiddenrole.WithResolver(PhaseNightInsomniac, insomniacResolver{}),
		hiddenrole.WithResolver(PhaseDay, noopResolver{}),
		hiddenrole.WithResolver(PhaseVote, voteResolver{}),

		hiddenrole.WithVictoryChecker(hiddenrole.VictoryFunc(checkVictory)),

		// Lay out the three centre cards at the moment play begins.
		hiddenrole.WithGameSetup(hiddenrole.GameSetupFunc(func(hiddenrole.GameView) []*hiddenrole.Effect {
			out := make([]*hiddenrole.Effect, 0, CenterCount)
			for i, role := range center {
				out = append(out, setCenterCard(i, role))
			}
			return out
		})),

		hiddenrole.WithAudience(audience()),
		hiddenrole.WithTeammates(teammates()),
		hiddenrole.WithSpeech(speech()),
	}

	// Every role sits down carrying "the card in my hand right now", whose
	// starting value is the card they were dealt.
	//
	// **VarCamp is deliberately not written.** The kernel would carry it into
	// SelfInfo.Camp, and in this ruleset "which side do I count for now" is a
	// secret: the drunk swaps their own card with the centre without looking,
	// and two players the troublemaker swapped do not know either. Filling it
	// into their own view would simply tell them. See SCARS.md, scar 4.
	for _, role := range AllRoles {
		r := role
		opts = append(opts,
			hiddenrole.WithRoleSetup(r, hiddenrole.RoleSetupFunc(
				func(_ string, dealt hiddenrole.RoleType) map[string]string {
					return map[string]string{varCard: string(dealt)}
				})),
			hiddenrole.WithRoleInfo(r, roleInfoFor(r)),
		)
	}
	return opts
}

// AllRoles is every role in this ruleset.
//
// The kernel does not know which roles exist -- it accepts a RoleType string
// at AddPlayer and nothing more. This list belongs to this package.
var AllRoles = []hiddenrole.RoleType{
	RoleWerewolf, RoleMinion, RoleMason, RoleSeer, RoleRobber,
	RoleTroublemaker, RoleDrunk, RoleInsomniac, RoleVillager,
	RoleHunter, RoleTanner,
}
