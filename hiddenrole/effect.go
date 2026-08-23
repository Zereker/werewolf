package hiddenrole

import (
	"encoding/json"
	"fmt"
)

// Effect describes one state change.
type Effect struct {
	Type     EventType
	SourceID string                 // where it came from (player ID)
	TargetID string                 // what it is aimed at (player ID)
	Data     map[string]interface{} // extra payload
	Canceled bool                   // vetoed, e.g. by a protection
	Reason   string                 // why it was vetoed
}

// eventKind classifies a kernel event.
//
// "How many classes of kernel event are there" used to be a single sentence
// of comment on the kernelPrimitives table -- "they are the state machine's
// bookkeeping (whose alive bit flipped, who gained a marker)". That sentence
// was **false** for GOTO_PHASE: it has no branch in applyEffect at all and
// changes no state whatsoever. The behaviour was always right (never sent
// out), the classification was wrong, and a classification that lives only in
// a comment makes no noise when it is.
//
// The class is now a value, so all three properties can be asserted (see
// effect_test.go): a state write must actually be able to change state, and a
// control directive or replay bookkeeping entry must not move a single byte.
type eventKind uint8

const (
	// kindRuleEvent is the rules' name for something that happened (KILL,
	// SHOOT, a duel). The kernel does not recognise it, pushes it to
	// OnEvent, and lets the rules decide its audience. This is the zero
	// value: anything absent from the table below falls into this class.
	kindRuleEvent eventKind = iota

	// kindStateWrite is a state-writing primitive, with its own branch in
	// applyEffect.
	kindStateWrite

	// kindControl is a control directive. It changes no state, and only
	// affects where the kernel goes next.
	kindControl

	// kindReplay is bookkeeping for effect-log replay, written into the log
	// by the kernel itself.
	kindReplay
)

// kernelEvents lists the events the kernel recognises, and what class each
// one is.
//
// Anything absent from the table is a rule event -- the table decides, not a
// numeric range. It used to read "anything >= 100 is internal", which
// collided head-on with the convention that third-party values start at 1000:
// every event type an extension defined was judged internal, so an
// extension's events could not be sent at all.
var kernelEvents = map[EventType]eventKind{
	EventSetAlive:  kindStateWrite,
	EventSetVar:    kindStateWrite,
	EventSetActors: kindStateWrite,
	EventDetour:    kindStateWrite,

	EventGotoPhase: kindControl,

	EventPlayerAdded:  kindReplay,
	EventPhaseChanged: kindReplay,
}

// isInternalEvent reports whether an event is one of the kernel's own
// primitives.
//
// None of the three kernel classes has any business in front of a player --
// AudienceOf answers "definitely shown to nobody" for them, and that part is
// not configurable. Rule events are the opposite: the rules decide their
// audience.
func isInternalEvent(t EventType) bool {
	return kernelEvents[t] != kindRuleEvent
}

// detourPhaseKey is the key under which a detour effect records which phase
// to visit.
const detourPhaseKey = "detour_phase"

// NewDetourEffect declares "for the sake of this player, take a trip through
// that phase" (see Detour).
//
// Werewolf uses it for "the hunter shoots after being killed", but what the
// kernel recognises is neither death nor a skill -- only "who, and to which
// phase". What triggered it and what they do once there is entirely the
// rules' business. Shooting on elimination, self-detonating, flipping a card,
// any "hold on, someone still has to act" goes through here.
//
// The division of labour with NewGotoPhaseEffect: that one is a **one-off
// rewrite of the next stop**, this one **files a debt** -- victory checks and
// the round boundary all wait until the queue drains.
func NewDetourEffect(playerID string, phase PhaseType) *Effect {
	return NewEffect(EventDetour, playerID, "").
		WithData(detourPhaseKey, phase)
}

// winnerKey is the key under which a GAME_ENDED effect records the winner.
//
// Two places use it: it is written on production (endPhaseInternal) and read
// back on replay (replayEffect). It is a constant rather than a literal in
// both places -- those two have to be the same key, and a literal tells
// nobody that.
const winnerKey = "winner"

// gotoPhaseKey is the key under which a next-phase override records its
// destination.
const gotoPhaseKey = "goto_phase"

// NewGotoPhaseEffect declares "once this phase resolves, go to that phase".
//
// It overrides the default exit in PhaseConfig.NextPhase. Phase progression
// used to be a purely static graph whose only dynamic jump was the detour
// queue -- so every conditional branch had to go through that back door,
// whose meaning is "someone's skill is pending", not "where to go next".
//
// The missions package's "go to the mission if the vote passes, back to
// nomination otherwise" is the plainest form of such a branch: the outcome is
// computed by this phase's resolution, and a static graph cannot express it.
//
// Priority: a pending detour queue > this effect > PhaseConfig.NextPhase.
// Detours come first because the queue has to drain -- victory checks and the
// round boundary are waiting on it, and jumping away mid-queue would drop a
// debt that has not been settled.
//
// When the destination is not in the configuration the kernel logs an error
// and falls back to NextPhase: one malformed effect should not bring down a
// whole game, but neither may it quietly jump somewhere nobody expected.
func NewGotoPhaseEffect(phase PhaseType) *Effect {
	return NewEffect(EventGotoPhase, "", "").WithData(gotoPhaseKey, phase)
}

