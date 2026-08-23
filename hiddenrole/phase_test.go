package hiddenrole

import (
	"errors"
	"testing"
)

// TestNewPhase 内核造出来的阶段机不带任何解析器。
//
// 这是拆分的核心断言：内核不知道 NIGHT_WITCH 该由谁结算。此前这里装着
// 一张九个阶段的默认表，也就是内核认得狼人杀的整套流程。
func TestNewPhase(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	if p.config != config {
		t.Error("expected config to be set")
	}
	if len(p.resolvers) != 0 {
		t.Errorf("内核不该自带解析器，实际 %d 个", len(p.resolvers))
	}
}

func TestGetPhaseConfig(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	dayConfig := p.phaseConfig(phaseDay)
	if dayConfig == nil {
		t.Fatal("expected day config")
	}

	voteConfig := p.phaseConfig(phaseVote)
	if voteConfig == nil {
		t.Fatal("expected vote config")
	}

	guardConfig := p.phaseConfig(phaseNightGuard)
	if guardConfig == nil {
		t.Fatal("expected guard phase config")
	}
}

func TestGetPhaseConfig_Invalid(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	startConfig := p.phaseConfig(PhaseStart)
	if startConfig != nil {
		t.Error("expected nil for START phase config")
	}

	endConfig := p.phaseConfig(PhaseEnd)
	if endConfig != nil {
		t.Error("expected nil for END phase config")
	}
}

// TestGetResolver 注册进去的解析器要能原样取回来。
//
// 内核不认得任何具体解析器——这里取回的是测试自己注册的那个。
func TestGetResolver(t *testing.T) {
	marker := noopResolver{}
	e := newTestEngine(t, append(withNoopResolvers(),
		WithResolver(phaseDay, marker))...)

	got := e.phase.resolver(phaseDay)
	if got == nil {
		t.Fatal("注册过的阶段应当取得到解析器")
	}
	if got != Resolver(marker) {
		t.Errorf("取回的不是注册进去的那个: %#v", got)
	}
	if e.phase.resolver(PhaseType("从未注册")) != nil {
		t.Error("没注册过的阶段应当返回 nil")
	}
}

func TestGetResolver_Nil(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	startResolver := p.resolver(PhaseStart)
	if startResolver != nil {
		t.Error("expected nil for START phase resolver")
	}
}

func TestGetAllowedSkills_Guard(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	skills := p.allowedSkills(phaseNightGuard, roleGuard)

	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}
	if skills[0] != skillProtect {
		t.Errorf("expected PROTECT, got %v", skills[0])
	}
}

func TestGetAllowedSkills_Werewolf(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	skills := p.allowedSkills(phaseNightWolf, roleWerewolf)

	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}
	if skills[0] != skillKill {
		t.Errorf("expected KILL, got %v", skills[0])
	}
}

func TestGetAllowedSkills_Witch(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	skills := p.allowedSkills(phaseNightWitch, roleWitch)

	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(skills))
	}

	hasAntidote := false
	hasPoison := false
	for _, s := range skills {
		if s == skillAntidote {
			hasAntidote = true
		}
		if s == skillPoison {
			hasPoison = true
		}
	}

	if !hasAntidote {
		t.Error("expected ANTIDOTE skill")
	}
	if !hasPoison {
		t.Error("expected POISON skill")
	}
}

func TestGetAllowedSkills_Seer(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	skills := p.allowedSkills(phaseNightSeer, roleSeer)

	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}
	if skills[0] != skillCheck {
		t.Errorf("expected CHECK, got %v", skills[0])
	}
}

func TestGetAllowedSkills_Villager_NightGuard(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	skills := p.allowedSkills(phaseNightGuard, roleVillager)

	// Villager has no skills in guard phase
	if len(skills) != 0 {
		t.Errorf("expected 0 skills for villager in guard phase, got %d", len(skills))
	}
}

func TestGetAllowedSkills_DayHasNoPlayerSkill(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	// 白天没有玩家技能——发言不是技能，走 SendMessage。
	// 此前 DAY 阶段声明了 SPEAK，提交能通过但结算零效果，是个悬空概念。
	roles := []RoleType{
		roleWerewolf,
		roleSeer,
		roleWitch,
		roleGuard,
		roleVillager,
	}
	for _, role := range roles {
		if skills := p.allowedSkills(phaseDay, role); len(skills) != 0 {
			t.Errorf("白天不应有可提交技能，%v 得到 %v", role, skills)
		}
	}
}

