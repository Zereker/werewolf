package werewolf

import (
	"errors"
	"sync"
	"testing"
)

// 本文件的建局默认走 rules_test.go 的 newRuleGame 辅助。
//
// 仍保留 mustAdd/mustStart 显式风格的用例只有四类，每一处都在注释里写明了原因：
// 期待 Start()/AddPlayer() 报错的、直接断言 engine.pendingUses 等内部字段的、
// 测试引擎构造本身（含开局前状态）的、以及并发用例。
// 这些用例换成 newRuleGame 之后就测不到它们原本想测的东西了。

// TestNewEngine_NilConfig 测的是构造函数与内部字段的初始化，不开局，
// 故不适用 newRuleGame。
func TestNewEngine_NilConfig(t *testing.T) {
	engine := MustNewEngine(nil)

	if engine.config == nil {
		t.Error("expected default config to be set")
	}
	if engine.state == nil {
		t.Error("expected state to be initialized")
	}
	if engine.phase == nil {
		t.Error("expected phase to be initialized")
	}
	if engine.pendingUses == nil {
		t.Error("expected pendingUses to be initialized")
	}
	if engine.eventHandlers == nil {
		t.Error("expected eventHandlers to be initialized")
	}
}

// TestNewEngine_CustomConfig 同上：测的是构造函数是否原样保留了传入的配置，
// 断言的是 engine.config 这个内部字段的指针身份。
func TestNewEngine_CustomConfig(t *testing.T) {
	config := &GameConfig{
		WitchCanSaveSelf: true,
		Phases:           DefaultGameConfig().Phases,
	}
	engine := MustNewEngine(config)

	if engine.config != config {
		t.Error("expected custom config to be set")
	}
	if !engine.config.WitchCanSaveSelf {
		t.Error("expected WitchCanSaveSelf=true")
	}
}

// TestEngine_AddPlayer 只加人不开局（板子只有一只狼，Start 会被拒），
// 且断言的是 engine.state 里的玩家记录，故保留显式风格。
func TestEngine_AddPlayer(t *testing.T) {
	engine := MustNewEngine(nil)

	mustAdd(t, engine, "p1", RoleWerewolf)

	player, ok := engine.state.getPlayer("p1")
	if !ok {
		t.Fatal("expected player to be added")
	}
	if player.Role != RoleWerewolf {
		t.Errorf("expected Role=WEREWOLF, got %v", player.Role)
	}
}

func TestEngine_Start(t *testing.T) {
	// newRuleGame 内部即断言 Start() 必须成功（失败则 Fatal）
	g := newRuleGame(t, nil, wolf("w1"), villager("v1"))

	if g.e.Phase() != PhaseNightGuard {
		t.Errorf("expected Phase=NIGHT_GUARD, got %v", g.e.Phase())
	}
	if g.e.Round() != 1 {
		t.Errorf("expected Round=1, got %d", g.e.Round())
	}
}

func TestEngine_Start_AlreadyStarted(t *testing.T) {
	// 首次 Start 由 newRuleGame 完成并断言成功，这里只看重复 Start
	g := newRuleGame(t, nil, wolf("w1"), villager("v1"))

	if err := g.e.Start(); err != ErrGameAlreadyStarted {
		t.Errorf("重复 Start 应返回 ErrGameAlreadyStarted，实际 %v", err)
	}
}

// TestEngine_Start_RejectsInvalidBoard 故意用不合法的板子并期待 Start() 报错，
// newRuleGame 会在建局时就 Fatal，测不到目标，故保留显式风格。
func TestEngine_Start_RejectsInvalidBoard(t *testing.T) {
	cases := []struct {
		name  string
		roles map[string]RoleType
		want  error
	}{
		{"空板子", nil, ErrNoWerewolf},
		{"只有狼", map[string]RoleType{"w1": RoleWerewolf}, ErrNoGoodPlayer},
		{"只有好人", map[string]RoleType{"v1": RoleVillager}, ErrNoWerewolf},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engine := MustNewEngine(nil)
			for id, role := range tc.roles {
				mustAdd(t, engine, id, role)
			}
			if err := engine.Start(); err != tc.want {
				t.Errorf("期望 %v，实际 %v", tc.want, err)
			}
		})
	}
}

