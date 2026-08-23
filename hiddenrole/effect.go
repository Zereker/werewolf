package hiddenrole

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

// eventKind 内核事件的类别。
//
// 「内核事件分几类」此前只是 kernelPrimitives 那张表上的一句注释——
// 「它们是状态机的记账（谁的存活位翻了、谁身上多了个标记）」。那句话
// 对 GOTO_PHASE 是**假的**：它在 applyEffect 里根本没有分支，一个状态
// 都不改。行为一直是对的（永不外发），分类是错的，而分类只是注释，
// 错了没有任何东西会响。
//
// 现在类别是一个值，三条性质因此都能断言（见 effect_test.go）：
// 改状态的必须真的改得动状态，控制指令与回放记账必须一个字节都不动。
type eventKind uint8

const (
	// kindRuleEvent 规则给「发生了什么」起的名字（KILL、SHOOT、决斗）。
	// 内核不认得，推给 OnEvent，受众由规则决定。这是缺省值：
	// 任何不在下面那张表里的取值都归这一类。
	kindRuleEvent eventKind = iota

	// kindStateWrite 改状态的原语，applyEffect 里有它的分支。
	kindStateWrite

	// kindControl 控制指令。不改任何状态，只影响内核下一步去哪。
	kindControl

	// kindReplay 效果流回放用的记账，由内核自己写进流里。
	kindReplay
)

// kernelEvents 内核自己认得的事件，以及各是哪一类。
//
// 不在这张表里的一律是规则事件——判断依据是这张表，不是编号区间。
// 此前写成「>= 100 即内部」，与「第三方取值从 1000 起」那条约定直接
// 打架：扩展定义的每一个事件类型都被判成内部事件，于是扩展的事件
// 根本发不出去。
var kernelEvents = map[EventType]eventKind{
	EventSetAlive:  kindStateWrite,
	EventSetVar:    kindStateWrite,
	EventSetActors: kindStateWrite,
	EventDetour:    kindStateWrite,

	EventGotoPhase: kindControl,

	EventPlayerAdded:  kindReplay,
	EventPhaseChanged: kindReplay,
}

// isInternalEvent 判断事件是不是内核自己的原语。
//
// 三类内核事件都不该出现在任何玩家面前——AudienceOf 对它们的回答是
// 「明确不给任何人看」，且这一条不可配置。规则事件则相反，受众由规则决定。
func isInternalEvent(t EventType) bool {
	return kernelEvents[t] != kindRuleEvent
}

// detourPhaseKey 绕道效果里记录「去哪个阶段」的键
const detourPhaseKey = "detour_phase"

// NewDetourEffect 声明「为了这个人，绕一趟那个阶段」（见 Detour）。
//
// 狼人杀用它做「猎人被刀之后开枪」，但内核认得的既不是「死亡」也不是
// 「技能」，只是「谁、去哪个阶段」——什么触发了它、他到了那儿要干什么，
// 全是规则的事。出局时开枪、自爆、翻牌、任何「等一下，还有人要动」都走这条。
//
// 与 NewGotoPhaseEffect 的分工：那个是**一次性改写下一站**，这个是
// **排一笔欠账**——队列排空之前，胜负判定与回合边界都得等着。
func NewDetourEffect(playerID string, phase PhaseType) *Effect {
	return NewEffect(EventDetour, playerID, "").
		WithData(detourPhaseKey, phase)
}

// winnerKey GAME_ENDED 效果里记录赢家的键。
//
// 两处用它：产出时写进去（endPhaseInternal），效果流回放时读回来
// （replayEffect）。写成常量而不是两处各写一遍字面量——那两处写的必须
// 是同一个键，而字面量不会告诉任何人这件事。
const winnerKey = "winner"

// gotoPhaseKey 改写下一阶段的效果里记录目标阶段的键
const gotoPhaseKey = "goto_phase"

// NewGotoPhaseEffect 声明「这个阶段结算完之后去指定的阶段」。
//
// 它改写 PhaseConfig.NextPhase 那个默认出口。阶段流转此前是一张纯静态的图，
// 唯一的动态跳转是绕道队列——于是所有条件分支都得从那个后门走，
// 而那个后门的语义是「某人的技能待结算」，根本不是「往哪走」。
//
// missions 包的「表决通过就去任务、否则回提名」是这类分支最朴素的样子：
// 结果由本阶段的结算算出来，静态图表达不了。
//
// 优先级：待结算的绕道队列 > 本效果 > PhaseConfig.NextPhase。绕道排在最前
// 是因为队列必须排空——胜负判定与回合边界都等着它，中途跳走会把还没结算的
// 那一笔欠账丢掉。
//
// 目标阶段不在配置里时，内核记一条错误日志并退回 NextPhase：一条效果写错了
// 不该让整局崩掉，但也不能安静地跳去一个没人预期的地方。
func NewGotoPhaseEffect(phase PhaseType) *Effect {
	return NewEffect(EventGotoPhase, "", "").WithData(gotoPhaseKey, phase)
}

