package hiddenrole

// VarScope is a variable's scope: how long a piece of custom state lives, and
// whom it belongs to.
//
// A scope is a 2x2 table -- lifetime (whole game / this round) crossed with
// ownership (unowned / belonging to some player):
//
//	                 unowned       owned by a player
//	whole game       ScopeGame     ScopeGame.Of(id)
//	this round       ScopeRound    ScopeRound.Of(id)
//
// The table used to exist only in a comment; the code had eight unrelated
// names (four constructors and four readers). Nothing forced it to be
// complete: a missing cell was nobody's job to notice, and in fact one was
// missing -- "whole game, unowned" -- until the mission-based rules ran into
// it: the score, the consecutive-reject count and whose turn it is to lead
// are all game-long and belong to nobody, and had to be filed under some
// arbitrary player as a ledger.
//
// Now the four cells fall out of two values crossed with one method, and a
// missing cell is not expressible.
type VarScope struct {
	// perRound true means the value lives for this round only and is cleared
	// at the round boundary.
	perRound bool
	// owner empty means this state belongs to no player.
	owner string
}

// ScopeGame lives for the whole game and belongs to no player. Scores,
// counters and whose-turn-it-is go here.
//
// Add .Of(playerID) to get "follows one player for the whole game": the
// witch's two potions, the knight's spent duel, the idiot's flipped card.
var ScopeGame = VarScope{}

// ScopeRound lives for this round only and belongs to no player. Tonight's
// kill target goes here.
//
// Add .Of(playerID) to get "marked on someone this round": who was guarded
// tonight, who was healed, who was poisoned. Both round-level cells are
// cleared together on entering the next round (or on a phase configured with
// ClearsRoundVars).
var ScopeRound = VarScope{perRound: true}

// Of binds a scope to a player, leaving the lifetime unchanged.
//
// It returns a copy; the ScopeGame and ScopeRound values themselves are never
// modified.
func (s VarScope) Of(playerID string) VarScope {
	s.owner = playerID
	return s
}

// String is for logging and debugging, in the form game, round, game:p1,
// round:p1.
func (s VarScope) String() string {
	name := "game"
	if s.perRound {
		name = "round"
	}
	if s.owner == "" {
		return name
	}
	return name + ":" + s.owner
}
