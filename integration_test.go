package werewolf

import (
	"testing"

	pb "github.com/Zereker/werewolf/proto"
)

// ==================== Complete Game Tests ====================

func TestFullGame_WolvesWin(t *testing.T) {
	// 本局无神职，屠边条件（屠神/屠民）不适用，故显式使用屠城判定
	config := DefaultGameConfig()
	config.VictoryMode = VictoryModeTownWipe
	engine := MustNewEngine(config)

	// 2 狼 vs 3 民：开局 3 > 2 游戏继续，v1 出局后 2 <= 2 狼人获胜。
	//
	// 此前是 2 狼 vs 2 民，屠城判定在开局那一刻就已成立——游戏在第一次
	// EndPhase 就结束了，根本没走到杀人这步。测试因为忽略了返回错误而
	// 「通过」，但它验证的并不是自己声称的那件事。
	mustAdd(t, engine, "wolf1", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "wolf2", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v2", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v3", pb.RoleType_ROLE_TYPE_VILLAGER)

	mustStart(t, engine)

	// NIGHT_GUARD -> NIGHT_WOLF
	mustEnd(t, engine)

	// NIGHT_WOLF: Wolves kill v1
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "wolf1",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "v1",
	})
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "wolf2",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "v1",
	})
	mustEnd(t, engine) // NIGHT_WOLF -> NIGHT_WITCH

	// NIGHT_WITCH -> NIGHT_SEER -> NIGHT_RESOLVE -> DAY (v1 dies)
	mustEnd(t, engine) // NIGHT_WITCH -> NIGHT_SEER
	mustEnd(t, engine) // NIGHT_SEER -> NIGHT_RESOLVE
	mustEnd(t, engine) // NIGHT_RESOLVE -> DAY (kill applied)

	// v1 出局后 good(2) <= evil(2)，狼人获胜
	if !engine.IsGameOver() {
		t.Error("expected game to be over (wolves win)")
	}

	winner := pb.Camp_CAMP_EVIL
	_, actualWinner := engine.state.checkVictory(engine.config.VictoryMode)
	if actualWinner != winner {
		t.Errorf("expected EVIL wins, got %v", actualWinner)
	}
}

func TestFullGame_GoodWins(t *testing.T) {
	engine := MustNewEngine(nil)

	// 1 wolf vs 3 villagers
	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v2", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v3", pb.RoleType_ROLE_TYPE_VILLAGER)

	mustStart(t, engine)

	// Night 1: NIGHT_GUARD -> NIGHT_WOLF
	mustEnd(t, engine)

	// NIGHT_WOLF: Wolf kills v1
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "v1",
	})
	mustEnd(t, engine) // NIGHT_WOLF -> NIGHT_WITCH
	mustEnd(t, engine) // NIGHT_WITCH -> NIGHT_SEER
	mustEnd(t, engine) // NIGHT_SEER -> NIGHT_RESOLVE
	mustEnd(t, engine) // NIGHT_RESOLVE -> DAY

	// Day 1: No actions
	mustEnd(t, engine) // Day -> Vote

	// Vote 1: Everyone votes wolf
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "v2",
		Skill:    pb.SkillType_SKILL_TYPE_VOTE,
		TargetID: "wolf",
	})
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "v3",
		Skill:    pb.SkillType_SKILL_TYPE_VOTE,
		TargetID: "wolf",
	})
	mustEnd(t, engine)

	// Wolf eliminated, evil(0), good wins
	if !engine.IsGameOver() {
		t.Error("expected game to be over (good wins)")
	}

	_, winner := engine.state.checkVictory(engine.config.VictoryMode)
	if winner != pb.Camp_CAMP_GOOD {
		t.Errorf("expected GOOD wins, got %v", winner)
	}
}

// ==================== Rule Scenario Tests ====================

