package hiddenrole

import (
	"testing"
)

func TestNewEffect(t *testing.T) {
	effect := NewEffect(eventKill, "wolf", "victim")

	if effect.Type != eventKill {
		t.Errorf("expected Type=KILL, got %v", effect.Type)
	}
	if effect.SourceID != "wolf" {
		t.Errorf("expected SourceID=wolf, got %s", effect.SourceID)
	}
	if effect.TargetID != "victim" {
		t.Errorf("expected TargetID=victim, got %s", effect.TargetID)
	}
	if effect.Data == nil {
		t.Error("expected Data to be initialized")
	}
	if effect.Canceled {
		t.Error("expected Canceled=false")
	}
	if effect.Reason != "" {
		t.Errorf("expected empty Reason, got %s", effect.Reason)
	}
}

func TestEffect_Cancel(t *testing.T) {
	effect := NewEffect(eventKill, "wolf", "victim")

	effect.Cancel("protected by guard")

	if !effect.Canceled {
		t.Error("expected Canceled=true")
	}
	if effect.Reason != "protected by guard" {
		t.Errorf("expected Reason='protected by guard', got %s", effect.Reason)
	}
}

func TestEffect_WithData(t *testing.T) {
	effect := NewEffect(eventCheck, "seer", "target")

	result := effect.WithData("camp", campGood)

	// Verify method chaining
	if result != effect {
		t.Error("expected WithData to return same effect")
	}

	if effect.Data["camp"] != campGood {
		t.Errorf("expected camp=GOOD, got %v", effect.Data["camp"])
	}
}

func TestEffect_WithData_Multiple(t *testing.T) {
	effect := NewEffect(eventKill, "wolf", "victim")

	effect.WithData("key1", "value1").WithData("key2", "value2")

	if effect.Data["key1"] != "value1" {
		t.Errorf("expected key1=value1, got %v", effect.Data["key1"])
	}
	if effect.Data["key2"] != "value2" {
		t.Errorf("expected key2=value2, got %v", effect.Data["key2"])
	}
}

func TestEffect_ToEvent_Kill(t *testing.T) {
	effect := NewEffect(eventKill, "wolf", "victim")

	event := effect.ToEvent()

	if event.Type != eventKill {
		t.Errorf("expected KILL, got %v", event.Type)
	}
	if event.SourceID != "wolf" {
		t.Errorf("expected SourceId=wolf, got %s", event.SourceID)
	}
	if event.TargetID != "victim" {
		t.Errorf("expected TargetId=victim, got %s", event.TargetID)
	}
}

func TestEffect_ToEvent_Poison(t *testing.T) {
	effect := NewEffect(eventPoison, "witch", "victim")

	event := effect.ToEvent()

	if event.Type != eventPoison {
		t.Errorf("expected POISON, got %v", event.Type)
	}
}

func TestEffect_ToEvent_Protect(t *testing.T) {
	effect := NewEffect(eventProtect, "guard", "target")

	event := effect.ToEvent()

	if event.Type != eventProtect {
		t.Errorf("expected PROTECT, got %v", event.Type)
	}
}

func TestEffect_ToEvent_Save(t *testing.T) {
	effect := NewEffect(eventSave, "witch", "victim")

	event := effect.ToEvent()

	if event.Type != eventSave {
		t.Errorf("expected SAVE, got %v", event.Type)
	}
}

func TestEffect_ToEvent_Check(t *testing.T) {
	effect := NewEffect(eventCheck, "seer", "target")

	event := effect.ToEvent()

	if event.Type != eventCheck {
		t.Errorf("expected CHECK, got %v", event.Type)
	}
}

func TestEffect_ToEvent_Eliminate(t *testing.T) {
	effect := NewEffect(eventEliminate, "", "target")

	event := effect.ToEvent()

	if event.Type != eventEliminate {
		t.Errorf("expected ELIMINATE, got %v", event.Type)
	}
}