// TestEngine_AddPlayer_Validation 故意用非法输入并期待 AddPlayer() 报错，
// 同理不能交给 newRuleGame 建局。
func TestEngine_AddPlayer_Validation(t *testing.T) {
	engine := MustNewEngine(nil)

	if err := engine.AddPlayer("", RoleVillager); err != ErrInvalidPlayerID {
		t.Errorf("空 ID 应返回 ErrInvalidPlayerID，实际 %v", err)
	}
	if err := engine.AddPlayer("god", RoleGod); !HasCode(err, CodeInvalidRole) {
		t.Errorf("上帝不是玩家身份，应返回 INVALID_ROLE，实际 %v", err)
	}

	if err := engine.AddPlayer("w1", RoleWerewolf); err != nil {
		t.Fatalf("正常添加应当成功，实际 %v", err)
	}
	if err := engine.AddPlayer("w1", RoleVillager); !HasCode(err, CodePlayerExists) {
		t.Errorf("重复 ID 应返回 PLAYER_EXISTS，实际 %v", err)
	}

	// 阵营由角色推导，调用方不再需要（也无法）传错
	w1, _ := engine.PlayerInfo("w1")
	if w1.Camp != CampEvil {
		t.Errorf("狼人阵营应为 EVIL，实际 %v", w1.Camp)
	}

	// 开局后不允许再改动玩家
	mustAdd(t, engine, "v1", RoleVillager)
	if err := engine.Start(); err != nil {
		t.Fatal(err)
	}
	if err := engine.AddPlayer("v2", RoleVillager); err != ErrGameAlreadyStarted {
		t.Errorf("开局后添加玩家应返回 ErrGameAlreadyStarted，实际 %v", err)
	}
	if _, ok := engine.PlayerInfo("v2"); ok {
		t.Error("被拒绝的玩家不应进入状态")
	}
}

// TestEngine_SubmitSkillUse_Valid 断言的是 engine.pendingUses 这个内部队列的
// 长度与元素内容（提交时是否补齐了 Phase/Round），故保留显式风格。
func TestEngine_SubmitSkillUse_Valid(t *testing.T) {
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "wolf", RoleWerewolf)
	mustAdd(t, engine, "guard", RoleGuard)
	mustAdd(t, engine, "victim", RoleVillager)
	mustStart(t, engine)

	// Guard can protect in NIGHT_GUARD phase
	err := engine.SubmitSkillUse(&SkillUse{
		PlayerID: "guard",
		Skill:    SkillProtect,
		TargetID: "victim",
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(engine.pendingUses) != 1 {
		t.Errorf("expected 1 pending use, got %d", len(engine.pendingUses))
	}

	// Verify phase and round are set on skill use
	use := engine.pendingUses[0]
	if use.Phase != PhaseNightGuard {
		t.Errorf("expected Phase=NIGHT_GUARD, got %v", use.Phase)
	}
	if use.Round != 1 {
		t.Errorf("expected Round=1, got %d", use.Round)
	}
}

func TestEngine_SubmitSkillUse_InvalidPlayer(t *testing.T) {
	g := newRuleGame(t, nil, wolf("wolf"), villager("v1"))

	err := g.use("nonexistent", SkillKill, "wolf")

	if err != ErrPlayerNotFound {
		t.Errorf("expected ErrPlayerNotFound, got %v", err)
	}
}

func TestEngine_SubmitSkillUse_DeadPlayer(t *testing.T) {
	g := newRuleGame(t, nil, wolf("wolf"), villager("victim"))
	// 开局之后再置为出局：开局前杀掉唯一的狼会让板子不合法
	g.setDead("wolf")

	err := g.use("wolf", SkillKill, "victim")

	if err != ErrPlayerDead {
		t.Errorf("expected ErrPlayerDead, got %v", err)
	}
}

func TestEngine_SubmitSkillUse_InvalidSkill(t *testing.T) {
	g := newRuleGame(t, nil, seats(wolf("wolf"), villagers("villager", "target"))...)

	// Villager cannot kill
	err := g.use("villager", SkillKill, "target")

	if err != ErrSkillNotAllowed {
		t.Errorf("expected ErrSkillNotAllowed, got %v", err)
	}
}

// TestEngine_EndPhase 断言 EndPhase 结束后 engine.pendingUses 被清空，
// 这是对内部队列的直接检查，故保留显式风格。
func TestEngine_EndPhase(t *testing.T) {
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "guard", RoleGuard)
	mustAdd(t, engine, "wolf", RoleWerewolf)
	mustAdd(t, engine, "v1", RoleVillager)
	mustStart(t, engine)

	// In NIGHT_GUARD phase, guard protects v1
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "guard",
		Skill:    SkillProtect,
		TargetID: "v1",
	})

	effects, err := engine.EndPhase()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Should have 2 effects: SET_LAST_PROTECTED + PROTECT
	if len(effects) != 2 {
		t.Errorf("expected 2 effects, got %d", len(effects))
	}

	// 检查包含 PROTECT effect
	if findEffect(effects, EventProtect) == nil {
		t.Error("expected to have EVENT_TYPE_PROTECT effect")
	}

	// Should transition to NIGHT_WOLF
	if engine.Phase() != PhaseNightWolf {
		t.Errorf("expected Phase=NIGHT_WOLF, got %v", engine.Phase())
	}

	// Pending uses should be cleared
	if len(engine.pendingUses) != 0 {
		t.Errorf("expected pending uses to be cleared, got %d", len(engine.pendingUses))
	}
}

