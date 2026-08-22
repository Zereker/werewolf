package werewolf

import (
	"testing"
)

// 本文件全部用例都走 rules_test.go 的 newRuleGame 建局辅助：
// 这里没有「期待建局报错」「断言内部字段」「并发」这几类用例，
// 座位表与阶段流转都能由辅助函数如实表达。

// ==================== Complete Game Tests ====================

func TestFullGame_WolvesWin(t *testing.T) {
	// 本局无神职，屠边条件（屠神/屠民）不适用，故显式使用屠城判定
	configRules := DefaultRules()
	configRules.VictoryMode = VictoryModeTownWipe

	// 2 狼 vs 3 民：开局 3 > 2 游戏继续，v1 出局后 2 <= 2 狼人获胜。
	//
	// 此前是 2 狼 vs 2 民，屠城判定在开局那一刻就已成立——游戏在第一次
	// EndPhase 就结束了，根本没走到杀人这步。测试因为忽略了返回错误而
	// 「通过」，但它验证的并不是自己声称的那件事。
	g := newRuleGameR(t, configRules, seats(
		wolf("wolf1"), wolf("wolf2"), villagers("v1", "v2", "v3"),
	)...)

	g.end(PhaseNightWolf)

	// NIGHT_WOLF: Wolves kill v1
	g.mustUse("wolf1", SkillKill, "v1")
	g.mustUse("wolf2", SkillKill, "v1")
	g.end(PhaseNightWitch)

	// NIGHT_WITCH -> NIGHT_SEER -> NIGHT_RESOLVE -> 结算（v1 dies）
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.endAny() // kill applied

	// v1 出局后 good(2) <= evil(2)，狼人获胜
	if !g.e.Status().Over {
		t.Error("expected game to be over (wolves win)")
	}

	if got := g.e.Status().Winner; got != CampEvil {
		t.Errorf("expected EVIL wins, got %v", got)
	}
}

func TestFullGame_GoodWins(t *testing.T) {
	// 1 wolf vs 3 villagers
	g := newRuleGame(t, nil, seats(
		wolf("wolf"), villagers("v1", "v2", "v3"),
	)...)

	// Night 1: NIGHT_GUARD -> NIGHT_WOLF
	g.end(PhaseNightWolf)

	// NIGHT_WOLF: Wolf kills v1
	g.mustUse("wolf", SkillKill, "v1")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseDay)

	// Day 1: No actions
	g.end(PhaseVote)

	// Vote 1: Everyone votes wolf
	g.vote("wolf", "v2", "v3")
	g.endAny()

	// Wolf eliminated, evil(0), good wins
	if !g.e.Status().Over {
		t.Error("expected game to be over (good wins)")
	}

	_, winner := checkVictory(g.e)
	if winner != CampGood {
		t.Errorf("expected GOOD wins, got %v", winner)
	}
}

// ==================== Rule Scenario Tests ====================

func TestScenario_WitchSavesVictim(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("wolf"), witch("witch"), villagers("victim", "v2"),
	)...)

	g.end(PhaseNightWolf)

	// NIGHT_WOLF: Wolf kills victim
	g.mustUse("wolf", SkillKill, "victim")
	g.end(PhaseNightWitch)

	// NIGHT_WITCH: Witch saves victim
	g.mustUse("witch", SkillAntidote, "victim")
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseDay)

	// Victim should still be alive
	g.assertAlive("victim", true, "expected victim to be saved by witch")
}

func TestScenario_GuardProtects(t *testing.T) {
	configRules := DefaultRules()
	configRules.SameGuardKillIsEmpty = true

	g := newRuleGameR(t, configRules, seats(
		wolf("wolf"), guard("guard"), villagers("victim", "v2"),
	)...)

	// NIGHT_GUARD: Guard protects victim
	g.mustUse("guard", SkillProtect, "victim")
	g.end(PhaseNightWolf)

	// NIGHT_WOLF: Wolf kills victim
	g.mustUse("wolf", SkillKill, "victim")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseDay)

	// Victim should still be alive (protected)
	g.assertAlive("victim", true, "expected victim to be protected by guard")
}

func TestScenario_VoteTie(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("wolf"), villagers("v1", "v2", "v3", "v4"),
	)...)

	// Night 1: No kill (skip all night phases)
	g.walkNight()
	g.end(PhaseVote)

	// Vote: 2 vs 2 tie
	g.vote("v1", "wolf", "v2")
	g.vote("wolf", "v1", "v3")
	g.endAny()

	// Both wolf and v1 should still be alive (tie = no one eliminated)
	if !g.alive("wolf") || !g.alive("v1") {
		t.Error("expected no one eliminated on tie vote")
	}
}

