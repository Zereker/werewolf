package engine

import "testing"

// TestValidate_RoundBoundaryRequiredOnlyWhenTheGraphLoops
// 回合边界只对会转圈的阶段图是必需的。
//
// 这一条是第三套规则包（一夜狼人）撞出来的：那一套整局只有一个夜晚、
// 一次讨论、一次投票，阶段图是**一条直线**，回合数从头到尾是 1——而那
// 恰恰是对的。检查此前是无条件的，于是内核为了防一类配置错误，逼一个
// 正确的配置去撒谎（把 EndsRound 挂在最后一个阶段上，虽然它之后没有下
// 一个回合）。
func TestValidate_RoundBoundaryRequiredOnlyWhenTheGraphLoops(t *testing.T) {
	phaseA, phaseB := PhaseType("A"), PhaseType("B")
	step := []PhaseStep{{Role: roleVillager, Skill: skillVote}}

	t.Run("直线图不需要回合边界", func(t *testing.T) {
		cfg := &Config{
			StartPhase: phaseA,
			Phases: map[PhaseType]*PhaseConfig{
				phaseA: {Type: phaseA, Steps: step, NextPhase: phaseB},
				phaseB: {Type: phaseB, Steps: step, NextPhase: PhaseEnd},
			},
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("走到 END 就结束的图不该被要求声明回合边界：%v", err)
		}
	})

	t.Run("转圈的图仍然要", func(t *testing.T) {
		cfg := &Config{
			StartPhase: phaseA,
			Phases: map[PhaseType]*PhaseConfig{
				phaseA: {Type: phaseA, Steps: step, NextPhase: phaseB},
				phaseB: {Type: phaseB, Steps: step, NextPhase: phaseA}, // 绕回去
			},
		}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("转圈的图没有回合边界，回合永远是 1、回合变量永不清，该被拒绝")
		}
		if !HasCode(err, CodeInvalidConfig) {
			t.Errorf("错误码应当是 %v，实际 %v", CodeInvalidConfig, CodeOf(err))
		}
	})

	t.Run("转圈的图只有 EndsRound 也不够", func(t *testing.T) {
		cfg := &Config{
			StartPhase: phaseA,
			Phases: map[PhaseType]*PhaseConfig{
				phaseA: {Type: phaseA, Steps: step, NextPhase: phaseB},
				phaseB: {Type: phaseB, Steps: step, NextPhase: phaseA, EndsRound: true},
			},
		}
		if err := cfg.Validate(); err == nil {
			t.Error("回合数会涨但回合变量永不清，同样该被拒绝")
		}
	})

	t.Run("自己指向自己也算转圈", func(t *testing.T) {
		cfg := &Config{
			StartPhase: phaseA,
			Phases: map[PhaseType]*PhaseConfig{
				phaseA: {Type: phaseA, Steps: step, NextPhase: phaseA},
			},
		}
		if err := cfg.Validate(); err == nil {
			t.Error("一个阶段指向自己也是圈")
		}
	})

	t.Run("三套真实规则包都通过", func(t *testing.T) {
		// 前两套是环、声明了回合边界；第三套是直线、一个都没声明。
		if err := testConfig().Validate(); err != nil {
			t.Errorf("内核测试用的那副环形板子: %v", err)
		}
	})
}
