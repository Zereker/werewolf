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
	if phase.NextPhase != pb.PhaseType_PHASE_TYPE_VOTE {
		t.Errorf("expected NextPhase=VOTE, got %v", phase.NextPhase)
	}
	if phase.Timeout != 60*time.Second {
		t.Errorf("expected Timeout=60s, got %v", phase.Timeout)
	}

	// 白天只有上帝公告一个步骤——发言走 SendMessage，不是技能
	if len(phase.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(phase.Steps))
	}
	godStep := phase.Steps[0]
	if godStep.Role != pb.RoleType_ROLE_TYPE_GOD {
		t.Errorf("expected Role=GOD, got %v", godStep.Role)
	}
	if godStep.Skill != pb.SkillType_SKILL_TYPE_ANNOUNCE {
		t.Errorf("expected Skill=ANNOUNCE, got %v", godStep.Skill)
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

	// Verify vote step
	voteStep := phase.Steps[1]
	if voteStep.Role != pb.RoleType_ROLE_TYPE_UNSPECIFIED {
		t.Errorf("expected Role=UNSPECIFIED, got %v", voteStep.Role)
	}
	if voteStep.Skill != pb.SkillType_SKILL_TYPE_VOTE {
		t.Errorf("expected Skill=VOTE, got %v", voteStep.Skill)
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

// ==================== 配置校验 ====================

func TestGameConfig_Validate(t *testing.T) {
	// 拿默认配置改一处，避免用例之间互相影响
	mutate := func(f func(c *GameConfig)) *GameConfig {
		c := DefaultGameConfig()
		f(c)
		return c
	}

	cases := []struct {
		name    string
		config  *GameConfig
		wantErr bool
	}{
		{"默认配置合法", DefaultGameConfig(), false},
		{"nil 配置", nil, true},
		{"没有任何阶段", mutate(func(c *GameConfig) {
			c.Phases = map[pb.PhaseType]*PhaseConfig{}
		}), true},
		{"起始阶段不存在", mutate(func(c *GameConfig) {
			c.StartPhase = pb.PhaseType_PHASE_TYPE_NIGHT_HUNTER
			delete(c.Phases, pb.PhaseType_PHASE_TYPE_NIGHT_HUNTER)
		}), true},
		{"NextPhase 悬空", mutate(func(c *GameConfig) {
			delete(c.Phases, pb.PhaseType_PHASE_TYPE_NIGHT_WITCH)
		}), true},
		{"map 的 key 与 Type 不一致", mutate(func(c *GameConfig) {
			c.Phases[pb.PhaseType_PHASE_TYPE_NIGHT_SEER] = StandardDayPhase()
		}), true},
		{"阶段配置为 nil", mutate(func(c *GameConfig) {
			c.Phases[pb.PhaseType_PHASE_TYPE_DAY] = nil
		}), true},
		{"同一阶段重复声明同一技能", mutate(func(c *GameConfig) {
			p := c.Phases[pb.PhaseType_PHASE_TYPE_NIGHT_SEER]
			p.Steps = append(p.Steps, p.Steps[len(p.Steps)-1])
		}), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.wantErr && err == nil {
				t.Error("期望校验失败，实际通过")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("期望校验通过，实际 %v", err)
			}
		})
	}
}

func TestNewEngine_RejectsInvalidConfig(t *testing.T) {
	cfg := DefaultGameConfig()
	delete(cfg.Phases, pb.PhaseType_PHASE_TYPE_NIGHT_WITCH) // NIGHT_WOLF 的 NextPhase 悬空

	if _, err := NewEngine(cfg); err == nil {
		t.Fatal("残缺配置应当在构造时被拒绝")
	}
}

// TestStart_RejectsMissingResolver 阶段没有解析器时，技能会被静默丢弃，
// 必须在开局前拦下。
func TestStart_RejectsMissingResolver(t *testing.T) {
	engine := MustNewEngine(nil)
	delete(engine.phase.resolvers, pb.PhaseType_PHASE_TYPE_NIGHT_WOLF)

	mustAdd(t, engine, "w1", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)

	if err := engine.Start(); err == nil {
		t.Error("缺少解析器时 Start 应当报错")
	}
}

// TestStartPhase_Configurable 起始阶段可配置，不再硬编码为 NIGHT_GUARD。
func TestStartPhase_Configurable(t *testing.T) {
	cfg := DefaultGameConfig()
	cfg.StartPhase = pb.PhaseType_PHASE_TYPE_DAY

	engine := MustNewEngine(cfg)
	mustAdd(t, engine, "w1", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	if err := engine.Start(); err != nil {
		t.Fatal(err)
	}

	if got := engine.Phase(); got != pb.PhaseType_PHASE_TYPE_DAY {
		t.Errorf("期望从 DAY 开局，实际 %v", got)
	}
}

// TestValidate_RejectsMissingNextPhase 漏填 NextPhase 与悬空的 NextPhase 后果相同。
//
// 想表达「到此结束」有 PHASE_TYPE_END，留空只可能是漏填。
// 而 nextSubPhase 对 UNSPECIFIED 的处理是直接进 END——游戏在那里
// 静默收场，连 GAME_ENDED 都不会发。
func TestValidate_RejectsMissingNextPhase(t *testing.T) {
	cfg := DefaultGameConfig()
	cfg.Phases[pb.PhaseType(77)] = &PhaseConfig{Type: pb.PhaseType(77)}

	if err := cfg.Validate(); err == nil {
		t.Fatal("漏填 NextPhase 应当被拒")
	}
}

// TestValidate_RejectsUnknownVictoryMode 越界的胜负判定方式不该被 default 分支吞掉。
func TestValidate_RejectsUnknownVictoryMode(t *testing.T) {
	cfg := DefaultGameConfig()
	cfg.VictoryMode = VictoryMode(99)

	if err := cfg.Validate(); err == nil {
		t.Fatal("越界的 VictoryMode 应当被拒")
	}
	if got := VictoryMode(99).String(); got != "UNKNOWN" {
		t.Errorf("String(): 期望 UNKNOWN，实际 %s", got)
	}
}

// TestValidate_RejectsAllRolesOverlap UNSPECIFIED（全体）与具体角色声明同一技能是重复。
//
// 去重此前只比 {Role, Skill} 这个键，而 UNSPECIFIED 表示「所有角色」，
// 于是 AllowedSkills 会返回重复项、PhaseReadiness 会重复计数——
// 正是这段校验声称要拦下的问题的另一半。
func TestValidate_RejectsAllRolesOverlap(t *testing.T) {
	cfg := DefaultGameConfig()
	vote := cfg.Phases[pb.PhaseType_PHASE_TYPE_VOTE]
	vote.Steps = append(vote.Steps, PhaseStep{
		Role:  pb.RoleType_ROLE_TYPE_WEREWOLF,
		Skill: pb.SkillType_SKILL_TYPE_VOTE,
	})

	if err := cfg.Validate(); err == nil {
		t.Fatal("全体步骤与具体角色步骤声明同一技能，应当被拒")
	}
}

// TestGameConfig_PhaseTimeout 建议超时要有一条送到调用方手上的路径。
func TestGameConfig_PhaseTimeout(t *testing.T) {
	cfg := DefaultGameConfig()

	if got := cfg.PhaseTimeout(pb.PhaseType_PHASE_TYPE_DAY); got != DayPhaseTimeout {
		t.Errorf("DAY: 期望 %v，实际 %v", DayPhaseTimeout, got)
	}
	// 未配置的阶段退回 DefaultTimeout
	if got := cfg.PhaseTimeout(pb.PhaseType(999)); got != cfg.DefaultTimeout {
		t.Errorf("未知阶段: 期望 %v，实际 %v", cfg.DefaultTimeout, got)
	}
	// DefaultTimeout 也没配时退回常量
	bare := &GameConfig{Phases: cfg.Phases}
	if got := bare.PhaseTimeout(pb.PhaseType(999)); got != DefaultPhaseTimeout {
		t.Errorf("兜底: 期望 %v，实际 %v", DefaultPhaseTimeout, got)
	}
}

// TestValidate_RejectsGroupSpanningRoles 互斥备选组是「同一个人几选一」，跨角色没有意义。
func TestValidate_RejectsGroupSpanningRoles(t *testing.T) {
	cfg := DefaultGameConfig()
	day := cfg.Phases[pb.PhaseType_PHASE_TYPE_DAY]
	day.Steps = append(day.Steps,
		PhaseStep{Role: pb.RoleType_ROLE_TYPE_SEER, Skill: pb.SkillType_SKILL_TYPE_CHECK, Group: "x"},
		PhaseStep{Role: pb.RoleType_ROLE_TYPE_WITCH, Skill: pb.SkillType_SKILL_TYPE_POISON, Group: "x"},
	)

	if err := cfg.Validate(); err == nil {
		t.Fatal("跨角色的同名 Group 应当被拒")
	}
}
