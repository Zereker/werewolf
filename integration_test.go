package werewolf

import (
	"testing"

	pb "github.com/Zereker/werewolf/proto"
)

// ==================== Complete Game Tests ====================

func TestFullGame_WolvesWin(t *testing.T) {
	engine := NewEngine(nil)

	// 2 wolves vs 2 villagers
	engine.AddPlayer("wolf1", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("wolf2", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("v1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v2", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	engine.Start()

	// NIGHT_GUARD -> NIGHT_WOLF
	engine.EndPhase()

	// NIGHT_WOLF: Wolves kill v1
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "wolf1",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "v1",
	})
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "wolf2",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "v1",
	})
	engine.EndPhase() // NIGHT_WOLF -> NIGHT_WITCH

	// NIGHT_WITCH -> NIGHT_SEER -> NIGHT_RESOLVE -> DAY (v1 dies)
	engine.EndPhase() // NIGHT_WITCH -> NIGHT_SEER
	engine.EndPhase() // NIGHT_SEER -> NIGHT_RESOLVE
	engine.EndPhase() // NIGHT_RESOLVE -> DAY (kill applied)

	// v1 dead, good(1) <= evil(2), wolves win
	if !engine.IsGameOver() {
		t.Error("expected game to be over (wolves win)")
	}

	winner := pb.Camp_CAMP_EVIL
	_, actualWinner := engine.state.CheckVictory()
	if actualWinner != winner {
		t.Errorf("expected EVIL wins, got %v", actualWinner)
	}
}

func TestFullGame_GoodWins(t *testing.T) {
	engine := NewEngine(nil)

	// 1 wolf vs 3 villagers
	engine.AddPlayer("wolf", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("v1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v2", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v3", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	engine.Start()

	// Night 1: NIGHT_GUARD -> NIGHT_WOLF
	engine.EndPhase()

	// NIGHT_WOLF: Wolf kills v1
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "v1",
	})
	engine.EndPhase() // NIGHT_WOLF -> NIGHT_WITCH
	engine.EndPhase() // NIGHT_WITCH -> NIGHT_SEER
	engine.EndPhase() // NIGHT_SEER -> NIGHT_RESOLVE
	engine.EndPhase() // NIGHT_RESOLVE -> DAY

	// Day 1: No actions
	engine.EndPhase() // Day -> Vote

	// Vote 1: Everyone votes wolf
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "v2",
		Skill:    pb.SkillType_SKILL_TYPE_VOTE,
		TargetID: "wolf",
	})
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "v3",
		Skill:    pb.SkillType_SKILL_TYPE_VOTE,
		TargetID: "wolf",
	})
	engine.EndPhase()

	// Wolf eliminated, evil(0), good wins
	if !engine.IsGameOver() {
		t.Error("expected game to be over (good wins)")
	}

	_, winner := engine.state.CheckVictory()
	if winner != pb.Camp_CAMP_GOOD {
		t.Errorf("expected GOOD wins, got %v", winner)
	}
}

// ==================== Rule Scenario Tests ====================

func TestScenario_WitchSavesVictim(t *testing.T) {
	engine := NewEngine(nil)

	engine.AddPlayer("wolf", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("witch", pb.RoleType_ROLE_TYPE_WITCH, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("victim", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v2", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	engine.Start()

	// NIGHT_GUARD -> NIGHT_WOLF
	engine.EndPhase()

	// NIGHT_WOLF: Wolf kills victim
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "victim",
	})
	engine.EndPhase() // NIGHT_WOLF -> NIGHT_WITCH

	// NIGHT_WITCH: Witch saves victim
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "witch",
		Skill:    pb.SkillType_SKILL_TYPE_ANTIDOTE,
		TargetID: "victim",
	})
	engine.EndPhase() // NIGHT_WITCH -> NIGHT_SEER
	engine.EndPhase() // NIGHT_SEER -> NIGHT_RESOLVE
	engine.EndPhase() // NIGHT_RESOLVE -> DAY

	// Victim should still be alive
	victim, _ := engine.state.getPlayer("victim")
	if !victim.Alive {
		t.Error("expected victim to be saved by witch")
	}
}