func TestEngine_EndPhase_GameOver_WolvesWin(t *testing.T) {
	g := newRuleGame(t, nil, wolf("wolf"), villager("v1"))

	// NIGHT_GUARD phase - skip
	g.end(PhaseNightWolf)

	// NIGHT_WOLF phase - wolf kills v1
	g.mustUse("wolf", SkillKill, "v1")
	g.end(PhaseNightWitch)

	// Continue through night phases
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.endAny() // NIGHT_RESOLVE -> 击杀在此结算

	// v1 出局后平民全灭，狼人按屠边获胜
	if !g.e.IsGameOver() {
		t.Error("expected game to be over")
	}
	if g.e.Phase() != PhaseEnd {
		t.Errorf("expected Phase=END, got %v", g.e.Phase())
	}
}

func TestEngine_EndPhase_GameOver_GoodWins(t *testing.T) {
	g := newRuleGame(t, nil, seats(wolf("wolf"), villagers("v1", "v2"))...)

	// NIGHT_GUARD -> NIGHT_WOLF
	g.end(PhaseNightWolf)

	// NIGHT_WOLF: wolf kills v1
	g.mustUse("wolf", SkillKill, "v1")
	g.end(PhaseNightWitch)

	// NIGHT_WITCH -> NIGHT_SEER -> NIGHT_RESOLVE -> DAY (v1 killed here) -> VOTE
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseDay)
	g.end(PhaseVote)

	// VOTE: v2 votes wolf
	g.vote("wolf", "v2")
	g.endAny()

	// After voting wolf out, evil(0), good wins
	if !g.e.IsGameOver() {
		t.Error("expected game to be over")
	}
}

// TestEngine_EndPhase_AlreadyEnded 直接把 engine.state.Phase 写成 END 构造局面，
// 不经开局，故保留显式风格。
func TestEngine_EndPhase_AlreadyEnded(t *testing.T) {
	engine := MustNewEngine(nil)
	engine.state.Phase = PhaseEnd

	_, err := engine.EndPhase()
	if err != ErrGameEnded {
		t.Errorf("expected ErrGameEnded, got %v", err)
	}
}

// TestEngine_GetCurrentPhase 断言的重点之一是「开局之前」的阶段为 START，
// newRuleGame 建局即开局，拿不到这个时刻，故保留显式风格。
func TestEngine_GetCurrentPhase(t *testing.T) {
	engine := MustNewEngine(nil)

	if engine.Phase() != PhaseStart {
		t.Errorf("expected Phase=START, got %v", engine.Phase())
	}

	mustAdd(t, engine, "w1", RoleWerewolf)
	mustAdd(t, engine, "v1", RoleVillager)
	mustStart(t, engine)

	if engine.Phase() != PhaseNightGuard {
		t.Errorf("expected Phase=NIGHT_GUARD after start, got %v", engine.Phase())
	}
}

// TestEngine_GetCurrentRound 同上：断言「开局之前」Round=0。
func TestEngine_GetCurrentRound(t *testing.T) {
	engine := MustNewEngine(nil)

	if engine.Round() != 0 {
		t.Errorf("expected Round=0, got %d", engine.Round())
	}

	mustAdd(t, engine, "w1", RoleWerewolf)
	mustAdd(t, engine, "v1", RoleVillager)
	mustStart(t, engine)

	if engine.Round() != 1 {
		t.Errorf("expected Round=1 after start, got %d", engine.Round())
	}
}

