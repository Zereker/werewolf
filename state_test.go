package werewolf

import (
	"testing"
)

func TestNewState(t *testing.T) {
	state := newState()

	if state.Phase != PhaseStart {
		t.Errorf("expected Phase=START, got %v", state.Phase)
	}
	if state.Round != 0 {
		t.Errorf("expected Round=0, got %d", state.Round)
	}
	if len(state.players) != 0 {
		t.Errorf("expected empty players, got %d", len(state.players))
	}
}

func TestAddPlayer(t *testing.T) {
	state := newState()

	mustAddTo(t, state, "p1", RoleWerewolf)

	player, ok := state.getPlayer("p1")
	if !ok {
		t.Fatal("player not found after AddPlayer")
	}
	if player.ID != "p1" {
		t.Errorf("expected ID=p1, got %s", player.ID)
	}
	if player.Role != RoleWerewolf {
		t.Errorf("expected Role=WEREWOLF, got %v", player.Role)
	}
	if got := player.Vars[VarCamp]; got != string(CampEvil) {
		t.Errorf("expected Camp=EVIL, got %v", got)
	}
	if !player.Alive {
		t.Error("expected Alive=true")
	}
}

func TestGetPlayer_Exists(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "p1", RoleSeer)

	player, ok := state.getPlayer("p1")
	if !ok {
		t.Error("expected ok=true for existing player")
	}
	if player == nil {
		t.Error("expected player not nil")
	}
}

func TestGetPlayer_NotExists(t *testing.T) {
	state := newState()

	player, ok := state.getPlayer("nonexistent")
	if ok {
		t.Error("expected ok=false for non-existing player")
	}
	if player != nil {
		t.Error("expected player to be nil")
	}
}

// TestApplyEffect_KernelPrimitives 内核的状态原语。
//
// applyEffect 此前认得十来种效果类型——狼刀、毒杀、放逐、开枪各是一种死法，
// PROTECT / SAVE 各标记一件事。那等于把「一局狼人杀会发生什么」写进了状态机：
// 换一套规则它们一条都用不上，而新规则要表达自己的状态变更又只能来改这里。
func TestApplyEffect_KernelPrimitives(t *testing.T) {
	t.Run("SET_ALIVE 改存活", func(t *testing.T) {
		state := newState()
		mustAddTo(t, state, "p1", RoleVillager)

		state.applyEffect(NewSetAliveEffect("p1", false))
		if p, _ := state.getPlayer("p1"); p.Alive {
			t.Error("SET_ALIVE(false) 之后应当已出局")
		}
		state.applyEffect(NewSetAliveEffect("p1", true))
		if p, _ := state.getPlayer("p1"); !p.Alive {
			t.Error("SET_ALIVE(true) 之后应当复活")
		}
	})

	t.Run("SET_PLAYER_ROUND_VAR 标记本回合", func(t *testing.T) {
		state := newState()
		mustAddTo(t, state, "p1", RoleVillager)

		state.applyEffect(NewSetPlayerRoundVarEffect("p1", PlayerRoundVarProtected, VarPresent))
		if !protectedIn(state, "p1") {
			t.Error("标记之后应当读得到")
		}
		state.applyEffect(NewSetPlayerRoundVarEffect("p1", PlayerRoundVarProtected, ""))
		if protectedIn(state, "p1") {
			t.Error("空值应当等同删除")
		}
	})

	t.Run("回合边界清掉标记", func(t *testing.T) {
		state := newState()
		mustAddTo(t, state, "p1", RoleVillager)

		state.applyEffect(NewSetPlayerRoundVarEffect("p1", PlayerRoundVarSaved, VarPresent))
		setKill(state, "p1")
		state.resetRoundState()

		if savedIn(state, "p1") {
			t.Error("玩家身上的回合标记应当随回合清掉")
		}
		if got := killTargetOf(state); got != "" {
			t.Errorf("回合变量应当随回合清掉，实际 %q", got)
		}
	})
}

// TestApplyEffect_RuleEventsDoNotTouchState 规则的事件不改状态。
//
// 这是「内核不认识狼人杀」的可执行说法：KILL / POISON / ELIMINATE / SHOOT /
// PROTECT / SAVE 现在只是规则给「发生了什么」起的名字，给受众与效果流看。
// 真正改状态的是它们旁边那条原语——所以单独发一个 KILL，谁都不会死。
func TestApplyEffect_RuleEventsDoNotTouchState(t *testing.T) {
	for _, typ := range []EventType{
		EventKill, EventPoison, EventEliminate, EventShoot,
		EventProtect, EventSave, EventCheck, EventVoteTied,
	} {
		state := newState()
		mustAddTo(t, state, "p1", RoleVillager)

		// 入座已经发过初始状态（阵营与类别），比的是「有没有再动过」
		before := copyVars(state.players["p1"].Vars)

		state.applyEffect(NewEffect(typ, "src", "p1"))

		p, _ := state.getPlayer("p1")
		switch {
		case !p.Alive:
			t.Errorf("%v 不该由内核改存活状态", typ)
		case len(p.RoundVars) != 0:
			t.Errorf("%v 不该由内核写回合标记，实际 %v", typ, p.RoundVars)
		case !sameVars(p.Vars, before):
			t.Errorf("%v 不该由内核改玩家状态，入座时 %v，现在 %v", typ, before, p.Vars)
		}
	}
}

// TestApplyEffect_SaveDoesNotResurrect 解药不是复活原语。
//
// 死亡统一在夜晚结算阶段发生，SAVE 生效时目标还活着；若在这里置
// Alive=true，任何一个 SAVE 效果都能把早已出局的玩家拉回场上。
func TestApplyEffect_SaveDoesNotResurrect(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "p1", RoleVillager)
	state.players["p1"].Alive = false

	state.applyEffect(NewEffect(EventSave, "witch", "p1"))

	player, _ := state.getPlayer("p1")
	if player.Alive {
		t.Error("已出局的玩家不应被解药复活")
	}
}

