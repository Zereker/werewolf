package werewolf

import (
	"reflect"
	"testing"
)

// playMidGame 走到一个有内容的局面：救过人、投出过人、猎人开过枪
func playMidGame(t *testing.T) *Engine {
	t.Helper()

	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), seer("s"), witch("wi"), guard("g"), hunter("h"),
		villagers("v1", "v2", "v3"),
	)...)

	g.mustUse("g", SkillProtect, "s")
	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "v1")
	g.mustUse("w2", SkillKill, "v1")
	g.end(PhaseNightWitch)
	g.mustUse("wi", SkillAntidote, "v1")
	g.end(PhaseNightSeer)
	g.mustUse("s", SkillCheck, "w1")
	g.end(PhaseNightResolve)
	g.end(PhaseDay)
	g.end(PhaseVote)

	// 投出猎人，触发开枪
	g.vote("h", "w1", "w2", "v1", "v2", "v3")
	g.end(PhaseDayHunter)
	g.mustUse("h", SkillShoot, "w1")
	g.end(PhaseNightGuard)

	return g.e
}

func TestEffectLog_RecordsWholeGame(t *testing.T) {
	e := playMidGame(t)
	log := e.EffectLog()

	if len(log) == 0 {
		t.Fatal("效果流不应为空")
	}

	// 建局与开局都要在流里，否则无法自洽回放
	var added, started int
	for _, ef := range log {
		switch ef.Type {
		case EventPlayerAdded:
			added++
		case EventGameStarted:
			started++
		}
	}
	if added != 9 {
		t.Errorf("应记录 9 名玩家入座，实际 %d", added)
	}
	if started != 1 {
		t.Errorf("应记录 1 次开局，实际 %d", started)
	}

	// 关键事件都要在
	want := []EventType{
		EventProtect,
		EventSetRoundVar,
		EventSave,
		EventCheck,
		EventEliminate,
		EventAbilityTriggered,
		EventShoot,
		EventPhaseChanged,
	}
	for _, typ := range want {
		found := false
		for _, ef := range log {
			if ef.Type == typ {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("效果流中缺少 %v", typ)
		}
	}
}

// TestEffectLog_ReplayRebuildsGame 回放出的引擎与原引擎状态一致。
func TestEffectLog_ReplayRebuildsGame(t *testing.T) {
	original := playMidGame(t)

	replayed, err := Replay(nil, DefaultRules(), original.EffectLog())
	if err != nil {
		t.Fatalf("回放失败: %v", err)
	}

	if !reflect.DeepEqual(original.Snapshot(), replayed.Snapshot()) {
		t.Errorf("回放后的局面与原局面不一致\n原始: %+v\n回放: %+v",
			original.Snapshot(), replayed.Snapshot())
	}
}

// TestEffectLog_ReplayedEngineContinuesIdentically 回放出的引擎能继续推进，
// 且与原引擎给出相同结果。
func TestEffectLog_ReplayedEngineContinuesIdentically(t *testing.T) {
	original := playMidGame(t)
	replayed, err := Replay(nil, DefaultRules(), original.EffectLog())
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 5; i++ {
		effOrig, err1 := original.EndPhase()
		effRepl, err2 := replayed.EndPhase()
		if err1 != nil || err2 != nil {
			t.Fatalf("第 %d 步: 原始 %v / 回放 %v", i, err1, err2)
		}
		if len(effOrig) != len(effRepl) {
			t.Fatalf("第 %d 步效果数量不一致: %d vs %d", i, len(effOrig), len(effRepl))
		}
		if !reflect.DeepEqual(original.Snapshot(), replayed.Snapshot()) {
			t.Fatalf("第 %d 步后两个引擎分叉", i)
		}
	}
}

// TestEffectLog_IsAppendOnlyCopy 返回的是切片副本，后续推进不影响已取出的日志。
func TestEffectLog_IsAppendOnlyCopy(t *testing.T) {
	e := playMidGame(t)
	before := e.EffectLog()
	n := len(before)

	for i := 0; i < 3; i++ {
		if _, err := e.EndPhase(); err != nil {
			break
		}
	}

	if len(before) != n {
		t.Errorf("已取出的日志被追加了内容: %d -> %d", n, len(before))
	}
	if len(e.EffectLog()) <= n {
		t.Error("引擎继续推进后日志应当增长")
	}
}

func TestReplayEngine_Rejects(t *testing.T) {
	t.Run("含 nil 条目", func(t *testing.T) {
		if _, err := Replay(nil, DefaultRules(), []*Effect{nil}); err == nil {
			t.Error("应当拒绝 nil 条目")
		}
	})

	t.Run("开局效果缺少阶段", func(t *testing.T) {
		bad := []*Effect{NewEffect(EventGameStarted, "", "")}
		if _, err := Replay(nil, DefaultRules(), bad); err == nil {
			t.Error("缺少阶段信息时应当报错")
		}
	})

	t.Run("流转效果缺少阶段", func(t *testing.T) {
		bad := []*Effect{NewEffect(EventPhaseChanged, "", "")}
		if _, err := Replay(nil, DefaultRules(), bad); err == nil {
			t.Error("缺少阶段信息时应当报错")
		}
	})

	t.Run("配置不合法", func(t *testing.T) {
		cfg := DefaultGameConfig()
		delete(cfg.Phases, PhaseNightWitch)
		if _, err := Replay(cfg, DefaultRules(), nil); err == nil {
			t.Error("残缺配置应当被拒绝")
		}
	})

	t.Run("空日志得到未开局的引擎", func(t *testing.T) {
		e, err := Replay(nil, DefaultRules(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if e.Phase() != PhaseStart {
			t.Errorf("期望 START，实际 %v", e.Phase())
		}
	})
}

// TestReplayEngine_MidRoundTriggerQueue 回合中途停下来回放，待结算队列要一致。
//
// 触发的入队走 ABILITY_TRIGGERED 效果，回放时能重建；出队却发生在
// calculateNextPhase 里，不产生任何效果。回放因此会留下一条本该消费掉的
// 触发，从下一步起原引擎与回放引擎流转到不同的阶段。
//
// 已有的回放测试都停在回合边界上，那里 resetRoundState 会把整个
// RoundCtx 换掉，正好把这个分叉盖住。
func TestReplayEngine_MidRoundTriggerQueue(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), hunter("h"), seer("s"),
		villagers("v1", "v2", "v3", "v4"),
	)...)

	// 夜里刀死猎人，他开枪打死一名平民，停在白天（回合中途）
	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "h")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseNightHunter)
	g.mustUse("h", SkillShoot, "v1")
	g.end(PhaseDay)

	replayed, err := Replay(nil, DefaultRules(), g.e.EffectLog())
	if err != nil {
		t.Fatalf("ReplayEngine 失败: %v", err)
	}

	origin := g.e.RoundContext().PendingTriggers
	copyOf := replayed.RoundContext().PendingTriggers
	if len(origin) != len(copyOf) {
		t.Fatalf("待结算队列不一致: 原引擎 %v，回放 %v", origin, copyOf)
	}

	// 再各推进一步，两边必须走到同一个阶段
	if _, err := g.e.EndPhase(); err != nil {
		t.Fatalf("原引擎推进失败: %v", err)
	}
	if _, err := replayed.EndPhase(); err != nil {
		t.Fatalf("回放引擎推进失败: %v", err)
	}
	if g.e.Phase() != replayed.Phase() {
		t.Errorf("推进一步后分叉: 原引擎 %v，回放 %v", g.e.Phase(), replayed.Phase())
	}
}