func TestEngine_GetAllowedSkills(t *testing.T) {
	g := newRuleGame(t, nil, wolf("wolf"), guard("guard"))

	// In NIGHT_GUARD phase, guard can protect
	skills := g.e.AllowedSkills("guard")

	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}
	if skills[0] != SkillProtect {
		t.Errorf("expected PROTECT, got %v", skills[0])
	}
}

func TestEngine_GetAllowedSkills_Dead(t *testing.T) {
	g := newRuleGame(t, nil, wolf("wolf"), villager("v1"))
	g.setDead("wolf")

	skills := g.e.AllowedSkills("wolf")

	// 空切片而非 nil：同一个字段序列化出去不该一会儿是 [] 一会儿是 null
	if skills == nil || len(skills) != 0 {
		t.Errorf("expected empty non-nil slice for dead player, got %v", skills)
	}
}

func TestEngine_GetAllowedSkills_NotFound(t *testing.T) {
	g := newRuleGame(t, nil, wolf("wolf"), villager("v1"))

	skills := g.e.AllowedSkills("nonexistent")

	if skills != nil {
		t.Errorf("expected nil for nonexistent player, got %v", skills)
	}
}

// TestEngine_IsGameOver 直接把 engine.state.Phase 写成 END，且要看开局前的
// IsGameOver=false，故保留显式风格。
func TestEngine_IsGameOver(t *testing.T) {
	engine := MustNewEngine(nil)

	if engine.IsGameOver() {
		t.Error("expected IsGameOver=false initially")
	}

	engine.state.Phase = PhaseEnd

	if !engine.IsGameOver() {
		t.Error("expected IsGameOver=true when Phase=END")
	}
}

func TestEngine_OnEvent(t *testing.T) {
	// Start() 本身不分发事件，故在开局之后注册 handler 与原先在开局之前注册等价
	g := newRuleGame(t, nil, guard("guard"), wolf("wolf"), villager("v1"))

	eventCount := 0
	g.e.OnEvent(func(event *Event) {
		eventCount++
	})

	// NIGHT_GUARD phase
	g.mustUse("guard", SkillProtect, "v1")
	g.end(PhaseNightWolf)

	// Should have 1 event (protect effect)
	if eventCount != 1 {
		t.Errorf("expected 1 event, got %d", eventCount)
	}
}

func TestEngine_MultipleHandlers(t *testing.T) {
	g := newRuleGame(t, nil, guard("guard"), wolf("wolf"), villager("v1"))

	count1 := 0
	count2 := 0

	g.e.OnEvent(func(event *Event) {
		count1++
	})
	g.e.OnEvent(func(event *Event) {
		count2++
	})

	// NIGHT_GUARD phase
	g.mustUse("guard", SkillProtect, "v1")
	g.end(PhaseNightWolf)

	if count1 != 1 {
		t.Errorf("expected handler1 called 1 time, got %d", count1)
	}
	if count2 != 1 {
		t.Errorf("expected handler2 called 1 time, got %d", count2)
	}
}

