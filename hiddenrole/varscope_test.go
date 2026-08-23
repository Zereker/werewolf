package hiddenrole

import "testing"

// allScopes 四格作用域的全集，按 (时间尺度, 有没有主人) 叉乘写出。
//
// 这个函数是这一批测试的意义所在：作用域此前是四个互不相干的构造器，
// 「一共有几格」没有任何地方说得清，于是少一格谁也不会发现（事实上就是
// 少了「整局·无主」，直到任务制那一套撞上）。现在它是一个类型，全集能枚举，
// 下面每条性质都对全集断言——再少一格，测试而不是下一个规则包会先撞上。
func allScopes(ownerID string) []struct {
	name     string
	scope    VarScope
	perRound bool
	owned    bool
} {
	return []struct {
		name     string
		scope    VarScope
		perRound bool
		owned    bool
	}{
		{"整局·无主", ScopeGame, false, false},
		{"整局·某人", ScopeGame.Of(ownerID), false, true},
		{"本回合·无主", ScopeRound, true, false},
		{"本回合·某人", ScopeRound.Of(ownerID), true, true},
	}
}

// TestVarScope_AllFourCellsRoundTrip 四格都能写、能读回来、空串等同删除。
func TestVarScope_AllFourCellsRoundTrip(t *testing.T) {
	for _, c := range allScopes("p1") {
		t.Run(c.name, func(t *testing.T) {
			state := newState()
			mustAddTo(t, state, "p1", roleVillager)

			if got := state.varOf(c.scope, "k"); got != "" {
				t.Fatalf("没写过就该是空串，得到 %q", got)
			}
			state.applyEffect(NewSetVarEffect(c.scope, "k", "v"))
			if got := state.varOf(c.scope, "k"); got != "v" {
				t.Fatalf("写完读回来应当是 v，得到 %q", got)
			}
			state.applyEffect(NewSetVarEffect(c.scope, "k", ""))
			if got := state.varOf(c.scope, "k"); got != "" {
				t.Fatalf("空串应当等同删除，得到 %q", got)
			}
			// 「删掉」与「置成空串」读回来都是空串，分不出——要分得看底下那张表。
			// 分不出的后果在快照里：置空串会留下一条空记录，存档因此越滚越大，
			// 而且与「从没写过」的存档不再逐字节相同。
			if vars, _ := state.varsFor(c.scope); vars != nil {
				if _, exists := vars["k"]; exists {
					t.Fatalf("空串应当把键删掉，而不是留一条空记录：%v", vars)
				}
			}
		})
	}
}

// TestVarScope_CellsDoNotLeakIntoEachOther 同一个键写进不同的格子互不干扰。
//
// 四格共用一个事件类型与一套数据键之后，这条不再是「显然的」：
// 写入点若把作用域读错，四格会串在一起。
func TestVarScope_CellsDoNotLeakIntoEachOther(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "p1", roleVillager)
	mustAddTo(t, state, "p2", roleVillager)

	cells := allScopes("p1")
	for i, c := range cells {
		state.applyEffect(NewSetVarEffect(c.scope, "k", c.name))
		_ = i
	}
	for _, c := range cells {
		if got := state.varOf(c.scope, "k"); got != c.name {
			t.Errorf("%s 被别的格子覆盖了：想要 %q，得到 %q", c.name, c.name, got)
		}
	}

	// 有主的两格认人：写给 p1 的，p2 读不到。
	for _, c := range cells {
		if !c.owned {
			continue
		}
		other := ScopeGame.Of("p2")
		if c.perRound {
			other = ScopeRound.Of("p2")
		}
		if got := state.varOf(other, "k"); got != "" {
			t.Errorf("%s 串到了别的玩家身上：p2 读到 %q", c.name, got)
		}
	}
}