func TestScenario_GuardProtects(t *testing.T) {
	config := DefaultGameConfig()
	config.SameGuardKillIsEmpty = true
	engine := NewEngine(config)

	engine.AddPlayer("wolf", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("guard", pb.RoleType_ROLE_TYPE_GUARD, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("victim", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v2", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	engine.Start()

	// NIGHT_GUARD: Guard protects victim
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "guard",
		Skill:    pb.SkillType_SKILL_TYPE_PROTECT,
		TargetID: "victim",
	})
	engine.EndPhase() // NIGHT_GUARD -> NIGHT_WOLF

	// NIGHT_WOLF: Wolf kills victim
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "victim",
	})
	engine.EndPhase() // NIGHT_WOLF -> NIGHT_WITCH
	engine.EndPhase() // NIGHT_WITCH -> NIGHT_SEER
	engine.EndPhase() // NIGHT_SEER -> NIGHT_RESOLVE
	engine.EndPhase() // NIGHT_RESOLVE -> DAY

	// Victim should still be alive (protected)
	victim, _ := engine.state.getPlayer("victim")
	if !victim.Alive {
		t.Error("expected victim to be protected by guard")
	}
}

func TestScenario_VoteTie(t *testing.T) {
	engine := NewEngine(nil)

	engine.AddPlayer("wolf", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("v1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v2", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v3", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v4", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	engine.Start()

	// Night 1: No kill (skip all night phases)
	engine.EndPhase() // NIGHT_GUARD -> NIGHT_WOLF
	engine.EndPhase() // NIGHT_WOLF -> NIGHT_WITCH
	engine.EndPhase() // NIGHT_WITCH -> NIGHT_SEER
	engine.EndPhase() // NIGHT_SEER -> NIGHT_RESOLVE
	engine.EndPhase() // NIGHT_RESOLVE -> DAY
	engine.EndPhase() // DAY -> VOTE

	// Vote: 2 vs 2 tie
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_VOTE,
		TargetID: "v1",
	})
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "v1",
		Skill:    pb.SkillType_SKILL_TYPE_VOTE,
		TargetID: "wolf",
	})
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "v2",
		Skill:    pb.SkillType_SKILL_TYPE_VOTE,
		TargetID: "v1",
	})
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "v3",
		Skill:    pb.SkillType_SKILL_TYPE_VOTE,
		TargetID: "wolf",
	})
	engine.EndPhase()

	// Both wolf and v1 should still be alive (tie = no one eliminated)
	wolf, _ := engine.state.getPlayer("wolf")
	v1, _ := engine.state.getPlayer("v1")
	if !wolf.Alive || !v1.Alive {
		t.Error("expected no one eliminated on tie vote")
	}
}

func TestScenario_MultipleRounds(t *testing.T) {
	engine := NewEngine(nil)

	engine.AddPlayer("wolf", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("v1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v2", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v3", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v4", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	engine.Start()

	if engine.GetCurrentRound() != 1 {
		t.Errorf("expected Round 1, got %d", engine.GetCurrentRound())
	}

	// Night 1 (all phases) -> Day 1 -> Vote 1 -> Night 2
	engine.EndPhase() // NIGHT_GUARD -> NIGHT_WOLF
	engine.EndPhase() // NIGHT_WOLF -> NIGHT_WITCH
	engine.EndPhase() // NIGHT_WITCH -> NIGHT_SEER
	engine.EndPhase() // NIGHT_SEER -> NIGHT_RESOLVE
	engine.EndPhase() // NIGHT_RESOLVE -> DAY
	engine.EndPhase() // DAY -> VOTE
	engine.EndPhase() // VOTE -> NIGHT_GUARD (round 2)

	if engine.GetCurrentRound() != 2 {
		t.Errorf("expected Round 2, got %d", engine.GetCurrentRound())
	}

	// Night 2 (all phases) -> Day 2 -> Vote 2 -> Night 3
	engine.EndPhase() // NIGHT_GUARD -> NIGHT_WOLF
	engine.EndPhase() // NIGHT_WOLF -> NIGHT_WITCH
	engine.EndPhase() // NIGHT_WITCH -> NIGHT_SEER
	engine.EndPhase() // NIGHT_SEER -> NIGHT_RESOLVE
	engine.EndPhase() // NIGHT_RESOLVE -> DAY
	engine.EndPhase() // DAY -> VOTE
	engine.EndPhase() // VOTE -> NIGHT_GUARD (round 3)

	if engine.GetCurrentRound() != 3 {
		t.Errorf("expected Round 3, got %d", engine.GetCurrentRound())
	}
}

// ==================== Configuration Variant Tests ====================

func TestConfig_WitchCanSaveSelf_Enabled(t *testing.T) {
	config := DefaultGameConfig()
	config.WitchCanSaveSelf = true
	engine := NewEngine(config)

	engine.AddPlayer("wolf", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("witch", pb.RoleType_ROLE_TYPE_WITCH, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v2", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	engine.Start()

	// NIGHT_GUARD -> NIGHT_WOLF
	engine.EndPhase()

	// NIGHT_WOLF: Wolf kills witch
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "witch",
	})
	engine.EndPhase() // NIGHT_WOLF -> NIGHT_WITCH

	// NIGHT_WITCH: Witch saves self
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "witch",
		Skill:    pb.SkillType_SKILL_TYPE_ANTIDOTE,
		TargetID: "witch",
	})
	engine.EndPhase() // NIGHT_WITCH -> NIGHT_SEER
	engine.EndPhase() // NIGHT_SEER -> NIGHT_RESOLVE
	engine.EndPhase() // NIGHT_RESOLVE -> DAY

	// Witch should still be alive (self-save allowed)
	witch, _ := engine.state.getPlayer("witch")
	if !witch.Alive {
		t.Error("expected witch to save self when WitchCanSaveSelf=true")
	}
}