func TestScenario_WitchSavesVictim(t *testing.T) {
	engine := MustNewEngine(nil)

	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "witch", pb.RoleType_ROLE_TYPE_WITCH)
	mustAdd(t, engine, "victim", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v2", pb.RoleType_ROLE_TYPE_VILLAGER)

	mustStart(t, engine)

	// NIGHT_GUARD -> NIGHT_WOLF
	mustEnd(t, engine)

	// NIGHT_WOLF: Wolf kills victim
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "victim",
	})
	mustEnd(t, engine) // NIGHT_WOLF -> NIGHT_WITCH

	// NIGHT_WITCH: Witch saves victim
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "witch",
		Skill:    pb.SkillType_SKILL_TYPE_ANTIDOTE,
		TargetID: "victim",
	})
	mustEnd(t, engine) // NIGHT_WITCH -> NIGHT_SEER
	mustEnd(t, engine) // NIGHT_SEER -> NIGHT_RESOLVE
	mustEnd(t, engine) // NIGHT_RESOLVE -> DAY

	// Victim should still be alive
	victim, _ := engine.state.getPlayer("victim")
	if !victim.Alive {
		t.Error("expected victim to be saved by witch")
	}
}

func TestScenario_GuardProtects(t *testing.T) {
	config := DefaultGameConfig()
	config.SameGuardKillIsEmpty = true
	engine := MustNewEngine(config)

	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "guard", pb.RoleType_ROLE_TYPE_GUARD)
	mustAdd(t, engine, "victim", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v2", pb.RoleType_ROLE_TYPE_VILLAGER)

	mustStart(t, engine)

	// NIGHT_GUARD: Guard protects victim
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "guard",
		Skill:    pb.SkillType_SKILL_TYPE_PROTECT,
		TargetID: "victim",
	})
	mustEnd(t, engine) // NIGHT_GUARD -> NIGHT_WOLF

	// NIGHT_WOLF: Wolf kills victim
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "victim",
	})
	mustEnd(t, engine) // NIGHT_WOLF -> NIGHT_WITCH
	mustEnd(t, engine) // NIGHT_WITCH -> NIGHT_SEER
	mustEnd(t, engine) // NIGHT_SEER -> NIGHT_RESOLVE
	mustEnd(t, engine) // NIGHT_RESOLVE -> DAY

	// Victim should still be alive (protected)
	victim, _ := engine.state.getPlayer("victim")
	if !victim.Alive {
		t.Error("expected victim to be protected by guard")
	}
}

func TestScenario_VoteTie(t *testing.T) {
	engine := MustNewEngine(nil)

	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v2", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v3", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v4", pb.RoleType_ROLE_TYPE_VILLAGER)

	mustStart(t, engine)

	// Night 1: No kill (skip all night phases)
	mustEnd(t, engine) // NIGHT_GUARD -> NIGHT_WOLF
	mustEnd(t, engine) // NIGHT_WOLF -> NIGHT_WITCH
	mustEnd(t, engine) // NIGHT_WITCH -> NIGHT_SEER
	mustEnd(t, engine) // NIGHT_SEER -> NIGHT_RESOLVE
	mustEnd(t, engine) // NIGHT_RESOLVE -> DAY
	mustEnd(t, engine) // DAY -> VOTE

	// Vote: 2 vs 2 tie
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_VOTE,
		TargetID: "v1",
	})
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "v1",
		Skill:    pb.SkillType_SKILL_TYPE_VOTE,
		TargetID: "wolf",
	})
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "v2",
		Skill:    pb.SkillType_SKILL_TYPE_VOTE,
		TargetID: "v1",
	})
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "v3",
		Skill:    pb.SkillType_SKILL_TYPE_VOTE,
		TargetID: "wolf",
	})
	mustEnd(t, engine)

	// Both wolf and v1 should still be alive (tie = no one eliminated)
	wolf, _ := engine.state.getPlayer("wolf")
	v1, _ := engine.state.getPlayer("v1")
	if !wolf.Alive || !v1.Alive {
		t.Error("expected no one eliminated on tie vote")
	}
}

