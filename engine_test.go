package werewolf

import (
	"sync"
	"testing"

	pb "github.com/Zereker/werewolf/proto"
)

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

func TestEngine_AddPlayer(t *testing.T) {
	engine := MustNewEngine(nil)

	mustAdd(t, engine, "p1", pb.RoleType_ROLE_TYPE_WEREWOLF)

	player, ok := engine.state.getPlayer("p1")
	if !ok {
		t.Fatal("expected player to be added")
	}
	if player.Role != pb.RoleType_ROLE_TYPE_WEREWOLF {
		t.Errorf("expected Role=WEREWOLF, got %v", player.Role)
	}
}

func TestEngine_Start(t *testing.T) {
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "w1", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)

	err := engine.Start()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if engine.GetCurrentPhase() != pb.PhaseType_PHASE_TYPE_NIGHT_GUARD {
		t.Errorf("expected Phase=NIGHT_GUARD, got %v", engine.GetCurrentPhase())
	}
	if engine.GetCurrentRound() != 1 {
		t.Errorf("expected Round=1, got %d", engine.GetCurrentRound())
	}
}

func TestEngine_Start_AlreadyStarted(t *testing.T) {
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "w1", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)

	if err := engine.Start(); err != nil {
		t.Fatalf("首次 Start 应当成功，实际 %v", err)
	}

	if err := engine.Start(); err != ErrGameAlreadyStarted {
		t.Errorf("重复 Start 应返回 ErrGameAlreadyStarted，实际 %v", err)
	}
}