// TestEngine_Concurrency 并发用例：要自己控制多 goroutine 的读写时序，
// 不走 newRuleGame 的单线程推进辅助。
func TestEngine_Concurrency(t *testing.T) {
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "guard", RoleGuard)
	mustAdd(t, engine, "wolf", RoleWerewolf)
	mustAdd(t, engine, "v1", RoleVillager)
	mustStart(t, engine)

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = engine.state
			engine.Phase()
			engine.Round()
			engine.AllowedSkills("guard")
			engine.IsGameOver()
		}()
	}

	// Concurrent writes
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := engine.SubmitSkillUse(&SkillUse{
				PlayerID: "guard",
				Skill:    SkillProtect,
				TargetID: "v1",
			})
			if err != nil && err != ErrPlayerDead && err != ErrTargetDead {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEngine_FullGameCycle(t *testing.T) {
	// v2 保证 v1 出局后平民未被屠尽，游戏得以进入白天
	g := newRuleGame(t, nil, seats(
		wolf("wolf"), seer("seer"), guard("guard"),
		villagers("v1", "v2"),
	)...)

	// Night 1 - GUARD phase
	if g.e.Phase() != PhaseNightGuard {
		t.Errorf("expected NIGHT_GUARD phase, got %v", g.e.Phase())
	}
	if g.e.Round() != 1 {
		t.Error("expected Round 1")
	}

	// Guard protects seer
	g.mustUse("guard", SkillProtect, "seer")
	g.end(PhaseNightWolf)

	// Night 1 - WOLF phase: wolf kills v1
	g.mustUse("wolf", SkillKill, "v1")
	g.end(PhaseNightWitch)

	// Night 1 - WITCH phase (skip)
	g.end(PhaseNightSeer)

	// Night 1 - SEER phase: seer checks wolf
	g.mustUse("seer", SkillCheck, "wolf")
	effects := g.end(PhaseNightResolve)

	// Should have check result showing wolf
	hasCheckResult := false
	for _, e := range effects {
		if e.Type == EventCheck && e.Data["isGood"] == false {
			hasCheckResult = true
		}
	}
	if !hasCheckResult {
		t.Error("expected seer check to show wolf is evil")
	}

	// NIGHT_RESOLVE phase - apply night kills
	g.end(PhaseDay) // v1 killed here

	// v1 should be dead after NIGHT_RESOLVE
	g.assertAlive("v1", false, "expected v1 to be dead")

	// Day 1 -> Vote 1
	g.end(PhaseVote)

	// Vote out wolf
	g.vote("wolf", "seer", "guard")
	g.endAny()

	// wolf should be eliminated, good wins
	g.assertAlive("wolf", false, "expected wolf to be eliminated")

	// Game over (no wolves left)
	if !g.e.IsGameOver() {
		t.Error("game should be over")
	}
}

func TestEngine_GetPhaseInfo_NightGuard(t *testing.T) {
	g := newRuleGame(t, nil, guard("guard"), wolf("wolf"), villager("v1"))

	info := g.e.PhaseInfo()

	if info.Phase != PhaseNightGuard {
		t.Errorf("expected NIGHT_GUARD phase, got %v", info.Phase)
	}
	if info.Round != 1 {
		t.Errorf("expected round 1, got %d", info.Round)
	}
	if len(info.ActiveRoles) != 1 || info.ActiveRoles[0] != RoleGuard {
		t.Errorf("expected GUARD as active role")
	}

	guardInfo, ok := info.RoleInfos[RoleGuard]
	if !ok {
		t.Fatal("expected guard role info")
	}
	if len(guardInfo.PlayerIDs) != 1 || guardInfo.PlayerIDs[0] != "guard" {
		t.Errorf("expected guard player ID, got %v", guardInfo.PlayerIDs)
	}
	if len(guardInfo.AllowedSkills) != 1 || guardInfo.AllowedSkills[0] != SkillProtect {
		t.Errorf("expected PROTECT skill, got %v", guardInfo.AllowedSkills)
	}
}

func TestEngine_GetPhaseInfo_NightWolf(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("wolf1"), wolf("wolf2"), villagers("v1", "v2", "v3"),
	)...)

	g.end(PhaseNightWolf) // GUARD -> WOLF

	info := g.e.PhaseInfo()

	if info.Phase != PhaseNightWolf {
		t.Errorf("expected NIGHT_WOLF phase, got %v", info.Phase)
	}

	wolfInfo, ok := info.RoleInfos[RoleWerewolf]
	if !ok {
		t.Fatal("expected wolf role info")
	}
	if len(wolfInfo.PlayerIDs) != 2 {
		t.Errorf("expected 2 wolf player IDs, got %d", len(wolfInfo.PlayerIDs))
	}
	// Check teammates
	for _, wolfID := range wolfInfo.PlayerIDs {
		teammates, ok := wolfInfo.Teammates[wolfID]
		if !ok {
			t.Errorf("expected teammates for %s", wolfID)
			continue
		}
		if len(teammates) != 1 {
			t.Errorf("expected 1 teammate for %s, got %d", wolfID, len(teammates))
		}
	}
}

