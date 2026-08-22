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
	EventUnspecified EventType = 0
	EventGameStarted EventType = 1
	EventGameEnded   EventType = 2
	EventKill        EventType = 3  // 狼人击杀
	EventProtect     EventType = 4  // 守卫保护
	EventSave        EventType = 5  // 女巫救人
	EventPoison      EventType = 6  // 女巫毒杀
	EventCheck       EventType = 7  // 预言家查验
	EventEliminate   EventType = 8  // 投票出局
	EventShoot       EventType = 9  // 猎人开枪
	EventSkip        EventType = 10 // 跳过行动
	EventVoteTied    EventType = 11 // 投票平票或无人得票，本轮无人出局
	// 100..104 曾是狼人杀的内部事件：设置与清除刀口、记录守护、消耗解药、
	// 消耗毒药。每一个在内核里都对应一条改状态的分支，也就是把「一局狼人杀
	// 的夜里会发生什么」写进了状态机。这些状态变更现在都由下面的通用原语
	// 表达，编号留空不再复用——效果流里出现旧编号即是旧数据。
	EventAbilityTriggered  EventType = 105 // 死亡技能触发，待进入对应阶段结算
	EventPlayerAdded       EventType = 106 // 玩家入座（用于效果流回放）
	EventPhaseChanged      EventType = 107 // 阶段流转（用于效果流回放）
	EventSetPlayerVar      EventType = 108 // 写玩家的自定义状态（供第三方角色使用）
	EventSetRoundVar       EventType = 109 // 写本回合的自定义状态（供第三方角色使用）
	EventSetAlive          EventType = 110 // 改玩家的存活状态
	EventSetPlayerRoundVar EventType = 111 // 写玩家的回合级自定义状态
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
	EventUnspecified:       "UNSPECIFIED",
	EventGameStarted:       "GAME_STARTED",
	EventGameEnded:         "GAME_ENDED",
	EventKill:              "KILL",
	EventProtect:           "PROTECT",
	EventSave:              "SAVE",
	EventPoison:            "POISON",
	EventCheck:             "CHECK",
	EventEliminate:         "ELIMINATE",
	EventShoot:             "SHOOT",
	EventSkip:              "SKIP",
	EventVoteTied:          "VOTE_TIED",
	EventAbilityTriggered:  "ABILITY_TRIGGERED",
	EventPlayerAdded:       "PLAYER_ADDED",
	EventPhaseChanged:      "PHASE_CHANGED",
	EventSetPlayerVar:      "SET_PLAYER_VAR",
	EventSetRoundVar:       "SET_ROUND_VAR",
	EventSetAlive:          "SET_ALIVE",
	EventSetPlayerRoundVar: "SET_PLAYER_ROUND_VAR",
}
