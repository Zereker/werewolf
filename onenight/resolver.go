// resolver.go is one resolver per phase, ten in all.
//
// Who may use each night ability is decided by **the card they were dealt**
// (see cards.go) -- a robber who steals the werewolf card does not wake with
// the wolves. The kernel decides actors by RoleType, and RoleType is exactly
// the card they were dealt, so this costs the rules nothing at all.

package onenight

import (
	"sort"
	"strings"

	"github.com/Zereker/hiddenrole"
)

// noopResolver produces no state change at all.
//
// The day is only discussion, and the host advances when they have seen
// enough. The minion, mason and insomniac phases take this path too: they only
// receive information and take no action, and the information is delivered by
// a RoleInfoProvider (see boundary.go).
type noopResolver struct{}

func (noopResolver) Resolve([]*hiddenrole.SkillUse, hiddenrole.GameView) []*hiddenrole.Effect {
	return nil
}

// firstUse returns the first valid submission from a set of skills, or nil.
//
// Night abilities are all "at most once": the kernel permits repeated
// submissions, and taking the first is this package's convention.
func firstUse(uses []*hiddenrole.SkillUse, skills ...hiddenrole.SkillType) *hiddenrole.SkillUse {
	want := make(map[hiddenrole.SkillType]bool, len(skills))
	for _, s := range skills {
		want[s] = true
	}
	for _, u := range uses {
		if want[u.Skill] {
			return u
		}
	}
	return nil
}

// centerIndexes reads a centre card's index off the end of a skill name.
//
// "Look at two centre cards" and "swap with a given centre card" do not point
// at a player, and the kernel's target validation only knows player IDs --
// every entry of SkillUse.Targets is passed to getPlayer, and anything that
// does not match is ErrTargetNotFound. So the index can only be encoded into
// the skill name and read back here. This is this package's first scar; see
// SCARS.md, scar 1.
func centerIndexes(skill hiddenrole.SkillType) []int {
	name := string(skill)
	i := strings.LastIndex(name, "_")
	if i < 0 {
		return nil
	}
	var out []int
	for _, c := range name[i+1:] {
		if c < '0' || c >= '0'+CenterCount {
			return nil
		}
		out = append(out, int(c-'0'))
	}
	return out
}

// ==================== Night ====================

// werewolfResolver is the werewolf phase.
//
// Wolves recognising each other is pure information and goes through a
// RoleInfoProvider. Only the **lone wolf** case is handled here: with a single
// wolf in play, they may look at one centre card. Whether "only one wolf may
// look" is not something the kernel should judge -- so it is judged here, not
// there.
type werewolfResolver struct{}

func (werewolfResolver) Resolve(uses []*hiddenrole.SkillUse, view hiddenrole.GameView) []*hiddenrole.Effect {
	use := firstUse(uses, SkillPeekCenter0, SkillPeekCenter1, SkillPeekCenter2)
	if use == nil {
		return nil
	}
	if len(dealtWith(view, RoleWerewolf)) != 1 {
		// With more than one wolf the option does not exist at all. The kernel
		// cannot block this -- "how many wolves are in play" is the rules'
		// judgement.
		return nil
	}
	idx := centerIndexes(use.Skill)
	if len(idx) != 1 {
		return nil
	}
	return []*hiddenrole.Effect{
		hiddenrole.NewEffect(EventLoneWolf, use.PlayerID, ""),
		learnCenter(use.PlayerID, idx[0], centerCard(view, idx[0])),
	}
}

// seerResolver is the seer phase: look at one player's card, or two centre
// cards.
type seerResolver struct{}

