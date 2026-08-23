package missions

import (
	"encoding/json"
	"testing"

	"github.com/Zereker/hiddenrole"
)

// TestMissionFailIsAnonymous 任务失败票不记名：全场只知道有几张，不知道是谁投的。
//
// 这是这套规则比狼人杀更严的一处信息约束——狼人杀里被否决的行动至少行动者
// 自己知道，而这里连「结果」都只能露出聚合值。实现方式不是靠内核帮忙，
// 而是**根本不为每一票产出事件**：解析器只产出一条带票数的聚合事件。
func TestMissionFailIsAnonymous(t *testing.T) {
	e := fivePlayer(t) // d=刺客 e=莫甘娜
	effects := runMission(t, e, 1, "d", "e")

	var failed *hiddenrole.Effect
	for _, ef := range effects {
		if ef.Type == EventMissionFailed {
			failed = ef
		}
		// 任何一条效果都不该把某个人和「投了失败」绑在一起。
		// d 是这一轮唯一投失败的人：除了「被提名」，不该有任何效果
		// 以他为来源或目标。
		if ef.Type != EventProposed && (ef.SourceID == "d" || ef.TargetID == "d") {
			t.Errorf("有一条效果指向了投失败票的人：%+v", ef)
		}
	}
	if failed == nil {
		t.Fatalf("任务该失败，效果里没有 MISSION_FAILED：%v", typesOf(effects))
	}
	if failed.SourceID != "" || failed.TargetID != "" {
		t.Errorf("失败事件不该带来源或目标，实际 source=%q target=%q",
			failed.SourceID, failed.TargetID)
	}
	if failed.Data["fails"] != "1" {
		t.Errorf("失败票数 = %v，期望 1", failed.Data["fails"])
	}

	// 全场都该看到这条，且看到的内容一模一样
	audience, known := e.AudienceOf(failed.ToEvent())
	if !known || len(audience) != 5 {
		t.Errorf("失败事件该全场可见，实际 known=%v 受众=%v", known, audience)
	}
}

// TestGoodPlayerCannotFail 好人投失败票会被驳回，而且只有他自己知道。
//
// 「只有他自己知道」是必须的：旁人要是能看到「有人试过投失败但被拦了」，
// 等于当场点名——场上只有好人会被拦。
func TestGoodPlayerCannotFail(t *testing.T) {
	e := fivePlayer(t) // c=忠臣（好人）
	leader := leaderID(e.View())
	mustSubmit(t, e, &hiddenrole.SkillUse{PlayerID: leader, Skill: SkillPropose, Targets: []string{"c", "d"}})
	mustEnd(t, e)
	for _, id := range e.AlivePlayerIDs() {
		mustSubmit(t, e, &hiddenrole.SkillUse{PlayerID: id, Skill: SkillApprove})
	}
	mustEnd(t, e)

	// 好人 c 想投失败
	mustSubmit(t, e, &hiddenrole.SkillUse{PlayerID: "c", Skill: SkillMissionFail})
	mustSubmit(t, e, &hiddenrole.SkillUse{PlayerID: "d", Skill: SkillMissionSuccess})
	effects := mustEnd(t, e)

	var rejected *hiddenrole.Effect
	for _, ef := range effects {
		if ef.Type == EventFailRejected {
			rejected = ef
		}
	}
	if rejected == nil {
		t.Fatalf("好人的失败票该被驳回：%v", typesOf(effects))
	}
	if !rejected.Canceled {
		t.Error("驳回的效果该带 Canceled")
	}
	audience, known := e.AudienceOf(rejected.ToEvent())
	if !known || len(audience) != 1 || audience[0] != "c" {
		t.Errorf("驳回只该发给 c 本人，实际 known=%v 受众=%v", known, audience)
	}

	// 而且这一轮任务该算成功——好人的失败票不生效
	var succeeded bool
	for _, ef := range effects {
		if ef.Type == EventMissionSucceeded {
			succeeded = true
		}
	}
	if !succeeded {
		t.Errorf("好人的失败票不该让任务失败：%v", typesOf(effects))
	}
}

// TestSnapshotAndReplay 绕法扛不扛得住持久化。
//
// 整局进度被挂在某个玩家的 PlayerVar 上（见 SCARS.md 疤 4）。这条绕法能
// 成立的前提是 PlayerVar 进快照、也进效果流。这个测试盯住那个前提——
// 它同时说明：疤 4 难看，但没有破坏内核的任何承诺。
func TestSnapshotAndReplay(t *testing.T) {
	e := fivePlayer(t)
	runMission(t, e, 0, "a", "b")
	runMission(t, e, 1, "d", "e", "a") // 一成一败，进度是 1:1

	wantMission, wantSucc, wantFail := mission(e.View()), successes(e.View()), failures(e.View())
	if wantMission != 3 || wantSucc != 1 || wantFail != 1 {
		t.Fatalf("前提坏了：第 %d 轮 %d 胜 %d 负", wantMission, wantSucc, wantFail)
	}

	// 一、快照往返
	raw, err := json.Marshal(e.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	var snap hiddenrole.Snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatal(err)
	}
	restored, err := Restore(&snap)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	checkProgress(t, "恢复", restored, wantMission, wantSucc, wantFail)

	// 二、效果流回放
	replayed, err := Replay(e.EffectLog())
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	checkProgress(t, "回放", replayed, wantMission, wantSucc, wantFail)

	// 三、恢复出来的引擎能继续打，且打得出同样的结果
	runMission(t, restored, 0, "a", "b")
	runMission(t, e, 0, "a", "b")
	if a, b := successes(e.View()), successes(restored.View()); a != b {
		t.Errorf("继续推进之后成功数不同：原局 %d，恢复 %d", a, b)
	}
}

func checkProgress(t *testing.T, what string, e *hiddenrole.Engine, m, s, f int) {
	t.Helper()
	v := e.View()
	if mission(v) != m || successes(v) != s || failures(v) != f {
		t.Errorf("%s 之后进度对不上：第 %d 轮 %d 胜 %d 负，期望第 %d 轮 %d 胜 %d 负",
			what, mission(v), successes(v), failures(v), m, s, f)
	}
}
