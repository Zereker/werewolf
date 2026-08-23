package hiddenrole

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// publicKernelEvents are the events the kernel emits that players **should**
// see.
//
// Together with kernelEvents they must cover every event type declared in
// event.go -- enforced by TestKernelEventTypes_AreAllClassified.
var publicKernelEvents = map[EventType]bool{
	EventUnspecified: true, // the zero value, not a real event
	EventGameStarted: true,
	EventGameEnded:   true,
}

// TestKernelEventTypes_AreAllClassified: every kernel event type must be
// explicitly classified.
//
// "State primitives never leave the building" is the kernel's only
// non-configurable rule, and what decides it is kernelEvents, a table
// maintained by hand. A hand-maintained table has one fixed bad ending:
// somebody adds an eighth kernel event type, forgets to add a row, and that
// event is then treated as an external one and handed to the
// AudienceProvider -- a provider that gives everything to everybody would
// push the state machine's bookkeeping to every player.
//
// This test treats event.go as the truth: it parses the source for every
// EventXxx declaration and requires each to appear in kernelEvents or
// publicKernelEvents. Add an event type without classifying it and it goes
// red -- you have to answer "should a player see this".
func TestKernelEventTypes_AreAllClassified(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "event.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing event.go: %v", err)
	}

	var declared []string
	ast.Inspect(f, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		ident, ok := spec.Type.(*ast.Ident)
		if !ok || ident.Name != "EventType" {
			return true
		}
		for _, name := range spec.Names {
			declared = append(declared, name.Name)
		}
		return true
	})
	if len(declared) == 0 {
		t.Fatal("no EventType constant was parsed out of event.go -- this test is meaningless")
	}

	// Name -> value. The constants are in this package, so map them by name.
	byName := map[string]EventType{
		"EventUnspecified":  EventUnspecified,
		"EventGameStarted":  EventGameStarted,
		"EventGameEnded":    EventGameEnded,
		"EventDetour":       EventDetour,
		"EventPlayerAdded":  EventPlayerAdded,
		"EventPhaseChanged": EventPhaseChanged,
		"EventSetAlive":     EventSetAlive,
		"EventSetVar":       EventSetVar,
		"EventGotoPhase":    EventGotoPhase,
		"EventSetActors":    EventSetActors,
	}

	for _, name := range declared {
		v, known := byName[name]
		if !known {
			t.Errorf("event.go gained %s but this test's value table has not caught up -- "+
				"add a row, and decide whether it belongs in kernelEvents or publicKernelEvents", name)
			continue
		}
		switch {
		case isInternalEvent(v) && publicKernelEvents[v]:
			t.Errorf("%s is classified as both a state primitive and a public event", name)
		case !isInternalEvent(v) && !publicKernelEvents[v]:
			t.Errorf("%s (%q) is unclassified: staying out of kernelEvents means it is "+
				"treated as an external event and handed to the AudienceProvider, and a "+
				"provider that gives everything to everybody would push it to every "+
				"player. Decide whether a player should see it", name, v)
		}
	}
}

// primitiveSpewer emits one of every kernel state primitive.
type primitiveSpewer struct{}

func (primitiveSpewer) Resolve([]*SkillUse, GameView) []*Effect {
	return []*Effect{
		NewSetAliveEffect("g", false),
		NewSetVarEffect(ScopeGame.Of("w1"), "probe.var", "1"),
		NewSetVarEffect(ScopeRound, "probe.round", "1"),
		NewSetVarEffect(ScopeRound.Of("w1"), "probe.mark", "1"),
		NewDetourEffect("w1", phaseNightHunter),
		NewGotoPhaseEffect(phaseDay),
		NewSetVarEffect(ScopeGame, "probe.game", "1"),
		NewSetActorsEffect(phaseDay, "w1"),
		NewEffect(EventType("PROBE_PUBLIC"), "w1", "g"), // an ordinary rule event, as a control
	}
}

// TestBoundary_StatePrimitivesNeverReachPlayers: state primitives reach no
// player, down either path.
//
// On the information boundary the kernel holds one line, and it is not
// configurable: its own state primitives never leave the building. There are
// two paths they could leak down, and only the first used to be watched:
//
//   - AudienceOf: it must hold even when the rules install a provider that
//     gives everything to everybody;
//   - OnEvent: forwarding whatever arrives is a natural thing for a host to
//     do, and a state primitive slipping down this path pushes the god's view
//     straight to everyone.
func TestBoundary_StatePrimitivesNeverReachPlayers(t *testing.T) {
	opts := append(withNoopResolvers(),
		WithResolver(phaseNightGuard, primitiveSpewer{}),
		// A worst-case provider: give everything to everybody.
		WithAudience(AudienceFunc(func(*Event, GameView) ([]string, bool) {
			return []string{"w1", "g"}, true
		})))
	e := newTestEngine(t, opts...)
	mustAdd(t, e, "w1", roleWerewolf)
	mustAdd(t, e, "g", roleGuard)

	var seen []EventType
	e.OnEvent(func(ev *Event) { seen = append(seen, ev.Type) })

	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := e.EndPhase(); err != nil {
		t.Fatalf("EndPhase: %v", err)
	}

	// 1. The AudienceOf path.
	for _, ef := range (primitiveSpewer{}).Resolve(nil, nil) {
		got, known := e.AudienceOf(ef.ToEvent())
		if !isInternalEvent(ef.Type) {
			continue // control: an ordinary rule event should go to the provider
		}
		if !known {
			t.Errorf("%v should be a definite verdict, not an \"I don't know\"", ef.Type)
		}
		if len(got) != 0 {
			t.Errorf("%v is a state primitive and should go to nobody, got %v", ef.Type, got)
		}
	}

	// 2. The OnEvent path.
	sawPublic := false
	for _, typ := range seen {
		if isInternalEvent(typ) {
			t.Errorf("state primitive %v reached OnEvent -- a host forwarding it verbatim would send out the god's view", typ)
		}
		if typ == EventType("PROBE_PUBLIC") {
			sawPublic = true
		}
	}
	if !sawPublic {
		t.Error("an ordinary rule event never reached OnEvent -- this test may be checking nothing")
	}
}
