// boundary.go is the information boundary: who is allowed to know what.
//
// This is the hardest part of these games, and the part this library is most
// worth taking off a caller's hands. But "hard" is not the same as "the
// kernel should decide it" -- the kernel used to know three werewolf facts:
//
//	AudienceOf            only the seer sees a check, everyone sees a kill... a hard-coded table of types
//	PlayerView.Teammates  same camp means mutually visible, and "camp" had exactly two values
//	MessageReceivers      at night only the wolves may speak, in the day everyone hears
//
// Change the rules and none of the three survives: Merlin in the missions
// package sees the bad guys while they do not know who he is, the demon and
// minions in Blood on the Clocktower see each other one-way only, and who may
// speak when is every ruleset's own business.
//
// All three questions are now answered by the rules, and the kernel
// guarantees exactly one thing: **its own state primitives never leave the
// building**. Werewolf's answers live in wolfboundary.go; they hold no
// privilege and can be replaced wholesale.

package hiddenrole

// AudienceProvider answers "which players should be told about this".
//
// Same shape as Resolver and VictoryChecker: it takes a read-only GameView,
// returns a conclusion, and touches no state. It is called while the engine
// holds its lock, so an implementation must not call back into any Engine
// method -- the consequence is a hang, not an error. See "Extension points
// must not call back into the engine" in doc.go.
//
// The second result is "do I recognise this event type", which is a different
// thing from "show it to nobody" and must stay distinguishable: the former
// asks the caller to route it themselves, the latter is a definite verdict.
// When it is false the first result is ignored.
type AudienceProvider interface {
	Audience(event *Event, view GameView) ([]string, bool)
}

// AudienceFunc lets a plain function satisfy AudienceProvider.
type AudienceFunc func(event *Event, view GameView) ([]string, bool)

// Audience implements AudienceProvider.
func (f AudienceFunc) Audience(event *Event, view GameView) ([]string, bool) {
	return f(event, view)
}

// WithAudience replaces the "who should be told" decision.
//
// The kernel's state primitives are filtered out before this point and never
// reach it: they are the state machine's bookkeeping, they have no business
// in front of any player, and that part is not configurable.
func WithAudience(provider AudienceProvider) EngineOption {
	return func(e *Engine) error {
		if provider == nil {
			return WrapError(CodeInvalidConfig, "audience provider must not be nil")
		}
		e.audience = provider
		return nil
	}
}

// TeammateProvider answers "who is this player told is on their side".
//
// The IDs it returns appear in PlayerView.Teammates and
// RolePhaseInfo.Teammates, and those players' roles are revealed to them. It
// excludes the player themselves; returning nil means they know of no
// teammates.
//
// This relation is allowed to be **asymmetric**: the demon in Blood on the
// Clocktower knows its minions, and the reverse does not hold. The kernel
// does not check the two directions against each other.
//
// It is called while the engine holds its lock, so an implementation must not
// call back into any Engine method -- the consequence is a hang, not an
// error. See "Extension points must not call back into the engine" in doc.go.
type TeammateProvider interface {
	Teammates(playerID string, view GameView) []string
}

// TeammateFunc lets a plain function satisfy TeammateProvider.
type TeammateFunc func(playerID string, view GameView) []string

// Teammates implements TeammateProvider.
func (f TeammateFunc) Teammates(playerID string, view GameView) []string {
	return f(playerID, view)
}

// WithTeammates replaces the "who is on whose side" decision.
func WithTeammates(provider TeammateProvider) EngineOption {
	return func(e *Engine) error {
		if provider == nil {
			return WrapError(CodeInvalidConfig, "teammate provider must not be nil")
		}
		e.teammates = provider
		return nil
	}
}

// SpeechProvider answers "if this player speaks right now, who hears it".
//
// By convention the returned list includes the sender, so a caller can
// broadcast to it directly. Returning nil means they cannot speak at this
// moment.
//
// It is called while the engine holds its lock, so an implementation must not
// call back into any Engine method -- the consequence is a hang, not an
// error. See "Extension points must not call back into the engine" in doc.go.
type SpeechProvider interface {
	Receivers(senderID string, view GameView) []string
}

// SpeechFunc lets a plain function satisfy SpeechProvider.
type SpeechFunc func(senderID string, view GameView) []string

// Receivers implements SpeechProvider.
func (f SpeechFunc) Receivers(senderID string, view GameView) []string {
	return f(senderID, view)
}

// WithSpeech replaces the audible range of speech.
func WithSpeech(provider SpeechProvider) EngineOption {
	return func(e *Engine) error {
		if provider == nil {
			return WrapError(CodeInvalidConfig, "speech provider must not be nil")
		}
		e.speech = provider
		return nil
	}
}

// teammatesOf computes one player's teammates. The caller must hold e.mu.
func (e *Engine) teammatesOf(playerID string) []string {
	if e.teammates == nil {
		return nil
	}
	return e.teammates.Teammates(playerID, newStateView(e.state))
}
