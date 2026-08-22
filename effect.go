package werewolf

import (
	"encoding/json"
	"fmt"
)

// Effect 效果 - 状态变更的描述
type Effect struct {
	Type     EventType
	SourceID string                 // 效果来源（玩家ID）
	TargetID string                 // 效果目标（玩家ID）
	Data     map[string]interface{} // 附加数据
	Canceled bool                   // 是否被取消（如被保护）
	Reason   string                 // 取消原因
}

// EventType 的编号分三段：
//
//	  1 ..  99   引擎的外部可见事件，通过 OnEvent 推给调用方
//	100 .. 999   引擎的内部状态变更，不外发
//	1000 起      第三方扩展自己的事件类型
//
// 内部段此前写成「>= 100」而不是一个区间，与「自定义取值从 1000 起」
// 这条约定直接打架：第三方定义的每一个事件类型都会被判成引擎内部事件，
// 于是白痴翻牌、狼王自爆这类本该全场可见的事情，扩展根本发不出去。
const (
	internalEventStart = 100
	customEventStart   = 1000
)

// isInternalEvent 判断事件是否为引擎内部状态变更。
//
// 只有中间那一段是内部的。1000 以上是第三方的地盘，引擎不认识它们，
// 但也不替它们决定「不该外发」——那由 AudienceOf 回答成「我不知道」。
func isInternalEvent(t EventType) bool {
	return t >= internalEventStart && t < customEventStart
}

// triggerPhaseKey 触发效果中记录「该去哪个阶段结算」的键
const triggerPhaseKey = "trigger_phase"

// NewAbilityTriggerEffect 声明「某玩家的死亡技能待结算」。
//
// 死亡触发是一整类能力（猎人开枪、狼王自爆、白痴翻牌），
// 引擎不认识其中任何一个具体角色，只认识「谁、去哪个阶段」。
func NewAbilityTriggerEffect(playerID string, phase PhaseType) *Effect {
	return NewEffect(EventAbilityTriggered, playerID, "").
		WithData(triggerPhaseKey, phase)
}

// triggerPhase 从触发效果中读出目标阶段
func (e *Effect) triggerPhase() (PhaseType, bool) {
	v, ok := e.Data[triggerPhaseKey]
	if !ok {
		return PhaseUnspecified, false
	}
	phase, ok := v.(PhaseType)
	return phase, ok
}

// 写玩家自定义状态时用到的两个键
const (
	playerVarKeyKey   = "var_key"
	playerVarValueKey = "var_value"

	roundVarKeyKey   = "round_var_key"
	roundVarValueKey = "round_var_value"
)

// NewSetPlayerVarEffect 声明「把某个玩家的某项自定义状态改成某值」。
//
// 这是第三方角色存放自身状态的正路。白痴的「翻过牌了」、骑士的
// 「决斗用掉了」这类东西，与女巫的药、守卫的守护记录是同一件事，
// 只是引擎为内置角色写死了字段、为第三方留了这个通用口子。
//
// 走这条路的好处是自动获得整套设施：状态随快照走、效果流能回放、
// Resolver 因此可以保持无状态——而无状态正是 Resolver 接口要求的。
// 值传空串即删除该项。
func NewSetPlayerVarEffect(playerID, key, value string) *Effect {
	return NewEffect(EventSetPlayerVar, "", playerID).
		WithData(playerVarKeyKey, key).
		WithData(playerVarValueKey, value)
}

// playerVarOf 从效果里读出要写的键值。
func playerVarOf(e *Effect) (key, value string) {
	key, _ = e.Data[playerVarKeyKey].(string)
	value, _ = e.Data[playerVarValueKey].(string)
	return key, value
}

// NewSetRoundVarEffect 声明「把本回合的某项自定义状态改成某值」。
//
// 与 NewSetPlayerVarEffect 的分工：那个跟着玩家走一整局（白痴翻过牌了），
// 这个每回合自动清零（今晚谁被标记了）。内置的刀口、被守、被救、被毒
// 都属于后者，只是它们在 RoundContext 上有专门的字段。
// 值传空串即删除该项。
func NewSetRoundVarEffect(key, value string) *Effect {
	return NewEffect(EventSetRoundVar, "", "").
		WithData(roundVarKeyKey, key).
		WithData(roundVarValueKey, value)
}

// roundVarOf 从效果里读出要写的键值。
func roundVarOf(e *Effect) (key, value string) {
	key, _ = e.Data[roundVarKeyKey].(string)
	value, _ = e.Data[roundVarValueKey].(string)
	return key, value
}

// NewEffect 创建效果
func NewEffect(eventType EventType, sourceID, targetID string) *Effect {
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

// WithData 添加附加数据。
//
// Data 为 nil 时就地建好：Effect 是导出类型、字段全导出，
// 第三方 Resolver 用字面量构造它是被文档鼓励的写法，
// 不该在这里撞上一个「assignment to entry in nil map」。
func (e *Effect) WithData(key string, value interface{}) *Effect {
	if e.Data == nil {
		e.Data = make(map[string]interface{}, 1)
	}
	e.Data[key] = value
	return e
}

// ToEvent 转换为事件（用于通知外部）。
//
// Data 从 map[string]interface{} 折成 map[string]string；
// Canceled / Reason 原样带上——被规则否决的行动如果在这里丢掉标记，
// 到了调用方手里就与真的发生过的一模一样。
func (e *Effect) ToEvent() *Event {
	event := &Event{
		Type:     e.Type,
		SourceID: e.SourceID,
		TargetID: e.TargetID,
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