func TestScenario_MultipleRounds(t *testing.T) {
	engine := MustNewEngine(nil)

	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v2", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v3", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v4", pb.RoleType_ROLE_TYPE_VILLAGER)

	mustStart(t, engine)

	if engine.Round() != 1 {
		t.Errorf("expected Round 1, got %d", engine.Round())
	}

	// Night 1 (all phases) -> Day 1 -> Vote 1 -> Night 2
	mustEnd(t, engine) // NIGHT_GUARD -> NIGHT_WOLF
	mustEnd(t, engine) // NIGHT_WOLF -> NIGHT_WITCH
	mustEnd(t, engine) // NIGHT_WITCH -> NIGHT_SEER
	mustEnd(t, engine) // NIGHT_SEER -> NIGHT_RESOLVE
	mustEnd(t, engine) // NIGHT_RESOLVE -> DAY
	mustEnd(t, engine) // DAY -> VOTE
	mustEnd(t, engine) // VOTE -> NIGHT_GUARD (round 2)

	if engine.Round() != 2 {
		t.Errorf("expected Round 2, got %d", engine.Round())
	}

	// Night 2 (all phases) -> Day 2 -> Vote 2 -> Night 3
	mustEnd(t, engine) // NIGHT_GUARD -> NIGHT_WOLF
	mustEnd(t, engine) // NIGHT_WOLF -> NIGHT_WITCH
	mustEnd(t, engine) // NIGHT_WITCH -> NIGHT_SEER
	mustEnd(t, engine) // NIGHT_SEER -> NIGHT_RESOLVE
	mustEnd(t, engine) // NIGHT_RESOLVE -> DAY
	mustEnd(t, engine) // DAY -> VOTE
	mustEnd(t, engine) // VOTE -> NIGHT_GUARD (round 3)

	if engine.Round() != 3 {
		t.Errorf("expected Round 3, got %d", engine.Round())
	}
}

// ==================== Configuration Variant Tests ====================

func TestConfig_WitchCanSaveSelf_Enabled(t *testing.T) {
	config := DefaultGameConfig()
	config.WitchCanSaveSelf = true
	engine := MustNewEngine(config)

	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "witch", pb.RoleType_ROLE_TYPE_WITCH)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v2", pb.RoleType_ROLE_TYPE_VILLAGER)

	mustStart(t, engine)

	// NIGHT_GUARD -> NIGHT_WOLF
	mustEnd(t, engine)

	// NIGHT_WOLF: Wolf kills witch
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "witch",
	})
	mustEnd(t, engine) // NIGHT_WOLF -> NIGHT_WITCH

	// NIGHT_WITCH: Witch saves self
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "witch",
		Skill:    pb.SkillType_SKILL_TYPE_ANTIDOTE,
		TargetID: "witch",
	})
	mustEnd(t, engine) // NIGHT_WITCH -> NIGHT_SEER
	mustEnd(t, engine) // NIGHT_SEER -> NIGHT_RESOLVE
	mustEnd(t, engine) // NIGHT_RESOLVE -> DAY

	// Witch should still be alive (self-save allowed)
	witch, _ := engine.state.getPlayer("witch")
	if !witch.Alive {
		t.Error("expected witch to save self when WitchCanSaveSelf=true")
	}
}

func TestConfig_WitchCanSaveSelf_Disabled(t *testing.T) {
	config := DefaultGameConfig()
	config.WitchCanSaveSelf = false
	engine := MustNewEngine(config)

	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "witch", pb.RoleType_ROLE_TYPE_WITCH)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v2", pb.RoleType_ROLE_TYPE_VILLAGER)

	mustStart(t, engine)

	// NIGHT_GUARD -> NIGHT_WOLF
	mustEnd(t, engine)

	// NIGHT_WOLF: Wolf kills witch
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "witch",
	})
	mustEnd(t, engine) // NIGHT_WOLF -> NIGHT_WITCH

	// NIGHT_WITCH: Witch tries to save self
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "witch",
		Skill:    pb.SkillType_SKILL_TYPE_ANTIDOTE,
		TargetID: "witch",
	})
	mustEnd(t, engine) // NIGHT_WITCH -> NIGHT_SEER
	mustEnd(t, engine) // NIGHT_SEER -> NIGHT_RESOLVE
	mustEnd(t, engine) // NIGHT_RESOLVE -> DAY

	// Witch should be dead (self-save not allowed)
	witch, _ := engine.state.getPlayer("witch")
	if witch.Alive {
		t.Error("expected witch to be dead when WitchCanSaveSelf=false")
	}
}

