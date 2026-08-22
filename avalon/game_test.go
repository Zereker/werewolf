package avalon

import (
	"testing"

	"github.com/Zereker/werewolf/engine"
)

// 5 人局：梅林、派西维尔、忠臣 / 刺客、莫甘娜
func fivePlayer(t *testing.T) *engine.Engine {
	t.Helper()
	e := MustNew()
	for id, role := range map[string]engine.RoleType{
		"a": RoleMerlin, "b": RolePercival, "c": RoleLoyalServant,
		"d": RoleAssassin, "e": RoleMorgana,
	} {
		if err := e.AddPlayer(id, role); err != nil {
			t.Fatalf("AddPlayer(%s): %v", id, err)
		}
	}
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return e
}

// TestFirstMission 走通一轮：提名 -> 表决通过 -> 任务成功
func TestFirstMission(t *testing.T) {
	e := fivePlayer(t)

	if got := e.Phase(); got != PhasePropose {
		t.Fatalf("开局阶段 = %v，期望 PROPOSE", got)
	}
	if n := MissionSize(5, 1); n != 2 {
		t.Fatalf("5 人局第一轮该 2 人，表里是 %d", n)
	}

	// 队长（座位 0 = "a"）提名两人
	for _, target := range []string{"a", "b"} {
		if err := e.SubmitSkillUse(&engine.SkillUse{
			PlayerID: "a", Skill: SkillPropose, TargetID: target,
		}); err != nil {
			t.Fatalf("提名 %s: %v", target, err)
		}
	}
	if _, err := e.EndPhase(); err != nil {
		t.Fatalf("EndPhase(PROPOSE): %v", err)
	}
	if got := e.Phase(); got != PhaseTeamVote {
		t.Fatalf("阶段 = %v，期望 TEAM_VOTE", got)
	}

	// 全员赞成
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		if err := e.SubmitSkillUse(&engine.SkillUse{PlayerID: id, Skill: SkillApprove}); err != nil {
			t.Fatalf("表决 %s: %v", id, err)
		}
	}
	if _, err := e.EndPhase(); err != nil {
		t.Fatalf("EndPhase(TEAM_VOTE): %v", err)
	}
	if got := e.Phase(); got != PhaseMission {
		t.Fatalf("阶段 = %v，期望 MISSION", got)
	}

	// 队员投成功
	for _, id := range []string{"a", "b"} {
		if err := e.SubmitSkillUse(&engine.SkillUse{PlayerID: id, Skill: SkillMissionSuccess}); err != nil {
			t.Fatalf("任务票 %s: %v", id, err)
		}
	}
	effects, err := e.EndPhase()
	if err != nil {
		t.Fatalf("EndPhase(MISSION): %v", err)
	}

	var succeeded bool
	for _, ef := range effects {
		if ef.Type == EventMissionSucceeded {
			succeeded = true
		}
	}
	if !succeeded {
		t.Fatalf("第一轮任务该成功，效果里没有 MISSION_SUCCEEDED：%v", typesOf(effects))
	}
	if e.IsGameOver() {
		t.Fatal("才赢一轮就结束了")
	}
}

// TestMerlinSeesEvilExceptMordred 梅林认得每一个坏人，除了莫德雷德
func TestMerlinSeesEvilExceptMordred(t *testing.T) {
	e := MustNew()
	for id, role := range map[string]engine.RoleType{
		"a": RoleMerlin, "b": RolePercival, "c": RoleLoyalServant, "d": RoleLoyalServant,
		"e": RoleAssassin, "f": RoleMordred, "g": RoleOberon,
	} {
		if err := e.AddPlayer(id, role); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.Start(); err != nil {
		t.Fatal(err)
	}

	v := e.PlayerView("a")
	got := v.RoleInfo[RoleInfoMerlinEvil]
	// 刺客 e、奥伯伦 g 看得见；莫德雷德 f 看不见
	if want := "e,g"; got != want {
		t.Errorf("梅林看到的坏人 = %q，期望 %q（莫德雷德不该在里面，奥伯伦该在）", got, want)
	}
}

// TestOberonIsAloneOnBothSides 奥伯伦既不认识同伙，也不被同伙认识
func TestOberonIsAloneOnBothSides(t *testing.T) {
	e := MustNew()
	for id, role := range map[string]engine.RoleType{
		"a": RoleMerlin, "b": RoleLoyalServant, "c": RoleLoyalServant, "d": RoleLoyalServant,
		"e": RoleAssassin, "f": RoleMorgana, "g": RoleOberon,
	} {
		if err := e.AddPlayer(id, role); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.Start(); err != nil {
		t.Fatal(err)
	}

	if mates := e.Teammates("g"); len(mates) != 0 {
		t.Errorf("奥伯伦不该认识任何同伙，实际 %v", mates)
	}
	for _, id := range []string{"e", "f"} {
		for _, m := range e.Teammates(id) {
			if m == "g" {
				t.Errorf("%s 不该认识奥伯伦，实际同伙 %v", id, e.Teammates(id))
			}
		}
	}
	// 刺客与莫甘娜互相认得
	if mates := e.Teammates("e"); len(mates) != 1 || mates[0] != "f" {
		t.Errorf("刺客的同伙 = %v，期望 [f]", mates)
	}
}

// TestPercivalCannotTellMerlinFromMorgana 派西维尔看到两个人，分不清谁是谁
func TestPercivalCannotTellMerlinFromMorgana(t *testing.T) {
	e := fivePlayer(t) // a=梅林 e=莫甘娜
	v := e.PlayerView("b")
	got := v.RoleInfo[RoleInfoPercivalCandidate]
	if want := "a,e"; got != want {
		t.Errorf("派西维尔看到 %q，期望 %q", got, want)
	}
	// 「分不清」的实现就是这一个字符串里没有任何区分标记
	if len(v.RoleInfo) != 1 {
		t.Errorf("派西维尔不该拿到别的信息：%v", v.RoleInfo)
	}
}

func typesOf(effects []*engine.Effect) []engine.EventType {
	out := make([]engine.EventType, 0, len(effects))
	for _, ef := range effects {
		out = append(out, ef.Type)
	}
	return out
}
