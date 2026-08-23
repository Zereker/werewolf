package hiddenrole

import "testing"

// TestValidate_RoundBoundaryRequiredOnlyWhenTheGraphLoops
// A round boundary is required only of a phase graph that loops.
//
// This one came out of the third rules package (One Night Ultimate
// Werewolf): that ruleset has one night, one discussion and one vote in the
// whole game, its phase graph is **a straight line**, and its round number is
// 1 from start to finish -- which is exactly right. The check used to be
// unconditional, so the kernel, guarding against one class of
// misconfiguration, forced a correct configuration to lie (hanging EndsRound
// on the last phase even though no round follows it).
func TestValidate_RoundBoundaryRequiredOnlyWhenTheGraphLoops(t *testing.T) {
	phaseA, phaseB := PhaseType("A"), PhaseType("B")
	step := []PhaseStep{{Role: roleVillager, Skill: skillVote}}

	t.Run("a straight-line graph needs no round boundary", func(t *testing.T) {
		cfg := &Config{
			StartPhase: phaseA,
			Phases: map[PhaseType]*PhaseConfig{
				phaseA: {Type: phaseA, Steps: step, NextPhase: phaseB},
				phaseB: {Type: phaseB, Steps: step, NextPhase: PhaseEnd},
			},
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("a graph that ends at END should not be required to declare a round boundary: %v", err)
		}
	})

	t.Run("a looping graph still does", func(t *testing.T) {
		cfg := &Config{
			StartPhase: phaseA,
			Phases: map[PhaseType]*PhaseConfig{
				phaseA: {Type: phaseA, Steps: step, NextPhase: phaseB},
				phaseB: {Type: phaseB, Steps: step, NextPhase: phaseA}, // loops back
			},
		}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("a looping graph with no round boundary leaves the round at 1 and round variables never cleared; it should be rejected")
		}
		if !HasCode(err, CodeInvalidConfig) {
			t.Errorf("error code should be %v, got %v", CodeInvalidConfig, CodeOf(err))
		}
	})

	t.Run("EndsRound alone is not enough for a looping graph", func(t *testing.T) {
		cfg := &Config{
			StartPhase: phaseA,
			Phases: map[PhaseType]*PhaseConfig{
				phaseA: {Type: phaseA, Steps: step, NextPhase: phaseB},
				phaseB: {Type: phaseB, Steps: step, NextPhase: phaseA, EndsRound: true},
			},
		}
		if err := cfg.Validate(); err == nil {
			t.Error("the round number would rise while round variables are never cleared; that should be rejected too")
		}
	})

	t.Run("a phase pointing at itself is a loop", func(t *testing.T) {
		cfg := &Config{
			StartPhase: phaseA,
			Phases: map[PhaseType]*PhaseConfig{
				phaseA: {Type: phaseA, Steps: step, NextPhase: phaseA},
			},
		}
		if err := cfg.Validate(); err == nil {
			t.Error("a phase pointing at itself is a loop")
		}
	})

	t.Run("all three real rules packages pass", func(t *testing.T) {
		// The first two loop and declare a round boundary; the third is a
		// straight line and declares none.
		if err := testConfig().Validate(); err != nil {
			t.Errorf("the looping board the kernel tests use: %v", err)
		}
	})
}