func TestEngine_GetPhaseInfo_NightWitch(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		witch("witch"), wolf("wolf"), villagers("v1", "v2", "v3"),
	)...)

	g.end(PhaseNightWolf) // GUARD -> WOLF

	// Wolf kills v1
	g.mustUse("wolf", SkillKill, "v1")
	g.end(PhaseNightWitch) // WOLF -> WITCH

	info := g.e.PhaseInfo()

	if info.Phase != PhaseNightWitch {
		t.Errorf("expected NIGHT_WITCH phase, got %v", info.Phase)
	}

	witchInfo, ok := info.RoleInfos[RoleWitch]
	if !ok {
		t.Fatal("expected witch role info")
	}
	if witchInfo.KillTarget != "v1" {
		t.Errorf("expected kill target v1, got %s", witchInfo.KillTarget)
	}
	if len(witchInfo.AllowedSkills) != 2 {
		t.Errorf("expected 2 skills (ANTIDOTE, POISON), got %d", len(witchInfo.AllowedSkills))
	}
}

func TestPhaseInfo_GodAnnouncement(t *testing.T) {
	g := newRuleGame(t, nil, guard("guard"), wolf("wolf"), villager("v1"))

	info := g.e.PhaseInfo()

	// 验证阶段步骤包含上帝公告
	if len(info.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(info.Steps))
	}

	// 验证第一步是上帝公告
	if !info.NeedsGodAnnouncement() {
		t.Error("expected god announcement needed")
	}

	godStep := info.GodAnnouncementStep()
	if godStep == nil {
		t.Fatal("expected god announcement step")
	}
	if godStep.Role != RoleGod {
		t.Errorf("expected GOD role, got %v", godStep.Role)
	}
	if godStep.Skill != SkillAnnounce {
		t.Errorf("expected ANNOUNCE skill, got %v", godStep.Skill)
	}

	// 验证玩家行动步骤
	actionSteps := info.PlayerActionSteps()
	if len(actionSteps) != 1 {
		t.Errorf("expected 1 action step, got %d", len(actionSteps))
	}
	if actionSteps[0].Role != RoleGuard {
		t.Errorf("expected GUARD role, got %v", actionSteps[0].Role)
	}
}

// ==================== 并发安全回归测试 ====================

// TestEngine_ConcurrentOnEventAndEndPhase 事件分发与 OnEvent 注册并发。
//
// 回归：publishEvent 此前在释放 e.mu 之后才遍历 e.eventHandlers，
// 与并发的 OnEvent 追加构成数据竞争（需 -race 才能发现）。
//
// 并发用例：EndPhase 由独立 goroutine 反复调用，不能走 newRuleGame 的
// 单线程推进辅助（g.end 会断言阶段流转，在并发下无意义）。
func TestEngine_ConcurrentOnEventAndEndPhase(t *testing.T) {
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "w1", RoleWerewolf)
	mustAdd(t, engine, "s", RoleSeer)
	mustAdd(t, engine, "v1", RoleVillager)
	mustAdd(t, engine, "v2", RoleVillager)
	mustAdd(t, engine, "v3", RoleVillager)
	mustStart(t, engine)

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			engine.OnEvent(func(*Event) {})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			engine.OnMessage(func(*Message, []string) {})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_, _ = engine.EndPhase()
			_ = engine.SendMessage("v1", "hi")
		}
	}()

	wg.Wait()
}

// TestEngine_HandlerPanicIsIsolatedAndLogged 单个 handler panic 不影响其他
// handler，且必须留下错误日志（此前是 `_ = recover()` 静默吞掉）。
func TestEngine_HandlerPanicIsIsolatedAndLogged(t *testing.T) {
	rec := &recordingLogger{}
	g := newRuleGameWith(t, nil, []EngineOption{WithLogger(rec)}, seats(
		wolf("w1"), villagers("v1", "v2", "v3"),
	)...)

	survivorCalled := false
	g.e.OnEvent(func(*Event) { panic("boom") })
	g.e.OnEvent(func(*Event) { survivorCalled = true })

	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "v1")
	// 走到 NIGHT_RESOLVE 之后，产生 KILL 事件
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseDay)

	if !survivorCalled {
		t.Error("前一个 handler panic 后，后续 handler 仍应被调用")
	}
	if !rec.hasError("event handler panicked") {
		t.Errorf("handler panic 应当被记录为 Error 日志，实际日志: %v", rec.errors)
	}
}

// recordingLogger 记录 Error 级别日志，用于断言 panic 被记录
type recordingLogger struct {
	mu     sync.Mutex
	errors []string
	infos  []string
}