func (seerResolver) Resolve(uses []*hiddenrole.SkillUse, view hiddenrole.GameView) []*hiddenrole.Effect {
	use := firstUse(uses, SkillSeerPlayer, SkillSeerCenter01, SkillSeerCenter02, SkillSeerCenter12)
	if use == nil {
		return nil
	}

	if use.Skill == SkillSeerPlayer {
		target := use.Target()
		if target == "" || target == use.PlayerID {
			return nil // looking at yourself is pointless, and the rules forbid it
		}
		return []*hiddenrole.Effect{
			hiddenrole.NewEffect(EventSeerLook, use.PlayerID, target),
			learnPlayer(use.PlayerID, target, card(view, target)),
		}
	}

	out := []*hiddenrole.Effect{hiddenrole.NewEffect(EventSeerLook, use.PlayerID, "")}
	for _, i := range centerIndexes(use.Skill) {
		out = append(out, learnCenter(use.PlayerID, i, centerCard(view, i)))
	}
	return out
}

// robberResolver is the robber phase: swap cards with one player and look at
// the new one.
//
// The order -- swap, then look -- is part of the rules: they know what they
// now hold, and the other player does not.
type robberResolver struct{}

func (robberResolver) Resolve(uses []*hiddenrole.SkillUse, view hiddenrole.GameView) []*hiddenrole.Effect {
	use := firstUse(uses, SkillRob)
	if use == nil {
		return nil
	}
	target := use.Target()
	if target == "" || target == use.PlayerID {
		return nil
	}

	got := card(view, target)
	out := []*hiddenrole.Effect{hiddenrole.NewEffect(EventRobbed, use.PlayerID, target)}
	out = append(out, swapCards(view, use.PlayerID, target)...)
	return append(out, learnSelf(use.PlayerID, got))
}

// troublemakerResolver is the troublemaker phase: swap two other players'
// cards, without looking.
//
// None of the three knows what happened -- the troublemaker did not look, and
// the two who were swapped are not told. It is the most asymmetric move in the
// game.
type troublemakerResolver struct{}

func (troublemakerResolver) Resolve(uses []*hiddenrole.SkillUse, view hiddenrole.GameView) []*hiddenrole.Effect {
	use := firstUse(uses, SkillMeddle)
	if use == nil {
		return nil
	}
	if len(use.Targets) != 2 {
		return nil // exactly two players, no more, no fewer
	}
	a, b := use.Targets[0], use.Targets[1]
	if a == b || a == use.PlayerID || b == use.PlayerID {
		return nil // "two other players" -- not themselves, and not the same person twice
	}

	out := []*hiddenrole.Effect{hiddenrole.NewEffect(EventMeddled, use.PlayerID, "").
		WithData("a", a).WithData("b", b)}
	return append(out, swapCards(view, a, b)...)
}

// drunkResolver is the drunk phase: swap with a centre card, **without
// looking**.
//
// So they do not know which side they now count for -- which is the whole of
// what this role is.
type drunkResolver struct{}

func (drunkResolver) Resolve(uses []*hiddenrole.SkillUse, view hiddenrole.GameView) []*hiddenrole.Effect {
	use := firstUse(uses, SkillDrinkCenter0, SkillDrinkCenter1, SkillDrinkCenter2)
	if use == nil {
		return nil
	}
	idx := centerIndexes(use.Skill)
	if len(idx) != 1 {
		return nil
	}

	out := []*hiddenrole.Effect{hiddenrole.NewEffect(EventDrunkSwap, use.PlayerID, "").
		WithData("center", idx[0])}
	// Note: swap only, and record nothing -- no learn* call.
	return append(out, swapWithCenter(view, use.PlayerID, idx[0])...)
}

// insomniacResolver is the insomniac phase: look at your own card as it now
// stands.
//
// They act last, so what they see is the result of every swap. The ability
// changes no state and leaves only a record -- and the record is necessary,
// see learnSelf.
type insomniacResolver struct{}

func (insomniacResolver) Resolve(_ []*hiddenrole.SkillUse, view hiddenrole.GameView) []*hiddenrole.Effect {
	var out []*hiddenrole.Effect
	for _, id := range dealtWith(view, RoleInsomniac) {
		out = append(out,
			hiddenrole.NewEffect(EventInsomnia, id, ""),
			learnSelf(id, card(view, id)))
	}
	return out
}

// ==================== The vote ====================