func TestEngine_Start_RejectsInvalidBoard(t *testing.T) {
	cases := []struct {
		name  string
		roles map[string]pb.RoleType
		want  error
	}{
		{"空板子", nil, ErrNoWerewolf},
		{"只有狼", map[string]pb.RoleType{"w1": pb.RoleType_ROLE_TYPE_WEREWOLF}, ErrNoGoodPlayer},
		{"只有好人", map[string]pb.RoleType{"v1": pb.RoleType_ROLE_TYPE_VILLAGER}, ErrNoWerewolf},
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

func TestEngine_AddPlayer_Validation(t *testing.T) {
	engine := MustNewEngine(nil)

	if err := engine.AddPlayer("", pb.RoleType_ROLE_TYPE_VILLAGER); err != ErrInvalidPlayerID {
		t.Errorf("空 ID 应返回 ErrInvalidPlayerID，实际 %v", err)
	}
	if err := engine.AddPlayer("god", pb.RoleType_ROLE_TYPE_GOD); !IsErrorCode(err, pb.ErrorCode_ERROR_CODE_INVALID_ROLE) {
		t.Errorf("上帝不是玩家身份，应返回 INVALID_ROLE，实际 %v", err)
	}

	if err := engine.AddPlayer("w1", pb.RoleType_ROLE_TYPE_WEREWOLF); err != nil {
		t.Fatalf("正常添加应当成功，实际 %v", err)
	}
	if err := engine.AddPlayer("w1", pb.RoleType_ROLE_TYPE_VILLAGER); !IsErrorCode(err, pb.ErrorCode_ERROR_CODE_PLAYER_EXISTS) {
		t.Errorf("重复 ID 应返回 PLAYER_EXISTS，实际 %v", err)
	}

	// 阵营由角色推导，调用方不再需要（也无法）传错
	w1, _ := engine.GetPlayerInfo("w1")
	if w1.Camp != pb.Camp_CAMP_EVIL {
		t.Errorf("狼人阵营应为 EVIL，实际 %v", w1.Camp)
	}

	// 开局后不允许再改动玩家
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	if err := engine.Start(); err != nil {
		t.Fatal(err)
	}
	if err := engine.AddPlayer("v2", pb.RoleType_ROLE_TYPE_VILLAGER); err != ErrGameAlreadyStarted {
		t.Errorf("开局后添加玩家应返回 ErrGameAlreadyStarted，实际 %v", err)
	}
	if _, ok := engine.GetPlayerInfo("v2"); ok {
		t.Error("被拒绝的玩家不应进入状态")
	}
}

func TestEngine_SubmitSkillUse_Valid(t *testing.T) {
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "guard", pb.RoleType_ROLE_TYPE_GUARD)
	mustAdd(t, engine, "victim", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustStart(t, engine)

	// Guard can protect in NIGHT_GUARD phase
	err := engine.SubmitSkillUse(&SkillUse{
		PlayerID: "guard",
		Skill:    pb.SkillType_SKILL_TYPE_PROTECT,
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
	if use.Phase != pb.PhaseType_PHASE_TYPE_NIGHT_GUARD {
		t.Errorf("expected Phase=NIGHT_GUARD, got %v", use.Phase)
	}
	if use.Round != 1 {
		t.Errorf("expected Round=1, got %d", use.Round)
	}
}

func TestEngine_SubmitSkillUse_InvalidPlayer(t *testing.T) {
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustStart(t, engine)

	err := engine.SubmitSkillUse(&SkillUse{
		PlayerID: "nonexistent",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "wolf",
	})

	if err != ErrPlayerNotFound {
		t.Errorf("expected ErrPlayerNotFound, got %v", err)
	}
}

func TestEngine_SubmitSkillUse_DeadPlayer(t *testing.T) {
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "victim", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustStart(t, engine)
	// 开局之后再置为出局：开局前杀掉唯一的狼会让板子不合法
	engine.state.players["wolf"].Alive = false

	err := engine.SubmitSkillUse(&SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "victim",
	})

	if err != ErrPlayerDead {
		t.Errorf("expected ErrPlayerDead, got %v", err)
	}
}

func TestEngine_SubmitSkillUse_InvalidSkill(t *testing.T) {
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "villager", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "target", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustStart(t, engine)

	// Villager cannot kill
	err := engine.SubmitSkillUse(&SkillUse{
		PlayerID: "villager",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "target",
	})

	if err != ErrSkillNotAllowed {
		t.Errorf("expected ErrSkillNotAllowed, got %v", err)
	}
}

func TestEngine_EndPhase(t *testing.T) {
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "guard", pb.RoleType_ROLE_TYPE_GUARD)
	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustStart(t, engine)

	// In NIGHT_GUARD phase, guard protects v1
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "guard",
		Skill:    pb.SkillType_SKILL_TYPE_PROTECT,
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
	hasProtect := false
	for _, e := range effects {
		if e.Type == pb.EventType_EVENT_TYPE_PROTECT {
			hasProtect = true
			break
		}
	}
	if !hasProtect {
		t.Error("expected to have EVENT_TYPE_PROTECT effect")
	}

	// Should transition to NIGHT_WOLF
	if engine.GetCurrentPhase() != pb.PhaseType_PHASE_TYPE_NIGHT_WOLF {
		t.Errorf("expected Phase=NIGHT_WOLF, got %v", engine.GetCurrentPhase())
	}

	// Pending uses should be cleared
	if len(engine.pendingUses) != 0 {
		t.Errorf("expected pending uses to be cleared, got %d", len(engine.pendingUses))
	}
}

func TestEngine_EndPhase_GameOver_WolvesWin(t *testing.T) {
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustStart(t, engine)

	// NIGHT_GUARD phase - skip
	mustEnd(t, engine)

	// NIGHT_WOLF phase - wolf kills v1
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "v1",
	})
	mustEnd(t, engine) // -> NIGHT_WITCH

	// Continue through night phases
	mustEnd(t, engine) // NIGHT_WITCH -> NIGHT_SEER
	mustEnd(t, engine) // NIGHT_SEER -> NIGHT_RESOLVE
	mustEnd(t, engine) // NIGHT_RESOLVE -> 击杀在此结算

	// v1 出局后平民全灭，狼人按屠边获胜
	if !engine.IsGameOver() {
		t.Error("expected game to be over")
	}
	if engine.GetCurrentPhase() != pb.PhaseType_PHASE_TYPE_END {
		t.Errorf("expected Phase=END, got %v", engine.GetCurrentPhase())
	}
}

