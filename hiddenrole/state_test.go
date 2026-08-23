package hiddenrole

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

	mustAddTo(t, state, "p1", roleWerewolf)

	player, ok := state.getPlayer("p1")
	if !ok {
		t.Fatal("player not found after AddPlayer")
	}
	if player.ID != "p1" {
		t.Errorf("expected ID=p1, got %s", player.ID)
	}
	if player.Role != roleWerewolf {
		t.Errorf("expected Role=WEREWOLF, got %v", player.Role)
	}
	// 内核入座只记 ID、角色、存活。阵营、道具这些初始状态由规则的
	// RoleSetup 发放（见 Engine.AddPlayer），裸 gameState 不经过那一步。
	if len(player.Vars) != 0 {
		t.Errorf("内核入座不该发放任何状态，实际 %v", player.Vars)
	}
	if !player.Alive {
		t.Error("expected Alive=true")
	}
}

func TestGetPlayer_Exists(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "p1", roleSeer)

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
		mustAddTo(t, state, "p1", roleVillager)

		state.applyEffect(NewSetAliveEffect("p1", false))
		if p, _ := state.getPlayer("p1"); p.Alive {
			t.Error("SET_ALIVE(false) 之后应当已出局")
		}
		state.applyEffect(NewSetAliveEffect("p1", true))
		if p, _ := state.getPlayer("p1"); !p.Alive {
			t.Error("SET_ALIVE(true) 之后应当复活")
		}
	})

	t.Run("SET_VAR 标记本回合", func(t *testing.T) {
		state := newState()
		mustAddTo(t, state, "p1", roleVillager)

		state.applyEffect(NewSetVarEffect(ScopeRound.Of("p1"), testMarkA, VarPresent))
		if !markedInA(state, "p1") {
			t.Error("标记之后应当读得到")
		}
		state.applyEffect(NewSetVarEffect(ScopeRound.Of("p1"), testMarkA, ""))
		if markedInA(state, "p1") {
			t.Error("空值应当等同删除")
		}
	})

	t.Run("回合边界清掉标记", func(t *testing.T) {
		state := newState()
		mustAddTo(t, state, "p1", roleVillager)

		state.applyEffect(NewSetVarEffect(ScopeRound.Of("p1"), testMarkB, VarPresent))
		setRoundVar(state, testKillTarget, "p1")
		state.resetRoundState()

		if markedInB(state, "p1") {
			t.Error("玩家身上的回合标记应当随回合清掉")
		}
		if got := killTargetOfState(state); got != "" {
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
		eventKill, eventPoison, eventEliminate, eventShoot,
		eventProtect, eventSave, eventCheck, eventVoteTied,
	} {
		state := newState()
		mustAddTo(t, state, "p1", roleVillager)

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
	mustAddTo(t, state, "p1", roleVillager)
	state.players["p1"].Alive = false

	state.applyEffect(NewEffect(eventSave, "witch", "p1"))

	player, _ := state.getPlayer("p1")
	if player.Alive {
		t.Error("已出局的玩家不应被解药复活")
	}
}

func TestApplyEffect_Canceled(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "p1", roleVillager)

	effect := NewEffect(eventKill, "wolf", "p1")
	effect.Cancel("protected")
	state.applyEffect(effect)

	player, _ := state.getPlayer("p1")
	if !player.Alive {
		t.Error("canceled effect should not kill player")
	}
}

func TestApplyEffect_InvalidTarget(t *testing.T) {
	state := newState()

	effect := NewEffect(eventKill, "wolf", "nonexistent")
	// Should not panic
	state.applyEffect(effect)
}

func TestResetRoundState(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "p1", roleVillager)
	mustAddTo(t, state, "p2", roleVillager)

	// 使用 NightContext 设置保护状态
	markRound(state, "p1", testMarkA)
	markRound(state, "p2", testMarkA)
	setRoundVar(state, testKillTarget, "p1")

	state.resetRoundState()

	// NightContext 应该被重置
	if markedInA(state, "p1") {
		t.Error("expected p1 not protected after reset")
	}
	if markedInA(state, "p2") {
		t.Error("expected p2 not protected after reset")
	}
	if killTargetOfState(state) != "" {
		t.Errorf("expected empty KillTarget after reset, got %s", killTargetOfState(state))
	}
}

func TestNextPhase_ToDay(t *testing.T) {
	state := newState()
	state.Phase = phaseNight
	state.Round = 1

	state.nextPhase(phaseDay, false, false) // 上一个阶段两样都没声明

	if state.Phase != phaseDay {
		t.Errorf("expected Phase=DAY, got %v", state.Phase)
	}
	if state.Round != 1 {
		t.Errorf("expected Round=1, got %d", state.Round)
	}
}

func TestNextPhase_ToNightGuard_IncrementsRound(t *testing.T) {
	state := newState()
	mustAddTo(t, state, "p1", roleVillager)
	markRound(state, "p1", testMarkA)
	setRoundVar(state, testKillTarget, "p1")
	state.Phase = phaseVote
	state.Round = 1

	// 第二个参数是「刚结算完的那个阶段是不是这一回合的终点」，
	// 由 PhaseConfig.EndsRound 声明——内核不再从阶段环里猜
	state.nextPhase(phaseNightGuard, true, true)

	if state.Phase != phaseNightGuard {
		t.Errorf("expected Phase=NIGHT_GUARD, got %v", state.Phase)
	}
	if state.Round != 2 {
		t.Errorf("expected Round=2, got %d", state.Round)
	}
	// NightContext 应该被重置
	if markedInA(state, "p1") {
		t.Error("expected NightContext to be reset")
	}
	if killTargetOfState(state) != "" {
		t.Error("expected KillTarget to be reset")
	}
}