// voteResolver is everyone voting at once.
//
// The rules (the official rulebook):
//   - whoever gets the most votes is eliminated and their card is revealed
//   - on a tie, **all** those tied at the top are eliminated
//   - **when everyone gets exactly one vote, nobody is eliminated** -- this is
//     written in the rules, not a special case of a tie
//   - if the hunter is eliminated, so is whoever they voted for
//
// "The hunter" is decided by **the card in their hand now**, not the one they
// were dealt: what is revealed in the morning is the card in hand, and whoever
// holds the hunter card is the hunter.
type voteResolver struct{}

func (voteResolver) Resolve(uses []*hiddenrole.SkillUse, view hiddenrole.GameView) []*hiddenrole.Effect {
	players := view.AllPlayers()

	// One vote each; a repeated submission keeps the first.
	votedBy := make(map[string]string, len(players))
	tally := make(map[string]int, len(players))
	var out []*hiddenrole.Effect
	for _, u := range uses {
		if u.Skill != SkillVote || votedBy[u.PlayerID] != "" {
			continue
		}
		target := u.Target()
		if target == "" || target == u.PlayerID {
			continue // you may not vote for yourself
		}
		if _, ok := view.Player(target); !ok {
			continue
		}
		votedBy[u.PlayerID] = target
		tally[target]++
		out = append(out, hiddenrole.NewEffect(EventVoted, u.PlayerID, target))
	}

	// The vote has resolved. This record is for the victory check: "nobody is
	// eliminated" is a legal outcome, and on the board it looks exactly like
	// "the vote has not happened".
	out = append(out, markVoteSettled())

	// Everyone got exactly one vote: nobody is eliminated. Written in the
	// rules, not a special case of a tie.
	if allTiedAtOne(players, tally) {
		return append(out, hiddenrole.NewEffect(EventNoOneDies, "", ""))
	}

	doomed := topVoted(tally)
	if len(doomed) == 0 {
		return append(out, hiddenrole.NewEffect(EventNoOneDies, "", ""))
	}

	// The hunter takes someone with them: collect the hunter's target first,
	// then resolve them together. The hunter may themselves be the one a
	// hunter took down (two hunters voting for each other), so this runs a
	// single pass -- the rules have no chained shots, unlike werewolf.
	dead := make(map[string]bool, len(doomed))
	for _, id := range doomed {
		dead[id] = true
	}
	for _, id := range doomed {
		if card(view, id) != RoleHunter {
			continue
		}
		hit := votedBy[id]
		if hit == "" || dead[hit] {
			continue
		}
		dead[hit] = true
		out = append(out, hiddenrole.NewEffect(EventHunterHit, id, hit))
	}

	for _, id := range sortedKeys(dead) {
		out = append(out,
			hiddenrole.NewEffect(EventLynched, "", id),
			hiddenrole.NewSetAliveEffect(id, false))
	}
	return out
}

// allTiedAtOne reports whether every player got exactly one vote.
func allTiedAtOne(players []hiddenrole.PlayerInfo, tally map[string]int) bool {
	if len(players) == 0 {
		return false
	}
	for _, p := range players {
		if tally[p.ID] != 1 {
			return false
		}
	}
	return true
}

// topVoted is whoever got the most votes, sorted by ID. Zero votes do not
// count.
func topVoted(tally map[string]int) []string {
	best := 0
	for _, n := range tally {
		if n > best {
			best = n
		}
	}
	if best == 0 {
		return nil
	}
	var out []string
	for id, n := range tally {
		if n == best {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// sortedKeys returns a set's keys in lexicographic order, which is what keeps
// the effect log deterministic.
func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// dealtWith is the players whose dealt card is a given role, sorted by ID.
//
// It uses AllPlayers rather than AlivePlayers: nobody is eliminated before the
// vote in this game, and "who may use an ability" has nothing to do with life
// and death in this ruleset.
func dealtWith(view hiddenrole.GameView, role hiddenrole.RoleType) []string {
	var out []string
	for _, p := range view.AllPlayers() {
		if p.Role == role {
			out = append(out, p.ID)
		}
	}
	sort.Strings(out)
	return out
}
