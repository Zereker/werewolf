package engine

import (
	"errors"
	"testing"
	"time"
)

// hostapi_test.go 宿主用得到、而内核自己此前没验过的那几条。
//
// 它们的共同点是「只被下游驱动过」：主持台读 PhaseInfo 组织流程、
// 服务端按错误码分支、超时读 PhaseTimeout。内核拆出去独立成库之后，
// 这些是使用者最先碰到的东西，不该靠规则包替它们作证。

// TestPhaseInfo_TellsTheHostWhatToDo PhaseInfo 是主持台的操作说明。
//
// 三个方法回答三个问题：该念公告吗、念哪一条、还有谁要行动。
// 它们此前覆盖率全是 0%。
func TestPhaseInfo_TellsTheHostWhatToDo(t *testing.T) {
	const phaseTalk = PhaseType("TALK")
	cfg := testConfig()
	cfg.Phases[phaseTalk] = &PhaseConfig{
		Type: phaseTalk,
		Steps: []PhaseStep{
			{Role: RoleSystem, Skill: SkillAnnounce},               // 该念公告了
			{Role: roleVillager, Skill: skillVote, Required: true}, // 玩家的行动
		},
		NextPhase: phaseDay,
	}
	cfg.Phases[phaseNightGuard].NextPhase = phaseTalk

	opts := append(withNoopResolvers(), WithResolver(phaseTalk, noopResolver{}))
	e, err := NewEngine(cfg, opts...)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	mustAdd(t, e, "v", roleVillager)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	t.Run("没有公告的阶段", func(t *testing.T) {
		info := e.PhaseInfo()
		if info.NeedsGodAnnouncement() {
			t.Error("守卫阶段第一步不是公告")
		}
		if info.GodAnnouncementStep() != nil {
			t.Error("没有公告就不该给出公告步骤")
		}
	})

	if _, err := e.EndPhase(); err != nil { // -> TALK
		t.Fatalf("EndPhase: %v", err)
	}

	t.Run("有公告的阶段", func(t *testing.T) {
		info := e.PhaseInfo()
		if !info.NeedsGodAnnouncement() {
			t.Fatal("第一步是 RoleSystem + ANNOUNCE，主持台该念公告")
		}
		step := info.GodAnnouncementStep()
		if step == nil || step.Skill != SkillAnnounce {
			t.Fatalf("公告步骤读错了：%+v", step)
		}
	})

	t.Run("玩家的行动与公告分开列", func(t *testing.T) {
		steps := e.PhaseInfo().PlayerActionSteps()
		for _, s := range steps {
			if s.Role == RoleSystem {
				t.Error("PlayerActionSteps 不该含没有玩家承担的那一步")
			}
		}
		if len(steps) != 1 || steps[0].Skill != skillVote {
			t.Errorf("玩家该行动的那一步读错了：%+v", steps)
		}
	})
}

// TestEngine_CheapReaders 便宜的读法读的是真东西。
//
// AlivePlayerIDs 与 PhaseTimeout 此前覆盖率 0%。
func TestEngine_CheapReaders(t *testing.T) {
	e := newTestEngine(t, withNoopResolvers()...)
	mustAdd(t, e, "b", roleVillager)
	mustAdd(t, e, "a", roleWerewolf)
	mustAdd(t, e, "c", roleVillager)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	t.Run("存活名单按 ID 排序", func(t *testing.T) {
		got := e.AlivePlayerIDs()
		want := []string{"a", "b", "c"}
		if len(got) != len(want) {
			t.Fatalf("名单 = %v，期望 %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("名单没排序：%v", got)
			}
		}
	})

	t.Run("出局的人不在名单里", func(t *testing.T) {
		e.Apply(NewSetAliveEffect("b", false))
		for _, id := range e.AlivePlayerIDs() {
			if id == "b" {
				t.Error("出局的人不该出现在存活名单里")
			}
		}
	})

	t.Run("回合上下文是副本", func(t *testing.T) {
		rc := e.RoundContext()
		if rc == nil {
			t.Fatal("开局之后该有回合上下文")
		}
		rc.Vars = map[string]string{"tampered": "1"}
		if got := e.Var(ScopeRound, "tampered"); got != "" {
			t.Error("改副本不该改到引擎内部")
		}
	})
}

