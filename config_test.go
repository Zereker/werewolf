package werewolf

import (
	"testing"
	"time"
)

// TestDefaultRules 规则开关的默认值。
//
// 它们此前挂在 GameConfig 上，与阶段机的配置混在同一个结构体里，
// 于是内核认得「女巫能不能自救」这种事。现在住在 Rules 上。
func TestDefaultRules(t *testing.T) {
	rules := DefaultRules()

	if rules.WitchCanSaveSelf {
		t.Error("expected WitchCanSaveSelf=false")
	}
	if !rules.GuardCanProtectSelf {
		t.Error("expected GuardCanProtectSelf=true")
	}
	if rules.GuardCanRepeat {
		t.Error("expected GuardCanRepeat=false")
	}
	if !rules.SameGuardKillIsEmpty {
		t.Error("expected SameGuardKillIsEmpty=true")
	}
	if rules.VictoryMode != VictoryModeSideWipe {
		t.Errorf("expected VictoryMode=SIDE_WIPE, got %v", rules.VictoryMode)
	}
	if err := rules.Validate(); err != nil {
		t.Errorf("默认规则应当合法: %v", err)
	}
}

func TestDefaultGameConfig(t *testing.T) {
	config := DefaultGameConfig()

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

	if phase.Type != PhaseDay {
		t.Errorf("expected Type=DAY, got %v", phase.Type)
	}
	if phase.NextPhase != PhaseVote {
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
	if godStep.Role != RoleGod {
		t.Errorf("expected Role=GOD, got %v", godStep.Role)
	}
	if godStep.Skill != SkillAnnounce {
		t.Errorf("expected Skill=ANNOUNCE, got %v", godStep.Skill)
	}
}

func TestStandardVotePhase(t *testing.T) {
	phase := StandardVotePhase()

	if phase.Type != PhaseVote {
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
	if godStep.Role != RoleGod {
		t.Errorf("expected Role=GOD, got %v", godStep.Role)
	}
	if godStep.Skill != SkillAnnounce {
		t.Errorf("expected Skill=ANNOUNCE, got %v", godStep.Skill)
	}

	// Verify vote step
	voteStep := phase.Steps[1]
	if voteStep.Role != RoleUnspecified {
		t.Errorf("expected Role=UNSPECIFIED, got %v", voteStep.Role)
	}
	if voteStep.Skill != SkillVote {
		t.Errorf("expected Skill=VOTE, got %v", voteStep.Skill)
	}
}

func TestSkillUse_Fields(t *testing.T) {
	use := &SkillUse{
		PlayerID: "p1",
		Skill:    SkillKill,
		TargetID: "p2",
		Phase:    PhaseNight,
		Round:    1,
	}

	if use.PlayerID != "p1" {
		t.Errorf("expected PlayerID=p1, got %s", use.PlayerID)
	}
	if use.Skill != SkillKill {
		t.Errorf("expected Skill=KILL, got %v", use.Skill)
	}
	if use.TargetID != "p2" {
		t.Errorf("expected TargetID=p2, got %s", use.TargetID)
	}
	if use.Phase != PhaseNight {
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
			c.Phases = map[PhaseType]*PhaseConfig{}
		}), true},
		{"起始阶段不存在", mutate(func(c *GameConfig) {
			c.StartPhase = PhaseNightHunter
			delete(c.Phases, PhaseNightHunter)
		}), true},
		{"NextPhase 悬空", mutate(func(c *GameConfig) {
			delete(c.Phases, PhaseNightWitch)
		}), true},
		{"map 的 key 与 Type 不一致", mutate(func(c *GameConfig) {
			c.Phases[PhaseNightSeer] = StandardDayPhase()
		}), true},
		{"阶段配置为 nil", mutate(func(c *GameConfig) {
			c.Phases[PhaseDay] = nil
		}), true},
		{"同一阶段重复声明同一技能", mutate(func(c *GameConfig) {
			p := c.Phases[PhaseNightSeer]
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
	delete(cfg.Phases, PhaseNightWitch) // NIGHT_WOLF 的 NextPhase 悬空

	if _, err := NewEngine(cfg); err == nil {
		t.Fatal("残缺配置应当在构造时被拒绝")
	}
}

// TestStart_RejectsMissingResolver 阶段没有解析器时，技能会被静默丢弃，
// 必须在开局前拦下。
func TestStart_RejectsMissingResolver(t *testing.T) {
	engine := MustNew(DefaultRules())
	delete(engine.phase.resolvers, PhaseNightWolf)

	mustAdd(t, engine, "w1", RoleWerewolf)
	mustAdd(t, engine, "v1", RoleVillager)

	if err := engine.Start(); err == nil {
		t.Error("缺少解析器时 Start 应当报错")
	}
}

// TestStartPhase_Configurable 起始阶段可配置，不再硬编码为 NIGHT_GUARD。
func TestStartPhase_Configurable(t *testing.T) {
	cfg := DefaultGameConfig()
	cfg.StartPhase = PhaseDay

	engine := MustNewWith(cfg, DefaultRules())
	mustAdd(t, engine, "w1", RoleWerewolf)
	mustAdd(t, engine, "v1", RoleVillager)
	if err := engine.Start(); err != nil {
		t.Fatal(err)
	}

	if got := engine.Phase(); got != PhaseDay {
		t.Errorf("期望从 DAY 开局，实际 %v", got)
	}
}

// TestValidate_RejectsMissingNextPhase 漏填 NextPhase 与悬空的 NextPhase 后果相同。
//
// 想表达「到此结束」有 END，留空只可能是漏填。
// 而 nextSubPhase 对 UNSPECIFIED 的处理是直接进 END——游戏在那里
// 静默收场，连 GAME_ENDED 都不会发。
func TestValidate_RejectsMissingNextPhase(t *testing.T) {
	cfg := DefaultGameConfig()
	cfg.Phases[PhaseType("ORPHAN")] = &PhaseConfig{Type: PhaseType("ORPHAN")}

	if err := cfg.Validate(); err == nil {
		t.Fatal("漏填 NextPhase 应当被拒")
	}
}

// TestValidate_RejectsUnknownVictoryMode 越界的胜负判定方式不该被 default 分支吞掉。
//
// 校验从 GameConfig.Validate 搬到了 Rules.Validate，跟着 VictoryMode 一起走。
func TestValidate_RejectsUnknownVictoryMode(t *testing.T) {
	rules := DefaultRules()
	rules.VictoryMode = VictoryMode(99)

	if err := rules.Validate(); err == nil {
		t.Fatal("越界的 VictoryMode 应当被拒")
	}
	if _, err := New(rules); err == nil {
		t.Fatal("越界的 VictoryMode 应当让组装失败")
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
	vote := cfg.Phases[PhaseVote]
	vote.Steps = append(vote.Steps, PhaseStep{
		Role:  RoleWerewolf,
		Skill: SkillVote,
	})

	if err := cfg.Validate(); err == nil {
		t.Fatal("全体步骤与具体角色步骤声明同一技能，应当被拒")
	}
}

// TestGameConfig_PhaseTimeout 建议超时要有一条送到调用方手上的路径。
func TestGameConfig_PhaseTimeout(t *testing.T) {
	cfg := DefaultGameConfig()

	if got := cfg.PhaseTimeout(PhaseDay); got != DayPhaseTimeout {
		t.Errorf("DAY: 期望 %v，实际 %v", DayPhaseTimeout, got)
	}
	// 未配置的阶段退回 DefaultTimeout
	if got := cfg.PhaseTimeout(PhaseType("NOT_CONFIGURED")); got != cfg.DefaultTimeout {
		t.Errorf("未知阶段: 期望 %v，实际 %v", cfg.DefaultTimeout, got)
	}
	// DefaultTimeout 也没配时退回常量
	bare := &GameConfig{Phases: cfg.Phases}
	if got := bare.PhaseTimeout(PhaseType("NOT_CONFIGURED")); got != DefaultPhaseTimeout {
		t.Errorf("兜底: 期望 %v，实际 %v", DefaultPhaseTimeout, got)
	}
}

// TestValidate_RejectsGroupSpanningRoles 互斥备选组是「同一个人几选一」，跨角色没有意义。
func TestValidate_RejectsGroupSpanningRoles(t *testing.T) {
	cfg := DefaultGameConfig()
	day := cfg.Phases[PhaseDay]
	day.Steps = append(day.Steps,
		PhaseStep{Role: RoleSeer, Skill: SkillCheck, Group: "x"},
		PhaseStep{Role: RoleWitch, Skill: SkillPoison, Group: "x"},
	)

	if err := cfg.Validate(); err == nil {
		t.Fatal("跨角色的同名 Group 应当被拒")
	}
}
