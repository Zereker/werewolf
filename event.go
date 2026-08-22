// event.go 对外事件：引擎发生了什么，说给调用方听。
//
// Event 与 Effect 是刻意分开的两层：Effect 是引擎内部对状态变更的描述，
// 带着 interface{} 的附加数据；Event 是交给调用方的形态，附加数据折成
// 字符串，可以直接序列化发出去。

package werewolf

import "fmt"

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

// EventType 事件/效果类型
type EventType int32

const (
	EventUnspecified      EventType = 0
	EventGameStarted      EventType = 1
	EventGameEnded        EventType = 2
	EventKill             EventType = 3   // 狼人击杀
	EventProtect          EventType = 4   // 守卫保护
	EventSave             EventType = 5   // 女巫救人
	EventPoison           EventType = 6   // 女巫毒杀
	EventCheck            EventType = 7   // 预言家查验
	EventEliminate        EventType = 8   // 投票出局
	EventShoot            EventType = 9   // 猎人开枪
	EventSkip             EventType = 10  // 跳过行动
	EventVoteTied         EventType = 11  // 投票平票或无人得票，本轮无人出局
	EventSetNightKill     EventType = 100 // 设置夜晚击杀目标
	EventClearNightKill   EventType = 101 // 清除夜晚击杀目标（被救）
	EventSetLastProtected EventType = 102 // 设置守卫上回合保护目标
	EventUseAntidote      EventType = 103 // 消耗解药
	EventUsePoison        EventType = 104 // 消耗毒药
	EventAbilityTriggered EventType = 105 // 死亡技能触发，待进入对应阶段结算
	EventPlayerAdded      EventType = 106 // 玩家入座（用于效果流回放）
	EventPhaseChanged     EventType = 107 // 阶段流转（用于效果流回放）
)

// String 实现 fmt.Stringer，输出沿用枚举全名。
func (v EventType) String() string {
	if s, ok := eventTypeNames[v]; ok {
		return s
	}
	return fmt.Sprintf("EventType(%d)", int32(v))
}

// eventTypeNames 全部取值到名字的映射，遍历它即可枚举所有取值。
var eventTypeNames = map[EventType]string{
	EventUnspecified:      "EVENT_TYPE_UNSPECIFIED",
	EventGameStarted:      "EVENT_TYPE_GAME_STARTED",
	EventGameEnded:        "EVENT_TYPE_GAME_ENDED",
	EventKill:             "EVENT_TYPE_KILL",
	EventProtect:          "EVENT_TYPE_PROTECT",
	EventSave:             "EVENT_TYPE_SAVE",
	EventPoison:           "EVENT_TYPE_POISON",
	EventCheck:            "EVENT_TYPE_CHECK",
	EventEliminate:        "EVENT_TYPE_ELIMINATE",
	EventShoot:            "EVENT_TYPE_SHOOT",
	EventSkip:             "EVENT_TYPE_SKIP",
	EventVoteTied:         "EVENT_TYPE_VOTE_TIED",
	EventSetNightKill:     "EVENT_TYPE_SET_NIGHT_KILL",
	EventClearNightKill:   "EVENT_TYPE_CLEAR_NIGHT_KILL",
	EventSetLastProtected: "EVENT_TYPE_SET_LAST_PROTECTED",
	EventUseAntidote:      "EVENT_TYPE_USE_ANTIDOTE",
	EventUsePoison:        "EVENT_TYPE_USE_POISON",
	EventAbilityTriggered: "EVENT_TYPE_ABILITY_TRIGGERED",
	EventPlayerAdded:      "EVENT_TYPE_PLAYER_ADDED",
	EventPhaseChanged:     "EVENT_TYPE_PHASE_CHANGED",
}