func TestGetAllowedSkills_AllVote(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	// All roles should be able to vote
	roles := []RoleType{
		roleWerewolf,
		roleSeer,
		roleWitch,
		roleGuard,
		roleVillager,
	}

	for _, role := range roles {
		skills := p.allowedSkills(phaseVote, role)
		if len(skills) != 1 {
			t.Errorf("expected 1 skill for %v during vote, got %d", role, len(skills))
		}
		if len(skills) > 0 && skills[0] != skillVote {
			t.Errorf("expected VOTE for %v during vote, got %v", role, skills[0])
		}
	}
}

func TestGetAllowedSkills_InvalidPhase(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	skills := p.allowedSkills(PhaseStart, roleWerewolf)
	if skills == nil || len(skills) != 0 {
		t.Errorf("expected empty non-nil slice for invalid phase, got %v", skills)
	}
}

func TestNextSubPhase_StartToGuard(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	next := p.nextSubPhase(PhaseStart)
	if next != phaseNightGuard {
		t.Errorf("expected NIGHT_GUARD, got %v", next)
	}
}

func TestNextSubPhase_NightFlow(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	// Guard -> Wolf -> Witch -> Seer -> Resolve -> Day
	next := p.nextSubPhase(phaseNightGuard)
	if next != phaseNightWolf {
		t.Errorf("expected NIGHT_WOLF, got %v", next)
	}

	next = p.nextSubPhase(phaseNightWolf)
	if next != phaseNightWitch {
		t.Errorf("expected NIGHT_WITCH, got %v", next)
	}

	next = p.nextSubPhase(phaseNightWitch)
	if next != phaseNightSeer {
		t.Errorf("expected NIGHT_SEER, got %v", next)
	}

	next = p.nextSubPhase(phaseNightSeer)
	if next != phaseNightResolve {
		t.Errorf("expected NIGHT_RESOLVE, got %v", next)
	}

	next = p.nextSubPhase(phaseNightResolve)
	if next != phaseDay {
		t.Errorf("expected DAY, got %v", next)
	}
}

func TestNextSubPhase_DayToVote(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	next := p.nextSubPhase(phaseDay)
	if next != phaseVote {
		t.Errorf("expected VOTE, got %v", next)
	}
}

func TestNextSubPhase_VoteToGuard(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	next := p.nextSubPhase(phaseVote)
	if next != phaseNightGuard {
		t.Errorf("expected NIGHT_GUARD, got %v", next)
	}
}

func TestNextSubPhase_EndStays(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	next := p.nextSubPhase(PhaseEnd)
	if next != PhaseEnd {
		t.Errorf("expected END, got %v", next)
	}
}

