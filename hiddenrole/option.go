// option.go holds the construction options: everything that must be
// configured before the game starts, gathered into the constructors.
//
// The engine has a set of things that can only be set before play begins:
// custom Resolvers, the logger, metrics. Each used to have its own setter,
// and a setter can only be called once you already hold the engine -- which
// is too late for RestoreEngine and ReplayEngine, the two entry points that
// hand back an engine already mid-game: the restored engine is no longer in
// the START phase, registration is rejected outright, and a custom role's
// skills are silently dropped from then on.
//
// Folding them into construction options buys one more thing: these values
// are fixed before the engine is handed to the caller and never change
// afterwards, so reading them outside the lock no longer needs a defensive
// copy.

package hiddenrole

// EngineOption is an optional setting applied while constructing an engine.
//
// All three entry points (NewEngine / RestoreEngine / ReplayEngine) accept
// them, so an extension role is written the same way whether the game is
// starting fresh or resuming from a save.
type EngineOption func(*Engine) error

// WithResolver registers or replaces one phase's resolver.
//
// This is the only way to extend the game with a new role, and it works for
// RestoreEngine and ReplayEngine just as well:
//
//	cfg := werewolf.DefaultGameConfig()
//	cfg.Phases[myPhase] = &werewolf.PhaseConfig{ ... }
//	engine, err := werewolf.RestoreEngine(cfg, snap,
//		werewolf.WithResolver(myPhase, myResolver))
func WithResolver(phase PhaseType, resolver Resolver) EngineOption {
	return func(e *Engine) error {
		if resolver == nil {
			return WrapError(CodeInvalidConfig,
				"resolver for phase %v must not be nil", phase)
		}
		e.phase.registerResolver(phase, resolver)
		return nil
	}
}

// WithLogger sets the logger. A nil logger leaves the default no-op in place.
func WithLogger(logger Logger) EngineOption {
	return func(e *Engine) error {
		if logger != nil {
			e.logger = logger
		}
		return nil
	}
}

// applyOptions applies the construction options in order.
// The engine has not been handed to the caller yet, so no lock is needed.
func (e *Engine) applyOptions(opts []EngineOption) error {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(e); err != nil {
			return err
		}
	}
	return nil
}

// WithGameSetup registers the game's opening initialisation.
//
// It is called once inside Start(), and the effects it produces land through
// the same write point as every other effect. Registering twice keeps the
// last registration.
func WithGameSetup(setup GameSetup) EngineOption {
	return func(e *Engine) error {
		if setup == nil {
			return WrapError(CodeInvalidConfig, "game setup must not be nil")
		}
		e.gameSetup = setup
		return nil
	}
}