func TestConfig_WitchCanSaveSelf_Disabled(t *testing.T) {
	config := DefaultGameConfig()
	config.WitchCanSaveSelf = false
	engine := NewEngine(config)

	engine.AddPlayer("wolf", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("witch", pb.RoleType_ROLE_TYPE_WITCH, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v2", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	engine.Start()

	// NIGHT_GUARD -> NIGHT_WOLF
	engine.EndPhase()

	// NIGHT_WOLF: Wolf kills witch
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "witch",
	})
	engine.EndPhase() // NIGHT_WOLF -> NIGHT_WITCH

	// NIGHT_WITCH: Witch tries to save self
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "witch",
		Skill:    pb.SkillType_SKILL_TYPE_ANTIDOTE,
		TargetID: "witch",
	})
	engine.EndPhase() // NIGHT_WITCH -> NIGHT_SEER
	engine.EndPhase() // NIGHT_SEER -> NIGHT_RESOLVE
	engine.EndPhase() // NIGHT_RESOLVE -> DAY

	// Witch should be dead (self-save not allowed)
	witch, _ := engine.state.getPlayer("witch")
	if witch.Alive {
		t.Error("expected witch to be dead when WitchCanSaveSelf=false")
	}
}

func TestConfig_SameGuardKill_Empty(t *testing.T) {
	config := DefaultGameConfig()
	config.SameGuardKillIsEmpty = true
	engine := NewEngine(config)

	engine.AddPlayer("wolf", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("guard", pb.RoleType_ROLE_TYPE_GUARD, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("victim", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v2", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	engine.Start()

	// NIGHT_GUARD: Guard protects victim
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "guard",
		Skill:    pb.SkillType_SKILL_TYPE_PROTECT,
		TargetID: "victim",
	})
	engine.EndPhase() // NIGHT_GUARD -> NIGHT_WOLF

	// NIGHT_WOLF: Wolf kills victim
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "victim",
	})
	engine.EndPhase() // NIGHT_WOLF -> NIGHT_WITCH
	engine.EndPhase() // NIGHT_WITCH -> NIGHT_SEER
	engine.EndPhase() // NIGHT_SEER -> NIGHT_RESOLVE
	engine.EndPhase() // NIGHT_RESOLVE -> DAY

	// Victim survives (空刀)
	victim, _ := engine.state.getPlayer("victim")
	if !victim.Alive {
		t.Error("expected victim alive when SameGuardKillIsEmpty=true")
	}
}