// gotoPhase reads the destination out of an override effect.
func (e *Effect) gotoPhase() (PhaseType, bool) {
	v, ok := e.Data[gotoPhaseKey]
	if !ok {
		return PhaseUnspecified, false
	}
	p, ok := v.(PhaseType)
	if !ok {
		return PhaseUnspecified, false
	}
	return p, true
}

// detourPhase reads the destination out of a detour effect.
func (e *Effect) detourPhase() (PhaseType, bool) {
	v, ok := e.Data[detourPhaseKey]
	if !ok {
		return PhaseUnspecified, false
	}
	phase, ok := v.(PhaseType)
	return phase, ok
}

// The three keys used to write custom state: scope, key, value.
//
// Each scope used to have its own set of key names (var_key / round_var_key /
// player_round_var_key and so on) -- six constants describing one thing.
const (
	varScopeKey = "var_scope"
	varKeyKey   = "var_key"
	varValueKey = "var_value"

	aliveKey = "alive"
)

// NewSetAliveEffect declares "set this player's alive flag to this value".
//
// This is the engine's only life-and-death primitive. A wolf kill, a
// poisoning, an exile and a gunshot each used to be an event type that
// changed the alive flag, which wrote a werewolf rule -- "here are the ways
// to die" -- into the engine; a different ruleset (death by duel, dying of a
// broken heart) meant one more event type and one more branch.
//
// The ways to die are now named by the rules: emit an event of your own (KILL
// / SHOOT / heartbreak) as the account of what happened, and emit a SET_ALIVE
// to actually change the state. Two effects, two things -- the first for the
// audience and the effect log, the second for the state machine.
func NewSetAliveEffect(playerID string, alive bool) *Effect {
	return NewEffect(EventSetAlive, "", playerID).
		WithData(aliveKey, alive)
}

// SetsAlive reports whether this effect changes the alive flag, and to what.
//
// An extension that wants to intercept a death needs it: the idiot surviving
// an exile by flipping their card works by vetoing the lethal primitive.
// Intercepting the primitive rather than the word "exile" makes it
// **independent of the cause** -- one piece of code stops a wolf kill, a
// poisoning, a gunshot and any third-party ruleset's way of dying, because
// all of them end up here.
func (e *Effect) SetsAlive() (alive, ok bool) {
	if e == nil || e.Type != EventSetAlive {
		return false, false
	}
	return aliveOf(e)
}

// aliveOf reads the alive flag an effect intends to write.
func aliveOf(e *Effect) (alive, ok bool) {
	alive, ok = e.Data[aliveKey].(bool)
	return alive, ok
}

// NewSetVarEffect declares "set this piece of custom state to this value", in
// the scope given by scope.
//
// The four scopes used to be four constructors, so nothing forced the 2x2
// table to be complete -- the "whole game, unowned" cell was missing for a
// long time and nobody noticed. The scope is now a parameter:
//
//	NewSetVarEffect(ScopeGame, "score", "3")              whole game, unowned
//	NewSetVarEffect(ScopeGame.Of(id), "antidote", "used") whole game, one player
//	NewSetVarEffect(ScopeRound, "kill", target)           this round, unowned
//	NewSetVarEffect(ScopeRound.Of(id), "guarded", "1")    this round, one player
//
// This is the proper way for a role to store its own state. The idiot's
// "card already flipped", the knight's "duel spent", the witch's two potions
// and the guard's protection record are all the same thing and take the same
// route. Taking it is what earns the whole apparatus for free: the state
// travels with the snapshot, the effect log can replay it, and a Resolver can
// therefore stay stateless -- which is what the Resolver interface demands.
//
// Passing an empty value deletes the entry, identically in all four scopes.
func NewSetVarEffect(scope VarScope, key, value string) *Effect {
	return NewEffect(EventSetVar, "", scope.owner).
		WithData(varScopeKey, scope).
		WithData(varKeyKey, key).
		WithData(varValueKey, value)
}

// SetsVar reports whether this effect writes a piece of custom state, and if
// so which cell, key and value.
//
// Same use as SetsAlive: an extension that wants to intercept or observe a
// class of write needs it. With the four scopes folded into one event type,
// Type alone no longer distinguishes whole-game from this-round, or owned
// from unowned -- read them from here.
func (e *Effect) SetsVar() (scope VarScope, key, value string, ok bool) {
	if e == nil || e.Type != EventSetVar {
		return VarScope{}, "", "", false
	}
	scope, key, value = varOf(e)
	return scope, key, value, key != ""
}

