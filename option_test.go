package werewolf

import (
	"testing"
)

// TestWithLoggerAndMetrics 日志与指标只能在构造时给出，且真的接上了。
func TestWithLoggerAndMetrics(t *testing.T) {
	rec := &recordingLogger{}
	counter := &countingMetrics{}

	g := newRuleGameWith(t, nil, []EngineOption{WithLogger(rec), WithMetrics(counter)},
		seats(wolf("w1"), villagers("v1", "v2", "v3"))...)

	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "v1")
	g.end(PhaseNightWitch)

	if counter.skills == 0 {
		t.Error("提交技能应当被计数")
	}
	if counter.phases == 0 {
		t.Error("阶段结束应当被计数")
	}
	if counter.effects == 0 {
		t.Error("效果应用应当被计数")
	}
	if len(rec.infos) == 0 {
		t.Error("开局应当留下 Info 级日志")
	}
}

// TestWithNilOption nil 选项与 nil 日志/指标都不该让构造失败。
func TestWithNilOption(t *testing.T) {
	if _, err := NewEngine(nil, nil, WithLogger(nil), WithMetrics(nil)); err != nil {
		t.Errorf("nil 选项应当被忽略，实际 %v", err)
	}
}

// countingMetrics 只数次数的 Metrics 实现。
type countingMetrics struct {
	skills, phases, effects, ended int
}

func (m *countingMetrics) IncSkillSubmitted(SkillType) { m.skills++ }
func (m *countingMetrics) IncPhaseEnded(PhaseType)     { m.phases++ }
func (m *countingMetrics) IncEffectApplied(EventType)  { m.effects++ }
func (m *countingMetrics) IncGameEnded(Camp)           { m.ended++ }
