package werewolf

import (
	"encoding/json"
	"fmt"

	pb "github.com/Zereker/werewolf/proto"
)

// Effect 效果 - 状态变更的描述
type Effect struct {
	Type     pb.EventType
	SourceID string                 // 效果来源（玩家ID）
	TargetID string                 // 效果目标（玩家ID）
	Data     map[string]interface{} // 附加数据
	Canceled bool                   // 是否被取消（如被保护）
	Reason   string                 // 取消原因
}

// internalEventThreshold 内部事件类型的起始编号。
//
// proto 中 EventType 按此分段：小于该值的是外部可见事件，会通过 OnEvent
// 推给调用方；大于等于该值的是引擎内部的状态变更，不外发。
const internalEventThreshold = 100

// isInternalEvent 判断事件是否为引擎内部状态变更。
func isInternalEvent(t pb.EventType) bool {
	return t >= internalEventThreshold
}

// triggerPhaseKey 触发效果中记录「该去哪个阶段结算」的键
const triggerPhaseKey = "trigger_phase"

// NewAbilityTriggerEffect 声明「某玩家的死亡技能待结算」。
//
// 死亡触发是一整类能力（猎人开枪、狼王自爆、白痴翻牌），
// 引擎不认识其中任何一个具体角色，只认识「谁、去哪个阶段」。
func NewAbilityTriggerEffect(playerID string, phase pb.PhaseType) *Effect {
	return NewEffect(pb.EventType_EVENT_TYPE_ABILITY_TRIGGERED, playerID, "").
		WithData(triggerPhaseKey, phase)
}

// triggerPhase 从触发效果中读出目标阶段
func (e *Effect) triggerPhase() (pb.PhaseType, bool) {
	v, ok := e.Data[triggerPhaseKey]
	if !ok {
		return pb.PhaseType_PHASE_TYPE_UNSPECIFIED, false
	}
	phase, ok := v.(pb.PhaseType)
	return phase, ok
}

// NewEffect 创建效果
func NewEffect(eventType pb.EventType, sourceID, targetID string) *Effect {
	return &Effect{
		Type:     eventType,
		SourceID: sourceID,
		TargetID: targetID,
		Data:     make(map[string]interface{}),
	}
}

// Cancel 取消效果
func (e *Effect) Cancel(reason string) {
	e.Canceled = true
	e.Reason = reason
}

// WithData 添加附加数据
func (e *Effect) WithData(key string, value interface{}) *Effect {
	e.Data[key] = value
	return e
}

// ToEvent 转换为事件（用于通知外部）。
//
// Data 从 map[string]interface{} 折成 map[string]string；
// Canceled / Reason 原样带上——被规则否决的行动如果在这里丢掉标记，
// 到了调用方手里就与真的发生过的一模一样。
func (e *Effect) ToEvent() *pb.Event {
	event := &pb.Event{
		Type:     e.Type,
		SourceId: e.SourceID,
		TargetId: e.TargetID,
		Data:     make(map[string]string),
		Canceled: e.Canceled,
		Reason:   e.Reason,
	}

	// 转换 Data: interface{} -> string
	for k, v := range e.Data {
		event.Data[k] = convertToString(v)
	}

	return event
}

// convertToString 将 interface{} 转换为 string
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
		// 对于复杂类型，尝试 JSON 序列化
		if data, err := json.Marshal(val); err == nil {
			return string(data)
		}
		return fmt.Sprintf("%v", val)
	}
}