func TestEngine_EndPhase_GameOver_GoodWins(t *testing.T) {
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v2", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustStart(t, engine)

	// NIGHT_GUARD -> NIGHT_WOLF
	mustEnd(t, engine)

	// NIGHT_WOLF: wolf kills v1
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "v1",
	})
	mustEnd(t, engine) // -> NIGHT_WITCH

	// NIGHT_WITCH -> NIGHT_SEER -> NIGHT_RESOLVE -> DAY -> VOTE
	mustEnd(t, engine) // NIGHT_WITCH -> NIGHT_SEER
	mustEnd(t, engine) // NIGHT_SEER -> NIGHT_RESOLVE
	mustEnd(t, engine) // NIGHT_RESOLVE -> DAY (v1 killed here)
	mustEnd(t, engine) // DAY -> VOTE

	// VOTE: v2 votes wolf
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "v2",
		Skill:    pb.SkillType_SKILL_TYPE_VOTE,
		TargetID: "wolf",
	})
	mustEnd(t, engine)

	// After voting wolf out, evil(0), good wins
	if !engine.IsGameOver() {
		t.Error("expected game to be over")
	}
}

func TestEngine_EndPhase_AlreadyEnded(t *testing.T) {
	engine := MustNewEngine(nil)
	engine.state.Phase = pb.PhaseType_PHASE_TYPE_END

	_, err := engine.EndPhase()
	if err != ErrGameEnded {
		t.Errorf("expected ErrGameEnded, got %v", err)
	}
}

func TestEngine_GetCurrentPhase(t *testing.T) {
	engine := MustNewEngine(nil)

	if engine.GetCurrentPhase() != pb.PhaseType_PHASE_TYPE_START {
		t.Errorf("expected Phase=START, got %v", engine.GetCurrentPhase())
	}

	mustAdd(t, engine, "w1", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustStart(t, engine)

	if engine.GetCurrentPhase() != pb.PhaseType_PHASE_TYPE_NIGHT_GUARD {
		t.Errorf("expected Phase=NIGHT_GUARD after start, got %v", engine.GetCurrentPhase())
	}
}

func TestEngine_GetCurrentRound(t *testing.T) {
	engine := MustNewEngine(nil)

	if engine.GetCurrentRound() != 0 {
		t.Errorf("expected Round=0, got %d", engine.GetCurrentRound())
	}

	mustAdd(t, engine, "w1", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustStart(t, engine)

	if engine.GetCurrentRound() != 1 {
		t.Errorf("expected Round=1 after start, got %d", engine.GetCurrentRound())
	}
}

func TestEngine_GetAllowedSkills(t *testing.T) {
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "guard", pb.RoleType_ROLE_TYPE_GUARD)
	mustStart(t, engine)

	// In NIGHT_GUARD phase, guard can protect
	skills := engine.GetAllowedSkills("guard")

	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}
	if skills[0] != pb.SkillType_SKILL_TYPE_PROTECT {
		t.Errorf("expected PROTECT, got %v", skills[0])
	}
}

func TestEngine_GetAllowedSkills_Dead(t *testing.T) {
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustStart(t, engine)
	engine.state.players["wolf"].Alive = false

	skills := engine.GetAllowedSkills("wolf")

	if skills != nil {
		t.Errorf("expected nil for dead player, got %v", skills)
	}
}

func TestEngine_GetAllowedSkills_NotFound(t *testing.T) {
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustStart(t, engine)

	skills := engine.GetAllowedSkills("nonexistent")

	if skills != nil {
		t.Errorf("expected nil for nonexistent player, got %v", skills)
	}
}

func TestEngine_IsGameOver(t *testing.T) {
	engine := MustNewEngine(nil)

	if engine.IsGameOver() {
		t.Error("expected IsGameOver=false initially")
	}

	engine.state.Phase = pb.PhaseType_PHASE_TYPE_END

	if !engine.IsGameOver() {
		t.Error("expected IsGameOver=true when Phase=END")
	}
}

