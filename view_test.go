package werewolf

import (
	"testing"
)

// TestGameView_IsReadOnly 视图不能被还原成可变的状态对象。
//
// 这是「状态变更一律经由 Effect」这条不变量的类型级保证：
// stateView 是不导出的值类型，且其字段不导出，包外无法从 GameView
// 断言出任何能改状态的东西。
func TestGameView_IsReadOnly(t *testing.T) {
	st := newState()
	if err := st.addPlayer("v1", RoleVillager); err != nil {
		t.Fatal(err)
	}
	view := newStateView(st)

	// 用 any 绕开编译期检查再断言：*gameState 根本不实现 GameView，
	// 因此拿不到任何可改状态的东西。
	// （直接写 view.(*gameState) 编译器会以 impossible type assertion 拒绝，
	//   这本身就是这条约束成立的证明。）
	if _, ok := any(view).(*gameState); ok {
		t.Fatal("GameView 不应能被断言回 *gameState")
	}
	if _, ok := any(view).(interface{ applyEffect(*Effect) }); ok {
		t.Fatal("GameView 不应暴露任何改状态的方法")
	}
}

func TestGameView_ReadsThrough(t *testing.T) {
	st := newState()
	for id, role := range map[string]RoleType{
		"w1": RoleWerewolf,
		"g":  RoleGuard,
		"v1": RoleVillager,
		"v2": RoleVillager,
	} {
		if err := st.addPlayer(id, role); err != nil {
			t.Fatal(err)
		}
	}
	st.applyEffect(NewSetAliveEffect("v2", false))

	// lastProtected 问的是「上一回合」，因此守护要发生在第 1 回合，
	// 再把状态推到第 2 回合去读
	st.Round = 1
	for _, ef := range markProtected(newStateView(st), "g", "v1") {
		st.applyEffect(ef)
	}
	st.Round = 2
	setKill(st, "v1")

	view := newStateView(st)

	if got := len(view.AlivePlayers()); got != 3 {
		t.Errorf("存活玩家数: 期望 3，实际 %d", got)
	}
	if got := view.AlivePlayerIDsByRole(RoleWerewolf); len(got) != 1 || got[0] != "w1" {
		t.Errorf("狼人列表: 期望 [w1]，实际 %v", got)
	}
	if got := lastProtected(view, "g"); got != "v1" {
		t.Errorf("守卫上回合目标: 期望 v1，实际 %q", got)
	}
	if got := lastProtected(view, "查无此人"); got != "" {
		t.Errorf("不存在的玩家应返回空，实际 %q", got)
	}
	// 再过一回合，那次守护就不再是「上一回合」了
	st.Round = 3
	if got := lastProtected(view, "g"); got != "" {
		t.Errorf("隔了一回合应返回空，实际 %q", got)
	}
	if got := nightKillTarget(view); got != "v1" {
		t.Errorf("刀口: 期望 v1，实际 %q", got)
	}
	if _, ok := view.Player("查无此人"); ok {
		t.Error("不存在的玩家应返回 false")
	}
}

// TestGameView_RoundContextIsCopy 视图返回的回合上下文是副本，
// 改它不会影响引擎状态。
func TestGameView_RoundContextIsCopy(t *testing.T) {
	st := newState()
	if err := st.addPlayer("v1", RoleVillager); err != nil {
		t.Fatal(err)
	}
	setKill(st, "v1")

	view := newStateView(st)
	rc := view.RoundContext()
	rc.Vars[RoundVarKillTarget] = "被篡改"
	rc.Vars["凭空多出来的"] = "1"

	if got := nightKillTarget(view); got != "v1" {
		t.Errorf("改动副本影响到了引擎状态: %q", got)
	}
	if fresh := view.RoundContext(); fresh.Vars["凭空多出来的"] != "" {
		t.Error("改动副本的 map 影响到了引擎状态")
	}
}
