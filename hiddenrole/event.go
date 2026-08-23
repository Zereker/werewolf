// event.go is the outward event: what the engine did, told to the caller.
//
// Event and Effect are deliberately two layers: an Effect is the engine's
// internal description of a state change and carries interface{} payloads; an
// Event is the shape handed to the caller, with the payload flattened into
// strings so it can be serialised and sent straight out.

package hiddenrole

// Event is one externally visible thing that happened.
//
// Built by Effect.ToEvent and received by handlers registered through
// Engine.OnEvent. Which players it should be sent to is answered by
// Engine.AudienceOf.
type Event struct {
	Type     EventType         `json:"type"`
	SourceID string            `json:"source_id,omitempty"` // player the event came from
	TargetID string            `json:"target_id,omitempty"` // player the event was aimed at
	Data     map[string]string `json:"data,omitempty"`      // extra payload

	// Canceled / Reason record whether the rules vetoed the action, and why.
	//
	// "The witch clicked poison but had already used the antidote tonight"
	// has to be expressible: without these two fields a vetoed action reaches
	// the caller looking exactly like a successful one, and gets broadcast as
	// though it really happened.
	Canceled bool   `json:"canceled,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// EventType is the type of an event or effect.
//
// There are two classes, and the split is decided by **who owns the name**,
// not by a numeric range:
//
//	kernel state primitives   SET_ALIVE / SET_VAR / ... -- state-machine bookkeeping, never sent out
//	everything else           the rules' name for something that happened -- pushed to OnEvent, audience decided by the rules
//
// In the numbered era this was three ranges: 1..99 external, 100..999
// internal, 1000 and up third-party. That convention bit itself: every
// third-party event type landed inside the "internal" range, so extension
// events could not be sent at all (a rules package's own public events were
// invisible to everyone). With names there are no ranges: the kernel
// recognises its own handful and treats everything else as external.
type EventType string

// EventUnspecified is unspecified.
const EventUnspecified EventType = ""

// The kernel's own events: it emits game start and game end; the rest are
// state primitives and are never sent out.
const (
	EventGameStarted EventType = "GAME_STARTED"
	EventGameEnded   EventType = "GAME_ENDED"

	// -- state primitives, never sent out --
	EventDetour       EventType = "DETOUR"        // detour through a phase for someone's sake, pending
	EventPlayerAdded  EventType = "PLAYER_ADDED"  // a player took a seat (for effect-log replay)
	EventPhaseChanged EventType = "PHASE_CHANGED" // a phase transition (for effect-log replay)
	EventSetAlive     EventType = "SET_ALIVE"     // change a player's alive flag
	EventSetVar       EventType = "SET_VAR"       // write custom state, scope carried in the effect
	EventGotoPhase    EventType = "GOTO_PHASE"    // the rules pick the next phase, overriding NextPhase
	EventSetActors    EventType = "SET_ACTORS"    // name the players who may act in a phase
)

// String implements fmt.Stringer.
func (v EventType) String() string {
	if v == EventUnspecified {
		return "UNSPECIFIED"
	}
	return string(v)
}