func TestScenario_MultipleRounds(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("wolf"), villagers("v1", "v2", "v3", "v4"),
	)...)

	if g.e.Status().Round != 1 {
		t.Errorf("expected Round 1, got %d", g.e.Status().Round)
	}

	// Night 1 (all phases) -> Day 1 -> Vote 1 -> Night 2
	g.walkNight()
	g.toNextNight()

	if g.e.Status().Round != 2 {
		t.Errorf("expected Round 2, got %d", g.e.Status().Round)
	}

	// Night 2 (all phases) -> Day 2 -> Vote 2 -> Night 3
	g.walkNight()
	g.toNextNight()

	if g.e.Status().Round != 3 {
		t.Errorf("expected Round 3, got %d", g.e.Status().Round)
	}
}

// ==================== Configuration Variant Tests ====================

func TestConfig_WitchCanSaveSelf_Enabled(t *testing.T) {
	configRules := DefaultRules()
	configRules.WitchCanSaveSelf = true

	g := newRuleGameR(t, configRules, seats(
		wolf("wolf"), witch("witch"), villagers("v1", "v2"),
	)...)

	g.end(PhaseNightWolf)

	// NIGHT_WOLF: Wolf kills witch
	g.mustUse("wolf", SkillKill, "witch")
	g.end(PhaseNightWitch)

	// NIGHT_WITCH: Witch saves self
	g.mustUse("witch", SkillAntidote, "witch")
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseDay)

	// Witch should still be alive (self-save allowed)
	g.assertAlive("witch", true, "expected witch to save self when WitchCanSaveSelf=true")
}

func TestConfig_WitchCanSaveSelf_Disabled(t *testing.T) {
	configRules := DefaultRules()
	configRules.WitchCanSaveSelf = false

	g := newRuleGameR(t, configRules, seats(
		wolf("wolf"), witch("witch"), villagers("v1", "v2"),
	)...)

	g.end(PhaseNightWolf)

	// NIGHT_WOLF: Wolf kills witch
	g.mustUse("wolf", SkillKill, "witch")
	g.end(PhaseNightWitch)

	// NIGHT_WITCH: Witch tries to save self
	g.mustUse("witch", SkillAntidote, "witch")
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	// 女巫是本局唯一神职，她一死屠神即成立，结算后直接进 END 而非 DAY
	g.end(PhaseEnd)

	// Witch should be dead (self-save not allowed)
	g.assertAlive("witch", false, "expected witch to be dead when WitchCanSaveSelf=false")
}

func TestConfig_SameGuardKill_Empty(t *testing.T) {
	configRules := DefaultRules()
	configRules.SameGuardKillIsEmpty = true

	g := newRuleGameR(t, configRules, seats(
		wolf("wolf"), guard("guard"), villagers("victim", "v2"),
	)...)

	// NIGHT_GUARD: Guard protects victim
	g.mustUse("guard", SkillProtect, "victim")
	g.end(PhaseNightWolf)

	// NIGHT_WOLF: Wolf kills victim
	g.mustUse("wolf", SkillKill, "victim")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseDay)

	// Victim survives (空刀)
	g.assertAlive("victim", true, "expected victim alive when SameGuardKillIsEmpty=true")
}

func TestConfig_SameGuardKill_NotEmpty(t *testing.T) {
	configRules := DefaultRules()
	configRules.SameGuardKillIsEmpty = false

	g := newRuleGameR(t, configRules, seats(
		wolf("wolf"), guard("guard"), villagers("victim", "v2"),
	)...)

	// NIGHT_GUARD: Guard protects victim
	g.mustUse("guard", SkillProtect, "victim")
	g.end(PhaseNightWolf)

	// NIGHT_WOLF: Wolf kills victim
	g.mustUse("wolf", SkillKill, "victim")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseDay)

	// Victim dies (guard doesn't cancel kill when SameGuardKillIsEmpty=false)
	g.assertAlive("victim", false, "expected victim dead when SameGuardKillIsEmpty=false")
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
	setup := func(t *testing.T, canUseBoth bool) *ruleGame {
		t.Helper()

		configRules := DefaultRules()
		configRules.WitchCanUseBothPotions = canUseBoth

		g := newRuleGameR(t, configRules, seats(
			wolf("wolf"), witch("witch"), villagers("victim", "v2", "v3"),
		)...)

		g.end(PhaseNightWolf)

		// NIGHT_WOLF: 狼刀 victim
		g.mustUse("wolf", SkillKill, "victim")
		g.end(PhaseNightWitch)

		// NIGHT_WITCH: 先救 victim，再毒 wolf
		g.mustUse("witch", SkillAntidote, "victim")
		g.mustUse("witch", SkillPoison, "wolf")

		// NIGHT_WITCH -> NIGHT_SEER -> NIGHT_RESOLVE -> DAY（放宽后狼被毒死，
		// 好人获胜，此处直接进 END，故最后一步不断言目标阶段）
		g.end(PhaseNightSeer)
		g.end(PhaseNightResolve)
		g.endAny()

		return g
	}

	t.Run("默认禁止同夜双开药", func(t *testing.T) {
		g := setup(t, false)
		witch := g.info("witch")

		// 先提交的解药生效
		g.assertAlive("victim", true, "解药先提交，victim 应当被救")
		if witch.Var(VarWitchAntidote) != "" {
			t.Error("解药应当已被消耗")
		}

		// 后提交的毒药被拒
		g.assertAlive("wolf", true, "同夜第二瓶药应当无效，wolf 不应被毒死")
		if witch.Var(VarWitchPoison) == "" {
			t.Error("毒药未生效，不应被消耗")
		}
		if g.e.Status().Over {
			t.Error("狼人存活，游戏不应结束")
		}
	})

	t.Run("放宽后允许同夜双开药", func(t *testing.T) {
		g := setup(t, true)
		witch := g.info("witch")

		g.assertAlive("victim", true, "expected victim to be saved")
		g.assertAlive("wolf", false, "expected wolf to be poisoned")
		if witch.Var(VarWitchAntidote) != "" || witch.Var(VarWitchPoison) != "" {
			t.Error("两瓶药都应当被消耗")
		}
		if !g.e.Status().Over {
			t.Error("expected game over (good wins after wolf poisoned)")
		}
	})
}