// gotoPhase 从改写效果里读出目标阶段
func (e *Effect) gotoPhase() (PhaseType, bool) {
	v, ok := e.Data[gotoPhaseKey]
	if !ok {
		return PhaseUnspecified, false
	}
	p, ok := v.(PhaseType)
	if !ok {
		return PhaseUnspecified, false
	}
	return p, true
}

// detourPhase 从触发效果中读出目标阶段
func (e *Effect) detourPhase() (PhaseType, bool) {
	v, ok := e.Data[detourPhaseKey]
	if !ok {
		return PhaseUnspecified, false
	}
	phase, ok := v.(PhaseType)
	return phase, ok
}

// 写自定义状态时用到的三个键：作用域、键、值。
//
// 此前每个作用域一套自己的键名（var_key / round_var_key /
// player_round_var_key……），六个常量描述的是同一件事。
const (
	varScopeKey = "var_scope"
	varKeyKey   = "var_key"
	varValueKey = "var_value"

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

// NewSetVarEffect 声明「把某项自定义状态改成某值」，作用域由 scope 指定。
//
// 四种作用域此前是四个构造器，于是没有任何东西强制那张 2×2 的表完整
// ——少了「整局·无主」那一格很久没人发现。现在作用域是一个参数：
//
//	NewSetVarEffect(ScopeGame, "score", "3")              整局·无主
//	NewSetVarEffect(ScopeGame.Of(id), "antidote", "used") 整局·某人
//	NewSetVarEffect(ScopeRound, "kill", target)           本回合·无主
//	NewSetVarEffect(ScopeRound.Of(id), "guarded", "1")    本回合·某人
//
// 这是角色存放自身状态的正路。白痴的「翻过牌了」、骑士的「决斗用掉了」、
// 女巫的两瓶药、守卫的守护记录，全都是同一件事，走的也是同一条路。
// 走这条路才自动获得整套设施：状态随快照走、效果流能回放、Resolver
// 因此可以保持无状态——而无状态正是 Resolver 接口要求的。
//
// 值传空串即删除该项，四种作用域同一个口径。
func NewSetVarEffect(scope VarScope, key, value string) *Effect {
	return NewEffect(EventSetVar, "", scope.owner).
		WithData(varScopeKey, scope).
		WithData(varKeyKey, key).
		WithData(varValueKey, value)
}

// SetsVar 这个效果是否在写一项自定义状态，以及写的是哪一格、什么键值。
//
// 与 SetsAlive 同一个用法：想拦下或者观察某一类写入的扩展需要它。
// 四种作用域收进一个事件类型之后，光看 Type 分不出「整局」还是「本回合」、
// 属不属于某个玩家——要分就从这里读。
func (e *Effect) SetsVar() (scope VarScope, key, value string, ok bool) {
	if e == nil || e.Type != EventSetVar {
		return VarScope{}, "", "", false
	}
	scope, key, value = varOf(e)
	return scope, key, value, key != ""
}

// varOf 从效果里读出作用域与键值。
func varOf(e *Effect) (scope VarScope, key, value string) {
	scope, _ = e.Data[varScopeKey].(VarScope)
	key, _ = e.Data[varKeyKey].(string)
	value, _ = e.Data[varValueKey].(string)
	return scope, key, value
}

// actorsPhaseKey / actorsListKey 行动者效果里的两个键
const (
	actorsPhaseKey = "actors_phase"
	actorsListKey  = "actors_list"
)

// NewSetActorsEffect 声明「这几个玩家可以在指定阶段行动」。
//
// 内核判定行动者的默认办法是拿 PhaseStep.Role 比对玩家角色——而角色是入座时
// 定死的，任何**运行时才选出来的**行动者集合都表达不了：missions 包的任务队伍是
// 上一个阶段投票选出来的，队长是按座位轮转的。没有这条效果，规则只能让所有人
// 都提交、再自己丢掉不该算的，而内核会对没资格的玩家说「你可以行动」。
//
// 优先级：待结算的绕道队列 > 本效果 > PhaseStep.Role。与 NewGotoPhaseEffect
// 是同一个分层——默认值加运行时改写。
//
// 名单在**更早的阶段**算出来是常态，所以要指定阶段而不是只作用于当前阶段。
// 某个阶段结算完，它的这一份就被消费掉：不清的话下一次进同一个阶段会沿用
// 上一轮的名单。
//
// 传空名单是有意义的：那是「这个阶段没有人能行动」，与「规则没指定」不同。
//
// 名单里不存在的玩家会被忽略；名单会按 ID 排序后存下，效果流因此是确定的。
func NewSetActorsEffect(phase PhaseType, playerIDs ...string) *Effect {
	return NewEffect(EventSetActors, "", "").
		WithData(actorsPhaseKey, phase).
		WithData(actorsListKey, append([]string(nil), playerIDs...))
}

// actorsOf 从效果里读出阶段与名单
func actorsOf(e *Effect) (PhaseType, []string, bool) {
	p, ok := e.Data[actorsPhaseKey].(PhaseType)
	if !ok {
		return PhaseUnspecified, nil, false
	}
	ids, ok := e.Data[actorsListKey].([]string)
	if !ok {
		return PhaseUnspecified, nil, false
	}
	return p, ids, true
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
