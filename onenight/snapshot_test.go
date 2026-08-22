package onenight

import (
	"testing"

	"github.com/Zereker/werewolf/engine"
)

// TestSnapshot_RoundTrip 存档往返之后局面一致，接着打完结局相同。
//
// 这一套规则的状态几乎全在「整局有效」那两格里（每个人手上的牌、中央三张、
// 谁看到了什么）。快照要是漏了它们，恢复出来的对局会从头错到尾。
func TestSnapshot_RoundTrip(t *testing.T) {
	g := newGame(t,
		[CenterCount]engine.RoleType{RoleWerewolf, RoleVillager, RoleTanner},
		at("s", RoleSeer), at("r", RoleRobber),
		at("w", RoleWerewolf), at("v", RoleVillager))

	g.advance(PhaseNightSeer)
	g.use("s", SkillSeerCenter02)
	g.advance(PhaseNightRobber)
	g.use("r", SkillRob, "w")
	g.advance(PhaseDay)

	snap := g.e.Snapshot()
	restored, err := engine.RestoreEngine(GameConfig(), snap,
		Options([CenterCount]engine.RoleType{RoleWerewolf, RoleVillager, RoleTanner})...)
	if err != nil {
		t.Fatalf("RestoreEngine: %v", err)
	}

	// 手上的牌
	for _, id := range []string{"s", "r", "w", "v"} {
		want := card(g.e.View(), id)
		if got := card(restored.View(), id); got != want {
			t.Errorf("%s 手上的牌恢复错了：%v，期望 %v", id, got, want)
		}
	}
	// 中央三张
	for i := 0; i < CenterCount; i++ {
		want := centerCard(g.e.View(), i)
		if got := centerCard(restored.View(), i); got != want {
			t.Errorf("中央第 %d 张恢复错了：%v，期望 %v", i, got, want)
		}
	}
	// 谁看到了什么
	if got := restored.PlayerView("s").RoleInfo["learn.center.0"]; got != string(RoleWerewolf) {
		t.Errorf("预言家看到的东西没随快照走，读到 %q", got)
	}
	if got := restored.PlayerView("r").RoleInfo["learn.self"]; got != string(RoleWerewolf) {
		t.Errorf("抢劫者看到的东西没随快照走，读到 %q", got)
	}

	// 接着打完，两边结局相同。
	finish := func(e *engine.Engine) engine.Camp {
		t.Helper()
		for _, id := range []string{"s", "r", "w", "v"} {
			target := "r"
			if id == "r" {
				target = "w"
			}
			if err := e.SubmitSkillUse(&engine.SkillUse{
				PlayerID: id, Skill: SkillVote, Targets: []string{target},
			}); err != nil {
				t.Fatalf("%s 投票: %v", id, err)
			}
		}
		if _, err := e.EndPhase(); err != nil {
			t.Fatalf("EndPhase: %v", err)
		}
		return e.Status().Winner
	}

	if err := func() error { _, err := g.e.EndPhase(); return err }(); err != nil {
		t.Fatalf("EndPhase: %v", err)
	}
	if _, err := restored.EndPhase(); err != nil {
		t.Fatalf("EndPhase: %v", err)
	}
	if a, b := finish(g.e), finish(restored); a != b {
		t.Errorf("恢复出来的对局结局不同：%v vs %v", a, b)
	}
}

// TestReplay_RebuildsGame 效果流回放出同一个局面。
func TestReplay_RebuildsGame(t *testing.T) {
	center := [CenterCount]engine.RoleType{RoleVillager, RoleWerewolf, RoleVillager}
	g := newGame(t, center,
		at("t", RoleTroublemaker), at("w", RoleWerewolf),
		at("d", RoleDrunk), at("v", RoleVillager))

	g.advance(PhaseNightTroublemake)
	g.use("t", SkillMeddle, "w", "v")
	g.advance(PhaseNightDrunk)
	g.use("d", SkillDrinkCenter1)
	g.advance(PhaseDay)

	replayed, err := engine.ReplayEngine(GameConfig(), g.e.EffectLog(), Options(center)...)
	if err != nil {
		t.Fatalf("ReplayEngine: %v", err)
	}

	if got, want := replayed.Status().Phase, g.e.Status().Phase; got != want {
		t.Errorf("回放后阶段 = %v，期望 %v", got, want)
	}
	for _, id := range []string{"t", "w", "d", "v"} {
		want := card(g.e.View(), id)
		if got := card(replayed.View(), id); got != want {
			t.Errorf("%s 手上的牌回放错了：%v，期望 %v", id, got, want)
		}
	}
	for i := 0; i < CenterCount; i++ {
		want := centerCard(g.e.View(), i)
		if got := centerCard(replayed.View(), i); got != want {
			t.Errorf("中央第 %d 张回放错了：%v，期望 %v", i, got, want)
		}
	}
}

// TestConfig_IsValid 阶段图自洽。
func TestConfig_IsValid(t *testing.T) {
	if err := GameConfig().Validate(); err != nil {
		t.Fatalf("阶段图不合法: %v", err)
	}
}

// TestRoundNeverAdvances 这一套规则整局只有一个回合。
//
// 前两套的阶段图都是环，回合数会一路涨。这一套是直线，Round 从头到尾是 1
// ——而这份配置一个回合边界都没声明，正因为它不需要（SCARS.md 疤 2）。
func TestRoundNeverAdvances(t *testing.T) {
	g := newGame(t,
		[CenterCount]engine.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("w", RoleWerewolf), at("v1", RoleVillager), at("v2", RoleVillager))

	for _, phase := range []engine.PhaseType{
		PhaseNightMinion, PhaseNightMason, PhaseNightSeer, PhaseNightRobber,
		PhaseNightTroublemake, PhaseNightDrunk, PhaseNightInsomniac,
		PhaseDay, PhaseVote,
	} {
		g.advance(phase)
		if got := g.e.Status().Round; got != 1 {
			t.Fatalf("走到 %v 时回合数 = %d，这一套规则整局只有一个回合", phase, got)
		}
	}
}