func TestValidateSkillUse_Valid(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	state := newState()
	mustAddTo(t, state, "wolf", roleWerewolf)
	mustAddTo(t, state, "victim", roleVillager)
	state.Phase = phaseNightWolf

	use := &SkillUse{
		PlayerID: "wolf",
		Skill:    skillKill,
		Targets:  []string{"victim"},
		Phase:    phaseNightWolf,
	}

	err := p.validateSkillUse(use, state)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateSkillUse_PlayerNotFound(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	state := newState()
	state.Phase = phaseNightWolf

	use := &SkillUse{
		PlayerID: "nonexistent",
		Skill:    skillKill,
		Targets:  []string{"victim"},
		Phase:    phaseNightWolf,
	}

	err := p.validateSkillUse(use, state)
	if err != ErrPlayerNotFound {
		t.Errorf("expected ErrPlayerNotFound, got %v", err)
	}
}

func TestValidateSkillUse_PlayerDead(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	state := newState()
	mustAddTo(t, state, "wolf", roleWerewolf)
	mustAddTo(t, state, "victim", roleVillager)
	state.players["wolf"].Alive = false
	state.Phase = phaseNightWolf

	use := &SkillUse{
		PlayerID: "wolf",
		Skill:    skillKill,
		Targets:  []string{"victim"},
		Phase:    phaseNightWolf,
	}

	err := p.validateSkillUse(use, state)
	if err != ErrPlayerDead {
		t.Errorf("expected ErrPlayerDead, got %v", err)
	}
}

func TestValidateSkillUse_SkillNotAllowed(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	state := newState()
	mustAddTo(t, state, "villager", roleVillager)
	mustAddTo(t, state, "victim", roleVillager)
	state.Phase = phaseNightWolf

	// Villager tries to kill at night (not allowed)
	use := &SkillUse{
		PlayerID: "villager",
		Skill:    skillKill,
		Targets:  []string{"victim"},
		Phase:    phaseNightWolf,
	}

	err := p.validateSkillUse(use, state)
	if err != ErrSkillNotAllowed {
		t.Errorf("expected ErrSkillNotAllowed, got %v", err)
	}
}

func TestValidateSkillUse_TargetNotFound(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	state := newState()
	mustAddTo(t, state, "wolf", roleWerewolf)
	state.Phase = phaseNightWolf

	use := &SkillUse{
		PlayerID: "wolf",
		Skill:    skillKill,
		Targets:  []string{"nonexistent"},
		Phase:    phaseNightWolf,
	}

	err := p.validateSkillUse(use, state)
	if err != ErrTargetNotFound {
		t.Errorf("expected ErrTargetNotFound, got %v", err)
	}
}

func TestValidateSkillUse_TargetDead(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	state := newState()
	mustAddTo(t, state, "wolf", roleWerewolf)
	mustAddTo(t, state, "victim", roleVillager)
	state.players["victim"].Alive = false
	state.Phase = phaseNightWolf

	use := &SkillUse{
		PlayerID: "wolf",
		Skill:    skillKill,
		Targets:  []string{"victim"},
		Phase:    phaseNightWolf,
	}

	err := p.validateSkillUse(use, state)
	if err != ErrTargetDead {
		t.Errorf("expected ErrTargetDead, got %v", err)
	}
}

func TestValidateSkillUse_AntidoteOnDead(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	state := newState()
	mustAddTo(t, state, "witch", roleWitch)
	mustAddTo(t, state, "victim", roleVillager)
	state.players["victim"].Alive = false
	state.Phase = phaseNightWitch

	// Antidote can be used on dead target
	use := &SkillUse{
		PlayerID: "witch",
		Skill:    skillAntidote,
		Targets:  []string{"victim"},
		Phase:    phaseNightWitch,
	}

	err := p.validateSkillUse(use, state)
	if err != nil {
		t.Errorf("expected no error for antidote on dead, got %v", err)
	}
}

func TestValidateSkillUse_NoTarget(t *testing.T) {
	config := testConfig()
	p := newPhaseManager(config)

	state := newState()
	mustAddTo(t, state, "wolf", roleWerewolf)
	state.Phase = phaseNightWolf

	// Empty target (wolf chooses not to kill)
	use := &SkillUse{
		PlayerID: "wolf",
		Skill:    skillKill,
		Targets:  []string{""},
		Phase:    phaseNightWolf,
	}

	err := p.validateSkillUse(use, state)
	if err != nil {
		t.Errorf("expected no error for empty target, got %v", err)
	}
}

// TestWatchOnlyStep 技能留空的步骤：他该醒了，但他没有行动。
//
// 这一条是第三套规则包（一夜狼人）撞出来的：爪牙睁眼看谁是狼、守夜人互认、
// 失眠者看自己的牌——只接收信息，不提交任何东西。此前表达不了，规则包只能
// 挂一个 SKIP 当占位，而 SKIP 的意思是「主动放弃行动」，他不是放弃。
//
// 三件事要同时成立，缺一件这个特性就没用：
//
//	不出现在 AllowedSkills 里    他没有可提交的东西
//	不进入就绪判定               没有东西可满足，否则阶段永远不就绪
//	**出现在 ActiveRoles 里**    主持人得知道该叫醒谁——全部意义所在
func TestWatchOnlyStep(t *testing.T) {
	const phaseWatch = PhaseType("WATCH")
	cfg := testConfig()
	cfg.Phases[phaseWatch] = &PhaseConfig{
		Type: phaseWatch,
		Steps: []PhaseStep{
			{Role: roleSeer}, // 留空：醒过来看一眼
			{Role: roleWitch, Skill: skillPoison, Required: true}, // 对照组
		},
		NextPhase: phaseDay,
	}
	cfg.Phases[phaseNightGuard].NextPhase = phaseWatch

	opts := append(withNoopResolvers(), WithResolver(phaseWatch, noopResolver{}))
	e, err := NewEngine(cfg, opts...)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	mustAdd(t, e, "s", roleSeer)
	mustAdd(t, e, "wi", roleWitch)
	mustAdd(t, e, "v", roleVillager)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := e.EndPhase(); err != nil { // NIGHT_GUARD -> WATCH
		t.Fatalf("EndPhase: %v", err)
	}
	if got := e.Status().Phase; got != phaseWatch {
		t.Fatalf("阶段 = %v，期望 %v", got, phaseWatch)
	}

	t.Run("不出现在 AllowedSkills 里", func(t *testing.T) {
		if got := e.AllowedSkills("s"); len(got) != 0 {
			t.Errorf("他没有可提交的技能，AllowedSkills 却给出 %v", got)
		}
		if got := e.PlayerView("s").AllowedSkills; len(got) != 0 {
			t.Errorf("PlayerView 也不该给出，实际 %v", got)
		}
	})

	t.Run("提交不了", func(t *testing.T) {
		err := e.SubmitSkillUse(&SkillUse{PlayerID: "s", Skill: SkillUnspecified})
		if err == nil {
			t.Error("留空的技能不是一次行动，提交该被拒")
		}
	})

	t.Run("不进入就绪判定", func(t *testing.T) {
		rd := e.PhaseReadiness()
		for _, p := range append(append([]PendingAction{}, rd.Pending...), rd.Optional...) {
			if p.Role == roleSeer {
				t.Errorf("醒过来看一眼的人不该出现在就绪判定里：%+v", p)
			}
		}
		// 对照组：女巫那一步是必需的，阶段因此不就绪。
		if rd.Ready {
			t.Error("女巫那一步是必需的且没人提交，阶段不该就绪")
		}
	})

	t.Run("出现在 ActiveRoles 里", func(t *testing.T) {
		var found bool
		for _, role := range e.PhaseInfo().ActiveRoles {
			if role == roleSeer {
				found = true
			}
		}
		if !found {
			t.Error("主持人得知道该叫醒谁——这正是留空步骤存在的全部理由")
		}
	})

	t.Run("阶段能推进", func(t *testing.T) {
		// 留空的步骤不该把阶段卡住：女巫提交之后就该就绪。
		if err := e.SubmitSkillUse(&SkillUse{
			PlayerID: "wi", Skill: skillPoison, Targets: []string{"v"},
		}); err != nil {
			t.Fatalf("女巫提交: %v", err)
		}
		if !e.PhaseReadiness().Ready {
			t.Error("必需的那一步满足了，阶段就该就绪")
		}
	})
}

// TestSkip_HasNoKernelPrivilege 弃权在内核眼里不是特殊技能。
//
// 此前 validateSkillUse 里有一条「弃权不需要目标，直接放行」。那条是空的：
// 不带目标的提交本来就过得了目标校验（循环一次都不跑），而带了目标的提交
// **本该**被校验。它唯一的实际效果是让内核认得一个具体技能——
// 而「内核不认得任何取值」是这个库的五条不变量之一。
//
// 删掉之后要保证两件事：不带目标照样能提交，带了坏目标会被拒。
func TestSkip_HasNoKernelPrivilege(t *testing.T) {
	const phasePass = PhaseType("PASS")
	cfg := testConfig()
	cfg.Phases[phasePass] = &PhaseConfig{
		Type: phasePass,
		Steps: []PhaseStep{
			{Role: roleVillager, Skill: skillVote, Group: "act"},
			{Role: roleVillager, Skill: SkillSkip, Group: "act"},
		},
		NextPhase: phaseDay,
	}
	cfg.Phases[phaseNightGuard].NextPhase = phasePass

	opts := append(withNoopResolvers(), WithResolver(phasePass, noopResolver{}))
	e, err := NewEngine(cfg, opts...)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	mustAdd(t, e, "v", roleVillager)
	mustAdd(t, e, "w", roleWerewolf)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := e.EndPhase(); err != nil { // NIGHT_GUARD -> PASS
		t.Fatalf("EndPhase: %v", err)
	}
	if got := e.Status().Phase; got != phasePass {
		t.Fatalf("阶段 = %v，期望 %v", got, phasePass)
	}

	t.Run("不带目标能提交", func(t *testing.T) {
		if err := e.SubmitSkillUse(&SkillUse{PlayerID: "v", Skill: SkillSkip}); err != nil {
			t.Errorf("弃权不需要目标，提交却被拒：%v", err)
		}
	})

	t.Run("带了不存在的目标会被拒", func(t *testing.T) {
		err := e.SubmitSkillUse(&SkillUse{
			PlayerID: "v", Skill: SkillSkip, Targets: []string{"ghost"},
		})
		if !errors.Is(err, ErrTargetNotFound) {
			t.Errorf("目标不存在该拒成 %v，实际 %v——"+
				"弃权在内核眼里不是特殊技能", ErrTargetNotFound, err)
		}
	})
}
