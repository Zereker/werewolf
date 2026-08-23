// victory.go is the victory-check extension point.
//
// The kernel does not know what winning means -- only that somebody might
// win, and what that conclusion looks like (a Camp label). The condition
// itself comes from the rules; werewolf's lives in victory.go of the root
// package.

package hiddenrole

// VictoryChecker decides whether the game is decided at this moment.
//
// Returning (false, CampUnspecified) means it is not decided yet. winner may
// be any camp the rules like -- Camp is a string underneath, the kernel
// presumes no values and only reports the conclusion back verbatim.
//
// Same contract as Resolver: it may read GameView only, and it is called
// while the engine holds its lock, so an implementation must not call back
// into any Engine method -- the consequence is a hang, not an error. See
// "Extension points must not call back into the engine" in doc.go.
type VictoryChecker interface {
	CheckVictory(view GameView) (over bool, winner Camp)
}

// VictoryFunc lets a plain function satisfy VictoryChecker.
//
// Like ResolverFunc, this is filling a gap: the eight extension points should
// all be assembled the same way, and there was no reason for these two to be
// the exceptions.
type VictoryFunc func(view GameView) (over bool, winner Camp)

// CheckVictory implements VictoryChecker.
func (f VictoryFunc) CheckVictory(view GameView) (bool, Camp) { return f(view) }

// WithVictoryChecker replaces the built-in victory check.
//
// Once replaced, Config.VictoryMode no longer has any effect -- that field
// only feeds the built-in check. To add a condition on top of the built-in
// rules (say "the lovers win if both survive"), wrap DefaultVictoryChecker:
// ask your own condition first, then ask it.
func WithVictoryChecker(checker VictoryChecker) EngineOption {
	return func(e *Engine) error {
		if checker == nil {
			return WrapError(CodeInvalidConfig, "victory checker must not be nil")
		}
		e.victory = checker
		return nil
	}
}

// neverEnds is the kernel's default check: the game never ends.
//
// The kernel does not know what winning means, so its default can only be "I
// don't know". It is a check that never fires rather than a nil field because
// an engine carrying nothing but the kernel should be able to advance phases
// and simply never decide a winner, instead of nil-panicking on the first
// Start. A rules package always replaces it via WithVictoryChecker (see
// werewolf.Options).
type neverEnds struct{}

func (neverEnds) CheckVictory(GameView) (bool, Camp) { return false, CampUnspecified }