func TestConfig_SameGuardKill_Empty(t *testing.T) {
	config := DefaultGameConfig()
	config.SameGuardKillIsEmpty = true
	engine := MustNewEngine(config)

	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "guard", pb.RoleType_ROLE_TYPE_GUARD)
	mustAdd(t, engine, "victim", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v2", pb.RoleType_ROLE_TYPE_VILLAGER)

	mustStart(t, engine)

	// NIGHT_GUARD: Guard protects victim
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "guard",
		Skill:    pb.SkillType_SKILL_TYPE_PROTECT,
		TargetID: "victim",
	})
	mustEnd(t, engine) // NIGHT_GUARD -> NIGHT_WOLF

	// NIGHT_WOLF: Wolf kills victim
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "victim",
	})
	mustEnd(t, engine) // NIGHT_WOLF -> NIGHT_WITCH
	mustEnd(t, engine) // NIGHT_WITCH -> NIGHT_SEER
	mustEnd(t, engine) // NIGHT_SEER -> NIGHT_RESOLVE
	mustEnd(t, engine) // NIGHT_RESOLVE -> DAY

	// Victim survives (空刀)
	victim, _ := engine.state.getPlayer("victim")
	if !victim.Alive {
		t.Error("expected victim alive when SameGuardKillIsEmpty=true")
	}
}

func TestConfig_SameGuardKill_NotEmpty(t *testing.T) {
	config := DefaultGameConfig()
	config.SameGuardKillIsEmpty = false
	engine := MustNewEngine(config)

	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "guard", pb.RoleType_ROLE_TYPE_GUARD)
	mustAdd(t, engine, "victim", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v2", pb.RoleType_ROLE_TYPE_VILLAGER)

	mustStart(t, engine)

	// NIGHT_GUARD: Guard protects victim
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "guard",
		Skill:    pb.SkillType_SKILL_TYPE_PROTECT,
		TargetID: "victim",
	})
	mustEnd(t, engine) // NIGHT_GUARD -> NIGHT_WOLF

	// NIGHT_WOLF: Wolf kills victim
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "victim",
	})
	mustEnd(t, engine) // NIGHT_WOLF -> NIGHT_WITCH
	mustEnd(t, engine) // NIGHT_WITCH -> NIGHT_SEER
	mustEnd(t, engine) // NIGHT_SEER -> NIGHT_RESOLVE
	mustEnd(t, engine) // NIGHT_RESOLVE -> DAY

	// Victim dies (guard doesn't cancel kill when SameGuardKillIsEmpty=false)
	victim, _ := engine.state.getPlayer("victim")
	if victim.Alive {
		t.Error("expected victim dead when SameGuardKillIsEmpty=false")
	}
}

// ==================== Complex Scenario Tests ====================