func TestScenario_SeerIdentifiesWolf(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		wolf("wolf"), seer("seer"), villagers("v1", "v2", "v3"),
	)...)

	g.end(PhaseNightWolf)

	// NIGHT_WOLF: Wolf kills v1
	g.mustUse("wolf", SkillKill, "v1")
	g.end(PhaseNightWitch)
	g.end(PhaseNightSeer)

	// NIGHT_SEER: Seer checks wolf
	g.mustUse("seer", SkillCheck, "wolf")
	effects := g.end(PhaseNightResolve)
	g.end(PhaseDay)

	// Check the seer's result
	checkEffect := findEffect(effects, EventCheck)
	if checkEffect == nil {
		t.Fatal("expected check effect")
	}
	if checkEffect.Data["isGood"] != false {
		t.Error("expected seer to identify wolf as evil")
	}
	if checkEffect.Data["camp"] != CampEvil {
		t.Errorf("expected camp=EVIL, got %v", checkEffect.Data["camp"])
	}
}

// ==================== Sub-Step Mode Tests ====================

func TestSubStepMode_FullNightCycle(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		guard("guard"), wolf("wolf1"), wolf("wolf2"),
		witch("witch"), seer("seer"), villager("v1"),
	)...)

	// 阶段1：守卫阶段
	if g.e.Status().Phase != PhaseNightGuard {
		t.Errorf("expected NIGHT_GUARD, got %v", g.e.Status().Phase)
	}

	g.mustUse("guard", SkillProtect, "seer")
	g.end(PhaseNightWolf)

	// 阶段2：狼人阶段——狼人可以查询队友
	teammates := g.e.Teammates("wolf1")
	if len(teammates) != 1 || teammates[0] != "wolf2" {
		t.Errorf("wolf1 should have wolf2 as teammate, got %v", teammates)
	}

	// 狼人投票击杀
	g.mustUse("wolf1", SkillKill, "v1")
	g.mustUse("wolf2", SkillKill, "v1")
	g.end(PhaseNightWitch)

	// 阶段3：女巫阶段——女巫可以查询被杀者
	if killTarget := NightKillTarget(g.e); killTarget != "v1" {
		t.Errorf("expected NightKillTarget=v1, got %s", killTarget)
	}

	// 女巫使用解药救人
	g.mustUse("witch", SkillAntidote, "v1")
	g.end(PhaseNightSeer)

	// 阶段4：预言家阶段
	g.mustUse("seer", SkillCheck, "wolf1")
	effects := g.end(PhaseNightResolve)

	// 验证预言家查验结果
	checkEffect := findEffect(effects, EventCheck)
	if checkEffect == nil {
		t.Fatal("expected check effect")
	}
	if checkEffect.Data["isGood"] != false {
		t.Error("expected wolf1 to be identified as evil")
	}

	// 阶段5：夜晚结算 -> 阶段6：白天
	g.end(PhaseDay)

	// 验证状态：v1 被救活
	g.assertAlive("v1", true, "expected v1 to be saved by witch")

	// 验证状态：seer 被保护（使用 NightContext）
	if !protectedInEngine(g.e, "seer") {
		t.Error("expected seer to be protected by guard")
	}
}

