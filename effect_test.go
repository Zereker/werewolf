package werewolf

import (
	"testing"

	pb "github.com/Zereker/werewolf/proto"
)

func TestNewEffect(t *testing.T) {
	effect := NewEffect(pb.EventType_EVENT_TYPE_KILL, "wolf", "victim")

	if effect.Type != pb.EventType_EVENT_TYPE_KILL {
		t.Errorf("expected Type=EVENT_TYPE_KILL, got %v", effect.Type)
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
	effect := NewEffect(pb.EventType_EVENT_TYPE_KILL, "wolf", "victim")

	effect.Cancel("protected by guard")

	if !effect.Canceled {
		t.Error("expected Canceled=true")
	}
	if effect.Reason != "protected by guard" {
		t.Errorf("expected Reason='protected by guard', got %s", effect.Reason)
	}
}

func TestEffect_WithData(t *testing.T) {
	effect := NewEffect(pb.EventType_EVENT_TYPE_CHECK, "seer", "target")

	result := effect.WithData("camp", pb.Camp_CAMP_GOOD)

	// Verify method chaining
	if result != effect {
		t.Error("expected WithData to return same effect")
	}

	if effect.Data["camp"] != pb.Camp_CAMP_GOOD {
		t.Errorf("expected camp=GOOD, got %v", effect.Data["camp"])
	}
}

func TestEffect_WithData_Multiple(t *testing.T) {
	effect := NewEffect(pb.EventType_EVENT_TYPE_KILL, "wolf", "victim")

	effect.WithData("key1", "value1").WithData("key2", "value2")

	if effect.Data["key1"] != "value1" {
		t.Errorf("expected key1=value1, got %v", effect.Data["key1"])
	}
	if effect.Data["key2"] != "value2" {
		t.Errorf("expected key2=value2, got %v", effect.Data["key2"])
	}
}

func TestEffect_ToEvent_Kill(t *testing.T) {
	effect := NewEffect(pb.EventType_EVENT_TYPE_KILL, "wolf", "victim")

	event := effect.ToEvent()

	if event.Type != pb.EventType_EVENT_TYPE_KILL {
		t.Errorf("expected EVENT_TYPE_KILL, got %v", event.Type)
	}
	if event.SourceId != "wolf" {
		t.Errorf("expected SourceId=wolf, got %s", event.SourceId)
	}
	if event.TargetId != "victim" {
		t.Errorf("expected TargetId=victim, got %s", event.TargetId)
	}
}

func TestEffect_ToEvent_Poison(t *testing.T) {
	effect := NewEffect(pb.EventType_EVENT_TYPE_POISON, "witch", "victim")

	event := effect.ToEvent()

	if event.Type != pb.EventType_EVENT_TYPE_POISON {
		t.Errorf("expected EVENT_TYPE_POISON, got %v", event.Type)
	}
}

func TestEffect_ToEvent_Protect(t *testing.T) {
	effect := NewEffect(pb.EventType_EVENT_TYPE_PROTECT, "guard", "target")

	event := effect.ToEvent()

	if event.Type != pb.EventType_EVENT_TYPE_PROTECT {
		t.Errorf("expected EVENT_TYPE_PROTECT, got %v", event.Type)
	}
}

func TestEffect_ToEvent_Save(t *testing.T) {
	effect := NewEffect(pb.EventType_EVENT_TYPE_SAVE, "witch", "victim")

	event := effect.ToEvent()

	if event.Type != pb.EventType_EVENT_TYPE_SAVE {
		t.Errorf("expected EVENT_TYPE_SAVE, got %v", event.Type)
	}
}

func TestEffect_ToEvent_Check(t *testing.T) {
	effect := NewEffect(pb.EventType_EVENT_TYPE_CHECK, "seer", "target")

	event := effect.ToEvent()

	if event.Type != pb.EventType_EVENT_TYPE_CHECK {
		t.Errorf("expected EVENT_TYPE_CHECK, got %v", event.Type)
	}
}

func TestEffect_ToEvent_Eliminate(t *testing.T) {
	effect := NewEffect(pb.EventType_EVENT_TYPE_ELIMINATE, "", "target")

	event := effect.ToEvent()

	if event.Type != pb.EventType_EVENT_TYPE_ELIMINATE {
		t.Errorf("expected EVENT_TYPE_ELIMINATE, got %v", event.Type)
	}
}

func TestEffect_ToEvent_Unspecified(t *testing.T) {
	effect := NewEffect(pb.EventType_EVENT_TYPE_UNSPECIFIED, "", "")

	event := effect.ToEvent()

	if event.Type != pb.EventType_EVENT_TYPE_UNSPECIFIED {
		t.Errorf("expected EVENT_TYPE_UNSPECIFIED, got %v", event.Type)
	}
}

func TestEventType_AllTypes(t *testing.T) {
	types := []pb.EventType{
		pb.EventType_EVENT_TYPE_UNSPECIFIED,
		pb.EventType_EVENT_TYPE_GAME_STARTED,
		pb.EventType_EVENT_TYPE_GAME_ENDED,
		pb.EventType_EVENT_TYPE_KILL,
		pb.EventType_EVENT_TYPE_PROTECT,
		pb.EventType_EVENT_TYPE_SAVE,
		pb.EventType_EVENT_TYPE_POISON,
		pb.EventType_EVENT_TYPE_CHECK,
		pb.EventType_EVENT_TYPE_ELIMINATE,
	}

	// Verify all types are distinct
	seen := make(map[pb.EventType]bool)
	for _, et := range types {
		if seen[et] {
			t.Errorf("duplicate EventType: %d", et)
		}
		seen[et] = true
	}
}

func TestEffect_ToEvent_WithData(t *testing.T) {
	effect := NewEffect(pb.EventType_EVENT_TYPE_CHECK, "seer", "target").
		WithData("camp", pb.Camp_CAMP_GOOD).
		WithData("isGood", true).
		WithData("votes", 5)

	event := effect.ToEvent()

	// 验证 Data 被正确转换
	if event.Data == nil {
		t.Fatal("expected Data to be initialized")
	}

	// Camp 应该被转换为字符串 (使用 Stringer 接口)
	if event.Data["camp"] != "CAMP_GOOD" {
		t.Errorf("expected camp=CAMP_GOOD, got %s", event.Data["camp"])
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
	effect := NewEffect(pb.EventType_EVENT_TYPE_ELIMINATE, "", "target").
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
