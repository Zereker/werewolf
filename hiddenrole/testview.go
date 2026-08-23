// testview.go builds a GameView by hand.
//
// A rules package needs this to unit-test its own resolvers:
// `Resolver.Resolve(uses, view)` takes a GameView, and a rules package sits
// outside the kernel with no access to the kernel's internal state. Without
// this entry point a rule's resolver could only be exercised by running a
// whole game -- which tests the integration, not the resolver.
//
// The names do not start with Test because this is a genuine public API, not
// a helper living in a test file.

package hiddenrole

// Board is a board laid out by hand, used to construct a GameView.
type Board struct {
	// Players are the players at the table. The order does not matter; the
	// view sorts by ID.
	Players []PlayerInfo

	// Round is the current round, counting from 1. Zero is treated as 1.
	Round int

	// Phase is the current phase.
	Phase PhaseType

	// Vars is state that lives for the whole game and belongs to no player
	// (ScopeGame).
	Vars map[string]string

	// RoundVars is state that lives for this round and belongs to no player
	// (ScopeRound).
	//
	// The two owned cells live on PlayerInfo (Vars / RoundVars), and all four
	// are needed before an arbitrary board can be laid out. The cell above
	// used to be missing here, for the same reason the kernel was missing one:
	// werewolf does not need it, so nobody noticed.
	RoundVars map[string]string
}

// Apply folds a batch of effects into the board and returns the modified
// copy.
//
// A rules test uses it to catch a resolver's output --
// `b = b.Apply(r.Resolve(uses, b.View()))` -- and then asserts what the board
// became. It goes through exactly the same write point as the engine, so an
// effect that fails to land shows up in a unit test rather than requiring a
// whole game to be run.
//
// A vetoed effect and a type the kernel does not recognise both change
// nothing -- which is precisely what this is meant to verify.
func (b Board) Apply(effects []*Effect) Board {
	s := b.state()
	for _, ef := range effects {
		s.applyEffect(ef)
	}
	return boardOf(s)
}

// View builds a read-only view of this board.
//
// The returned view is a snapshot: modifying the Board afterwards does not
// affect it.
func (b Board) View() GameView { return newStateView(b.state()) }

// Player returns one player; the second result is false when there is no
// such player.
func (b Board) Player(id string) (PlayerInfo, bool) {
	for _, p := range b.Players {
		if p.ID == id {
			return p, true
		}
	}
	return PlayerInfo{}, false
}

// Var reads one piece of state in the given scope; all four cells are
// readable (see VarScope).
func (b Board) Var(scope VarScope, key string) string { return b.state().varOf(scope, key) }

// state turns the board back into internal state.
func (b Board) state() *gameState {
	round := b.Round
	if round < 1 {
		round = 1
	}
	s := newState()
	s.Round = round
	s.Phase = b.Phase
	s.Vars = copyVars(b.Vars)
	s.RoundCtx = newRoundContext()
	s.RoundCtx.Vars = copyVars(b.RoundVars)
	for _, p := range b.Players {
		s.players[p.ID] = &playerState{
			ID: p.ID, Role: p.Role, Alive: p.Alive,
			Vars: copyVars(p.Vars), RoundVars: copyVars(p.RoundVars),
		}
	}
	return s
}

// boardOf exports internal state back into a board.
func boardOf(s *gameState) Board {
	b := Board{
		Round: s.Round, Phase: s.Phase,
		Vars: copyVars(s.Vars), RoundVars: copyVars(s.RoundCtx.Vars),
	}
	for _, id := range s.allPlayerIDs() {
		if info, ok := s.PlayerInfo(id); ok {
			b.Players = append(b.Players, info)
		}
	}
	return b
}

// Seat builds one player for use in a Board. vars is a variadic list of
// alternating keys and values.
//
//	engine.Seat("wi", "WITCH", true, engine.VarCamp, "GOOD", "witch.antidote", "1")
//
// If the count is odd the trailing lone key is ignored -- this is a test
// helper, and a mistyped call is not worth an error return.
func Seat(id string, role RoleType, alive bool, vars ...string) PlayerInfo {
	p := PlayerInfo{ID: id, Role: role, Alive: alive}
	for i := 0; i+1 < len(vars); i += 2 {
		if p.Vars == nil {
			p.Vars = make(map[string]string, len(vars)/2)
		}
		p.Vars[vars[i]] = vars[i+1]
	}
	return p
}

// Mark adds this round's markers to a player and returns the modified copy.
func Mark(p PlayerInfo, keys ...string) PlayerInfo {
	if len(keys) == 0 {
		return p
	}
	p.RoundVars = copyVars(p.RoundVars)
	if p.RoundVars == nil {
		p.RoundVars = make(map[string]string, len(keys))
	}
	for _, k := range keys {
		p.RoundVars[k] = VarPresent
	}
	return p
}