// TestVarScope_RoundBoundaryClearsExactlyTheRoundCells 回合边界清掉且只清掉回合级的两格。
//
// 「本回合有效」这个轴的全部含义就在这条：清多了，整局状态会丢；
// 清少了，上一夜的标记会累积到下一夜——两种都出过。
func TestVarScope_RoundBoundaryClearsExactlyTheRoundCells(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "p1", roleVillager)

	cells := allScopes("p1")
	for _, c := range cells {
		state.applyEffect(NewSetVarEffect(c.scope, "k", "v"))
	}

	state.resetRoundState()

	for _, c := range cells {
		got := state.varOf(c.scope, "k")
		if c.perRound && got != "" {
			t.Errorf("%s 是回合级的，过了回合边界应当清空，得到 %q", c.name, got)
		}
		if !c.perRound && got != "v" {
			t.Errorf("%s 跟着整局走，不该被回合边界清掉，得到 %q", c.name, got)
		}
	}
}

// TestVarScope_SetsVarReportsTheCell 效果里读得回作用域。
//
// 四格收进 SET_VAR 一个事件类型之后，光看 Type 分不出这一条写的是哪一格。
// 想拦下或者观察某一类写入的扩展只能走 SetsVar——它要认得回全部四格。
func TestVarScope_SetsVarReportsTheCell(t *testing.T) {
	for _, c := range allScopes("p1") {
		t.Run(c.name, func(t *testing.T) {
			ef := NewSetVarEffect(c.scope, "k", "v")
			scope, key, value, ok := ef.SetsVar()
			if !ok {
				t.Fatal("SET_VAR 效果应当被 SetsVar 认出来")
			}
			if key != "k" || value != "v" {
				t.Errorf("键值读错了：%q=%q", key, value)
			}
			if scope != c.scope {
				t.Errorf("作用域读错了：想要 %v，得到 %v", c.scope, scope)
			}
		})
	}

	if _, _, _, ok := NewSetAliveEffect("p1", false).SetsVar(); ok {
		t.Error("SET_ALIVE 不是写状态变量，不该被 SetsVar 认出来")
	}
	var nilEffect *Effect
	if _, _, _, ok := nilEffect.SetsVar(); ok {
		t.Error("nil 效果不该被 SetsVar 认出来")
	}
}

// TestVarScope_OfDoesNotMutateTheSharedValues ScopeGame 与 ScopeRound 是值，.Of 返回副本。
//
// 它们是包级变量，若 .Of 改的是接收者本身，一次 Of 就会把全局的
// ScopeGame 永久绑到某个玩家身上——所有后续写入都会写错格子。
func TestVarScope_OfDoesNotMutateTheSharedValues(t *testing.T) {
	beforeGame, beforeRound := ScopeGame, ScopeRound

	_ = ScopeGame.Of("p1")
	_ = ScopeRound.Of("p1")

	if ScopeGame != beforeGame {
		t.Errorf("ScopeGame 被 .Of 改掉了：%v", ScopeGame)
	}
	if ScopeRound != beforeRound {
		t.Errorf("ScopeRound 被 .Of 改掉了：%v", ScopeRound)
	}
	if ScopeGame.String() != "game" || ScopeRound.String() != "round" {
		t.Errorf("无主的两格打印错了：%q / %q", ScopeGame, ScopeRound)
	}
	if got := ScopeRound.Of("p1").String(); got != "round:p1" {
		t.Errorf("有主的格子打印错了：%q", got)
	}
}

// TestVarScope_UnknownOwnerIsANoOp 写给不存在的玩家不该改到任何东西。
func TestVarScope_UnknownOwnerIsANoOp(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "p1", roleVillager)

	state.applyEffect(NewSetVarEffect(ScopeGame.Of("ghost"), "k", "v"))
	state.applyEffect(NewSetVarEffect(ScopeRound.Of("ghost"), "k", "v"))

	for _, c := range allScopes("p1") {
		if got := state.varOf(c.scope, "k"); got != "" {
			t.Errorf("写给不存在的玩家却改到了 %s：%q", c.name, got)
		}
	}
}
