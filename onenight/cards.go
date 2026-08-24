// cards.go covers "the card you were dealt" and "the card in your hand now".
//
// This is the deepest difference between One Night and the first two rules
// packages, and the reason this package exists at all: in the first two,
// "which role are you" has one answer from beginning to end; here it has two.
//
//	the card you were dealt    decides what you do at night    never changes    <- the kernel's RoleType
//	the card in your hand now  decides which side you score for gets swapped    <- this package's own game-long state
//
// A robber who steals the werewolf card does not become a wolf and does not
// wake with them -- what they do at night is decided by the card they were
// dealt; but when the game is scored they count as the wolf team. The drunk
// swaps their own card with one from the centre **without looking**, so not
// even they know which side they now count for.
//
// The kernel's RoleType is fixed at seating time with no write path, which is
// exactly the first layer. The second layer is a piece of "game-long, owned by
// one player" state that the rules manage themselves.
//
// **This was originally listed as an abstraction gap in the kernel**
// (DESIGN.md §8.1, "identity is fixed at seating", guessed to block
// card-swapping games). Writing it revealed the guess was wrong: what a
// card-swapping game wants is not a writable RoleType but **two layers of
// identity** -- and one layer from the kernel plus one from the rules is
// exactly enough. See SCARS.md, scar 1.

package onenight

import "github.com/Zereker/hiddenrole"

const (
	// varCard is which card this player holds now. Game-long, owned by one
	// player.
	//
	// Handed out by RoleSetup at seating time with the value of the card they
	// were dealt; rewritten afterwards by the robber, the troublemaker and the
	// drunk.
	varCard = "card"

	// varCenter0/1/2 are the three centre cards. Game-long, owned by nobody.
	//
	// The cell the mission-based rules ran into (whole game, unowned) earns its
	// place again here: a centre card belongs to nobody and should not be
	// cleared each round. Without that cell, three public cards could only be
	// filed under some player as a ledger.
	varCenter0 = "center.0"
	varCenter1 = "center.1"
	varCenter2 = "center.2"
)

// centerKeys are the keys of the three centre cards, in index order.
var centerKeys = [CenterCount]string{varCenter0, varCenter1, varCenter2}

// dealt is the card this player **was dealt**, unchanged for the whole game.
//
// What they do at night is decided by it, not by card: a robber who steals the
// werewolf card does not wake with the wolves. The kernel's RoleType is
// exactly this, and the function only writes "why Role and not card" into a
// name.
func dealt(view hiddenrole.GameView, playerID string) hiddenrole.RoleType {
	p, ok := view.Player(playerID)
	if !ok {
		return hiddenrole.RoleUnspecified
	}
	return p.Role
}

// card is the card this player **holds now**. It decides which side they
// score for.
//
// If it was never written, it falls back to the card they were dealt --
// RoleSetup hands it out at seating time, so this path is only a backstop.
func card(view hiddenrole.GameView, playerID string) hiddenrole.RoleType {
	if v := view.Var(hiddenrole.ScopeGame.Of(playerID), varCard); v != "" {
		return hiddenrole.RoleType(v)
	}
	return dealt(view, playerID)
}

// setCard changes the card in someone's hand.
func setCard(playerID string, role hiddenrole.RoleType) *hiddenrole.Effect {
	return hiddenrole.NewSetVarEffect(hiddenrole.ScopeGame.Of(playerID), varCard, string(role))
}

// centerCard is centre card i, or empty if the index is out of range.
func centerCard(view hiddenrole.GameView, i int) hiddenrole.RoleType {
	if i < 0 || i >= CenterCount {
		return hiddenrole.RoleUnspecified
	}
	return hiddenrole.RoleType(view.Var(hiddenrole.ScopeGame, centerKeys[i]))
}

// setCenterCard changes centre card i.
func setCenterCard(i int, role hiddenrole.RoleType) *hiddenrole.Effect {
	if i < 0 || i >= CenterCount {
		return nil
	}
	return hiddenrole.NewSetVarEffect(hiddenrole.ScopeGame, centerKeys[i], string(role))
}

// swapCards swaps the cards in two players' hands.
//
// The troublemaker and the robber both take this path; the only difference is
// who gets to see the result, which is the information boundary's business,
// not the state's.
func swapCards(view hiddenrole.GameView, a, b string) []*hiddenrole.Effect {
	return []*hiddenrole.Effect{
		setCard(a, card(view, b)),
		setCard(b, card(view, a)),
	}
}

// swapWithCenter swaps someone's card with centre card i.
func swapWithCenter(view hiddenrole.GameView, playerID string, i int) []*hiddenrole.Effect {
	held := card(view, playerID)
	return []*hiddenrole.Effect{
		setCard(playerID, centerCard(view, i)),
		setCenterCard(i, held),
	}
}

// CampOf says which side a card belongs to.
//
// It is **for the host revealing cards**, not for the kernel: the kernel
// recognises the canonical key VarCamp and would carry its value into
// SelfInfo.Camp -- and this ruleset **must not let it**. The drunk does not
// know which card they now hold, and filling their current camp into their own
// view would simply tell them. So this package never writes VarCamp, and the
// camp is computed by the host at the moment cards are revealed. See
// SCARS.md, scar 4.
func CampOf(role hiddenrole.RoleType) hiddenrole.Camp {
	switch role {
	case RoleWerewolf, RoleMinion:
		return CampWolf
	case RoleTanner:
		return CampTanner
	default:
		return CampVillage
	}
}

// isWolfCard reports whether a card is the werewolf card.
//
// Only WEREWOLF counts: the minion is on the wolf team but **is not a wolf**
// -- the victory check for "was a werewolf eliminated" counts werewolf cards,
// not the wolf team.
func isWolfCard(role hiddenrole.RoleType) bool { return role == RoleWerewolf }