func (l *recordingLogger) Debug(string, ...Field) {}
func (l *recordingLogger) Warn(string, ...Field)  {}
func (l *recordingLogger) Info(msg string, _ ...Field) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.infos = append(l.infos, msg)
}
func (l *recordingLogger) Error(msg string, _ ...Field) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errors = append(l.errors, msg)
}

func (l *recordingLogger) hasError(msg string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.errors {
		if e == msg {
			return true
		}
	}
	return false
}

// TestEngine_EndPhase_BeforeStart 未开局时不能推进阶段。
//
// EndPhase 此前只拒绝 END 阶段，从 START 推进会直接把游戏开起来，
// 绕过 Start 的板子校验与解析器校验；而 Start 此后永远返回
// 「已开始」，那些校验再也跑不到。
func TestEngine_EndPhase_BeforeStart(t *testing.T) {
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "w1", RoleWerewolf)
	mustAdd(t, engine, "v1", RoleVillager)

	if _, err := engine.EndPhase(); !errors.Is(err, ErrGameNotStarted) {
		t.Fatalf("未开局推进阶段应返回 ErrGameNotStarted，实际 %v", err)
	}
	if got := engine.Phase(); got != PhaseStart {
		t.Errorf("阶段不应变化，实际 %v", got)
	}

	// 非法板子（全好人）同样推不动，Start 的校验因此仍然有效
	bad := MustNewEngine(nil)
	mustAdd(t, bad, "v1", RoleVillager)
	if _, err := bad.EndPhase(); !errors.Is(err, ErrGameNotStarted) {
		t.Fatalf("期望 ErrGameNotStarted，实际 %v", err)
	}
	if err := bad.Start(); !errors.Is(err, ErrNoWerewolf) {
		t.Fatalf("板子校验应当仍然生效，实际 %v", err)
	}
}

// TestEngine_Start_DispatchesGameStarted 开局事件要推给 OnEvent 的订阅者。
func TestEngine_Start_DispatchesGameStarted(t *testing.T) {
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "w1", RoleWerewolf)
	mustAdd(t, engine, "v1", RoleVillager)

	var seen []EventType
	engine.OnEvent(func(ev *Event) { seen = append(seen, ev.Type) })

	if err := engine.Start(); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}
	if len(seen) != 1 || seen[0] != EventGameStarted {
		t.Errorf("期望收到 GAME_STARTED，实际 %v", seen)
	}
}

// TestEngine_AllowedSkills_MatchesPlayerView 三个入口对「谁能行动」必须同口径。
//
// 死亡技能阶段只有触发者能行动。AllowedSkills 此前按角色作答，
// 会告诉同为猎人的另一名玩家「你可以开枪」，而 PlayerView 与
// SubmitSkillUse 都不认这个答案。
func TestEngine_AllowedSkills_MatchesPlayerView(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("w1"), wolf("w2"), hunter("h1"), hunter("h2"), seer("s"),
		villagers("v1", "v2", "v3", "v4"),
	)...)

	// 刀死 h1，进入他的开枪阶段
	g.end(PhaseNightWolf)
	g.mustUse("w1", SkillKill, "h1")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseNightHunter)

	for _, id := range []string{"h1", "h2", "s", "w1"} {
		allowed := g.e.AllowedSkills(id)
		fromView := g.e.PlayerView(id).AllowedSkills
		if len(allowed) != len(fromView) {
			t.Errorf("%s: AllowedSkills=%v 与 PlayerView=%v 不一致", id, allowed, fromView)
		}
	}

	if got := g.e.AllowedSkills("h2"); len(got) != 0 {
		t.Errorf("触发者是 h1，h2 不该有可用技能，实际 %v", got)
	}
	// 而 SubmitSkillUse 也确实会拒掉 h2
	if err := g.use("h2", SkillShoot, "w1"); err == nil {
		t.Error("h2 不是触发者，开枪应当被拒")
	}
	if got := g.e.AllowedSkills("h1"); len(got) == 0 {
		t.Error("触发者 h1 应当可以开枪")
	}
}