func TestSubStepMode_WolfVoteTie(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		guard("guard"), wolf("wolf1"), wolf("wolf2"),
		witch("witch"), seer("seer"), villagers("v1", "v2"),
	)...)

	// 守卫阶段
	g.end(PhaseNightWolf)

	// 狼人阶段：平票（wolf1 投 v1, wolf2 投 v2）
	g.mustUse("wolf1", SkillKill, "v1")
	g.mustUse("wolf2", SkillKill, "v2")
	g.end(PhaseNightWitch)

	// 女巫阶段：平票导致无击杀，NightKillTarget 应为空
	if killTarget := NightKillTarget(g.e); killTarget != "" {
		t.Errorf("expected empty NightKillTarget for tie vote, got %s", killTarget)
	}

	// 完成剩余阶段
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseDay)

	// v1 和 v2 都应该存活（没有达成共识）
	if !g.alive("v1") || !g.alive("v2") {
		t.Error("expected both v1 and v2 to be alive when wolves have tie vote")
	}
}

func TestSubStepMode_GuardProtectsFromKill(t *testing.T) {
	configRules := DefaultRules()
	configRules.SameGuardKillIsEmpty = true

	g := newRuleGameR(t, configRules, seats(
		guard("guard"), wolf("wolf"), witch("witch"),
		seer("seer"), villager("victim"),
	)...)

	// 守卫保护 victim
	g.mustUse("guard", SkillProtect, "victim")
	g.end(PhaseNightWolf)

	// 狼人杀 victim（应该被守卫挡住）
	g.mustUse("wolf", SkillKill, "victim")
	g.end(PhaseNightWitch)

	// 女巫阶段：刀口照常记录（女巫不知道守卫守了谁），
	// 守护是否抵消由 NIGHT_RESOLVE 判定
	if killTarget := NightKillTarget(g.e); killTarget != "victim" {
		t.Errorf("expected NightKillTarget=victim (protection resolves later), got %s", killTarget)
	}

	// 完成剩余阶段
	g.end(PhaseNightSeer)
	g.end(PhaseNightResolve)
	g.end(PhaseDay)

	// victim 存活
	g.assertAlive("victim", true, "expected victim to be protected by guard")
}

func TestSubStepMode_MultipleRounds(t *testing.T) {
	g := newRuleGame(t, nil, seats(
		guard("guard"), wolf("wolf"), witch("witch"), seer("seer"),
		villagers("v1", "v2", "v3"),
	)...)

	if g.e.Status().Round != 1 {
		t.Errorf("expected Round 1, got %d", g.e.Status().Round)
	}

	// 第一轮夜晚：空过，walkNight 逐步断言到 DAY
	g.walkNight()

	// 白天 -> 投票 -> 第二轮夜晚，toNextNight 逐步断言 VOTE 与 NIGHT_GUARD
	g.toNextNight()

	if g.e.Status().Round != 2 {
		t.Errorf("expected Round 2, got %d", g.e.Status().Round)
	}
}

func TestScenario_AllRolesActive(t *testing.T) {
	// Full game with all roles
	g := newRuleGame(t, nil, seats(
		wolf("wolf1"), wolf("wolf2"), seer("seer"), witch("witch"),
		guard("guard"), villagers("v1", "v2"),
	)...)

	// NIGHT_GUARD: Guard protects seer
	g.mustUse("guard", SkillProtect, "seer")
	effects1 := g.end(PhaseNightWolf)

	// NIGHT_WOLF: Wolves kill witch
	g.mustUse("wolf1", SkillKill, "witch")
	g.mustUse("wolf2", SkillKill, "witch")
	effects2 := g.end(PhaseNightWitch)

	// NIGHT_WITCH: Witch poisons wolf1
	g.mustUse("witch", SkillPoison, "wolf1")
	effects3 := g.end(PhaseNightSeer)

	// NIGHT_SEER: Seer checks wolf2
	g.mustUse("seer", SkillCheck, "wolf2")
	effects4 := g.end(PhaseNightResolve)
	effects5 := g.end(PhaseDay)

	// Collect all effects
	allEffects := append(append(append(append(effects1, effects2...), effects3...), effects4...), effects5...)

	// Verify effects
	hasProtect := false
	hasKill := false
	hasPoison := false
	hasCheck := false

	for _, e := range allEffects {
		switch e.Type {
		case EventProtect:
			hasProtect = true
		case EventKill:
			hasKill = true
		case EventPoison:
			hasPoison = true
		case EventCheck:
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
	g.assertAlive("witch", false, "expected witch to be dead")
	g.assertAlive("wolf1", false, "expected wolf1 to be poisoned")
	g.assertAlive("seer", true, "expected seer to be alive (protected)")
}