func TestConfig_SameGuardKill_NotEmpty(t *testing.T) {
	config := DefaultGameConfig()
	config.SameGuardKillIsEmpty = false
	engine := NewEngine(config)

	engine.AddPlayer("wolf", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("guard", pb.RoleType_ROLE_TYPE_GUARD, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("victim", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v2", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	engine.Start()

	// NIGHT_GUARD: Guard protects victim
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "guard",
		Skill:    pb.SkillType_SKILL_TYPE_PROTECT,
		TargetID: "victim",
	})
	engine.EndPhase() // NIGHT_GUARD -> NIGHT_WOLF

	// NIGHT_WOLF: Wolf kills victim
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "victim",
	})
	engine.EndPhase() // NIGHT_WOLF -> NIGHT_WITCH
	engine.EndPhase() // NIGHT_WITCH -> NIGHT_SEER
	engine.EndPhase() // NIGHT_SEER -> NIGHT_RESOLVE
	engine.EndPhase() // NIGHT_RESOLVE -> DAY

	// Victim dies (guard doesn't cancel kill when SameGuardKillIsEmpty=false)
	victim, _ := engine.state.getPlayer("victim")
	if victim.Alive {
		t.Error("expected victim dead when SameGuardKillIsEmpty=false")
	}
}

// ==================== Complex Scenario Tests ====================

func TestScenario_WitchPoisonAndSaveOnSameNight(t *testing.T) {
	engine := NewEngine(nil)

	engine.AddPlayer("wolf", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("witch", pb.RoleType_ROLE_TYPE_WITCH, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("victim", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v2", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	engine.Start()

	// NIGHT_GUARD -> NIGHT_WOLF
	engine.EndPhase()

	// NIGHT_WOLF: Wolf kills victim
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "victim",
	})
	engine.EndPhase() // NIGHT_WOLF -> NIGHT_WITCH

	// NIGHT_WITCH: Witch saves victim AND poisons wolf
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "witch",
		Skill:    pb.SkillType_SKILL_TYPE_ANTIDOTE,
		TargetID: "victim",
	})
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "witch",
		Skill:    pb.SkillType_SKILL_TYPE_POISON,
		TargetID: "wolf",
	})
	engine.EndPhase() // NIGHT_WITCH -> NIGHT_SEER
	engine.EndPhase() // NIGHT_SEER -> NIGHT_RESOLVE
	engine.EndPhase() // NIGHT_RESOLVE -> DAY

	// Victim saved, wolf poisoned
	victim, _ := engine.state.getPlayer("victim")
	wolf, _ := engine.state.getPlayer("wolf")

	if !victim.Alive {
		t.Error("expected victim to be saved")
	}
	if wolf.Alive {
		t.Error("expected wolf to be poisoned")
	}

	// Game should be over (good wins)
	if !engine.IsGameOver() {
		t.Error("expected game over (good wins after wolf poisoned)")
	}
}