func TestEffect_ToEvent_Unspecified(t *testing.T) {
	effect := NewEffect(EventUnspecified, "", "")

	event := effect.ToEvent()

	if event.Type != EventUnspecified {
		t.Errorf("expected UNSPECIFIED, got %v", event.Type)
	}
}

func TestEventType_AllTypes(t *testing.T) {
	types := []EventType{
		EventUnspecified,
		EventGameStarted,
		EventGameEnded,
		eventKill,
		eventProtect,
		eventSave,
		eventPoison,
		eventCheck,
		eventEliminate,
	}

	// Verify all types are distinct
	seen := make(map[EventType]bool)
	for _, et := range types {
		if seen[et] {
			t.Errorf("duplicate EventType: %v", et)
		}
		seen[et] = true
	}
}

func TestEffect_ToEvent_WithData(t *testing.T) {
	effect := NewEffect(eventCheck, "seer", "target").
		WithData("camp", campGood).
		WithData("isGood", true).
		WithData("votes", 5)

	event := effect.ToEvent()

	// Check that Data was converted correctly.
	if event.Data == nil {
		t.Fatal("expected Data to be initialized")
	}

	// A Camp should be rendered as a string (through the Stringer interface).
	if event.Data["camp"] != "GOOD" {
		t.Errorf("expected camp=GOOD, got %s", event.Data["camp"])
	}

	// A bool should become "true".
	if event.Data["isGood"] != "true" {
		t.Errorf("expected isGood=true, got %s", event.Data["isGood"])
	}

	// An int should become "5".
	if event.Data["votes"] != "5" {
		t.Errorf("expected votes=5, got %s", event.Data["votes"])
	}
}

func TestEffect_ToEvent_WithComplexData(t *testing.T) {
	voters := []string{"p1", "p2", "p3"}
	effect := NewEffect(eventEliminate, "", "target").
		WithData("voters", voters).
		WithData("result", "tied")

	event := effect.ToEvent()

	// A string should pass through unchanged.
	if event.Data["result"] != "tied" {
		t.Errorf("expected result=tied, got %s", event.Data["result"])
	}

	// A slice should be JSON-encoded.
	if event.Data["voters"] != `["p1","p2","p3"]` {
		t.Errorf("expected voters=[\"p1\",\"p2\",\"p3\"], got %s", event.Data["voters"])
	}
}

