package werewolf

import (
	"testing"

	pb "github.com/Zereker/werewolf/proto"
)

// TestGameView_IsReadOnly 视图不能被还原成可变的状态对象。
//
// 这是「状态变更一律经由 Effect」这条不变量的类型级保证：
// stateView 是不导出的值类型，且其字段不导出，包外无法从 GameView
// 断言出任何能改状态的东西。
func TestGameView_IsReadOnly(t *testing.T) {
	st := newState()
	if err := st.addPlayer("v1", pb.RoleType_ROLE_TYPE_VILLAGER); err != nil {
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
	for id, role := range map[string]pb.RoleType{
		"w1": pb.RoleType_ROLE_TYPE_WEREWOLF,
		"g":  pb.RoleType_ROLE_TYPE_GUARD,
		"v1": pb.RoleType_ROLE_TYPE_VILLAGER,
		"v2": pb.RoleType_ROLE_TYPE_VILLAGER,
	} {
		if err := st.addPlayer(id, role); err != nil {
			t.Fatal(err)
		}
	}
	st.applyEffect(NewEffect(pb.EventType_EVENT_TYPE_KILL, "", "v2"))
	st.applyEffect(NewEffect(pb.EventType_EVENT_TYPE_SET_LAST_PROTECTED, "g", "v1"))
	st.applyEffect(NewEffect(pb.EventType_EVENT_TYPE_SET_NIGHT_KILL, "", "v1"))

	view := newStateView(st)

	if got := len(view.AlivePlayers()); got != 3 {
		t.Errorf("存活玩家数: 期望 3，实际 %d", got)
	}
	if got := view.AlivePlayerIDsByRole(pb.RoleType_ROLE_TYPE_WEREWOLF); len(got) != 1 || got[0] != "w1" {
		t.Errorf("狼人列表: 期望 [w1]，实际 %v", got)
	}
	if got := view.LastProtectedTarget("g"); got != "v1" {
		t.Errorf("守卫上回合目标: 期望 v1，实际 %q", got)
	}
	if got := view.LastProtectedTarget("查无此人"); got != "" {
		t.Errorf("不存在的玩家应返回空，实际 %q", got)
	}
	if got := view.RoundContext().KillTarget; got != "v1" {
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
	if err := st.addPlayer("v1", pb.RoleType_ROLE_TYPE_VILLAGER); err != nil {
		t.Fatal(err)
	}
	st.applyEffect(NewEffect(pb.EventType_EVENT_TYPE_SET_NIGHT_KILL, "", "v1"))

	view := newStateView(st)
	rc := view.RoundContext()
	rc.KillTarget = "被篡改"
	rc.ProtectedPlayers["v1"] = true

	if got := view.RoundContext().KillTarget; got != "v1" {
		t.Errorf("改动副本影响到了引擎状态: %q", got)
	}
	fresh := view.RoundContext()
	if fresh.IsProtected("v1") {
		t.Error("改动副本的 map 影响到了引擎状态")
	}
}
