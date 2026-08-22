package engine

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

	// 验证 Data 被正确转换
	if event.Data == nil {
		t.Fatal("expected Data to be initialized")
	}

	// Camp 应该被转换为字符串 (使用 Stringer 接口)
	if event.Data["camp"] != "GOOD" {
		t.Errorf("expected camp=GOOD, got %s", event.Data["camp"])
	}

	// bool 应该转换为 "true"
	if event.Data["isGood"] != "true" {
		t.Errorf("expected isGood=true, got %s", event.Data["isGood"])
	}

	// int 应该转换为 "5"
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

	// 字符串应该保持不变
	if event.Data["result"] != "tied" {
		t.Errorf("expected result=tied, got %s", event.Data["result"])
	}

	// 切片应该被 JSON 序列化
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

// TestEventType_KernelPrimitivesAreTheOnlyInternalOnes 只有内核自己的状态原语算内部事件。
//
// 这条判定此前按编号区间做：「>= 100 即内部」。它与另一条约定
// 「第三方取值从 1000 起」直接打架——第三方定义的每一个事件类型都被判成
// 内部事件，于是白痴翻牌、狼王自爆这类本该全场可见的事，扩展根本发不出去。
//
// 枚举改成字符串之后不再有「区间」这回事，判定依据是内核自己那张表：
// 表里的是记账，表外的一律是规则的事件，推给 OnEvent。
func TestEventType_KernelPrimitivesAreTheOnlyInternalOnes(t *testing.T) {
	cases := []struct {
		typ      EventType
		internal bool
		why      string
	}{
		{eventKill, false, "规则给「发生了什么」起的名字"},
		{eventVoteTied, false, "规则给「发生了什么」起的名字"},
		{EventSetVar, true, "内核的状态原语"},
		{EventSetAlive, true, "内核的状态原语"},
		{EventPhaseChanged, true, "内核的记账"},
		{EventType("IDIOT_REVEALED"), false, "第三方的事件"},
		{EventType("SET_ALIVE_BUT_NOT_REALLY"), false, "名字像也不算，判的是表不是前缀"},
	}
	for _, c := range cases {
		if got := isInternalEvent(c.typ); got != c.internal {
			t.Errorf("isInternalEvent(%v) = %v，期望 %v（%s）", c.typ, got, c.internal, c.why)
		}
	}
}

// TestAudienceOf_CustomEventIsUnknownNotHidden 自定义事件的答案是「不知道」，不是「不给任何人看」。
//
// 这两件事必须分得开：前者要求调用方自己路由，后者是引擎的明确判定。
// 自定义事件此前落进了后者——因为编号 >= 100 就被当成内部事件了。
func TestAudienceOf_CustomEventIsUnknownNotHidden(t *testing.T) {
	e := newViewGame(t)

	custom := NewEffect(EventType("CUSTOM_EVENT"), "s", "v1")
	audience, known := e.AudienceOf(custom.ToEvent())
	if known {
		t.Error("引擎不该声称认得第三方的事件类型")
	}
	if len(audience) != 0 {
		t.Errorf("不认得的类型不该给出受众，实际 %v", audience)
	}

	// 对照：引擎自己的内部事件是「明确不给任何人看」
	internal := NewSetAliveEffect("v1", false)
	if _, known := e.AudienceOf(internal.ToEvent()); !known {
		t.Error("引擎应当明确判定自己的内部事件不外发")
	}
}

// TestCustomEventReachesOnEvent 第三方的事件要能真的推给订阅者。
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
	t.Errorf("第三方的事件应当推给 OnEvent 的订阅者，实际只收到 %v", seen)
}

// customEventResolver 产出一个第三方自定义类型的事件。
type customEventResolver struct{ typ EventType }

func (r customEventResolver) Resolve([]*SkillUse, GameView) []*Effect {
	return []*Effect{NewEffect(r.typ, "v1", "")}
}

// TestEventKind_StateWritesActuallyWriteState 归到 kindStateWrite 的原语必须真的改得动状态。
//
// 这条此前只是 kernelPrimitives 上的一句注释——「它们是状态机的记账」。
// 那句话对 GOTO_PHASE 是假的：它在 applyEffect 里根本没有分支，一个状态
// 都不改。分类只是注释，错了没有任何东西会响，于是错了很久。
//
// 现在类别是一个值，这条性质就能断言：每一条 kindStateWrite 都拿一份
// 干净状态试一遍，改不动就说明分错了类（或者写入点漏了分支）。
func TestEventKind_StateWritesActuallyWriteState(t *testing.T) {
	// 每一类原语的一个代表性样本，外加它的验证方式。
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
		EventAbilityTriggered: {
			func() *Effect { return NewAbilityTriggerEffect("p1", phaseDay) },
			func(s *gameState) bool { return s.hasPendingTrigger() },
		},
	}

	for typ, kind := range kernelEvents {
		if kind != kindStateWrite {
			continue
		}
		probe, ok := probes[typ]
		if !ok {
			t.Errorf("%v 归在 kindStateWrite，但这个测试没有它的样本——"+
				"补一条，否则「改状态的真的改得动」这句话对它是没验过的", typ)
			continue
		}
		t.Run(string(typ), func(t *testing.T) {
			state := newState()
			mustAddTo(t, state, "p1", roleVillager)
			state.applyEffect(probe.effect())
			if !probe.changed(state) {
				t.Errorf("%v 归在 kindStateWrite 却什么都没改——"+
					"要么分错了类，要么 applyEffect 漏了它的分支", typ)
			}
		})
	}
}

// TestEventKind_ControlAndReplayWriteNothing 控制指令与回放记账一个字节都不该动。
//
// 与上一条互为反面：GOTO_PHASE 的正确性恰恰在于它**不**改状态
// （下一步去哪由 calculateNextPhase 读效果流决定），PLAYER_ADDED 与
// PHASE_CHANGED 则只在 replayEffect 那条路上有意义。
// 哪天有人给它们在 applyEffect 里加了分支，这条会先响。
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
			t.Errorf("%v 不是 kindStateWrite，但这个测试没有它的样本——补一条", typ)
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
				t.Errorf("%v 归在 %v 却改动了状态——applyEffect 里不该有它的分支", typ, kind)
			}
		})
	}
}

// sameState 两份状态在写入点看得见的那些字段上是否一致。
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
		len(a.RoundCtx.PendingTriggers) == len(b.RoundCtx.PendingTriggers)
}
