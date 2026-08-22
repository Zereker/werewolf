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
	if player.Camp != CampEvil {
		t.Errorf("expected Camp=EVIL, got %v", player.Camp)
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

func TestApplyEffect_Kill(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "p1", RoleVillager)

	effect := NewEffect(EventKill, "wolf", "p1")
	state.applyEffect(effect)

	player, _ := state.getPlayer("p1")
	if player.Alive {
		t.Error("expected player to be dead after Kill effect")
	}
}

func TestApplyEffect_Poison(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "p1", RoleVillager)

	effect := NewEffect(EventPoison, "witch", "p1")
	state.applyEffect(effect)

	player, _ := state.getPlayer("p1")
	if player.Alive {
		t.Error("expected player to be dead after Poison effect")
	}
}

func TestApplyEffect_Eliminate(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "p1", RoleVillager)

	effect := NewEffect(EventEliminate, "", "p1")
	state.applyEffect(effect)

	player, _ := state.getPlayer("p1")
	if player.Alive {
		t.Error("expected player to be dead after Eliminate effect")
	}
}

func TestApplyEffect_Protect(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "p1", RoleVillager)

	effect := NewEffect(EventProtect, "guard", "p1")
	state.applyEffect(effect)

	// 使用 NightContext 检查保护状态
	if !state.RoundCtx.IsProtected("p1") {
		t.Error("expected player to be protected after Protect effect")
	}
}

func TestApplyEffect_Save(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "p1", RoleVillager)

	state.applyEffect(NewEffect(EventSave, "witch", "p1"))

	// 解药只记录「被救过」，生死由夜晚结算阶段综合守护与解药判定
	if !state.RoundCtx.IsSaved("p1") {
		t.Error("expected p1 to be marked as saved")
	}
	player, _ := state.getPlayer("p1")
	if !player.Alive {
		t.Error("expected p1 to still be alive")
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
	state.RoundCtx.ProtectedPlayers["p1"] = true
	state.RoundCtx.ProtectedPlayers["p2"] = true
	state.RoundCtx.KillTarget = "p1"

	state.resetRoundState()

	// NightContext 应该被重置
	if state.RoundCtx.IsProtected("p1") {
		t.Error("expected p1 not protected after reset")
	}
	if state.RoundCtx.IsProtected("p2") {
		t.Error("expected p2 not protected after reset")
	}
	if state.RoundCtx.KillTarget != "" {
		t.Errorf("expected empty KillTarget after reset, got %s", state.RoundCtx.KillTarget)
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
	state.RoundCtx.ProtectedPlayers["p1"] = true
	state.RoundCtx.KillTarget = "p1"
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
	if state.RoundCtx.IsProtected("p1") {
		t.Error("expected NightContext to be reset")
	}
	if state.RoundCtx.KillTarget != "" {
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

	gameOver, winner := state.checkVictory(VictoryModeSideWipe)
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

	gameOver, winner := state.checkVictory(VictoryModeSideWipe)
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
	gameOver, winner := state.checkVictory(VictoryModeSideWipe)
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
	gameOver, winner := state.checkVictory(VictoryModeSideWipe)
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
	gameOver, winner := state.checkVictory(VictoryModeTownWipe)
	if !gameOver {
		t.Error("expected gameOver=true when good == evil")
	}
	if winner != CampEvil {
		t.Errorf("expected EVIL wins, got %v", winner)
	}
}
