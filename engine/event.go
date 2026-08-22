// event.go 对外事件：引擎发生了什么，说给调用方听。
//
// Event 与 Effect 是刻意分开的两层：Effect 是引擎内部对状态变更的描述，
// 带着 interface{} 的附加数据；Event 是交给调用方的形态，附加数据折成
// 字符串，可以直接序列化发出去。

package engine

// Event 一件对外可见的事情。
//
// 由 Effect.ToEvent 构造，经 Engine.OnEvent 注册的处理器收到。
// 该发给哪些玩家由 Engine.AudienceOf 回答。
type Event struct {
	Type     EventType         `json:"type"`
	SourceID string            `json:"source_id,omitempty"` // 事件来源玩家
	TargetID string            `json:"target_id,omitempty"` // 事件目标玩家
	Data     map[string]string `json:"data,omitempty"`      // 附加数据

	// Canceled / Reason 该行动是否被规则否决，以及原因。
	//
	// 「女巫点了毒药但今晚已经用过解药」这类事情必须能表达出来：
	// 少了这两个字段，被否决的行动到了调用方手里与成功的一模一样，
	// 会被当成真的发生过而广播出去。
	Canceled bool   `json:"canceled,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// EventType 事件/效果类型。
//
// 分两类，且这个分法由**名字的归属**决定，不由编号区间决定：
//
//	内核的状态原语   SET_ALIVE / SET_PLAYER_VAR / ... —— 状态机的记账，永不外发
//	其余一切        规则给「发生了什么」起的名字 —— 推给 OnEvent，受众由规则决定
//
// 编号时代这里是三段：1..99 外部、100..999 内部、1000 起第三方。那个约定
// 自己咬到过自己：第三方定义的每一个事件类型都落在「内部」段里，于是
// 扩展的事件根本发不出去（规则包自己的公开事件全场看不到）。名字之后
// 不再有段，内核只认自己那七个，别的一律当外部事件。
type EventType string

// EventUnspecified 未指定。
const EventUnspecified EventType = ""

// 内核自己的事件：开局与结束由它发出，其余七个是状态原语，永不外发。
const (
	EventGameStarted EventType = "GAME_STARTED"
	EventGameEnded   EventType = "GAME_ENDED"

	// —— 状态原语，永不外发 ——
	EventAbilityTriggered  EventType = "ABILITY_TRIGGERED"    // 死亡技能触发，待进入对应阶段结算
	EventPlayerAdded       EventType = "PLAYER_ADDED"         // 玩家入座（用于效果流回放）
	EventPhaseChanged      EventType = "PHASE_CHANGED"        // 阶段流转（用于效果流回放）
	EventSetPlayerVar      EventType = "SET_PLAYER_VAR"       // 写玩家的自定义状态
	EventSetRoundVar       EventType = "SET_ROUND_VAR"        // 写本回合的自定义状态
	EventSetAlive          EventType = "SET_ALIVE"            // 改玩家的存活状态
	EventSetPlayerRoundVar EventType = "SET_PLAYER_ROUND_VAR" // 写玩家的回合级自定义状态
	EventGotoPhase         EventType = "GOTO_PHASE"           // 规则指定下一阶段，改写 NextPhase
	EventSetGameVar        EventType = "SET_GAME_VAR"         // 写整局有效、不属于任何玩家的状态
)

// String 实现 fmt.Stringer。
func (v EventType) String() string {
	if v == EventUnspecified {
		return "UNSPECIFIED"
	}
	return string(v)
}