func TestScenario_SeerIdentifiesWolf(t *testing.T) {
	engine := NewEngine(nil)

	engine.AddPlayer("wolf", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("seer", pb.RoleType_ROLE_TYPE_SEER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v2", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v3", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	engine.Start()

	// NIGHT_GUARD -> NIGHT_WOLF
	engine.EndPhase()

	// NIGHT_WOLF: Wolf kills v1
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "v1",
	})
	engine.EndPhase() // NIGHT_WOLF -> NIGHT_WITCH
	engine.EndPhase() // NIGHT_WITCH -> NIGHT_SEER

	// NIGHT_SEER: Seer checks wolf
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "seer",
		Skill:    pb.SkillType_SKILL_TYPE_CHECK,
		TargetID: "wolf",
	})
	effects, _ := engine.EndPhase() // NIGHT_SEER -> NIGHT_RESOLVE
	engine.EndPhase()               // NIGHT_RESOLVE -> DAY

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
	engine := NewEngine(nil)

	// 设置玩家
	engine.AddPlayer("guard", pb.RoleType_ROLE_TYPE_GUARD, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("wolf1", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("wolf2", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("witch", pb.RoleType_ROLE_TYPE_WITCH, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("seer", pb.RoleType_ROLE_TYPE_SEER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	// 启动分步模式
	err := engine.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// 阶段1：守卫阶段
	if engine.GetCurrentPhase() != pb.PhaseType_PHASE_TYPE_NIGHT_GUARD {
		t.Errorf("expected NIGHT_GUARD, got %v", engine.GetCurrentPhase())
	}

	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "guard",
		Skill:    pb.SkillType_SKILL_TYPE_PROTECT,
		TargetID: "seer",
	})
	engine.EndSubStep()

	// 阶段2：狼人阶段
	if engine.GetCurrentPhase() != pb.PhaseType_PHASE_TYPE_NIGHT_WOLF {
		t.Errorf("expected NIGHT_WOLF, got %v", engine.GetCurrentPhase())
	}

	// 狼人可以查询队友
	teammates := engine.GetWolfTeammates("wolf1")
	if len(teammates) != 1 || teammates[0] != "wolf2" {
		t.Errorf("wolf1 should have wolf2 as teammate, got %v", teammates)
	}

	// 狼人投票击杀
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "wolf1",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "v1",
	})
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "wolf2",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "v1",
	})
	engine.EndSubStep()

	// 阶段3：女巫阶段
	if engine.GetCurrentPhase() != pb.PhaseType_PHASE_TYPE_NIGHT_WITCH {
		t.Errorf("expected NIGHT_WITCH, got %v", engine.GetCurrentPhase())
	}

	// 女巫可以查询被杀者
	killTarget := engine.GetNightKillTarget()
	if killTarget != "v1" {
		t.Errorf("expected NightKillTarget=v1, got %s", killTarget)
	}

	// 女巫使用解药救人
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "witch",
		Skill:    pb.SkillType_SKILL_TYPE_ANTIDOTE,
		TargetID: "v1",
	})
	engine.EndSubStep()

	// 阶段4：预言家阶段
	if engine.GetCurrentPhase() != pb.PhaseType_PHASE_TYPE_NIGHT_SEER {
		t.Errorf("expected NIGHT_SEER, got %v", engine.GetCurrentPhase())
	}

	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "seer",
		Skill:    pb.SkillType_SKILL_TYPE_CHECK,
		TargetID: "wolf1",
	})
	effects, _ := engine.EndSubStep()

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
	if engine.GetCurrentPhase() != pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE {
		t.Errorf("expected NIGHT_RESOLVE, got %v", engine.GetCurrentPhase())
	}
	engine.EndSubStep() // NIGHT_RESOLVE -> DAY

	// 阶段6：白天
	if engine.GetCurrentPhase() != pb.PhaseType_PHASE_TYPE_DAY {
		t.Errorf("expected DAY, got %v", engine.GetCurrentPhase())
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
	engine := NewEngine(nil)

	engine.AddPlayer("guard", pb.RoleType_ROLE_TYPE_GUARD, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("wolf1", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("wolf2", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("witch", pb.RoleType_ROLE_TYPE_WITCH, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("seer", pb.RoleType_ROLE_TYPE_SEER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v2", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	engine.Start()

	// 守卫阶段
	engine.EndSubStep()

	// 狼人阶段：平票（wolf1 投 v1, wolf2 投 v2）
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "wolf1",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "v1",
	})
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "wolf2",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "v2",
	})
	engine.EndSubStep()

	// 女巫阶段：平票导致无击杀，NightKillTarget 应为空
	killTarget := engine.GetNightKillTarget()
	if killTarget != "" {
		t.Errorf("expected empty NightKillTarget for tie vote, got %s", killTarget)
	}

	// 完成剩余阶段
	engine.EndSubStep() // NIGHT_WITCH -> NIGHT_SEER
	engine.EndSubStep() // NIGHT_SEER -> NIGHT_RESOLVE
	engine.EndSubStep() // NIGHT_RESOLVE -> DAY

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
	engine := NewEngine(config)

	engine.AddPlayer("guard", pb.RoleType_ROLE_TYPE_GUARD, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("wolf", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("witch", pb.RoleType_ROLE_TYPE_WITCH, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("seer", pb.RoleType_ROLE_TYPE_SEER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("victim", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	engine.Start()

	// 守卫保护 victim
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "guard",
		Skill:    pb.SkillType_SKILL_TYPE_PROTECT,
		TargetID: "victim",
	})
	engine.EndSubStep()

	// 狼人杀 victim（应该被守卫挡住）
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "wolf",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "victim",
	})
	engine.EndSubStep()

	// 女巫阶段：守卫保护成功，NightKillTarget 应为空
	killTarget := engine.GetNightKillTarget()
	if killTarget != "" {
		t.Errorf("expected empty NightKillTarget when guard protects, got %s", killTarget)
	}

	// 完成剩余阶段
	engine.EndSubStep() // NIGHT_WITCH -> NIGHT_SEER
	engine.EndSubStep() // NIGHT_SEER -> NIGHT_RESOLVE
	engine.EndSubStep() // NIGHT_RESOLVE -> DAY

	// victim 存活
	victim, _ := engine.state.getPlayer("victim")
	if !victim.Alive {
		t.Error("expected victim to be protected by guard")
	}
}

func TestSubStepMode_MultipleRounds(t *testing.T) {
	engine := NewEngine(nil)

	engine.AddPlayer("guard", pb.RoleType_ROLE_TYPE_GUARD, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("wolf", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("witch", pb.RoleType_ROLE_TYPE_WITCH, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("seer", pb.RoleType_ROLE_TYPE_SEER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v2", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v3", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	engine.Start()

	if engine.GetCurrentRound() != 1 {
		t.Errorf("expected Round 1, got %d", engine.GetCurrentRound())
	}

	// 第一轮夜晚：空过
	engine.EndSubStep() // NIGHT_GUARD -> NIGHT_WOLF
	engine.EndSubStep() // NIGHT_WOLF -> NIGHT_WITCH
	engine.EndSubStep() // NIGHT_WITCH -> NIGHT_SEER
	engine.EndSubStep() // NIGHT_SEER -> NIGHT_RESOLVE
	engine.EndSubStep() // NIGHT_RESOLVE -> DAY

	if engine.GetCurrentPhase() != pb.PhaseType_PHASE_TYPE_DAY {
		t.Errorf("expected DAY, got %v", engine.GetCurrentPhase())
	}

	// 白天 -> 投票
	engine.EndSubStep() // DAY -> VOTE

	if engine.GetCurrentPhase() != pb.PhaseType_PHASE_TYPE_VOTE {
		t.Errorf("expected VOTE, got %v", engine.GetCurrentPhase())
	}

	// 投票 -> 第二轮夜晚
	engine.EndSubStep() // VOTE -> NIGHT_GUARD

	if engine.GetCurrentPhase() != pb.PhaseType_PHASE_TYPE_NIGHT_GUARD {
		t.Errorf("expected NIGHT_GUARD, got %v", engine.GetCurrentPhase())
	}

	if engine.GetCurrentRound() != 2 {
		t.Errorf("expected Round 2, got %d", engine.GetCurrentRound())
	}
}

func TestScenario_AllRolesActive(t *testing.T) {
	engine := NewEngine(nil)

	// Full game with all roles
	engine.AddPlayer("wolf1", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("wolf2", pb.RoleType_ROLE_TYPE_WEREWOLF, pb.Camp_CAMP_EVIL)
	engine.AddPlayer("seer", pb.RoleType_ROLE_TYPE_SEER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("witch", pb.RoleType_ROLE_TYPE_WITCH, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("guard", pb.RoleType_ROLE_TYPE_GUARD, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v1", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)
	engine.AddPlayer("v2", pb.RoleType_ROLE_TYPE_VILLAGER, pb.Camp_CAMP_GOOD)

	engine.Start()

	// NIGHT_GUARD: Guard protects seer
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "guard",
		Skill:    pb.SkillType_SKILL_TYPE_PROTECT,
		TargetID: "seer",
	})
	effects1, _ := engine.EndPhase() // NIGHT_GUARD -> NIGHT_WOLF

	// NIGHT_WOLF: Wolves kill witch
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "wolf1",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "witch",
	})
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "wolf2",
		Skill:    pb.SkillType_SKILL_TYPE_KILL,
		TargetID: "witch",
	})
	effects2, _ := engine.EndPhase() // NIGHT_WOLF -> NIGHT_WITCH

	// NIGHT_WITCH: Witch poisons wolf1
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "witch",
		Skill:    pb.SkillType_SKILL_TYPE_POISON,
		TargetID: "wolf1",
	})
	effects3, _ := engine.EndPhase() // NIGHT_WITCH -> NIGHT_SEER

	// NIGHT_SEER: Seer checks wolf2
	engine.SubmitSkillUse(&SkillUse{
		PlayerID: "seer",
		Skill:    pb.SkillType_SKILL_TYPE_CHECK,
		TargetID: "wolf2",
	})
	effects4, _ := engine.EndPhase() // NIGHT_SEER -> NIGHT_RESOLVE
	effects5, _ := engine.EndPhase() // NIGHT_RESOLVE -> DAY

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