func TestApplyEffect_Canceled(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "p1", RoleVillager)

	effect := NewEffect(EventKill, "wolf", "p1")
	effect.Cancel("protected")
	state.applyEffect(effect)

	player, _ := state.getPlayer("p1")
	if !player.Alive {
		t.Error("canceled effect should not kill player")
	}
}

func TestApplyEffect_InvalidTarget(t *testing.T) {
	state := newState()

	effect := NewEffect(EventKill, "wolf", "nonexistent")
	// Should not panic
	state.applyEffect(effect)
}

func TestResetRoundState(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "p1", RoleVillager)
	mustAddTo(t, state, "p2", RoleVillager)

	// 使用 NightContext 设置保护状态
	markRound(state, "p1", PlayerRoundVarProtected)
	markRound(state, "p2", PlayerRoundVarProtected)
	setKill(state, "p1")

	state.resetRoundState()

	// NightContext 应该被重置
	if protectedIn(state, "p1") {
		t.Error("expected p1 not protected after reset")
	}
	if protectedIn(state, "p2") {
		t.Error("expected p2 not protected after reset")
	}
	if killTargetOf(state) != "" {
		t.Errorf("expected empty KillTarget after reset, got %s", killTargetOf(state))
	}
}

func TestNextPhase_ToDay(t *testing.T) {
	state := newState()
	state.Phase = PhaseNight
	state.Round = 1

	state.nextPhase(PhaseDay, PhaseNightGuard)

	if state.Phase != PhaseDay {
		t.Errorf("expected Phase=DAY, got %v", state.Phase)
	}
	if state.Round != 1 {
		t.Errorf("expected Round=1, got %d", state.Round)
	}
}

func TestNextPhase_ToNightGuard_IncrementsRound(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "p1", RoleVillager)
	markRound(state, "p1", PlayerRoundVarProtected)
	setKill(state, "p1")
	state.Phase = PhaseVote
	state.Round = 1

	// 第二个参数是本局的起始阶段，绕回它即是新的一回合
	state.nextPhase(PhaseNightGuard, PhaseNightGuard)

	if state.Phase != PhaseNightGuard {
		t.Errorf("expected Phase=NIGHT_GUARD, got %v", state.Phase)
	}
	if state.Round != 2 {
		t.Errorf("expected Round=2, got %d", state.Round)
	}
	// NightContext 应该被重置
	if protectedIn(state, "p1") {
		t.Error("expected NightContext to be reset")
	}
	if killTargetOf(state) != "" {
		t.Error("expected KillTarget to be reset")
	}
}

func TestCheckVictory_AllWolvesDead(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "w1", RoleWerewolf)
	mustAddTo(t, state, "s1", RoleSeer)
	mustAddTo(t, state, "v1", RoleVillager)

	// Kill all wolves
	state.players["w1"].Alive = false

	gameOver, winner := DefaultVictoryChecker{Mode: VictoryModeSideWipe}.CheckVictory(newStateView(state))
	if !gameOver {
		t.Error("expected gameOver=true when all wolves dead")
	}
	if winner != CampGood {
		t.Errorf("expected GOOD wins, got %v", winner)
	}
}

func TestCheckVictory_GoodLessOrEqual(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "w1", RoleWerewolf)
	mustAddTo(t, state, "w2", RoleWerewolf)
	mustAddTo(t, state, "s1", RoleSeer)
	mustAddTo(t, state, "v1", RoleVillager)

	// Kill one good player, now good(1) <= evil(2)
	state.players["s1"].Alive = false

	gameOver, winner := DefaultVictoryChecker{Mode: VictoryModeSideWipe}.CheckVictory(newStateView(state))
	if !gameOver {
		t.Error("expected gameOver=true when good <= evil")
	}
	if winner != CampEvil {
		t.Errorf("expected EVIL wins, got %v", winner)
	}
}

func TestCheckVictory_GameContinues(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "w1", RoleWerewolf)
	mustAddTo(t, state, "s1", RoleSeer)
	mustAddTo(t, state, "v1", RoleVillager)
	mustAddTo(t, state, "v2", RoleVillager)

	// good(3) > evil(1), game continues
	gameOver, winner := DefaultVictoryChecker{Mode: VictoryModeSideWipe}.CheckVictory(newStateView(state))
	if gameOver {
		t.Error("expected gameOver=false when good > evil")
	}
	if winner != CampUnspecified {
		t.Errorf("expected UNSPECIFIED, got %v", winner)
	}
}

func TestCheckVictory_NoPlayers(t *testing.T) {
	state := newState()

	// No players means 0 wolves, good wins
	gameOver, winner := DefaultVictoryChecker{Mode: VictoryModeSideWipe}.CheckVictory(newStateView(state))
	if !gameOver {
		t.Error("expected gameOver=true with no players")
	}
	if winner != CampGood {
		t.Errorf("expected GOOD wins (0 wolves), got %v", winner)
	}
}

func TestCheckVictory_Equal(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "w1", RoleWerewolf)
	mustAddTo(t, state, "v1", RoleVillager)

	// 屠城模式下 good(1) == evil(1) 即狼人胜利
	gameOver, winner := DefaultVictoryChecker{Mode: VictoryModeTownWipe}.CheckVictory(newStateView(state))
	if !gameOver {
		t.Error("expected gameOver=true when good == evil")
	}
	if winner != CampEvil {
		t.Errorf("expected EVIL wins, got %v", winner)
	}
}