func TestConvertToString(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"string", "hello", "hello"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"int", 42, "42"},
		{"int64", int64(100), "100"},
		{"float64", 3.14, "3.14"},
		{"slice", []string{"a", "b"}, `["a","b"]`},
		{"map", map[string]int{"x": 1}, `{"x":1}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToString(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestEventType_KernelPrimitivesAreTheOnlyInternalOnes: only the kernel's own
// state primitives count as internal events.
//
// This decision used to be made by numeric range: ">= 100 is internal". That
// collided head-on with the other convention, "third-party values start at
// 1000" -- every event type a third party defined was judged internal, so
// things that should be visible to the whole table (the idiot flipping a
// card, the wolf king self-detonating) could not be emitted by an extension
// at all.
//
// With the enums as strings there are no ranges any more, and what decides is
// the kernel's own table: what is in it is bookkeeping, what is outside it is
// the rules' event and gets pushed to OnEvent.
func TestEventType_KernelPrimitivesAreTheOnlyInternalOnes(t *testing.T) {
	cases := []struct {
		typ      EventType
		internal bool
		why      string
	}{
		{eventKill, false, "the rules' name for something that happened"},
		{eventVoteTied, false, "the rules' name for something that happened"},
		{EventSetVar, true, "a kernel state primitive"},
		{EventSetAlive, true, "a kernel state primitive"},
		{EventPhaseChanged, true, "kernel bookkeeping"},
		{EventType("IDIOT_REVEALED"), false, "a third party's event"},
		{EventType("SET_ALIVE_BUT_NOT_REALLY"), false, "a lookalike name does not count; the table decides, not a prefix"},
	}
	for _, c := range cases {
		if got := isInternalEvent(c.typ); got != c.internal {
			t.Errorf("isInternalEvent(%v) = %v, want %v (%s)", c.typ, got, c.internal, c.why)
		}
	}
}

// TestAudienceOf_CustomEventIsUnknownNotHidden: the answer for a custom event
// is "don't know", not "show it to nobody".
//
// The two must stay distinguishable: the former asks the caller to route it
// themselves, the latter is the engine's definite verdict. A custom event
// used to land in the latter -- because a number >= 100 was taken as
// internal.
func TestAudienceOf_CustomEventIsUnknownNotHidden(t *testing.T) {
	e := newViewGame(t)

	custom := NewEffect(EventType("CUSTOM_EVENT"), "s", "v1")
	audience, known := e.AudienceOf(custom.ToEvent())
	if known {
		t.Error("the engine should not claim to recognise a third party's event type")
	}
	if len(audience) != 0 {
		t.Errorf("an unrecognised type should yield no audience, got %v", audience)
	}

	// Control: the engine's own internal events are a definite "shown to
	// nobody".
	internal := NewSetAliveEffect("v1", false)
	if _, known := e.AudienceOf(internal.ToEvent()); !known {
		t.Error("the engine should definitively rule that its internal events are not sent out")
	}
}

// TestCustomEventReachesOnEvent: a third party's event really does reach
// subscribers.
func TestCustomEventReachesOnEvent(t *testing.T) {
	const customPhase = PhaseType("CUSTOM_PHASE")
	const customEvent = EventType("CUSTOM_EVENT")

	cfg := testConfig()
	cfg.Phases[customPhase] = &PhaseConfig{
		Type:      customPhase,
		NextPhase: phaseDay,
		Steps:     []PhaseStep{{Role: roleVillager, Skill: SkillSkip}},
	}
	cfg.Phases[phaseNightResolve].NextPhase = customPhase

	opts := append(withNoopResolvers(),
		WithResolver(customPhase, customEventResolver{typ: customEvent}))
	e, err := NewEngine(cfg, opts...)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	for id, role := range map[string]RoleType{
		"w1": roleWerewolf, "w2": roleWerewolf, "s": roleSeer,
		"v1": roleVillager, "v2": roleVillager, "v3": roleVillager,
	} {
		mustAdd(t, e, id, role)
	}
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	var seen []EventType
	e.OnEvent(func(ev *Event) { seen = append(seen, ev.Type) })

	for i := 0; e.Status().Phase != customPhase && i < 20; i++ {
		if _, err := e.EndPhase(); err != nil {
			t.Fatalf("EndPhase: %v", err)
		}
	}
	if _, err := e.EndPhase(); err != nil {
		t.Fatalf("EndPhase: %v", err)
	}

	for _, typ := range seen {
		if typ == customEvent {
			return
		}
	}
	t.Errorf("a third party's event should reach OnEvent subscribers, only got %v", seen)
}

// customEventResolver emits an event of a third-party custom type.
type customEventResolver struct{ typ EventType }

func (r customEventResolver) Resolve([]*SkillUse, GameView) []*Effect {
	return []*Effect{NewEffect(r.typ, "v1", "")}
}

// TestEventKind_StateWritesActuallyWriteState: a primitive classified as
// kindStateWrite must really be able to change state.
//
// This used to be one sentence of comment on kernelPrimitives -- "they are
// the state machine's bookkeeping". That sentence was false for GOTO_PHASE:
// it has no branch in applyEffect at all and changes no state. A
// classification that lives only in a comment makes no sound when it is
// wrong, and so it stayed wrong for a long time.
//
// The class is a value now, so the property can be asserted: every
// kindStateWrite is tried against a clean state, and one that cannot change
// anything is misclassified (or the write point is missing its branch).
func TestEventKind_StateWritesActuallyWriteState(t *testing.T) {
	// One representative sample of each primitive, plus how to verify it.
	probes := map[EventType]struct {
		effect  func() *Effect
		changed func(*gameState) bool
	}{
		EventSetAlive: {
			func() *Effect { return NewSetAliveEffect("p1", false) },
			func(s *gameState) bool { p, ok := s.getPlayer("p1"); return ok && !p.Alive },
		},
		EventSetVar: {
			func() *Effect { return NewSetVarEffect(ScopeGame, "probe", "1") },
			func(s *gameState) bool { return s.varOf(ScopeGame, "probe") == "1" },
		},
		EventSetActors: {
			func() *Effect { return NewSetActorsEffect(phaseDay, "p1") },
			func(s *gameState) bool { ids, ok := s.actorsFor(phaseDay); return ok && len(ids) == 1 },
		},
		EventDetour: {
			func() *Effect { return NewDetourEffect("p1", phaseDay) },
			func(s *gameState) bool { return s.hasPendingDetour() },
		},
	}

	for typ, kind := range kernelEvents {
		if kind != kindStateWrite {
			continue
		}
		probe, ok := probes[typ]
		if !ok {
			t.Errorf("%v is classified kindStateWrite but this test has no sample for it -- "+
				"add one, or \"a state write really can write state\" is unverified for it", typ)
			continue
		}
		t.Run(string(typ), func(t *testing.T) {
			state := newState()
			mustAddTo(t, state, "p1", roleVillager)
			state.applyEffect(probe.effect())
			if !probe.changed(state) {
				t.Errorf("%v is classified kindStateWrite yet changed nothing -- "+
					"either it is misclassified, or applyEffect is missing its branch", typ)
			}
		})
	}
}

// TestEventKind_ControlAndReplayWriteNothing: a control directive or a replay
// bookkeeping entry must not move a single byte.
//
// The mirror of the previous test: GOTO_PHASE is correct precisely because it
// does **not** change state (where to go next is decided by
// calculateNextPhase reading the effect log), while PLAYER_ADDED and
// PHASE_CHANGED only mean anything on the replayEffect path. The day somebody
// gives them a branch in applyEffect, this is what goes red first.
func TestEventKind_ControlAndReplayWriteNothing(t *testing.T) {
	probes := map[EventType]*Effect{
		EventGotoPhase:    NewGotoPhaseEffect(phaseDay),
		EventPlayerAdded:  newPlayerAddedEffect("p2", roleVillager, nil),
		EventPhaseChanged: newPhaseChangedEffect(phaseDay),
	}

	for typ, kind := range kernelEvents {
		if kind == kindStateWrite {
			continue
		}
		probe, ok := probes[typ]
		if !ok {
			t.Errorf("%v is not kindStateWrite but this test has no sample for it -- add one", typ)
			continue
		}
		t.Run(string(typ), func(t *testing.T) {
			before := newState()
			mustAddTo(t, before, "p1", roleVillager)
			before.startAt(phaseNight)

			after := newState()
			mustAddTo(t, after, "p1", roleVillager)
			after.startAt(phaseNight)
			after.applyEffect(probe)

			if !sameState(before, after) {
				t.Errorf("%v is classified %v yet changed state -- applyEffect should have no branch for it", typ, kind)
			}
		})
	}
}

// sameState reports whether two states agree on the fields the write point
// can see.
func sameState(a, b *gameState) bool {
	if a.Phase != b.Phase || a.Round != b.Round || len(a.players) != len(b.players) {
		return false
	}
	if len(a.Vars) != len(b.Vars) || len(a.Actors) != len(b.Actors) {
		return false
	}
	for k, v := range a.Vars {
		if b.Vars[k] != v {
			return false
		}
	}
	for _, pa := range a.players {
		pb, ok := b.players[pa.ID]
		if !ok || pa.Alive != pb.Alive || pa.Role != pb.Role {
			return false
		}
		if len(pa.Vars) != len(pb.Vars) || len(pa.RoundVars) != len(pb.RoundVars) {
			return false
		}
	}
	if a.RoundCtx == nil || b.RoundCtx == nil {
		return a.RoundCtx == b.RoundCtx
	}
	return len(a.RoundCtx.Vars) == len(b.RoundCtx.Vars) &&
		len(a.RoundCtx.Detours) == len(b.RoundCtx.Detours)
}
