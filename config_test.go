package werewolf

import (
	"testing"
	"time"

	pb "github.com/Zereker/werewolf/proto"
)

func TestDefaultGameConfig(t *testing.T) {
	config := DefaultGameConfig()

	if config.WitchCanSaveSelf {
		t.Error("expected WitchCanSaveSelf=false")
	}
	if !config.GuardCanProtectSelf {
		t.Error("expected GuardCanProtectSelf=true")
	}
	if config.GuardCanRepeat {
		t.Error("expected GuardCanRepeat=false")
	}
	if !config.SameGuardKillIsEmpty {
		t.Error("expected SameGuardKillIsEmpty=true")
	}
	if config.DefaultTimeout != 30*time.Second {
		t.Errorf("expected DefaultTimeout=30s, got %v", config.DefaultTimeout)
	}
	// 3 day phases (day, vote, day_hunter) + 6 night sub-phases = 9
	if len(config.Phases) != 9 {
		t.Errorf("expected 9 phases, got %d", len(config.Phases))
	}
}

func TestStandardDayPhase(t *testing.T) {
	phase := StandardDayPhase()

	if phase.Type != pb.PhaseType_PHASE_TYPE_DAY {
		t.Errorf("expected Type=DAY, got %v", phase.Type)
	}
	if len(phase.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(phase.Steps))
	}
	if phase.Timeout != 60*time.Second {
		t.Errorf("expected Timeout=60s, got %v", phase.Timeout)
	}

	// Verify god announce step
	godStep := phase.Steps[0]
	if godStep.Role != pb.RoleType_ROLE_TYPE_GOD {
		t.Errorf("expected Role=GOD, got %v", godStep.Role)
	}
	if godStep.Skill != pb.SkillType_SKILL_TYPE_ANNOUNCE {
		t.Errorf("expected Skill=ANNOUNCE, got %v", godStep.Skill)
	}
	if !godStep.Required {
		t.Error("expected god announce step to be Required")
	}

	// Verify speak step
	speakStep := phase.Steps[1]
	if speakStep.Role != pb.RoleType_ROLE_TYPE_UNSPECIFIED {
		t.Errorf("expected Role=UNSPECIFIED, got %v", speakStep.Role)
	}
	if speakStep.Skill != pb.SkillType_SKILL_TYPE_SPEAK {
		t.Errorf("expected Skill=SPEAK, got %v", speakStep.Skill)
	}
	if !speakStep.Multiple {
		t.Error("expected speak step to allow Multiple")
	}
}

func TestStandardVotePhase(t *testing.T) {
	phase := StandardVotePhase()

	if phase.Type != pb.PhaseType_PHASE_TYPE_VOTE {
		t.Errorf("expected Type=VOTE, got %v", phase.Type)
	}
	if len(phase.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(phase.Steps))
	}
	if phase.Timeout != 30*time.Second {
		t.Errorf("expected Timeout=30s, got %v", phase.Timeout)
	}

	// Verify god announce step
	godStep := phase.Steps[0]
	if godStep.Role != pb.RoleType_ROLE_TYPE_GOD {
		t.Errorf("expected Role=GOD, got %v", godStep.Role)
	}
	if godStep.Skill != pb.SkillType_SKILL_TYPE_ANNOUNCE {
		t.Errorf("expected Skill=ANNOUNCE, got %v", godStep.Skill)
	}
	if !godStep.Required {
		t.Error("expected god announce step to be Required")
	}

	// Verify vote step
	voteStep := phase.Steps[1]
	if voteStep.Role != pb.RoleType_ROLE_TYPE_UNSPECIFIED {
		t.Errorf("expected Role=UNSPECIFIED, got %v", voteStep.Role)
	}
	if voteStep.Skill != pb.SkillType_SKILL_TYPE_VOTE {
		t.Errorf("expected Skill=VOTE, got %v", voteStep.Skill)
	}
	if !voteStep.Required {
		t.Error("expected vote step to be Required")
	}
	if !voteStep.Multiple {
		t.Error("expected vote step to allow Multiple")
	}
}

func TestSkillUse_Fields(t *testing.T) {
	use := &SkillUse{
		PlayerID: "p1",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "p2",
		Phase:    pb.PhaseType_PHASE_TYPE_NIGHT,
		Round:    1,
	}

	if use.PlayerID != "p1" {
		t.Errorf("expected PlayerID=p1, got %s", use.PlayerID)
	}
	if use.Skill != pb.SkillType_SKILL_TYPE_KILL {
		t.Errorf("expected Skill=KILL, got %v", use.Skill)
	}
	if use.TargetID != "p2" {
		t.Errorf("expected TargetID=p2, got %s", use.TargetID)
	}
	if use.Phase != pb.PhaseType_PHASE_TYPE_NIGHT {
		t.Errorf("expected Phase=NIGHT, got %v", use.Phase)
	}
	if use.Round != 1 {
		t.Errorf("expected Round=1, got %d", use.Round)
	}
}