// TestEngine_ResolverReturningNilEffect 第三方 Resolver 返回的切片里混进 nil，不该让整局崩掉。
//
// applyEffect 里有 nil 保护，但 advancePhase 的循环在它之前就先取了
// effect.Type / effect.Canceled——那道保护够不着。
func TestEngine_ResolverReturningNilEffect(t *testing.T) {
	engine := MustNewEngine(nil,
		WithResolver(PhaseNightGuard, resolverReturningNil{}))
	mustAdd(t, engine, "w1", RoleWerewolf)
	mustAdd(t, engine, "v1", RoleVillager)
	if err := engine.Start(); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}

	effects, err := engine.EndPhase()
	if err != nil {
		t.Fatalf("EndPhase 失败: %v", err)
	}
	for _, ef := range effects {
		if ef == nil {
			t.Error("nil 效果不该出现在返回值与效果流里")
		}
	}
	if got := engine.Phase(); got != PhaseNightWolf {
		t.Errorf("阶段应当照常流转，实际 %v", got)
	}
}

// resolverReturningNil 返回一个含 nil 的效果切片。
type resolverReturningNil struct{}

func (resolverReturningNil) Resolve([]*SkillUse, GameView, *GameConfig) []*Effect {
	return []*Effect{nil, NewEffect(EventSkip, "v1", "")}
}

// TestEngine_RoundBoundaryFollowsStartPhase 回合边界要跟着起始阶段走，而不是写死守卫阶段。
//
// 起始阶段是可配置的，阶段环也是。环里不含 NIGHT_GUARD 时，回合数永远
// 停在 1，回合上下文也永远不重置：上一夜的「被救」记录一直留着，
// 女巫那瓶用掉的解药会一夜又一夜地把同一个人救回来。
func TestEngine_RoundBoundaryFollowsStartPhase(t *testing.T) {
	cfg := DefaultGameConfig()
	cfg.StartPhase = PhaseNightWolf
	cfg.Phases[PhaseVote].NextPhase = PhaseNightWolf
	cfg.Phases[PhaseDayHunter].NextPhase = PhaseNightWolf
	delete(cfg.Phases, PhaseNightGuard)
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate 失败: %v", err)
	}

	g := newRuleGame(t, cfg, seats(
		wolf("w1"), wolf("w2"), witch("wi"), seer("s"),
		villagers("v1", "v2", "v3", "v4"),
	)...)

	// 第一夜：狼刀 v1，女巫用解药救回
	g.mustUse("w1", SkillKill, "v1")
	g.end(PhaseNightWitch)
	g.mustUse("wi", SkillAntidote, "v1")
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseDay)
	g.end(PhaseVote)
	g.end(PhaseNightWolf)
	g.assertAlive("v1", true, "第一夜被解药救回")

	if got := g.e.Round(); got != 2 {
		t.Errorf("绕回起始阶段应当进入第 2 回合，实际 %d", got)
	}
	if got := g.e.RoundContext().SavedPlayers; len(got) != 0 {
		t.Errorf("回合上下文应当已重置，实际还留着 SavedPlayers=%v", got)
	}

	// 第二夜再刀 v1，女巫的解药已经用完，这一刀必须命中
	g.mustUse("w1", SkillKill, "v1")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.endAny()
	g.assertAlive("v1", false, "解药已用完，第二夜的刀口应当死亡")
}

// TestEngine_EndPhase_ReturnsGameEnded 结束事件必须出现在 EndPhase 的返回值里。
//
// 引擎有三条出口：EndPhase 的返回值、OnEvent 的事件流、EffectLog。
// GAME_ENDED 此前只走后两条，照着「EndPhase -> AudienceOf -> 发给玩家」
// 路由的调用方会漏掉整局最重要的一件事——谁赢了。
func TestEngine_EndPhase_ReturnsGameEnded(t *testing.T) {
	g := newRuleGame(t, nil, seats(wolf("w1"), villagers("v1", "v2"))...)
	g.setDead("w1")

	effects, err := g.e.EndPhase()
	if err != nil {
		t.Fatalf("EndPhase 失败: %v", err)
	}

	ended := findEffect(effects, EventGameEnded)
	if ended == nil {
		t.Fatal("EndPhase 的返回值里应当包含 GAME_ENDED")
	}
	if got, ok := ended.Data["winner"].(Camp); !ok || got != CampGood {
		t.Errorf("胜方: 期望 CampGood，实际 %v", ended.Data["winner"])
	}

	// 而且它得有受众——不然调用方还是不知道该发给谁
	audience, known := g.e.AudienceOf(ended)
	if !known || len(audience) == 0 {
		t.Errorf("GAME_ENDED 应当是全场可见，实际 (%v, %v)", audience, known)
	}
}
