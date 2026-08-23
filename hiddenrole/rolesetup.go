// rolesetup.go covers a role's initial state: what a player of this role
// brings to the table.
//
// The engine used to say, at seating time, `if role == some specific role {
// hand out something }` -- the last place inside the kernel where control
// flow branched on a concrete role. The cost was not those three lines; it
// was that a third-party role had **no way at all** to give itself an initial
// state: a knight who starts with one duel, a dream-weaver who starts with
// two lives, both required editing the engine to express. Adding a role
// should not require editing the engine.

package hiddenrole

// RoleSetup answers "what state does this role sit down with".
//
// Same shape as Resolver, VictoryChecker and RoleInfoProvider: it touches no
// state and only returns a conclusion. The key/value pairs it returns are
// written verbatim into that player's Vars, and are afterwards read with
// GameView.Var(ScopeGame.Of(id), key) and changed with NewSetVarEffect.
//
// Seating happens before the game starts, when there is no board to look at
// yet, which is why the signature has no GameView: an initial state can only
// be decided by the role itself, never by who sat down first or who else is
// at the table. Initialisation that does need to see the board (cupid
// pairing lovers, the thief picking a spare card) is a phase, and belongs in
// a Resolver.
//
// Returning nil or an empty map means this role carries no initial state.
//
// It is called while the engine holds its lock, so an implementation must not
// call back into any Engine method -- the consequence is a hang, not an
// error. See "Extension points must not call back into the engine" in doc.go.
type RoleSetup interface {
	Setup(playerID string, role RoleType) map[string]string
}

// RoleSetupFunc lets a plain function satisfy RoleSetup.
type RoleSetupFunc func(playerID string, role RoleType) map[string]string

// Setup implements RoleSetup.
func (f RoleSetupFunc) Setup(playerID string, role RoleType) map[string]string {
	return f(playerID, role)
}

// WithRoleSetup registers an initial state for one role.
//
//	const roleKnight = engine.RoleType("KNIGHT")
//
//	e, _ := engine.NewEngine(cfg,
//		engine.WithResolver(phaseKnight, knightResolver{}),
//		engine.WithRoleSetup(roleKnight, engine.RoleSetupFunc(
//			func(id string, role engine.RoleType) map[string]string {
//				return map[string]string{"knight.duel": "1"}
//			})))
//
// Registering the same role twice keeps the last registration, so this is
// also how you replace a built-in one (a witch who starts with her antidote
// already spent, say).
//
// Neither replay nor restore needs it passed again: the initial state is
// recorded on the seating entry of the effect log (ReplayEngine) and in the
// snapshot's Vars (RestoreEngine). This differs from resolvers: a resolver is
// a rule and must be supplied by the caller, whereas an initial state is a
// fact, and recording it is enough.
func WithRoleSetup(role RoleType, setup RoleSetup) EngineOption {
	return func(e *Engine) error {
		if setup == nil {
			return WrapError(CodeInvalidConfig,
				"role setup for %v must not be nil", role)
		}
		e.roleSetup[role] = setup
		return nil
	}
}

// VarPresent is the conventional "present" value for boolean-ish Vars.
//
// Vars values are strings, and at the write point an empty string is
// equivalent to deletion, so a has-it/hasn't-it state needs nothing more than
// one non-empty value. The built-in roles all use this one; extensions are
// under no obligation to.
const VarPresent = "1"

// setupFor computes one player's initial state. The caller must hold e.mu.
func (e *Engine) setupFor(playerID string, role RoleType) map[string]string {
	setup, ok := e.roleSetup[role]
	if !ok {
		return nil
	}
	return setup.Setup(playerID, role)
}

// GameSetup is how the rules lay out the board at the moment play begins.
//
// It pairs with RoleSetup: that one covers what **one player** sits down
// with, this one covers the initial state of the **whole game**. It sees the
// board with everyone already seated, so it can do what RoleSetup cannot --
// "which seat leads first", for instance, which depends on who is at the
// table.
//
// Typical uses: initialising game-long counters (a game-scoped Var), and
// **naming the actors of the first phase** (SetActors). The latter is the
// direct reason this extension point exists: the set of actors is normally
// computed by the previous phase's resolver, and the first phase has no
// previous phase.
//
// It is called once inside Start(), and the effects it produces go through
// exactly the same write point as every other effect, so they enter the
// effect log, replay, and the snapshot.
//
// Same contract as every other extension point: it may read GameView only,
// it is called while the engine holds its lock, and it must not call back
// into any Engine method -- the consequence is a hang, not an error. See
// "Extension points must not call back into the engine" in doc.go.
type GameSetup interface {
	Setup(view GameView) []*Effect
}

// GameSetupFunc lets a plain function satisfy GameSetup.
type GameSetupFunc func(view GameView) []*Effect

// Setup implements GameSetup.
func (f GameSetupFunc) Setup(view GameView) []*Effect { return f(view) }
