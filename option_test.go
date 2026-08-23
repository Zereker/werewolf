package werewolf

import (
	"github.com/Zereker/hiddenrole"
	"testing"
)

// TestWithLogger 日志只能在构造时给出，且真的接上了。
func TestWithLogger(t *testing.T) {
	rec := &recordingLogger{}

	g := newRuleGameWith(t, nil, []EngineOption{hiddenrole.WithLogger(rec)},
		seats(wolf("w1"), villagers("v1", "v2", "v3"))...)

	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "v1")
	g.end(PhaseNightWitch)

	if len(rec.infos) == 0 {
		t.Error("开局应当留下 Info 级日志")
	}
}

// TestWithNilOption nil 选项与 nil 日志都不该让构造失败。
func TestWithNilOption(t *testing.T) {
	if _, err := hiddenrole.NewEngine(DefaultGameConfig(), nil, hiddenrole.WithLogger(nil)); err != nil {
		t.Errorf("nil 选项应当被忽略，实际 %v", err)
	}
}
