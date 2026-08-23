package hiddenrole

import "testing"

// effectProducer 产出一条带 Data 的效果，好让 EndPhase 有东西返回。
type effectProducer struct{ tag string }

func (r effectProducer) Resolve([]*SkillUse, GameView) []*Effect {
	return []*Effect{
		NewEffect(EventType("PROBE"), "src", "dst").WithData("tag", r.tag),
	}
}

// TestEffectLog_HistoryIsNotWritableFromOutside 效果流是历史，历史不可被外部改写。
//
// 这条不变量此前只写在文档里：EndPhase 返回的与 EffectLog 返回的，都是
// 引擎内部那份历史的同一批指针。调用方改一个字段——或者调一下 Cancel，
// 它是导出方法——引擎的历史就被改了，而回放会照着被改过的历史重建出
// 另一局游戏。「可回放、可审计」这两条收益全部落空。
//
// 现在进日志的是副本、出日志的也是副本。这个测试同时盯住两侧。
func TestEffectLog_HistoryIsNotWritableFromOutside(t *testing.T) {
	opts := append(withNoopResolvers(), WithResolver(phaseNightGuard, effectProducer{tag: "原始"}))
	e := newTestEngine(t, opts...)
	mustAdd(t, e, "w1", roleWerewolf)
	mustAdd(t, e, "g", roleGuard)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	effects, err := e.EndPhase()
	if err != nil {
		t.Fatalf("EndPhase: %v", err)
	}
	probe := findProbe(t, effects)
	before := len(e.EffectLog())

	// 一、改 EndPhase 交出去的那批
	probe.TargetID = "改过了"
	probe.Cancel("外部改的")
	probe.Data["tag"] = "改过了"
	assertHistoryIntact(t, e, before, "EndPhase 的返回值")

	// 二、改 EffectLog 交出去的那批
	log := e.EffectLog()
	logged := findProbe(t, log)
	logged.TargetID = "改过了"
	logged.Cancel("外部改的")
	logged.Data["tag"] = "改过了"
	assertHistoryIntact(t, e, before, "EffectLog 的返回值")
}

func findProbe(t *testing.T, effects []*Effect) *Effect {
	t.Helper()
	for _, ef := range effects {
		if ef.Type == EventType("PROBE") {
			return ef
		}
	}
	t.Fatalf("没找到探针效果，拿到 %d 条", len(effects))
	return nil
}

func assertHistoryIntact(t *testing.T, e *Engine, wantLen int, via string) {
	t.Helper()
	log := e.EffectLog()
	if len(log) != wantLen {
		t.Fatalf("经 %s：历史长度从 %d 变成了 %d", via, wantLen, len(log))
	}
	ef := findProbe(t, log)
	switch {
	case ef.TargetID != "dst":
		t.Errorf("经 %s：历史里的 TargetID 被改成了 %q", via, ef.TargetID)
	case ef.Canceled:
		t.Errorf("经 %s：历史里的效果被取消了，Reason=%q", via, ef.Reason)
	case ef.Data["tag"] != "原始":
		t.Errorf("经 %s：历史里的 Data 被改成了 %v", via, ef.Data["tag"])
	}
}