// TestConfig_WitchCanUseBothPotions 女巫同夜双开药的规则变体。
//
// 维基「狼人殺」条目规定「解藥和毒藥不可以在同一夜使用」，故默认禁止；
// 需要放宽的板子可通过 GameConfig.WitchCanUseBothPotions 打开。
//
// 规则层面的完整断言见 rules_test.go:TestRule_R3_WitchCannotUseBothPotionsInOneNight，
// 本用例聚焦配置开关本身的双向行为。
func TestConfig_WitchCanUseBothPotions(t *testing.T) {
	setup := func(canUseBoth bool) *Engine {
		config := DefaultGameConfig()
		config.WitchCanUseBothPotions = canUseBoth
		engine := MustNewEngine(config)

		mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
		mustAdd(t, engine, "witch", pb.RoleType_ROLE_TYPE_WITCH)
		mustAdd(t, engine, "victim", pb.RoleType_ROLE_TYPE_VILLAGER)
		mustAdd(t, engine, "v2", pb.RoleType_ROLE_TYPE_VILLAGER)
		mustAdd(t, engine, "v3", pb.RoleType_ROLE_TYPE_VILLAGER)

		if err := engine.Start(); err != nil {
			t.Fatalf("Start() 失败: %v", err)
		}

		// NIGHT_GUARD -> NIGHT_WOLF
		if _, err := engine.EndPhase(); err != nil {
			t.Fatalf("EndPhase() 失败: %v", err)
		}

		// NIGHT_WOLF: 狼刀 victim
		if err := engine.SubmitSkillUse(&SkillUse{
			PlayerID: "wolf",
			Skill:    pb.SkillType_SKILL_TYPE_KILL,
			TargetID: "victim",
		}); err != nil {
			t.Fatalf("提交击杀失败: %v", err)
		}
		if _, err := engine.EndPhase(); err != nil { // -> NIGHT_WITCH
			t.Fatalf("EndPhase() 失败: %v", err)
		}

		// NIGHT_WITCH: 先救 victim，再毒 wolf
		if err := engine.SubmitSkillUse(&SkillUse{
			PlayerID: "witch",
			Skill:    pb.SkillType_SKILL_TYPE_ANTIDOTE,
			TargetID: "victim",
		}); err != nil {
			t.Fatalf("提交解药失败: %v", err)
		}
		if err := engine.SubmitSkillUse(&SkillUse{
			PlayerID: "witch",
			Skill:    pb.SkillType_SKILL_TYPE_POISON,
			TargetID: "wolf",
		}); err != nil {
			t.Fatalf("提交毒药失败: %v", err)
		}

		for i := 0; i < 3; i++ { // NIGHT_WITCH -> NIGHT_SEER -> NIGHT_RESOLVE -> DAY
			if _, err := engine.EndPhase(); err != nil {
				t.Fatalf("EndPhase() 失败: %v", err)
			}
		}
		return engine
	}

	t.Run("默认禁止同夜双开药", func(t *testing.T) {
		engine := setup(false)

		victim, _ := engine.state.getPlayer("victim")
		wolf, _ := engine.state.getPlayer("wolf")
		witch, _ := engine.state.getPlayer("witch")

		// 先提交的解药生效
		if !victim.Alive {
			t.Error("解药先提交，victim 应当被救")
		}
		if witch.HasAntidote {
			t.Error("解药应当已被消耗")
		}

		// 后提交的毒药被拒
		if !wolf.Alive {
			t.Error("同夜第二瓶药应当无效，wolf 不应被毒死")
		}
		if !witch.HasPoison {
			t.Error("毒药未生效，不应被消耗")
		}
		if engine.IsGameOver() {
			t.Error("狼人存活，游戏不应结束")
		}
	})

	t.Run("放宽后允许同夜双开药", func(t *testing.T) {
		engine := setup(true)

		victim, _ := engine.state.getPlayer("victim")
		wolf, _ := engine.state.getPlayer("wolf")
		witch, _ := engine.state.getPlayer("witch")

		if !victim.Alive {
			t.Error("expected victim to be saved")
		}
		if wolf.Alive {
			t.Error("expected wolf to be poisoned")
		}
		if witch.HasAntidote || witch.HasPoison {
			t.Error("两瓶药都应当被消耗")
		}
		if !engine.IsGameOver() {
			t.Error("expected game over (good wins after wolf poisoned)")
		}
	})
}

func TestScenario_SeerIdentifiesWolf(t *testing.T) {
	engine := MustNewEngine(nil)

	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "seer", pb.RoleType_ROLE_TYPE_SEER)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v2", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v3", pb.RoleType_ROLE_TYPE_VILLAGER)

	mustStart(t, engine)

	// NIGHT_GUARD -> NIGHT_WOLF
	mustEnd(t, engine)

	// NIGHT_WOLF: Wolf kills v1
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "v1",
	})
	mustEnd(t, engine) // NIGHT_WOLF -> NIGHT_WITCH
	mustEnd(t, engine) // NIGHT_WITCH -> NIGHT_SEER

	// NIGHT_SEER: Seer checks wolf
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "seer",
		Skill:    pb.SkillType_SKILL_TYPE_CHECK,
		TargetID: "wolf",
	})
	effects := mustEnd(t, engine) // NIGHT_SEER -> NIGHT_RESOLVE
	mustEnd(t, engine)            // NIGHT_RESOLVE -> DAY

	// Check the seer's result
	var checkEffect *Effect
	for _, e := range effects {
		if e.Type == pb.EventType_EVENT_TYPE_CHECK {
			checkEffect = e
			break
		}
	}

	if checkEffect == nil {
		t.Fatal("expected check effect")
	}
	if checkEffect.Data["isGood"] != false {
		t.Error("expected seer to identify wolf as evil")
	}
	if checkEffect.Data["camp"] != pb.Camp_CAMP_EVIL {
		t.Errorf("expected camp=EVIL, got %v", checkEffect.Data["camp"])
	}
}

// ==================== Sub-Step Mode Tests ====================