// varOf reads the scope, key and value out of an effect.
func varOf(e *Effect) (scope VarScope, key, value string) {
	scope, _ = e.Data[varScopeKey].(VarScope)
	key, _ = e.Data[varKeyKey].(string)
	value, _ = e.Data[varValueKey].(string)
	return scope, key, value
}

// actorsPhaseKey / actorsListKey are the two keys in an actors effect.
const (
	actorsPhaseKey = "actors_phase"
	actorsListKey  = "actors_list"
)

// NewSetActorsEffect declares "these players may act in the given phase".
//
// The kernel's default way of deciding actors is to match PhaseStep.Role
// against a player's role -- and a role is fixed at seating time, so any set
// of actors **chosen at runtime** is inexpressible: the missions package's
// team is voted on in the previous phase, and its leader rotates by seat.
// Without this effect the rules could only let everyone submit and then throw
// away what should not count, while the kernel told unqualified players "you
// may act".
//
// Priority: a pending detour queue > this effect > PhaseStep.Role. Same
// layering as NewGotoPhaseEffect -- a default plus a runtime override.
//
// The list is normally computed in an **earlier phase**, which is why it
// names a phase rather than applying to the current one. A phase's list is
// consumed once that phase resolves: without clearing it, the next visit to
// the same phase would inherit the previous round's list.
//
// Passing an empty list is meaningful: it says "nobody can act in this
// phase", which is different from "the rules did not say".
//
// Players in the list who do not exist are ignored; the list is stored sorted
// by ID, which keeps the effect log deterministic.
func NewSetActorsEffect(phase PhaseType, playerIDs ...string) *Effect {
	return NewEffect(EventSetActors, "", "").
		WithData(actorsPhaseKey, phase).
		WithData(actorsListKey, append([]string(nil), playerIDs...))
}

// actorsOf reads the phase and the list out of an effect.
func actorsOf(e *Effect) (PhaseType, []string, bool) {
	p, ok := e.Data[actorsPhaseKey].(PhaseType)
	if !ok {
		return PhaseUnspecified, nil, false
	}
	ids, ok := e.Data[actorsListKey].([]string)
	if !ok {
		return PhaseUnspecified, nil, false
	}
	return p, ids, true
}

// NewEffect builds an effect.
func NewEffect(eventType EventType, sourceID, targetID string) *Effect {
	return &Effect{
		Type:     eventType,
		SourceID: sourceID,
		TargetID: targetID,
		Data:     make(map[string]interface{}),
	}
}

// Cancel vetoes an effect.
func (e *Effect) Cancel(reason string) {
	e.Canceled = true
	e.Reason = reason
}

// WithData attaches extra payload.
//
// It builds Data in place when nil: Effect is an exported type with all
// fields exported, constructing one as a literal is the documented thing for
// a third-party Resolver to do, and it should not run into an "assignment to
// entry in nil map" here.
func (e *Effect) WithData(key string, value interface{}) *Effect {
	if e.Data == nil {
		e.Data = make(map[string]interface{}, 1)
	}
	e.Data[key] = value
	return e
}

// clone deep-copies one effect, Data included.
//
// The effect log is this engine's history, and "history cannot be rewritten"
// cannot rest on documentation alone: what EndPhase returned and what
// EffectLog returned used to be the very same pointers as the engine's own
// history, so a caller changing one field in passing (or calling Cancel,
// which is exported) rewrote the history, and a replay would rebuild a
// different game from it.
//
// Copies now go into the log and copies come out, so neither side shares an
// object with the caller.
func (e *Effect) clone() *Effect {
	if e == nil {
		return nil
	}
	c := *e
	if e.Data != nil {
		c.Data = make(map[string]interface{}, len(e.Data))
		for k, v := range e.Data {
			c.Data[k] = v
		}
	}
	return &c
}

// ToEvent converts an effect into an outward event.
//
// Data is flattened from map[string]interface{} to map[string]string;
// Canceled and Reason are carried over verbatim -- an action the rules vetoed
// that lost its marker here would reach the caller looking exactly like one
// that really happened.
func (e *Effect) ToEvent() *Event {
	event := &Event{
		Type:     e.Type,
		SourceID: e.SourceID,
		TargetID: e.TargetID,
		Data:     make(map[string]string),
		Canceled: e.Canceled,
		Reason:   e.Reason,
	}

	// Flatten Data: interface{} -> string.
	for k, v := range e.Data {
		event.Data[k] = convertToString(v)
	}

	return event
}

// convertToString renders an interface{} as a string.
func convertToString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", val)
	case float32, float64:
		return fmt.Sprintf("%v", val)
	case fmt.Stringer:
		return val.String()
	default:
		// For a composite type, try JSON.
		if data, err := json.Marshal(val); err == nil {
			return string(data)
		}
		return fmt.Sprintf("%v", val)
	}
}