// TestConfig_PhaseTimeout 建议超时：阶段自己声明的优先，否则退回默认。
//
// 引擎**不据此计时**——它是给调用方的建议值。这一点值得有测试钉住，
// 免得日后有人以为引擎会自己超时推进。
func TestConfig_PhaseTimeout(t *testing.T) {
	cfg := testConfig()
	cfg.DefaultTimeout = 7 * time.Second
	cfg.Phases[phaseDay].Timeout = 99 * time.Second

	if got := cfg.PhaseTimeout(phaseDay); got != 99*time.Second {
		t.Errorf("阶段声明了超时就该用它，实际 %v", got)
	}
	if got := cfg.PhaseTimeout(phaseVote); got != 7*time.Second {
		t.Errorf("没声明就退回默认，实际 %v", got)
	}
	if got := cfg.PhaseTimeout(PhaseType("NOT_THERE")); got != 7*time.Second {
		t.Errorf("阶段不存在也该给默认值，实际 %v", got)
	}

	empty := &Config{}
	if got := empty.PhaseTimeout(phaseDay); got != DefaultPhaseTimeout {
		t.Errorf("连默认值都没设时退回 DefaultPhaseTimeout，实际 %v", got)
	}
}

// TestSkillUse_Target 单目标读法。
func TestSkillUse_Target(t *testing.T) {
	cases := []struct {
		use  *SkillUse
		want string
	}{
		{&SkillUse{Targets: []string{"a"}}, "a"},
		{&SkillUse{Targets: []string{"a", "b"}}, "a"},
		{&SkillUse{Targets: nil}, ""},
		{&SkillUse{Targets: []string{}}, ""},
	}
	for _, c := range cases {
		if got := c.use.Target(); got != c.want {
			t.Errorf("Target(%v) = %q，期望 %q", c.use.Targets, got, c.want)
		}
	}
}

// TestAddPlayer_RejectsBadSeats 入座的四种拒绝，宿主按错误码分支。
func TestAddPlayer_RejectsBadSeats(t *testing.T) {
	e := newTestEngine(t, withNoopResolvers()...)
	mustAdd(t, e, "v", roleVillager)

	cases := []struct {
		name string
		id   string
		role RoleType
		want error
	}{
		{"空 ID", "", roleVillager, ErrInvalidPlayerID},
		{"重复 ID", "v", roleWerewolf, ErrPlayerExists},
		{"系统角色不能入座", "sys", RoleSystem, ErrInvalidRole},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := e.AddPlayer(c.id, c.role); !errors.Is(err, c.want) {
				t.Errorf("应当拒成 %v，实际 %v", c.want, err)
			}
		})
	}

	t.Run("开局之后不能再入座", func(t *testing.T) {
		if err := e.Start(); err != nil {
			t.Fatalf("Start: %v", err)
		}
		if err := e.AddPlayer("late", roleVillager); !errors.Is(err, ErrGameAlreadyStarted) {
			t.Errorf("应当拒成 %v，实际 %v", ErrGameAlreadyStarted, err)
		}
	})
}

// TestRestoreEngine_RejectsBadSnapshots 坏快照要被拒绝。
func TestRestoreEngine_RejectsBadSnapshots(t *testing.T) {
	e := newTestEngine(t, withNoopResolvers()...)
	mustAdd(t, e, "v", roleVillager)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	good := e.Snapshot()

	t.Run("nil 快照", func(t *testing.T) {
		if _, err := RestoreEngine(testConfig(), nil, withNoopResolvers()...); !errors.Is(err, ErrNilSnapshot) {
			t.Errorf("应当拒成 %v，实际 %v", ErrNilSnapshot, err)
		}
	})

	t.Run("版本对不上", func(t *testing.T) {
		bad := *good
		bad.Version = SnapshotVersion + 1
		if _, err := RestoreEngine(testConfig(), &bad, withNoopResolvers()...); !HasCode(err, CodeInvalidSnapshot) {
			t.Errorf("应当拒成 %v，实际 %v", CodeInvalidSnapshot, CodeOf(err))
		}
	})

	t.Run("阶段不在配置里", func(t *testing.T) {
		bad := *good
		bad.Phase = PhaseType("NOT_IN_CONFIG")
		if _, err := RestoreEngine(testConfig(), &bad, withNoopResolvers()...); !HasCode(err, CodeInvalidSnapshot) {
			t.Errorf("应当拒成 %v，实际 %v", CodeInvalidSnapshot, CodeOf(err))
		}
	})
}
