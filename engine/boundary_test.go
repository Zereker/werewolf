package engine

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// publicKernelEvents 内核自己发出、且**应当**让玩家看到的事件。
//
// 与 kernelPrimitives 一起，必须覆盖 event.go 里声明的每一个事件类型——
// 由 TestKernelEventTypes_AreAllClassified 强制。
var publicKernelEvents = map[EventType]bool{
	EventUnspecified: true, // 零值，不是真事件
	EventGameStarted: true,
	EventGameEnded:   true,
}

// TestKernelEventTypes_AreAllClassified 每一个内核事件类型都必须被明确分类。
//
// 「状态原语永不外发」是内核唯一一条不可配置的规则，判断依据是
// kernelPrimitives 这张手工维护的表。手工维护的表有一个固定的坏结局：
// 有人加了第八个内核事件类型、忘了往表里添一行，于是这条事件默认按
// 「外部事件」处理，交给 AudienceProvider 决定——一个「什么都给全场」的
// provider 就能把状态机的记账推给所有玩家。
//
// 这个测试把 event.go 当作真值：它解析源码取出全部 EventXxx 声明，
// 要求每一个都落在 kernelPrimitives 或 publicKernelEvents 里。新增一个
// 事件类型而不分类，它就变红——你必须回答「这条该不该让玩家看见」。
func TestKernelEventTypes_AreAllClassified(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "event.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 event.go: %v", err)
	}

	var declared []string
	ast.Inspect(f, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		ident, ok := spec.Type.(*ast.Ident)
		if !ok || ident.Name != "EventType" {
			return true
		}
		for _, name := range spec.Names {
			declared = append(declared, name.Name)
		}
		return true
	})
	if len(declared) == 0 {
		t.Fatal("在 event.go 里一个 EventType 常量都没解析到——这个测试失去了意义")
	}

	// 名字 -> 取值。常量在同包内，直接按名字对照取值表。
	byName := map[string]EventType{
		"EventUnspecified":       EventUnspecified,
		"EventGameStarted":       EventGameStarted,
		"EventGameEnded":         EventGameEnded,
		"EventAbilityTriggered":  EventAbilityTriggered,
		"EventPlayerAdded":       EventPlayerAdded,
		"EventPhaseChanged":      EventPhaseChanged,
		"EventSetPlayerVar":      EventSetPlayerVar,
		"EventSetRoundVar":       EventSetRoundVar,
		"EventSetAlive":          EventSetAlive,
		"EventSetPlayerRoundVar": EventSetPlayerRoundVar,
		"EventGotoPhase":         EventGotoPhase,
	}

	for _, name := range declared {
		v, known := byName[name]
		if !known {
			t.Errorf("event.go 里新增了 %s，但这个测试的取值表还没跟上——"+
				"补一行，同时决定它属于 kernelPrimitives 还是 publicKernelEvents", name)
			continue
		}
		switch {
		case kernelPrimitives[v] && publicKernelEvents[v]:
			t.Errorf("%s 同时被判成状态原语和公开事件", name)
		case !kernelPrimitives[v] && !publicKernelEvents[v]:
			t.Errorf("%s（%q）没有被分类：不进 kernelPrimitives 就意味着它会被"+
				"当成外部事件交给 AudienceProvider，一个「什么都给全场」的 provider "+
				"就能把它推给所有玩家。请明确它该不该让玩家看见", name, v)
		}
	}
}

// primitiveSpewer 把每一条内核状态原语都产出一遍。
type primitiveSpewer struct{}

func (primitiveSpewer) Resolve([]*SkillUse, GameView) []*Effect {
	return []*Effect{
		NewSetAliveEffect("g", false),
		NewSetPlayerVarEffect("w1", "probe.var", "1"),
		NewSetRoundVarEffect("probe.round", "1"),
		NewSetPlayerRoundVarEffect("w1", "probe.mark", "1"),
		NewAbilityTriggerEffect("w1", phaseNightHunter),
		NewGotoPhaseEffect(phaseDay),
		NewEffect(EventType("PROBE_PUBLIC"), "w1", "g"), // 一条普通的规则事件作对照
	}
}

// TestBoundary_StatePrimitivesNeverReachPlayers 状态原语走不到玩家面前，两条路都走不到。
//
// 内核在信息边界上只守一条底线，且不可配置：自己的状态原语永远不外发。
// 它有两条可能泄漏的路，此前只有第一条有测试盯着：
//
//   - AudienceOf：即使规则装了一个「什么都给全场」的 provider 也必须被拦住；
//   - OnEvent：宿主拿到什么就转发什么是很自然的写法，状态原语要是混进这一路，
//     等于把上帝视角直接推给所有人。
func TestBoundary_StatePrimitivesNeverReachPlayers(t *testing.T) {
	opts := append(withNoopResolvers(),
		WithResolver(phaseNightGuard, primitiveSpewer{}),
		// 一个最坏情况的 provider：什么都给全场
		WithAudience(AudienceFunc(func(*Event, GameView) ([]string, bool) {
			return []string{"w1", "g"}, true
		})))
	e := newTestEngine(t, opts...)
	mustAdd(t, e, "w1", roleWerewolf)
	mustAdd(t, e, "g", roleGuard)

	var seen []EventType
	e.OnEvent(func(ev *Event) { seen = append(seen, ev.Type) })

	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := e.EndPhase(); err != nil {
		t.Fatalf("EndPhase: %v", err)
	}

	// 一、AudienceOf 这一路
	for _, ef := range (primitiveSpewer{}).Resolve(nil, nil) {
		got, known := e.AudienceOf(ef.ToEvent())
		if !kernelPrimitives[ef.Type] {
			continue // 对照组：普通规则事件该走 provider
		}
		if !known {
			t.Errorf("%v 应当是明确的判定，不是「不知道」", ef.Type)
		}
		if len(got) != 0 {
			t.Errorf("%v 是状态原语，不该发给任何人，实际 %v", ef.Type, got)
		}
	}

	// 二、OnEvent 这一路
	sawPublic := false
	for _, typ := range seen {
		if kernelPrimitives[typ] {
			t.Errorf("状态原语 %v 出现在 OnEvent 里——宿主原样转发就把上帝视角发出去了", typ)
		}
		if typ == EventType("PROBE_PUBLIC") {
			sawPublic = true
		}
	}
	if !sawPublic {
		t.Error("普通规则事件没能到达 OnEvent——这个测试可能什么都没验到")
	}
}
