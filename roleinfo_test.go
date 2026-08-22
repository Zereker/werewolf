package werewolf

import (
	"testing"

	"github.com/Zereker/werewolf/engine"
)

const (
	roleThief         = RoleType("THIEF")
	roleInfoSpareCard = "spare_card"
)

// TestRoleInfo_ThirdPartyRoleCanShowItsOwnInfo 第三方角色能给自己的玩家发专属信息。
//
// 此前上帝视角与玩家视角里，「谁额外看得到什么」是引擎里一个认得所有
// 内置角色的 switch：狼人给队友、女巫给刀口，别的角色什么都没有。
// 加一个盗贼（要看两张底牌）就得改引擎——而加一个角色不该要求改引擎。
func TestRoleInfo_ThirdPartyRoleCanShowItsOwnInfo(t *testing.T) {
	provider := engine.RoleInfoFunc(func(playerID string, view GameView) map[string]string {
		return map[string]string{roleInfoSpareCard: "SEER"}
	})

	e := MustNew(DefaultRules(),
		engine.WithRoleInfo(roleThief, provider),
		engine.WithRoleSetup(roleThief, sideSetup(CampGood, RoleCategoryGod)))
	mustAdd(t, e, "w1", RoleWerewolf)
	if err := e.AddPlayer("th", roleThief); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, e, "v1", RoleVillager)
	mustAdd(t, e, "v2", RoleVillager)
	if err := e.Start(); err != nil {
		t.Fatal(err)
	}

	// 玩家视角
	if got := e.PlayerView("th").RoleInfo[roleInfoSpareCard]; got != "SEER" {
		t.Errorf("盗贼应当看到自己的底牌，实际 %q", got)
	}
	// 别人看不到
	if got := e.PlayerView("v1").RoleInfo[roleInfoSpareCard]; got != "" {
		t.Errorf("平民不该看到盗贼的底牌，实际 %q", got)
	}
}

// TestRoleInfo_CustomWolfGetsTeammatesInPhaseInfo 自定义狼队角色在上帝视角里也该有队友。
//
// 队友此前按**角色**判（case RoleWerewolf），于是自定义的狼队角色
// 狼王在 PhaseInfo 这一份名单里拿不到队友——而 PlayerView 与 WolfTeammates
// 那两条路都是对的，只有主持人照着组织流程的这一份漏了。
func TestRoleInfo_CustomWolfGetsTeammatesInPhaseInfo(t *testing.T) {
	const roleWolfKing2 = RoleType("WOLF_KING_2")

	cfg := DefaultGameConfig()
	// 让狼王和狼人在同一个阶段行动，才会出现在 PhaseInfo 里
	wolfPhase := cfg.Phases[PhaseNightWolf]
	wolfPhase.Steps = append(wolfPhase.Steps,
		PhaseStep{Role: roleWolfKing2, Skill: SkillKill})

	e := MustNewWith(cfg, DefaultRules(), engine.WithRoleSetup(roleWolfKing2, sideSetup(CampEvil, RoleCategoryWolf)))
	mustAdd(t, e, "w1", RoleWerewolf)
	if err := e.AddPlayer("wk", roleWolfKing2); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, e, "s", RoleSeer)
	mustAdd(t, e, "v1", RoleVillager)
	mustAdd(t, e, "v2", RoleVillager)
	if err := e.Start(); err != nil {
		t.Fatal(err)
	}
	for e.Status().Phase != PhaseNightWolf {
		if _, err := e.EndPhase(); err != nil {
			t.Fatal(err)
		}
	}

	ri, ok := e.PhaseInfo().RoleInfos[roleWolfKing2]
	if !ok {
		t.Fatal("狼王应当出现在本阶段的角色里")
	}
	if mates := ri.Teammates["wk"]; len(mates) != 1 || mates[0] != "w1" {
		t.Errorf("狼王在上帝视角里也该看到队友，实际 %v", ri.Teammates)
	}
}

// TestRoleInfo_WitchIsJustAnotherProvider 内置女巫走的是同一条路，没有特权。
func TestRoleInfo_WitchIsJustAnotherProvider(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), witch("wi"), seer("s"),
		villagers("v1", "v2", "v3", "v4"),
	)...)

	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "v1")
	g.end(PhaseNightWitch)

	if got := g.e.PlayerView("wi").RoleInfo[RoleInfoKillTarget]; got != "v1" {
		t.Errorf("女巫应当看到刀口，实际 %q", got)
	}
	if got := g.e.PlayerView("s").RoleInfo[RoleInfoKillTarget]; got != "" {
		t.Errorf("预言家不该看到刀口，实际 %q", got)
	}

	// 换掉内置女巫的提供者：内置的没有特权，能被覆盖
	e2 := MustNew(DefaultRules(), engine.WithRoleInfo(RoleWitch,
		engine.RoleInfoFunc(func(string, GameView) map[string]string {
			return map[string]string{"custom": "yes"}
		})))
	mustAdd(t, e2, "w1", RoleWerewolf)
	mustAdd(t, e2, "wi", RoleWitch)
	mustAdd(t, e2, "v1", RoleVillager)
	if err := e2.Start(); err != nil {
		t.Fatal(err)
	}
	info := e2.PlayerView("wi").RoleInfo
	if info["custom"] != "yes" {
		t.Errorf("内置提供者应当能被换掉，实际 %v", info)
	}
	if _, ok := info[RoleInfoKillTarget]; ok {
		t.Error("换掉之后不该还带着内置的那一项")
	}
}

// TestWithRoleInfo_RejectsNil 传 nil 只可能是漏了。
func TestWithRoleInfo_RejectsNil(t *testing.T) {
	if _, err := engine.NewEngine(nil, engine.WithRoleInfo(RoleWitch, nil)); err == nil {
		t.Error("nil 提供者应当被拒绝")
	}
}

// TestRoleInfo_PerWitchNotAnyWitch 多女巫板子上，刀口按人判而不是按「场上还有谁持有解药」。
//
// 旧实现用的是 anyAliveWitchHasAntidote：只要场上还有一名女巫持有解药，
// 所有女巫都能看到刀口——一个已经用掉解药的女巫因此仍然看得见。
// 换成 RoleInfoProvider 之后是按人判的。
func TestRoleInfo_PerWitchNotAnyWitch(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), witch("wa"), witch("wb"),
		villagers("v1", "v2", "v3", "v4"),
	)...)

	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "v1")
	g.end(PhaseNightWitch)

	// 两个女巫都还有解药，都看得到
	for _, id := range []string{"wa", "wb"} {
		if got := g.e.PlayerView(id).RoleInfo[RoleInfoKillTarget]; got != "v1" {
			t.Fatalf("%s 应当看到刀口，实际 %q", id, got)
		}
	}

	// wa 用掉解药
	g.mustUse("wa", SkillAntidote, "v1")
	g.endAny()
	g.end(PhaseNightResolve)
	g.end(PhaseDay)
	g.toNextNight()
	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "v2")
	g.end(PhaseNightWitch)

	if got := g.e.PlayerView("wa").RoleInfo[RoleInfoKillTarget]; got != "" {
		t.Errorf("解药已用完的女巫不该再看到刀口，实际 %q", got)
	}
	if got := g.e.PlayerView("wb").RoleInfo[RoleInfoKillTarget]; got != "v2" {
		t.Errorf("解药还在的女巫应当看到刀口，实际 %q", got)
	}
}
