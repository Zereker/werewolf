package werewolf

import (
	"testing"

	pb "github.com/Zereker/werewolf/proto"
)

func TestNewState(t *testing.T) {
	state := NewState()

	if state.Phase != pb.PhaseType_PHASE_TYPE_START {
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
	state := NewState()

	state.AddPlayer("p1", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)

	player, ok := state.getPlayer("p1")
	if !ok {
		t.Fatal("player not found after AddPlayer")
	}
	if player.ID != "p1" {
		t.Errorf("expected ID=p1, got %s", player.ID)
	}
	if player.Role != pb.RoleType_ROLE_TYPE_WEREWOLF {
		t.Errorf("expected Role=WEREWOLF, got %v", player.Role)
	}
	if player.Camp != pb.Camp_CAMP_EVIL {
		t.Errorf("expected Camp=EVIL, got %v", player.Camp)
	}
	if !player.Alive {
		t.Error("expected Alive=true")
	}
}

func TestGetPlayer_Exists(t *testing.T) {
	state := NewState()
	state.AddPlayer("p1", pb.RoleType_ROLE_TYPE_SEER, pb.Camp_CAMP_GOOD)

	player, ok := state.getPlayer("p1")
	if !ok {
		t.Error("expected ok=true for existing player")
	}
	if player == nil {
		t.Error("expected player not nil")
	}
}

func TestGetPlayer_NotExists(t *testing.T) {
	state := NewState()

	player, ok := state.getPlayer("nonexistent")
	if ok {
		t.Error("expected ok=false for non-existing player")
	}
	if player != nil {
		t.Error("expected player to be nil")
	}
}

func TestGetAlivePlayers(t *testing.T) {
	state := NewState()
	state.AddPlayer("p1", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.AddPlayer("p2", pb.RoleType_ROLE_TYPE_SEER, pb.Camp_CAMP_GOOD)
	state.AddPlayer("p3", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	// Kill p2
	state.players["p2"].Alive = false

	alive := state.getAlivePlayers()
	if len(alive) != 2 {
		t.Errorf("expected 2 alive players, got %d", len(alive))
	}

	// Verify p2 is not in the list
	for _, p := range alive {
		if p.ID == "p2" {
			t.Error("dead player p2 should not be in alive list")
		}
	}
}

func TestGetAlivePlayersByRole(t *testing.T) {
	state := NewState()
	state.AddPlayer("w1", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.AddPlayer("w2", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.AddPlayer("s1", pb.RoleType_ROLE_TYPE_SEER, pb.Camp_CAMP_GOOD)

	// Kill w1
	state.players["w1"].Alive = false

	wolves := state.getAlivePlayersByRole(pb.RoleType_ROLE_TYPE_WEREWOLF)
	if len(wolves) != 1 {
		t.Errorf("expected 1 alive werewolf, got %d", len(wolves))
	}
	if wolves[0].ID != "w2" {
		t.Errorf("expected w2, got %s", wolves[0].ID)
	}

	seers := state.getAlivePlayersByRole(pb.RoleType_ROLE_TYPE_SEER)
	if len(seers) != 1 {
		t.Errorf("expected 1 seer, got %d", len(seers))
	}
}

func TestGetAlivePlayersByCamp(t *testing.T) {
	state := NewState()
	state.AddPlayer("w1", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.AddPlayer("s1", pb.RoleType_ROLE_TYPE_SEER, pb.Camp_CAMP_GOOD)
	state.AddPlayer("v1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	// Kill s1
	state.players["s1"].Alive = false

	good := state.getAlivePlayersByCamp(pb.Camp_CAMP_GOOD)
	if len(good) != 1 {
		t.Errorf("expected 1 alive good player, got %d", len(good))
	}

	evil := state.getAlivePlayersByCamp(pb.Camp_CAMP_EVIL)
	if len(evil) != 1 {
		t.Errorf("expected 1 alive evil player, got %d", len(evil))
	}
}

func TestApplyEffect_Kill(t *testing.T) {
	state := NewState()
	state.AddPlayer("p1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	effect := NewEffect(pb.EventType_EVENT_TYPE_KILL, "wolf", "p1")
	state.ApplyEffect(effect)

	player, _ := state.getPlayer("p1")
	if player.Alive {
		t.Error("expected player to be dead after Kill effect")
	}
}

func TestApplyEffect_Poison(t *testing.T) {
	state := NewState()
	state.AddPlayer("p1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	effect := NewEffect(pb.EventType_EVENT_TYPE_POISON, "witch", "p1")
	state.ApplyEffect(effect)

	player, _ := state.getPlayer("p1")
	if player.Alive {
		t.Error("expected player to be dead after Poison effect")
	}
}

func TestApplyEffect_Eliminate(t *testing.T) {
	state := NewState()
	state.AddPlayer("p1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	effect := NewEffect(pb.EventType_EVENT_TYPE_ELIMINATE, "", "p1")
	state.ApplyEffect(effect)

	player, _ := state.getPlayer("p1")
	if player.Alive {
		t.Error("expected player to be dead after Eliminate effect")
	}
}

func TestApplyEffect_Protect(t *testing.T) {
	state := NewState()
	state.AddPlayer("p1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	effect := NewEffect(pb.EventType_EVENT_TYPE_PROTECT, "guard", "p1")
	state.ApplyEffect(effect)

	// 使用 NightContext 检查保护状态
	if !state.RoundCtx.IsProtected("p1") {
		t.Error("expected player to be protected after Protect effect")
	}
}

func TestApplyEffect_Save(t *testing.T) {
	state := NewState()
	state.AddPlayer("p1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	state.players["p1"].Alive = false

	effect := NewEffect(pb.EventType_EVENT_TYPE_SAVE, "witch", "p1")
	state.ApplyEffect(effect)

	player, _ := state.getPlayer("p1")
	if !player.Alive {
		t.Error("expected player to be alive after Save effect")
	}
}

func TestApplyEffect_Canceled(t *testing.T) {
	state := NewState()
	state.AddPlayer("p1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	effect := NewEffect(pb.EventType_EVENT_TYPE_KILL, "wolf", "p1")
	effect.Cancel("protected")
	state.ApplyEffect(effect)

	player, _ := state.getPlayer("p1")
	if !player.Alive {
		t.Error("canceled effect should not kill player")
	}
}

func TestApplyEffect_InvalidTarget(t *testing.T) {
	state := NewState()

	effect := NewEffect(pb.EventType_EVENT_TYPE_KILL, "wolf", "nonexistent")
	// Should not panic
	state.ApplyEffect(effect)
}

func TestResetRoundState(t *testing.T) {
	state := NewState()
	state.AddPlayer("p1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	state.AddPlayer("p2", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	// 使用 NightContext 设置保护状态
	state.RoundCtx.ProtectedPlayers["p1"] = true
	state.RoundCtx.ProtectedPlayers["p2"] = true
	state.RoundCtx.KillTarget = "p1"

	state.ResetRoundState()

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
	state := NewState()
	state.Phase = pb.PhaseType_PHASE_TYPE_NIGHT
	state.Round = 1

	state.NextPhase(pb.PhaseType_PHASE_TYPE_DAY)

	if state.Phase != pb.PhaseType_PHASE_TYPE_DAY {
		t.Errorf("expected Phase=DAY, got %v", state.Phase)
	}
	if state.Round != 1 {
		t.Errorf("expected Round=1, got %d", state.Round)
	}
}

func TestNextPhase_ToNightGuard_IncrementsRound(t *testing.T) {
	state := NewState()
	state.AddPlayer("p1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	state.RoundCtx.ProtectedPlayers["p1"] = true
	state.RoundCtx.KillTarget = "p1"
	state.Phase = pb.PhaseType_PHASE_TYPE_VOTE
	state.Round = 1

	state.NextPhase(pb.PhaseType_PHASE_TYPE_NIGHT_GUARD)

	if state.Phase != pb.PhaseType_PHASE_TYPE_NIGHT_GUARD {
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
	state := NewState()
	state.AddPlayer("w1", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.AddPlayer("s1", pb.RoleType_ROLE_TYPE_SEER, pb.Camp_CAMP_GOOD)
	state.AddPlayer("v1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	// Kill all wolves
	state.players["w1"].Alive = false

	gameOver, winner := state.CheckVictory()
	if !gameOver {
		t.Error("expected gameOver=true when all wolves dead")
	}
	if winner != pb.Camp_CAMP_GOOD {
		t.Errorf("expected GOOD wins, got %v", winner)
	}
}

func TestCheckVictory_GoodLessOrEqual(t *testing.T) {
	state := NewState()
	state.AddPlayer("w1", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.AddPlayer("w2", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.AddPlayer("s1", pb.RoleType_ROLE_TYPE_SEER, pb.Camp_CAMP_GOOD)
	state.AddPlayer("v1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	// Kill one good player, now good(1) <= evil(2)
	state.players["s1"].Alive = false

	gameOver, winner := state.CheckVictory()
	if !gameOver {
		t.Error("expected gameOver=true when good <= evil")
	}
	if winner != pb.Camp_CAMP_EVIL {
		t.Errorf("expected EVIL wins, got %v", winner)
	}
}

func TestCheckVictory_GameContinues(t *testing.T) {
	state := NewState()
	state.AddPlayer("w1", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.AddPlayer("s1", pb.RoleType_ROLE_TYPE_SEER, pb.Camp_CAMP_GOOD)
	state.AddPlayer("v1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	state.AddPlayer("v2", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	// good(3) > evil(1), game continues
	gameOver, winner := state.CheckVictory()
	if gameOver {
		t.Error("expected gameOver=false when good > evil")
	}
	if winner != pb.Camp_CAMP_UNSPECIFIED {
		t.Errorf("expected UNSPECIFIED, got %v", winner)
	}
}

func TestCheckVictory_NoPlayers(t *testing.T) {
	state := NewState()

	// No players means 0 wolves, good wins
	gameOver, winner := state.CheckVictory()
	if !gameOver {
		t.Error("expected gameOver=true with no players")
	}
	if winner != pb.Camp_CAMP_GOOD {
		t.Errorf("expected GOOD wins (0 wolves), got %v", winner)
	}
}

func TestCheckVictory_Equal(t *testing.T) {
	state := NewState()
	state.AddPlayer("w1", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	state.AddPlayer("v1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	// good(1) == evil(1), evil wins
	gameOver, winner := state.CheckVictory()
	if !gameOver {
		t.Error("expected gameOver=true when good == evil")
	}
	if winner != pb.Camp_CAMP_EVIL {
		t.Errorf("expected EVIL wins, got %v", winner)
	}
}