func TestSubStepMode_FullNightCycle(t *testing.T) {
	engine := MustNewEngine(nil)

	// 设置玩家
	mustAdd(t, engine, "guard", pb.RoleType_ROLE_TYPE_GUARD)
	mustAdd(t, engine, "wolf1", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "wolf2", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "witch", pb.RoleType_ROLE_TYPE_WITCH)
	mustAdd(t, engine, "seer", pb.RoleType_ROLE_TYPE_SEER)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)

	// 启动分步模式
	err := engine.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// 阶段1：守卫阶段
	if engine.Phase() != pb.PhaseType_PHASE_TYPE_NIGHT_GUARD {
		t.Errorf("expected NIGHT_GUARD, got %v", engine.Phase())
	}

	mustSubmit(t, engine, &SkillUse{
		PlayerID: "guard",
		Skill:    pb.SkillType_SKILL_TYPE_PROTECT,
		TargetID: "seer",
	})
	mustEnd(t, engine)

	// 阶段2：狼人阶段
	if engine.Phase() != pb.PhaseType_PHASE_TYPE_NIGHT_WOLF {
		t.Errorf("expected NIGHT_WOLF, got %v", engine.Phase())
	}

	// 狼人可以查询队友
	teammates := engine.WolfTeammates("wolf1")
	if len(teammates) != 1 || teammates[0] != "wolf2" {
		t.Errorf("wolf1 should have wolf2 as teammate, got %v", teammates)
	}

	// 狼人投票击杀
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "wolf1",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "v1",
	})
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "wolf2",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "v1",
	})
	mustEnd(t, engine)

	// 阶段3：女巫阶段
	if engine.Phase() != pb.PhaseType_PHASE_TYPE_NIGHT_WITCH {
		t.Errorf("expected NIGHT_WITCH, got %v", engine.Phase())
	}

	// 女巫可以查询被杀者
	killTarget := engine.NightKillTarget()
	if killTarget != "v1" {
		t.Errorf("expected NightKillTarget=v1, got %s", killTarget)
	}

	// 女巫使用解药救人
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "witch",
		Skill:    pb.SkillType_SKILL_TYPE_ANTIDOTE,
		TargetID: "v1",
	})
	mustEnd(t, engine)

	// 阶段4：预言家阶段
	if engine.Phase() != pb.PhaseType_PHASE_TYPE_NIGHT_SEER {
		t.Errorf("expected NIGHT_SEER, got %v", engine.Phase())
	}

	mustSubmit(t, engine, &SkillUse{
		PlayerID: "seer",
		Skill:    pb.SkillType_SKILL_TYPE_CHECK,
		TargetID: "wolf1",
	})
	effects := mustEnd(t, engine)

	// 验证预言家查验结果
	var checkEffect *Effect
	for _, e := range effects {
		if e.Type == pb.EventType_EVENT_TYPE_CHECK {
			checkEffect = e
			break
		}
	}
	if checkEffect == nil {
		t.Fatal("expected check effect")
	}
	if checkEffect.Data["isGood"] != false {
		t.Error("expected wolf1 to be identified as evil")
	}

	// 阶段5：夜晚结算
	if engine.Phase() != pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE {
		t.Errorf("expected NIGHT_RESOLVE, got %v", engine.Phase())
	}
	mustEnd(t, engine) // NIGHT_RESOLVE -> DAY

	// 阶段6：白天
	if engine.Phase() != pb.PhaseType_PHASE_TYPE_DAY {
		t.Errorf("expected DAY, got %v", engine.Phase())
	}

	// 验证状态：v1 被救活
	v1, _ := engine.state.getPlayer("v1")
	if !v1.Alive {
		t.Error("expected v1 to be saved by witch")
	}

	// 验证状态：seer 被保护（使用 NightContext）
	if !engine.state.RoundCtx.IsProtected("seer") {
		t.Error("expected seer to be protected by guard")
	}
}

