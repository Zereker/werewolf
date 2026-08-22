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

	if got := e.Status().Phase; got != PhasePropose {
		t.Fatalf("开局阶段 = %v，期望 PROPOSE", got)
	}
	if n := MissionSize(5, 1); n != 2 {
		t.Fatalf("5 人局第一轮该 2 人，表里是 %d", n)
	}

	// 队长（座位 0 = "a"）提名两人
	for _, target := range []string{"a", "b"} {
		if err := e.SubmitSkillUse(&engine.SkillUse{
			PlayerID: "a", Skill: SkillPropose, Targets: []string{target},
		}); err != nil {
			t.Fatalf("提名 %s: %v", target, err)
		}
	}
	if _, err := e.EndPhase(); err != nil {
		t.Fatalf("EndPhase(PROPOSE): %v", err)
	}
	if got := e.Status().Phase; got != PhaseTeamVote {
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
	if got := e.Status().Phase; got != PhaseMission {
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
	if e.Status().Over {
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

// runMission 打完一整轮任务：提名 -> 全票通过 -> 队员按 fails 投失败票
//
// members 里前 fails 个人投失败，其余投成功。
func runMission(t *testing.T, e *engine.Engine, fails int, members ...string) []*engine.Effect {
	t.Helper()
	leader := leaderID(e.View())
	mustSubmit(t, e, &engine.SkillUse{PlayerID: leader, Skill: SkillPropose, Targets: members})
	mustEnd(t, e)

	for _, id := range e.AlivePlayerIDs() {
		mustSubmit(t, e, &engine.SkillUse{PlayerID: id, Skill: SkillApprove})
	}
	mustEnd(t, e)

	for i, id := range members {
		skill := SkillMissionSuccess
		if i < fails {
			skill = SkillMissionFail
		}
		mustSubmit(t, e, &engine.SkillUse{PlayerID: id, Skill: skill})
	}
	return mustEnd(t, e)
}

// TestFullGame_GoodWinsThreeThenSurvivesAssassination
// 好人连赢三轮，刺客指错人，好人获胜。
func TestFullGame_GoodWinsThreeThenSurvivesAssassination(t *testing.T) {
	e := fivePlayer(t) // a=梅林 b=派西维尔 c=忠臣 d=刺客 e=莫甘娜

	// 5 人局任务人数：2,3,2,3,3。三轮全成功，队伍里只放好人。
	runMission(t, e, 0, "a", "b")
	runMission(t, e, 0, "a", "b", "c")
	last := runMission(t, e, 0, "a", "b")

	t.Logf("三轮成功之后：阶段=%v 结束=%v 效果=%v", e.Status().Phase, e.Status().Over, typesOf(last))

	if e.Status().Over {
		t.Fatal("刺杀还没进行，这局不该结束——胜负判定必须推迟到刺杀之后")
	}
	if e.Status().Phase != PhaseAssassin {
		t.Fatalf("阶段 = %v，期望被触发队列带到 ASSASSIN", e.Status().Phase)
	}

	// 刺客指错人（指了派西维尔，梅林是 a）
	mustSubmit(t, e, &engine.SkillUse{PlayerID: "d", Skill: SkillAssassinate, Targets: []string{"b"}})
	mustEnd(t, e)

	if !e.Status().Over {
		t.Fatal("刺杀结束之后这局该结束了")
	}
	if got := e.Status().Winner; got != CampGood {
		t.Errorf("赢家 = %v，期望 GOOD（刺客指错了）", got)
	}
}

// TestFullGame_AssassinFindsMerlin 好人连赢三轮，但刺客指中梅林，坏人反败为胜。
func TestFullGame_AssassinFindsMerlin(t *testing.T) {
	e := fivePlayer(t)

	runMission(t, e, 0, "a", "b")
	runMission(t, e, 0, "a", "b", "c")
	runMission(t, e, 0, "a", "b")

	if e.Status().Phase != PhaseAssassin {
		t.Fatalf("阶段 = %v，期望 ASSASSIN", e.Status().Phase)
	}
	mustSubmit(t, e, &engine.SkillUse{PlayerID: "d", Skill: SkillAssassinate, Targets: []string{"a"}})
	mustEnd(t, e)

	if !e.Status().Over {
		t.Fatal("刺杀之后该结束")
	}
	if got := e.Status().Winner; got != CampEvil {
		t.Errorf("赢家 = %v，期望 EVIL（刺中梅林反败为胜）", got)
	}
}

// TestFullGame_EvilWinsThreeMissions 坏人破坏三轮任务直接获胜，不经过刺杀。
func TestFullGame_EvilWinsThreeMissions(t *testing.T) {
	e := fivePlayer(t) // d=刺客 e=莫甘娜 都是坏人

	runMission(t, e, 1, "d", "e")
	runMission(t, e, 1, "d", "e", "a")
	runMission(t, e, 1, "d", "e")

	if !e.Status().Over {
		t.Fatal("三轮失败之后该结束")
	}
	if got := e.Status().Winner; got != CampEvil {
		t.Errorf("赢家 = %v，期望 EVIL", got)
	}
}

// TestHammer_FiveRejectionsEndTheGame 连续五次组队被否决，坏人直接获胜。
func TestHammer_FiveRejectionsEndTheGame(t *testing.T) {
	e := fivePlayer(t)

	for i := 1; i <= HammerRejections; i++ {
		leader := leaderID(e.View())
		mustSubmit(t, e, &engine.SkillUse{PlayerID: leader, Skill: SkillPropose, Targets: []string{"a", "b"}})
		mustEnd(t, e)
		for _, id := range e.AlivePlayerIDs() {
			mustSubmit(t, e, &engine.SkillUse{PlayerID: id, Skill: SkillReject})
		}
		mustEnd(t, e)
		if i < HammerRejections {
			if e.Status().Over {
				t.Fatalf("才否决 %d 次就结束了，应当到 %d 次", i, HammerRejections)
			}
			if got := e.Status().Phase; got != PhasePropose {
				t.Fatalf("被否决之后该直接回 PROPOSE，实际 %v", got)
			}
		}
	}

	if !e.Status().Over {
		t.Fatalf("连续 %d 次否决之后该结束", HammerRejections)
	}
	if got := e.Status().Winner; got != CampEvil {
		t.Errorf("赢家 = %v，期望 EVIL", got)
	}
}
