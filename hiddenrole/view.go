package hiddenrole

// GameView is a read-only view of the game.
//
// A Resolver is handed this rather than a *gameState. "Every state change
// goes through an Effect" is this engine's most important invariant, and it
// used to live only in the documentation with no help from the type system --
// any Resolver, including a third-party one, could mutate state directly and
// bypass the whole effect pipeline, forfeiting replayability and
// auditability. The constraint is now part of the signature.
//
// The view offers facts, never judgements: judging is the Resolver's job, so
// what you get here is "who was guarded last round", not "may I guard right
// now".
type GameView interface {
	// Player returns a read-only copy of a player's information.
	Player(id string) (PlayerInfo, bool)

	// AlivePlayers returns every living player, sorted by ID.
	//
	// The ordering is something the rules may rely on: the order of the
	// effects a rule produces has to be uniquely determined by the board, or
	// replay and snapshot comparison lose their determinism.
	AlivePlayers() []PlayerInfo

	// AllPlayers returns every player including the eliminated ones, sorted
	// by ID.
	//
	// Victory checks need it: "how many special roles were there at the
	// start" has to count the dead ones too, and a wipe-out condition cannot
	// be computed from the living alone.
	AllPlayers() []PlayerInfo

	// AlivePlayerIDsByRole returns the IDs of living players with the given role.
	AlivePlayerIDsByRole(role RoleType) []string

	// RoundContext returns a read-only copy of this round's context.
	RoundContext() RoundContext

	// Var returns one piece of custom state in the given scope, or the empty
	// string if it is not set.
	//
	// Scopes form a 2x2 table (see VarScope):
	//
	//	Var(ScopeGame, "score")            whole game, unowned
	//	Var(ScopeGame.Of(id), "antidote")  whole game, one player
	//	Var(ScopeRound, "kill")            this round, unowned
	//	Var(ScopeRound.Of(id), "guarded")  this round, one player
	//
	// The rules keep all of their own state here, and built-in roles take the
	// same route as third-party ones. Writes go through NewSetVarEffect, and
	// a player's initial state is handed out by RoleSetup.
	Var(scope VarScope, key string) string

	// Round returns the current round number.
	Round() int

	// Phase returns the current phase.
	Phase() PhaseType
}

// stateView implements GameView.
//
// It is deliberately an unexported wrapper rather than letting *gameState
// implement the interface directly: the latter could be type-asserted back
// into the mutable state object, which would be no constraint at all.
type stateView struct {
	s *gameState
}

func newStateView(s *gameState) GameView { return stateView{s: s} }

func (v stateView) Player(id string) (PlayerInfo, bool) {
	return v.s.PlayerInfo(id)
}

func (v stateView) AlivePlayers() []PlayerInfo {
	ids := v.s.getAlivePlayerIDs()
	out := make([]PlayerInfo, 0, len(ids))
	for _, id := range ids {
		if info, ok := v.s.PlayerInfo(id); ok {
			out = append(out, info)
		}
	}
	return out
}

func (v stateView) AllPlayers() []PlayerInfo {
	ids := v.s.allPlayerIDs()
	out := make([]PlayerInfo, 0, len(ids))
	for _, id := range ids {
		if info, ok := v.s.PlayerInfo(id); ok {
			out = append(out, info)
		}
	}
	return out
}

func (v stateView) AlivePlayerIDsByRole(role RoleType) []string {
	return v.s.getAlivePlayerIDsByRole(role)
}

func (v stateView) RoundContext() RoundContext {
	rc := v.s.RoundContext()
	if rc == nil {
		return RoundContext{}
	}
	return *rc
}

func (v stateView) Var(scope VarScope, key string) string {
	return v.s.varOf(scope, key)
}

func (v stateView) Round() int { return v.s.currentRound() }

func (v stateView) Phase() PhaseType { return v.s.currentPhase() }