func TestSubStepMode_WolfVoteTie(t *testing.T) {
	engine := MustNewEngine(nil)

	mustAdd(t, engine, "guard", pb.RoleType_ROLE_TYPE_GUARD)
	mustAdd(t, engine, "wolf1", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "wolf2", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "witch", pb.RoleType_ROLE_TYPE_WITCH)
	mustAdd(t, engine, "seer", pb.RoleType_ROLE_TYPE_SEER)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v2", pb.RoleType_ROLE_TYPE_VILLAGER)

	mustStart(t, engine)

	// 守卫阶段
	mustEnd(t, engine)

	// 狼人阶段：平票（wolf1 投 v1, wolf2 投 v2）
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "wolf1",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "v1",
	})
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "wolf2",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "v2",
	})
	mustEnd(t, engine)

	// 女巫阶段：平票导致无击杀，NightKillTarget 应为空
	killTarget := engine.NightKillTarget()
	if killTarget != "" {
		t.Errorf("expected empty NightKillTarget for tie vote, got %s", killTarget)
	}

	// 完成剩余阶段
	mustEnd(t, engine) // NIGHT_WITCH -> NIGHT_SEER
	mustEnd(t, engine) // NIGHT_SEER -> NIGHT_RESOLVE
	mustEnd(t, engine) // NIGHT_RESOLVE -> DAY

	// v1 和 v2 都应该存活（没有达成共识）
	v1, _ := engine.state.getPlayer("v1")
	v2, _ := engine.state.getPlayer("v2")
	if !v1.Alive || !v2.Alive {
		t.Error("expected both v1 and v2 to be alive when wolves have tie vote")
	}
}

func TestSubStepMode_GuardProtectsFromKill(t *testing.T) {
	config := DefaultGameConfig()
	config.SameGuardKillIsEmpty = true
	engine := MustNewEngine(config)

	mustAdd(t, engine, "guard", pb.RoleType_ROLE_TYPE_GUARD)
	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "witch", pb.RoleType_ROLE_TYPE_WITCH)
	mustAdd(t, engine, "seer", pb.RoleType_ROLE_TYPE_SEER)
	mustAdd(t, engine, "victim", pb.RoleType_ROLE_TYPE_VILLAGER)

	mustStart(t, engine)

	// 守卫保护 victim
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "guard",
		Skill:    pb.SkillType_SKILL_TYPE_PROTECT,
		TargetID: "victim",
	})
	mustEnd(t, engine)

	// 狼人杀 victim（应该被守卫挡住）
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "victim",
	})
	mustEnd(t, engine)

	// 女巫阶段：刀口照常记录（女巫不知道守卫守了谁），
	// 守护是否抵消由 NIGHT_RESOLVE 判定
	killTarget := engine.NightKillTarget()
	if killTarget != "victim" {
		t.Errorf("expected NightKillTarget=victim (protection resolves later), got %s", killTarget)
	}

	// 完成剩余阶段
	mustEnd(t, engine) // NIGHT_WITCH -> NIGHT_SEER
	mustEnd(t, engine) // NIGHT_SEER -> NIGHT_RESOLVE
	mustEnd(t, engine) // NIGHT_RESOLVE -> DAY

	// victim 存活
	victim, _ := engine.state.getPlayer("victim")
	if !victim.Alive {
		t.Error("expected victim to be protected by guard")
	}
}

func TestSubStepMode_MultipleRounds(t *testing.T) {
	engine := MustNewEngine(nil)

	mustAdd(t, engine, "guard", pb.RoleType_ROLE_TYPE_GUARD)
	mustAdd(t, engine, "wolf", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "witch", pb.RoleType_ROLE_TYPE_WITCH)
	mustAdd(t, engine, "seer", pb.RoleType_ROLE_TYPE_SEER)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v2", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v3", pb.RoleType_ROLE_TYPE_VILLAGER)

	mustStart(t, engine)

	if engine.Round() != 1 {
		t.Errorf("expected Round 1, got %d", engine.Round())
	}

	// 第一轮夜晚：空过
	mustEnd(t, engine) // NIGHT_GUARD -> NIGHT_WOLF
	mustEnd(t, engine) // NIGHT_WOLF -> NIGHT_WITCH
	mustEnd(t, engine) // NIGHT_WITCH -> NIGHT_SEER
	mustEnd(t, engine) // NIGHT_SEER -> NIGHT_RESOLVE
	mustEnd(t, engine) // NIGHT_RESOLVE -> DAY

	if engine.Phase() != pb.PhaseType_PHASE_TYPE_DAY {
		t.Errorf("expected DAY, got %v", engine.Phase())
	}

	// 白天 -> 投票
	mustEnd(t, engine) // DAY -> VOTE

	if engine.Phase() != pb.PhaseType_PHASE_TYPE_VOTE {
		t.Errorf("expected VOTE, got %v", engine.Phase())
	}

	// 投票 -> 第二轮夜晚
	mustEnd(t, engine) // VOTE -> NIGHT_GUARD

	if engine.Phase() != pb.PhaseType_PHASE_TYPE_NIGHT_GUARD {
		t.Errorf("expected NIGHT_GUARD, got %v", engine.Phase())
	}

	if engine.Round() != 2 {
		t.Errorf("expected Round 2, got %d", engine.Round())
	}
}

