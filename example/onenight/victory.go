// victory.go decides who won.
//
// This is where this package ran into the kernel hardest; see SCARS.md,
// scar 5: **this ruleset can have more than one winner**, and the kernel's
// VictoryChecker can only return one Camp.

package onenight

import (
	"sort"
	"strings"

	"github.com/Zereker/hiddenrole"
)

// The victory conditions, from the official rulebook:
//
//	village wins   at least one werewolf is eliminated (a non-wolf also being eliminated does not matter)
//	               or: there are no werewolves in play at all (all three wolf cards are in the centre) and nobody is eliminated
//	wolves win     at least one werewolf is in play and no werewolf is eliminated
//	tanner wins    only by being eliminated themselves. Eliminated with no wolf eliminated -> the wolves do not win;
//	               eliminated with a wolf also eliminated -> the village wins too
//
// "A werewolf was eliminated" counts **werewolf cards**, not the wolf team:
// the minion is on the wolf team but is not a wolf, and their elimination does
// not count.
//
// # One disagreement with the source
//
// A widely repeated minion clause says "with no werewolf in play, the minion
// wins as long as they survive and at least one villager dies". It is **not
// found** in the publisher's own rules text and appears only in second-hand
// restatements. This package follows the official text and does not implement
// it -- the same approach the missions package took on Merlin: when sources
// disagree, state the reason and follow the more authoritative one.
//
// The consequence is one corner case with no winner: all wolf cards in the
// centre, a minion in play, and somebody eliminated. It is pinned by a test
// (TestVictory_NoWolfInPlayAndSomeoneDies), so changing sources shows up
// immediately.
// checkVictory decides the outcome.
//
// There is only an answer once the vote has resolved -- in this ruleset the
// game **never ends early**.
func checkVictory(view hiddenrole.GameView) (bool, hiddenrole.Camp) {
	if view.Phase() != PhaseVote {
		return false, hiddenrole.CampUnspecified
	}

	players := view.AllPlayers()
	var wolfInPlay, anyDied, wolfDied, tannerDied bool
	for _, p := range players {
		held := card(view, p.ID)
		if isWolfCard(held) {
			wolfInPlay = true
		}
		if p.Alive {
			continue
		}
		anyDied = true
		if isWolfCard(held) {
			wolfDied = true
		}
		if held == RoleTanner {
			tannerDied = true
		}
	}

	// Do not decide while the vote is unresolved (nobody dead and no "nobody
	// is eliminated" conclusion yet).
	if !anyDied && !votingSettled(view) {
		return false, hiddenrole.CampUnspecified
	}

	var winners []hiddenrole.Camp
	if wolfDied || (!wolfInPlay && !anyDied) {
		winners = append(winners, CampVillage)
	}
	if tannerDied {
		winners = append(winners, CampTanner)
	}
	if wolfInPlay && !wolfDied && !tannerDied {
		winners = append(winners, CampWolf)
	}

	return true, joinCamps(winners)
}

// votingSettled reports whether the vote phase has already resolved.
//
// "Nobody is eliminated" is a legal outcome (one vote each), and on the board
// it looks exactly like "the vote has not happened" -- in both cases nobody is
// eliminated. A piece of game-long state tells them apart.
func votingSettled(view hiddenrole.GameView) bool {
	return view.Var(hiddenrole.ScopeGame, varVoteSettled) != ""
}

// varVoteSettled marks the vote as resolved.
const varVoteSettled = "vote.settled"

// markVoteSettled records that the vote resolved, so the victory check can
// tell "nobody was eliminated" from "nobody has voted yet".
func markVoteSettled() *hiddenrole.Effect {
	return hiddenrole.NewSetVarEffect(hiddenrole.ScopeGame, varVoteSettled, hiddenrole.VarPresent)
}

// joinCamps packs several winners into one Camp.
//
// The kernel's VictoryChecker returns (bool, Camp) -- **one** Camp. And in
// this ruleset the tanner can win alongside the village: they are eliminated
// and so is a wolf, and both sides win.
//
// Camp is a string underneath and the kernel does not interpret values, so
// packing several into one works: "TANNER+VILLAGE". But that is a **string
// encoding**, not a type -- the caller has to know the rule for taking it
// apart, and that rule is something the kernel neither knows nor has anywhere
// to record. See SCARS.md, scar 5.
//
// They are joined in lexicographic order, so the result is deterministic.
func joinCamps(winners []hiddenrole.Camp) hiddenrole.Camp {
	if len(winners) == 0 {
		return CampNobody
	}
	out := make([]string, 0, len(winners))
	for _, c := range winners {
		out = append(out, string(c))
	}
	sort.Strings(out)
	return hiddenrole.Camp(strings.Join(out, "+"))
}

// CampNobody means no side met its victory condition.
//
// This is not "not decided yet" (that is hiddenrole.CampUnspecified), it is
// "the game ended and nobody won". A real corner case: all wolf cards in the
// centre, a minion in play, and somebody eliminated.
const CampNobody hiddenrole.Camp = "NOBODY"

// Winners unpacks the Camp that checkVictory packed back into a set.
//
// This function existing is itself evidence of scar 5: the kernel cannot
// express "a set of winners", so the encoding and decoding rules have to be
// carried by the rules package.
func Winners(c hiddenrole.Camp) []hiddenrole.Camp {
	if c == hiddenrole.CampUnspecified || c == CampNobody {
		return nil
	}
	parts := strings.Split(string(c), "+")
	out := make([]hiddenrole.Camp, 0, len(parts))
	for _, p := range parts {
		out = append(out, hiddenrole.Camp(p))
	}
	return out
}

// Won reports whether a given side is among the winners.
func Won(c hiddenrole.Camp, want hiddenrole.Camp) bool {
	for _, w := range Winners(c) {
		if w == want {
			return true
		}
	}
	return false
}