func TestEngine_OnEvent(t *testing.T) {
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "guard", pb.RoleType_ROLE_TYPE_GUARD)
	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)

	eventCount := 0
	engine.OnEvent(func(event *pb.Event) {
		eventCount++
	})

	mustStart(t, engine)
	// NIGHT_GUARD phase
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "guard",
		Skill:    pb.SkillType_SKILL_TYPE_PROTECT,
		TargetID: "v1",
	})
	mustEnd(t, engine)

	// Should have 1 event (protect effect)
	if eventCount != 1 {
		t.Errorf("expected 1 event, got %d", eventCount)
	}
}

func TestEngine_MultipleHandlers(t *testing.T) {
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "guard", pb.RoleType_ROLE_TYPE_GUARD)
	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)

	count1 := 0
	count2 := 0

	engine.OnEvent(func(event *pb.Event) {
		count1++
	})
	engine.OnEvent(func(event *pb.Event) {
		count2++
	})

	mustStart(t, engine)
	// NIGHT_GUARD phase
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "guard",
		Skill:    pb.SkillType_SKILL_TYPE_PROTECT,
		TargetID: "v1",
	})
	mustEnd(t, engine)

	if count1 != 1 {
		t.Errorf("expected handler1 called 1 time, got %d", count1)
	}
	if count2 != 1 {
		t.Errorf("expected handler2 called 1 time, got %d", count2)
	}
}

func TestEngine_Concurrency(t *testing.T) {
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "guard", pb.RoleType_ROLE_TYPE_GUARD)
	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustStart(t, engine)

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = engine.state
			engine.GetCurrentPhase()
			engine.GetCurrentRound()
			engine.GetAllowedSkills("guard")
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
				Skill:    pb.SkillType_SKILL_TYPE_PROTECT,
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
	engine := MustNewEngine(nil)

	// Setup players
	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "seer", pb.RoleType_ROLE_TYPE_SEER)
	mustAdd(t, engine, "guard", pb.RoleType_ROLE_TYPE_GUARD)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	// v2 保证 v1 出局后平民未被屠尽，游戏得以进入白天
	mustAdd(t, engine, "v2", pb.RoleType_ROLE_TYPE_VILLAGER)

	// Start game
	err := engine.Start()
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// Night 1 - GUARD phase
	if engine.GetCurrentPhase() != pb.PhaseType_PHASE_TYPE_NIGHT_GUARD {
		t.Errorf("expected NIGHT_GUARD phase, got %v", engine.GetCurrentPhase())
	}
	if engine.GetCurrentRound() != 1 {
		t.Error("expected Round 1")
	}

	// Guard protects seer
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "guard",
		Skill:    pb.SkillType_SKILL_TYPE_PROTECT,
		TargetID: "seer",
	})
	mustEnd(t, engine) // NIGHT_GUARD -> NIGHT_WOLF

	// Night 1 - WOLF phase
	if engine.GetCurrentPhase() != pb.PhaseType_PHASE_TYPE_NIGHT_WOLF {
		t.Errorf("expected NIGHT_WOLF phase, got %v", engine.GetCurrentPhase())
	}

	// Wolf kills v1
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "v1",
	})
	mustEnd(t, engine) // NIGHT_WOLF -> NIGHT_WITCH

	// Night 1 - WITCH phase (skip)
	mustEnd(t, engine) // NIGHT_WITCH -> NIGHT_SEER

	// Night 1 - SEER phase
	if engine.GetCurrentPhase() != pb.PhaseType_PHASE_TYPE_NIGHT_SEER {
		t.Errorf("expected NIGHT_SEER phase, got %v", engine.GetCurrentPhase())
	}

	// Seer checks wolf
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "seer",
		Skill:    pb.SkillType_SKILL_TYPE_CHECK,
		TargetID: "wolf",
	})
	effects := mustEnd(t, engine) // NIGHT_SEER -> NIGHT_RESOLVE

	// Should have check result showing wolf
	hasCheckResult := false
	for _, e := range effects {
		if e.Type == pb.EventType_EVENT_TYPE_CHECK && e.Data["isGood"] == false {
			hasCheckResult = true
		}
	}
	if !hasCheckResult {
		t.Error("expected seer check to show wolf is evil")
	}

	// NIGHT_RESOLVE phase - apply night kills
	if engine.GetCurrentPhase() != pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE {
		t.Errorf("expected NIGHT_RESOLVE phase, got %v", engine.GetCurrentPhase())
	}
	mustEnd(t, engine) // NIGHT_RESOLVE -> DAY (v1 killed here)

	// v1 should be dead after NIGHT_RESOLVE
	v1, _ := engine.state.getPlayer("v1")
	if v1.Alive {
		t.Error("expected v1 to be dead")
	}

	// Day 1
	if engine.GetCurrentPhase() != pb.PhaseType_PHASE_TYPE_DAY {
		t.Errorf("expected DAY phase, got %v", engine.GetCurrentPhase())
	}

	mustEnd(t, engine) // Day -> Vote

	// Vote 1
	if engine.GetCurrentPhase() != pb.PhaseType_PHASE_TYPE_VOTE {
		t.Errorf("expected VOTE phase, got %v", engine.GetCurrentPhase())
	}

	// Vote out wolf
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "seer",
		Skill:    pb.SkillType_SKILL_TYPE_VOTE,
		TargetID: "wolf",
	})
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "guard",
		Skill:    pb.SkillType_SKILL_TYPE_VOTE,
		TargetID: "wolf",
	})

	mustEnd(t, engine)

	// wolf should be eliminated, good wins
	wolf, _ := engine.state.getPlayer("wolf")
	if wolf.Alive {
		t.Error("expected wolf to be eliminated")
	}

	// Game over (no wolves left)
	if !engine.IsGameOver() {
		t.Error("game should be over")
	}
}

