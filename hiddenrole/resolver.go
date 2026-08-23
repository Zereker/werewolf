// resolver.go is the phase-resolution extension point.
//
// The kernel does not know how any phase resolves -- only that "time is up,
// go ask this phase's resolver what happened". Werewolf's seven resolvers
// live in resolver.go of the root package.

package hiddenrole

// Resolver resolves the conflicts of one phase.
//
// An implementation may read GameView only, and may express state changes
// only by returning Effects -- the engine's most important invariant, held up
// by the signature rather than by convention.
//
// Note: Resolve is called while the engine holds its lock, so an
// implementation must not call back into any Engine method -- the consequence
// is a hang, not an error. See "Extension points must not call back into the
// engine" in doc.go.
type Resolver interface {
	Resolve(uses []*SkillUse, view GameView) []*Effect
}

// ResolverFunc lets a plain function satisfy Resolver.
//
// Same thing as AudienceFunc and RoleSetupFunc. Of the eight extension
// points, Resolver and VictoryChecker were the only two without this adapter
// -- no reason, just history -- which meant that installing a three-line
// resolver first required declaring an empty struct.
type ResolverFunc func(uses []*SkillUse, view GameView) []*Effect

// Resolve implements Resolver.
func (f ResolverFunc) Resolve(uses []*SkillUse, view GameView) []*Effect {
	return f(uses, view)
}