func TestScenario_AllRolesActive(t *testing.T) {
	engine := MustNewEngine(nil)

	// Full game with all roles
	mustAdd(t, engine, "wolf1", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "wolf2", pb.RoleType_ROLE_TYPE_WEREWOLF)
	mustAdd(t, engine, "seer", pb.RoleType_ROLE_TYPE_SEER)
	mustAdd(t, engine, "witch", pb.RoleType_ROLE_TYPE_WITCH)
	mustAdd(t, engine, "guard", pb.RoleType_ROLE_TYPE_GUARD)
	mustAdd(t, engine, "v1", pb.RoleType_ROLE_TYPE_VILLAGER)
	mustAdd(t, engine, "v2", pb.RoleType_ROLE_TYPE_VILLAGER)

	mustStart(t, engine)

	// NIGHT_GUARD: Guard protects seer
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "guard",
		Skill:    pb.SkillType_SKILL_TYPE_PROTECT,
		TargetID: "seer",
	})
	effects1 := mustEnd(t, engine) // NIGHT_GUARD -> NIGHT_WOLF

	// NIGHT_WOLF: Wolves kill witch
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "wolf1",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "witch",
	})
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "wolf2",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "witch",
	})
	effects2 := mustEnd(t, engine) // NIGHT_WOLF -> NIGHT_WITCH

	// NIGHT_WITCH: Witch poisons wolf1
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "witch",
		Skill:    pb.SkillType_SKILL_TYPE_POISON,
		TargetID: "wolf1",
	})
	effects3 := mustEnd(t, engine) // NIGHT_WITCH -> NIGHT_SEER

	// NIGHT_SEER: Seer checks wolf2
	mustSubmit(t, engine, &SkillUse{
		PlayerID: "seer",
		Skill:    pb.SkillType_SKILL_TYPE_CHECK,
		TargetID: "wolf2",
	})
	effects4 := mustEnd(t, engine) // NIGHT_SEER -> NIGHT_RESOLVE
	effects5 := mustEnd(t, engine) // NIGHT_RESOLVE -> DAY

	// Collect all effects
	allEffects := append(append(append(append(effects1, effects2...), effects3...), effects4...), effects5...)

	// Verify effects
	hasProtect := false
	hasKill := false
	hasPoison := false
	hasCheck := false

	for _, e := range allEffects {
		switch e.Type {
		case pb.EventType_EVENT_TYPE_PROTECT:
			hasProtect = true
		case pb.EventType_EVENT_TYPE_KILL:
			hasKill = true
		case pb.EventType_EVENT_TYPE_POISON:
			hasPoison = true
		case pb.EventType_EVENT_TYPE_CHECK:
			hasCheck = true
			if e.Data["isGood"] != false {
				t.Error("expected seer to see wolf2 as evil")
			}
		}
	}

	if !hasProtect {
		t.Error("expected protect effect")
	}
	if !hasKill {
		t.Error("expected kill effect")
	}
	if !hasPoison {
		t.Error("expected poison effect")
	}
	if !hasCheck {
		t.Error("expected check effect")
	}

	// Verify state
	witch, _ := engine.state.getPlayer("witch")
	wolf1, _ := engine.state.getPlayer("wolf1")
	seer, _ := engine.state.getPlayer("seer")

	if witch.Alive {
		t.Error("expected witch to be dead")
	}
	if wolf1.Alive {
		t.Error("expected wolf1 to be poisoned")
	}
	if !seer.Alive {
		t.Error("expected seer to be alive (protected)")
	}
}