func TestEngine_GetPhaseInfo_NightGuard(t *testing.T) {
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "guard", pb.RoleType_ROLE_TYPE_GUARD)
	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)

	mustStart(t, engine)

	info := engine.GetPhaseInfo()

	if info.Phase != pb.PhaseType_PHASE_TYPE_NIGHT_GUARD {
		t.Errorf("expected NIGHT_GUARD phase, got %v", info.Phase)
	}
	if info.Round != 1 {
		t.Errorf("expected round 1, got %d", info.Round)
	}
	if len(info.ActiveRoles) != 1 || info.ActiveRoles[0] != pb.RoleType_ROLE_TYPE_GUARD {
		t.Errorf("expected GUARD as active role")
	}

	guardInfo, ok := info.RoleInfos[pb.RoleType_ROLE_TYPE_GUARD]
	if !ok {
		t.Fatal("expected guard role info")
	}
	if len(guardInfo.PlayerIDs) != 1 || guardInfo.PlayerIDs[0] != "guard" {
		t.Errorf("expected guard player ID, got %v", guardInfo.PlayerIDs)
	}
	if len(guardInfo.AllowedSkills) != 1 || guardInfo.AllowedSkills[0] != pb.SkillType_SKILL_TYPE_PROTECT {
		t.Errorf("expected PROTECT skill, got %v", guardInfo.AllowedSkills)
	}
}

