// roleinfo.go is role-specific information: what extra a player of some role
// gets to see.
//
// The kernel knows only the generic facts -- who is alive, whose turn it is
// to act. "The witch sees tonight's kill" and "the thief sees the two spare
// cards" are the role's own rules, and the rules answer them.

package hiddenrole

// RoleInfoProvider answers "what else should this player know".
//
// Same shape as Resolver and VictoryChecker: it takes a read-only GameView,
// returns a conclusion, and touches no state. It is called while the engine
// holds its lock, so an implementation must not call back into any Engine
// method -- the consequence is a hang, not an error. See "Extension points
// must not call back into the engine" in doc.go.
//
// Returning nil or an empty map means there is nothing extra. The keys are
// the role's own, and appear verbatim in PlayerView.RoleInfo and
// RolePhaseInfo.RoleInfo.
type RoleInfoProvider interface {
	RoleInfo(playerID string, view GameView) map[string]string
}

// RoleInfoFunc lets a plain function satisfy RoleInfoProvider.
type RoleInfoFunc func(playerID string, view GameView) map[string]string

// RoleInfo implements RoleInfoProvider.
func (f RoleInfoFunc) RoleInfo(playerID string, view GameView) map[string]string {
	return f(playerID, view)
}

// WithRoleInfo registers a provider of role-specific information for one role.
//
//	engine, _ := werewolf.NewEngine(cfg,
//		werewolf.WithResolver(phaseThief, thiefResolver{}),
//		werewolf.WithRoleInfo(roleThief, werewolf.RoleInfoFunc(
//			func(id string, view werewolf.GameView) map[string]string {
//				return map[string]string{"spare_cards": view.Var(werewolf.ScopeRound, "thief.spares")}
//			})))
//
// Registering the same role twice keeps the last registration, so this is
// also how you replace a built-in provider.
func WithRoleInfo(role RoleType, provider RoleInfoProvider) EngineOption {
	return func(e *Engine) error {
		if provider == nil {
			return WrapError(CodeInvalidConfig,
				"role info provider for %v must not be nil", role)
		}
		e.roleInfo[role] = provider
		return nil
	}
}

// roleInfoFor computes one player's role-specific information. The caller
// must hold e.mu.
func (e *Engine) roleInfoFor(playerID string, role RoleType) map[string]string {
	provider, ok := e.roleInfo[role]
	if !ok {
		return nil
	}
	info := provider.RoleInfo(playerID, newStateView(e.state))
	if len(info) == 0 {
		return nil
	}
	return info
}
