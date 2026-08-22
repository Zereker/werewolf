package werewolf

import (
	"testing"
)

func TestNewEffect(t *testing.T) {
	effect := NewEffect(EventKill, "wolf", "victim")

	if effect.Type != EventKill {
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
	effect := NewEffect(EventKill, "wolf", "victim")

	effect.Cancel("protected by guard")

	if !effect.Canceled {
		t.Error("expected Canceled=true")
	}
	if effect.Reason != "protected by guard" {
		t.Errorf("expected Reason='protected by guard', got %s", effect.Reason)
	}
}

func TestEffect_WithData(t *testing.T) {
	effect := NewEffect(EventCheck, "seer", "target")

	result := effect.WithData("camp", CampGood)

	// Verify method chaining
	if result != effect {
		t.Error("expected WithData to return same effect")
	}

	if effect.Data["camp"] != CampGood {
		t.Errorf("expected camp=GOOD, got %v", effect.Data["camp"])
	}
}

func TestEffect_WithData_Multiple(t *testing.T) {
	effect := NewEffect(EventKill, "wolf", "victim")

	effect.WithData("key1", "value1").WithData("key2", "value2")

	if effect.Data["key1"] != "value1" {
		t.Errorf("expected key1=value1, got %v", effect.Data["key1"])
	}
	if effect.Data["key2"] != "value2" {
		t.Errorf("expected key2=value2, got %v", effect.Data["key2"])
	}
}

func TestEffect_ToEvent_Kill(t *testing.T) {
	effect := NewEffect(EventKill, "wolf", "victim")

	event := effect.ToEvent()

	if event.Type != EventKill {
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
	effect := NewEffect(EventPoison, "witch", "victim")

	event := effect.ToEvent()

	if event.Type != EventPoison {
		t.Errorf("expected POISON, got %v", event.Type)
	}
}

func TestEffect_ToEvent_Protect(t *testing.T) {
	effect := NewEffect(EventProtect, "guard", "target")

	event := effect.ToEvent()

	if event.Type != EventProtect {
		t.Errorf("expected PROTECT, got %v", event.Type)
	}
}

func TestEffect_ToEvent_Save(t *testing.T) {
	effect := NewEffect(EventSave, "witch", "victim")

	event := effect.ToEvent()

	if event.Type != EventSave {
		t.Errorf("expected SAVE, got %v", event.Type)
	}
}

func TestEffect_ToEvent_Check(t *testing.T) {
	effect := NewEffect(EventCheck, "seer", "target")

	event := effect.ToEvent()

	if event.Type != EventCheck {
		t.Errorf("expected CHECK, got %v", event.Type)
	}
}

func TestEffect_ToEvent_Eliminate(t *testing.T) {
	effect := NewEffect(EventEliminate, "", "target")

	event := effect.ToEvent()

	if event.Type != EventEliminate {
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
		EventKill,
		EventProtect,
		EventSave,
		EventPoison,
		EventCheck,
		EventEliminate,
	}

	// Verify all types are distinct
	seen := make(map[EventType]bool)
	for _, et := range types {
		if seen[et] {
			t.Errorf("duplicate EventType: %d", et)
		}
		seen[et] = true
	}
}

func TestEffect_ToEvent_WithData(t *testing.T) {
	effect := NewEffect(EventCheck, "seer", "target").
		WithData("camp", CampGood).
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
	effect := NewEffect(EventEliminate, "", "target").
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

// TestEventType_CustomRangeIsExternal 第三方自定义的事件类型不是引擎内部事件。
//
// 编号分三段：1..99 引擎外部事件、100..999 引擎内部状态变更、1000 起第三方。
// 内部段此前写成「>= 100」，与「自定义取值从 1000 起」这条约定直接打架——
// 第三方定义的每一个事件类型都会被判成引擎内部事件，于是白痴翻牌、
// 狼王自爆这类本该全场可见的事情，扩展根本发不出去。
func TestEventType_CustomRangeIsExternal(t *testing.T) {
	cases := []struct {
		typ      EventType
		internal bool
		why      string
	}{
		{EventKill, false, "引擎的外部事件"},
		{EventVoteTied, false, "引擎的外部事件"},
		{EventSetNightKill, true, "引擎的内部状态变更"},
		{EventPhaseChanged, true, "引擎的内部状态变更"},
		{EventType(999), true, "内部段的上界之内"},
		{EventType(1000), false, "第三方的地盘"},
		{EventType(1001), false, "第三方的地盘"},
	}
	for _, c := range cases {
		if got := isInternalEvent(c.typ); got != c.internal {
			t.Errorf("isInternalEvent(%d) = %v，期望 %v（%s）", c.typ, got, c.internal, c.why)
		}
	}
}

// TestAudienceOf_CustomEventIsUnknownNotHidden 自定义事件的答案是「不知道」，不是「不给任何人看」。
//
// 这两件事必须分得开：前者要求调用方自己路由，后者是引擎的明确判定。
// 自定义事件此前落进了后者——因为编号 >= 100 就被当成内部事件了。
func TestAudienceOf_CustomEventIsUnknownNotHidden(t *testing.T) {
	e := newViewGame(t)

	custom := NewEffect(EventType(1001), "s", "v1")
	audience, known := e.AudienceOf(custom.ToEvent())
	if known {
		t.Error("引擎不该声称认得第三方的事件类型")
	}
	if len(audience) != 0 {
		t.Errorf("不认得的类型不该给出受众，实际 %v", audience)
	}

	// 对照：引擎自己的内部事件是「明确不给任何人看」
	internal := NewEffect(EventSetNightKill, "", "v1")
	if _, known := e.AudienceOf(internal.ToEvent()); !known {
		t.Error("引擎应当明确判定自己的内部事件不外发")
	}
}

// TestCustomEventReachesOnEvent 第三方的事件要能真的推给订阅者。
func TestCustomEventReachesOnEvent(t *testing.T) {
	const customPhase = PhaseType(1000)
	const customEvent = EventType(1001)

	cfg := DefaultGameConfig()
	cfg.Phases[customPhase] = &PhaseConfig{
		Type:      customPhase,
		NextPhase: PhaseDay,
		Steps:     []PhaseStep{{Role: RoleVillager, Skill: SkillSkip}},
	}
	cfg.Phases[PhaseNightResolve].NextPhase = customPhase

	g := newRuleGameWith(t, cfg,
		[]EngineOption{WithResolver(customPhase, customEventResolver{typ: customEvent})},
		seats(wolf("w1"), wolf("w2"), seer("s"), villagers("v1", "v2", "v3"))...)

	var seen []EventType
	g.e.OnEvent(func(ev *Event) { seen = append(seen, ev.Type) })

	for g.e.Phase() != customPhase {
		g.endAny()
	}
	g.endAny()

	for _, typ := range seen {
		if typ == customEvent {
			return
		}
	}
	t.Errorf("第三方的事件应当推给 OnEvent 的订阅者，实际只收到 %v", seen)
}

// customEventResolver 产出一个第三方自定义类型的事件。
type customEventResolver struct{ typ EventType }

func (r customEventResolver) Resolve([]*SkillUse, GameView, *GameConfig) []*Effect {
	return []*Effect{NewEffect(r.typ, "v1", "")}
}