func TestEngine_GetPhaseInfo_NightWolf(t *testing.T) {
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "wolf1", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "wolf2", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v2", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v3", pb.RoleType_ROLE_TYPE_VILLAGER)

	mustStart(t, engine)
	mustEnd(t, engine) // GUARD -> WOLF

	info := engine.GetPhaseInfo()

	if info.Phase != pb.PhaseType_PHASE_TYPE_NIGHT_WOLF {
		t.Errorf("expected NIGHT_WOLF phase, got %v", info.Phase)
	}

	wolfInfo, ok := info.RoleInfos[pb.RoleType_ROLE_TYPE_WEREWOLF]
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
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "witch", pb.RoleType_ROLE_TYPE_WITCH)
	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v2", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v3", pb.RoleType_ROLE_TYPE_VILLAGER)

	mustStart(t, engine)
	mustEnd(t, engine) // GUARD -> WOLF

	// Wolf kills v1
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "v1",
	})
	mustEnd(t, engine) // WOLF -> WITCH

	info := engine.GetPhaseInfo()

	if info.Phase != pb.PhaseType_PHASE_TYPE_NIGHT_WITCH {
		t.Errorf("expected NIGHT_WITCH phase, got %v", info.Phase)
	}

	witchInfo, ok := info.RoleInfos[pb.RoleType_ROLE_TYPE_WITCH]
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
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "guard", pb.RoleType_ROLE_TYPE_GUARD)
	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)

	mustStart(t, engine)

	info := engine.GetPhaseInfo()

	// 验证阶段步骤包含上帝公告
	if len(info.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(info.Steps))
	}

	// 验证第一步是上帝公告
	if !info.NeedsGodAnnouncement() {
		t.Error("expected god announcement needed")
	}

	godStep := info.GetGodAnnouncementStep()
	if godStep == nil {
		t.Fatal("expected god announcement step")
	}
	if godStep.Role != pb.RoleType_ROLE_TYPE_GOD {
		t.Errorf("expected GOD role, got %v", godStep.Role)
	}
	if godStep.Skill != pb.SkillType_SKILL_TYPE_ANNOUNCE {
		t.Errorf("expected ANNOUNCE skill, got %v", godStep.Skill)
	}

	// 验证玩家行动步骤
	actionSteps := info.GetPlayerActionSteps()
	if len(actionSteps) != 1 {
		t.Errorf("expected 1 action step, got %d", len(actionSteps))
	}
	if actionSteps[0].Role != pb.RoleType_ROLE_TYPE_GUARD {
		t.Errorf("expected GUARD role, got %v", actionSteps[0].Role)
	}
}

// ==================== 并发安全回归测试 ====================

// TestEngine_ConcurrentOnEventAndEndPhase 事件分发与 OnEvent 注册并发。
//
// 回归：publishEvent 此前在释放 e.mu 之后才遍历 e.eventHandlers，
// 与并发的 OnEvent 追加构成数据竞争（需 -race 才能发现）。
func TestEngine_ConcurrentOnEventAndEndPhase(t *testing.T) {
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "w1", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "s", pb.RoleType_ROLE_TYPE_SEER)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v2", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v3", pb.RoleType_ROLE_TYPE_VILLAGER)
	if err := engine.Start(); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			engine.OnEvent(func(*pb.Event) {})
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
			engine.SetLogger(NewNopLogger())
		}
	}()

	wg.Wait()
}

// TestEngine_HandlerPanicIsIsolatedAndLogged 单个 handler panic 不影响其他
// handler，且必须留下错误日志（此前是 `_ = recover()` 静默吞掉）。
func TestEngine_HandlerPanicIsIsolatedAndLogged(t *testing.T) {
	engine := MustNewEngine(nil)
	rec := &recordingLogger{}
	engine.SetLogger(rec)

	mustAdd(t, engine, "w1", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v2", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v3", pb.RoleType_ROLE_TYPE_VILLAGER)
	if err := engine.Start(); err != nil {
		t.Fatal(err)
	}

	survivorCalled := false
	engine.OnEvent(func(*pb.Event) { panic("boom") })
	engine.OnEvent(func(*pb.Event) { survivorCalled = true })

	mustEnd(t, engine) // NIGHT_GUARD -> NIGHT_WOLF
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "w1",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "v1",
	})
	for i := 0; i < 4; i++ {
		mustEnd(t, engine) // 走到 NIGHT_RESOLVE 之后，产生 KILL 事件
	}

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
}

func (l *recordingLogger) Debug(string, ...Field) {}
func (l *recordingLogger) Info(string, ...Field)  {}
func (l *recordingLogger) Warn(string, ...Field)  {}
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
