package engine

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

// kernelPrimitives 内核自己的状态原语。
//
// 它们是状态机的记账（谁的存活位翻了、谁身上多了个标记），不该出现在
// 任何玩家面前——AudienceOf 对它们的回答是「明确不给任何人看」，
// 且这一条不可配置。
//
// 判断依据是这张表，不是编号区间。此前写成「>= 100 即内部」，与
// 「第三方取值从 1000 起」那条约定直接打架：扩展定义的每一个事件类型
// 都被判成内部事件，于是扩展的事件根本发不出去。
var kernelPrimitives = map[EventType]bool{
	EventSetAlive:          true,
	EventSetPlayerVar:      true,
	EventSetRoundVar:       true,
	EventSetPlayerRoundVar: true,
	EventAbilityTriggered:  true,
	EventPlayerAdded:       true,
	EventPhaseChanged:      true,
}

// isInternalEvent 判断事件是否为内核的状态原语。
func isInternalEvent(t EventType) bool {
	return kernelPrimitives[t]
}

// triggerPhaseKey 触发效果中记录「该去哪个阶段结算」的键
const triggerPhaseKey = "trigger_phase"

// NewAbilityTriggerEffect 声明「某玩家的死亡技能待结算」。
//
// 死亡触发是一整类能力（出局时开枪、自爆、翻牌），
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

	playerRoundVarKeyKey   = "player_round_var_key"
	playerRoundVarValueKey = "player_round_var_value"

	aliveKey = "alive"
)

// NewSetAliveEffect 声明「把某个玩家的存活状态改成某值」。
//
// 这是引擎唯一的生死原语。狼刀、毒杀、放逐、开枪此前各自是一个会改
// 存活状态的事件类型，于是「有哪些死法」这件狼人杀的规则被写进了引擎；
// 换一套规则（决斗致死、殉情）就得再加一个事件类型、再加一条分支。
//
// 现在死法由规则自己命名：产出一个自己的事件（KILL / SHOOT / 殉情）
// 作为「发生了什么」的说法，再产出一个 SET_ALIVE 真正改状态。
// 两个效果，两件事——前者给受众与效果流看，后者给状态机看。
func NewSetAliveEffect(playerID string, alive bool) *Effect {
	return NewEffect(EventSetAlive, "", playerID).
		WithData(aliveKey, alive)
}

// SetsAlive 这个效果是否在改存活状态，以及改成什么。
//
// 想拦下一次死亡的扩展需要它：白痴被投票放逐时翻牌不出局，靠的是把
// 那条致死的原语否决掉。拦原语而不是拦「放逐」这个说法，好处是**与死因
// 无关**——同一段代码能挡住狼刀、毒杀、枪口和任何第三方规则的死法，
// 因为它们最终都要走这一条。
func (e *Effect) SetsAlive() (alive, ok bool) {
	if e == nil || e.Type != EventSetAlive {
		return false, false
	}
	return aliveOf(e)
}

// aliveOf 从效果里读出要写的存活状态。
func aliveOf(e *Effect) (alive, ok bool) {
	alive, ok = e.Data[aliveKey].(bool)
	return alive, ok
}

// NewSetPlayerRoundVarEffect 声明「把某个玩家本回合的某项状态改成某值」。
//
// 三种作用域的第三种：PlayerVar 跟着玩家走一整局，RoundVar 每回合清零
// 且不属于任何人，这个则是「某个玩家在本回合的标记」——今晚谁被守了、
// 谁被救了、谁被毒了都是这一类，它们此前是 RoundContext 上三张
// map[string]bool，第三方角色改不了也读不到。
// 值传空串即删除该项。
func NewSetPlayerRoundVarEffect(playerID, key, value string) *Effect {
	return NewEffect(EventSetPlayerRoundVar, "", playerID).
		WithData(playerRoundVarKeyKey, key).
		WithData(playerRoundVarValueKey, value)
}

// playerRoundVarOf 从效果里读出要写的键值。
func playerRoundVarOf(e *Effect) (key, value string) {
	key, _ = e.Data[playerRoundVarKeyKey].(string)
	value, _ = e.Data[playerRoundVarValueKey].(string)
	return key, value
}

// NewSetPlayerVarEffect 声明「把某个玩家的某项自定义状态改成某值」。
//
// 这是角色存放自身状态的正路。白痴的「翻过牌了」、骑士的「决斗用掉了」、
// 女巫的两瓶药、守卫的守护记录，全都是同一件事，走的也是同一条路。
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
// 这个每回合自动清零，且不属于任何玩家（今晚的刀口是谁）。
// 「某个玩家在本回合的标记」是第三种，用 NewSetPlayerRoundVarEffect。
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

// clone 深拷贝一条效果，连同它的 Data。
//
// 效果流是这个引擎的历史，「历史不可改写」不能只靠文档：此前
// EndPhase 返回的与 EffectLog 返回的，都是引擎内部那份历史的同一批
// 指针，调用方随手改一个字段（或者调一下 Cancel，它是导出的）就把
// 历史改了，而回放会照着被改过的历史重建出另一局游戏。
//
// 现在进日志的是副本、出日志的也是副本，两侧都不与调用方共享对象。
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
