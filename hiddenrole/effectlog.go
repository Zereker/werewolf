package hiddenrole

// Keys used in the effect log to rebuild the board.
//
// There used to be camp and category here too: they were once part of kernel
// state, so the seating effect had to record them separately. They are now
// just two pieces of state on a player, and travel along with vars.

const (
	roleKey  = "role"
	phaseKey = "phase"
	varsKey  = "vars"
)

// newPlayerAddedEffect records a player taking a seat, along with the role's
// initial state.
//
// It records vars rather than asking RoleSetup again during replay; see
// Engine.seatPlayer for why.
func newPlayerAddedEffect(id string, role RoleType, vars map[string]string) *Effect {
	effect := NewEffect(EventPlayerAdded, "", id).
		WithData(roleKey, role)
	if len(vars) > 0 {
		effect = effect.WithData(varsKey, copyVars(vars))
	}
	return effect
}

// newPhaseChangedEffect records a phase transition.
func newPhaseChangedEffect(phase PhaseType) *Effect {
	return NewEffect(EventPhaseChanged, "", "").
		WithData(phaseKey, phase)
}

// newGameStartedEffect records the start of the game.
func newGameStartedEffect(phase PhaseType) *Effect {
	return NewEffect(EventGameStarted, "", "").
		WithData(phaseKey, phase)
}

// EffectLog returns the complete effect log since the game was created.
//
// This architecture already produces a clean event stream -- a Resolver is a
// pure function, and every state change goes through the single write point
// of applyEffect -- so accumulating it is nearly free, and it gives replays,
// post-game analysis and "what actually happened on night three"
// investigations something to stand on.
//
// The returned slice is a copy, but the *Effect values inside it are the
// engine's own objects; do not modify them.
//
// # Division of labour with Snapshot
//
// The effect log is history; a snapshot is state. For persistence use
// Snapshot: Effect.Data is a map[string]interface{} whose types degrade on a
// JSON round trip, and the effect log is designed for in-process replay and
// auditing, not as a storage format.
func (e *Engine) EffectLog() []*Effect {
	e.mu.RLock()
	defer e.mu.RUnlock()

	out := make([]*Effect, len(e.effectLog))
	for i, ef := range e.effectLog {
		out[i] = ef.clone()
	}
	return out
}

// ReplayEngine rebuilds an engine from an effect log.
//
// config must match the one used during recording -- the effect log records
// what happened, not the rules.
//
// The rebuilt engine matches the recorded one in player state, phase and
// round; but skills submitted in the current phase and not yet resolved are
// not in the effect log (they have not become effects yet), so use Snapshot
// if you need those.
//
// Resolvers for custom roles must be passed through opts, for the same reason
// as with RestoreEngine. Initial state need not be: it is recorded on the
// seating entry of the effect log (see Engine.seatPlayer).
func ReplayEngine(config *Config, log []*Effect, opts ...EngineOption) (*Engine, error) {
	engine, err := NewEngine(config, opts...)
	if err != nil {
		return nil, err
	}

	if err := engine.phase.validateResolvers(); err != nil {
		return nil, err
	}

	engine.mu.Lock()
	defer engine.mu.Unlock()

	for i, effect := range log {
		if effect == nil {
			return nil, WrapError(CodeInvalidEffectLog,
				"effect log contains a nil entry at index %d", i)
		}
		if err := engine.replayEffect(effect); err != nil {
			return nil, err
		}
	}

	engine.recordEffects(log...)

	return engine, nil
}

// replayEffect replays one effect.
func (e *Engine) replayEffect(effect *Effect) error {
	switch effect.Type {
	case EventPlayerAdded:
		role, _ := effect.Data[roleKey].(RoleType)
		// Seating has to hand out the initial state too. Without this the
		// replayed witch holds no potions and the wolves belong to no camp,
		// and the divergence only surfaces when a potion is used or victory is
		// checked.
		vars, _ := effect.Data[varsKey].(map[string]string)
		if err := e.seatPlayer(effect.TargetID, role, vars); err != nil {
			return err
		}

	case EventGameStarted:
		phase, ok := effect.Data[phaseKey].(PhaseType)
		if !ok {
			return WrapError(CodeInvalidEffectLog,
				"game started effect carries no phase")
		}
		e.state.startAt(phase)

	case EventPhaseChanged:
		phase, ok := effect.Data[phaseKey].(PhaseType)
		if !ok {
			return WrapError(CodeInvalidEffectLog,
				"phase changed effect carries no phase")
		}
		// Leaving a phase consumes what belongs to it, exactly as normal
		// progression does. Without this the replayed engine carries a detour
		// that should have been consumed and diverges from the original on the
		// very next step.
		//
		// The round boundary is declared by the phase **just left**, so read
		// it before transitioning. Same rule as normal progression: it must
		// not fall while a detour is still pending, or it would wipe out the
		// pending queue that lives in the round context.
		endsRound := e.config.endsRound(e.state.Phase)
		// The actor list is one-shot too, exactly as in normal progression.
		// Without this the replayed engine carries the previous phase's list
		// and diverges from the original on the next step -- which was missing
		// here all along, only werewolf does not use SET_ACTORS, so no effect
		// log had ever reached this line.
		e.state.leavePhase()
		settled := !e.state.hasPendingDetour()
		e.state.nextPhase(phase, endsRound && settled,
			settled && e.config.clearsRoundVars(phase))

	case EventGameEnded:
		// Ending also leaves the current phase -- on the normal path
		// leavePhase runs **before** the victory check, whether the game ends
		// or not. Miss it and the replayed engine carries the last phase's
		// actor list and an unconsumed detour, and diverges from the original.
		e.state.leavePhase()
		e.state.nextPhase(PhaseEnd, false, false) // the game ends; this is not a new round
		// The winner travels in the effect log. Who won was decided by the
		// VictoryChecker at the moment the game ended, and replay does not run
		// the check again -- without reading it the replayed engine has
		// Over=true and an empty Winner, and diverges from the original.
		//
		// This and the snapshot path were the same bug in two places. The
		// previous round fixed only the snapshot one; this one was caught by
		// the random-game invariants, which compare the replayed and original
		// snapshots byte for byte.
		if winner, ok := effect.Data[winnerKey].(Camp); ok {
			e.winner = winner
		}

	default:
		e.state.applyEffect(effect)
	}
	return nil
}

// recordEffects appends a batch of effects to the history.
//
// It stores copies: the same effects are returned verbatim to EndPhase's
// caller, and were they to share pointers, the caller changing one field
// would change the engine's history. This is the effect log's single write
// point, mirroring applyEffect as the single write point for state.
func (e *Engine) recordEffects(effects ...*Effect) {
	for _, ef := range effects {
		e.effectLog = append(e.effectLog, ef.clone())
	}
}
