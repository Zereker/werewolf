package werewolf

import (
	"testing"

	pb "github.com/Zereker/werewolf/proto"
)

// TestWithLoggerAndMetrics 日志与指标只能在构造时给出，且真的接上了。
func TestWithLoggerAndMetrics(t *testing.T) {
	rec := &recordingLogger{}
	counter := &countingMetrics{}

	g := newRuleGameWith(t, nil, []EngineOption{WithLogger(rec), WithMetrics(counter)},
		seats(wolf("w1"), villagers("v1", "v2", "v3"))...)

	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WOLF)
	g.mustUse("w1", pb.SkillType_SKILL_TYPE_KILL, "v1")
	g.end(pb.PhaseType_PHASE_TYPE_NIGHT_WITCH)

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

func (m *countingMetrics) IncSkillSubmitted(pb.SkillType) { m.skills++ }
func (m *countingMetrics) IncPhaseEnded(pb.PhaseType)     { m.phases++ }
func (m *countingMetrics) IncEffectApplied(pb.EventType)  { m.effects++ }
func (m *countingMetrics) IncGameEnded(pb.Camp)           { m.ended++ }
