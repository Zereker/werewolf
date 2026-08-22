package werewolf

import (
	"testing"

	"github.com/Zereker/werewolf/engine"
)

// campLovers 一个第三方阵营。
//
// Camp 的底层是字符串，内核不预设任何取值——好人与狼人也只是狼人杀
// 定义的两个，与这里的「情侣」没有身份差别，不需要「自定义取值从 1000 起」
// 这类约定来避让。
const campLovers Camp = "LOVERS"

// loversWin 情侣胜利：只剩这两个人时他们赢，无论阵营。
//
// 这是内置判定根本表达不了的东西——它只认好人与狼人两边，
// 第三方阵营连「赢」这个结论都报不出去。
type loversWin struct {
	a, b  string
	inner engine.VictoryChecker
}

func (l loversWin) CheckVictory(view GameView) (bool, Camp) {
	alive := view.AlivePlayers()
	if len(alive) == 2 {
		got := map[string]bool{alive[0].ID: true, alive[1].ID: true}
		if got[l.a] && got[l.b] {
			return true, campLovers
		}
	}
	// 自己这条不成立就走原来的规则
	return l.inner.CheckVictory(view)
}

// TestVictoryChecker_ThirdCamp 第三方阵营能赢。
//
// 胜负判定此前写死在引擎里，只认好人与狼人。丘比特的情侣、第三方阵营
// 这类板子做不出来——不是「做起来麻烦」，是根本没有地方表达。
func TestVictoryChecker_ThirdCamp(t *testing.T) {
	checker := loversWin{a: "w1", b: "v1", inner: DefaultVictoryChecker{}}

	g := newRuleGameWith(t, nil, []EngineOption{engine.WithVictoryChecker(checker)},
		seats(wolf("w1"), wolf("w2"), seer("s"), villagers("v1", "v2", "v3"))...)

	// 还没到只剩两人，走的是内置规则
	if g.e.Status().Over {
		t.Fatal("前置条件：现在不该分出胜负")
	}

	// 只留下这对情侣
	g.setDead("w2", "s", "v2", "v3")
	g.endAny()

	if !g.e.Status().Over {
		t.Fatal("只剩情侣两人，应当分出胜负")
	}
	if winner := g.e.Status().Winner; winner != campLovers {
		t.Errorf("胜方应当是情侣阵营，实际 %v", winner)
	}
}

// TestVictoryChecker_FallsBackToBuiltin 自定义判定不成立时走内置规则。
func TestVictoryChecker_FallsBackToBuiltin(t *testing.T) {
	checker := loversWin{a: "w1", b: "v1", inner: DefaultVictoryChecker{}}

	g := newRuleGameWith(t, nil, []EngineOption{engine.WithVictoryChecker(checker)},
		seats(wolf("w1"), wolf("w2"), seer("s"), villagers("v1", "v2", "v3"))...)

	// 狼全死：情侣那条不成立，内置规则判好人胜
	g.setDead("w1", "w2")

	over, winner := checkVictory(g.e)
	if !over || winner != CampGood {
		t.Errorf("应当回落到内置规则判好人胜，实际 over=%v winner=%v", over, winner)
	}
}

// TestVictoryChecker_DefaultIsUnchanged 不换判定器时，行为与从前一致。
func TestVictoryChecker_DefaultIsUnchanged(t *testing.T) {
	for _, mode := range []VictoryMode{VictoryModeSideWipe, VictoryModeTownWipe} {
		cfgRules := DefaultRules()
		cfgRules.VictoryMode = mode

		g := newRuleGameR(t, cfgRules, seats(
			wolf("w1"), wolf("w2"), seer("s"), villagers("v1", "v2", "v3"),
		)...)
		g.setDead("w1", "w2")

		over, winner := checkVictory(g.e)
		if !over || winner != CampGood {
			t.Errorf("%v: 狼全死应当好人胜，实际 over=%v winner=%v", mode, over, winner)
		}
	}
}

// TestWithVictoryChecker_RejectsNil 传 nil 只可能是漏了，构造时就该报出来。
func TestWithVictoryChecker_RejectsNil(t *testing.T) {
	if _, err := engine.NewEngine(nil, engine.WithVictoryChecker(nil)); err == nil {
		t.Error("nil 判定器应当被拒绝")
	}
}
