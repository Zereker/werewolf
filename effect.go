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

// ToEvent 转换为事件（用于通知外部）
// 将 Effect.Data (map[string]interface{}) 转换为 Event.Data (map[string]string)
func (e *Effect) ToEvent() *pb.Event {
	event := &pb.Event{
		Type:     e.Type,
		SourceId: e.SourceID,
		TargetId: e.TargetID,
		Data:     make(map[string]string),
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
