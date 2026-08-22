package engine

// 内核测试用的词汇表。
//
// 内核不认识「女巫」「狼人」这些词，但测试总得摆出一个具体的局面。
// 这里定义一套只在测试里存在的取值——它们是**测试夹具**，不是内核 API。
//
// 名字沿用狼人杀的说法，是为了让这些测试读起来仍然像人话；对内核而言
// 它们与 RoleType("KNIGHT") 没有任何区别，换成别的字符串测试照样过。
// 这一点本身就是被测的性质：状态机不认得任何具体取值。
const (
	roleWerewolf = RoleType("WEREWOLF")
	roleSeer     = RoleType("SEER")
	roleWitch    = RoleType("WITCH")
	roleHunter   = RoleType("HUNTER")
	roleVillager = RoleType("VILLAGER")
	roleGuard    = RoleType("GUARD")

	phaseNight        = PhaseType("NIGHT")
	phaseNightGuard   = PhaseType("NIGHT_GUARD")
	phaseNightWolf    = PhaseType("NIGHT_WOLF")
	phaseNightWitch   = PhaseType("NIGHT_WITCH")
	phaseNightSeer    = PhaseType("NIGHT_SEER")
	phaseNightResolve = PhaseType("NIGHT_RESOLVE")
	phaseNightHunter  = PhaseType("NIGHT_HUNTER")
	phaseDay          = PhaseType("DAY")
	phaseDayHunter    = PhaseType("DAY_HUNTER")
	phaseVote         = PhaseType("VOTE")

	skillKill     = SkillType("KILL")
	skillCheck    = SkillType("CHECK")
	skillProtect  = SkillType("PROTECT")
	skillAntidote = SkillType("ANTIDOTE")
	skillPoison   = SkillType("POISON")
	skillVote     = SkillType("VOTE")
	skillShoot    = SkillType("SHOOT")

	eventKill      = EventType("KILL")
	eventProtect   = EventType("PROTECT")
	eventSave      = EventType("SAVE")
	eventPoison    = EventType("POISON")
	eventCheck     = EventType("CHECK")
	eventEliminate = EventType("ELIMINATE")
	eventShoot     = EventType("SHOOT")
	eventVoteTied  = EventType("VOTE_TIED")

	campGood = Camp("GOOD")
	campEvil = Camp("EVIL")

	// testKillTarget 一个回合变量的键。内核不知道它是「刀口」，
	// 只知道有人往本回合的状态里写了一项东西。
	testKillTarget = "test.kill_target"

	// 三个「本回合标记了某个玩家」的键。内核不知道它们叫「被守」「被救」
	// 「被毒」，只知道有人往某个玩家身上写了一项本回合有效的东西。
	testMarkA = "test.mark_a"
	testMarkB = "test.mark_b"
	testMarkC = "test.mark_c"

	// testVarStock 一项跟着玩家走一整局的状态。
	testVarStock = "test.stock"
)

// testConfig 一副够用的阶段图：守卫 -> 狼 -> 女巫 -> 预言家 -> 结算 -> 白天 -> 投票 -> 回到守卫。
//
// 内核的测试需要一个合法的 GameConfig，但内核自己没有默认板子——
// 「有哪些阶段」是规则的事。这里手摆一副，与狼人杀那副长得像，
// 但它只属于这些测试。
func testConfig() *GameConfig {
	step := func(role RoleType, skill SkillType) []PhaseStep {
		return []PhaseStep{{Role: role, Skill: skill, Required: true}}
	}
	return &GameConfig{
		StartPhase: phaseNightGuard,
		Phases: map[PhaseType]*PhaseConfig{
			phaseNightGuard:   {Type: phaseNightGuard, Steps: step(roleGuard, skillProtect), NextPhase: phaseNightWolf},
			phaseNightWolf:    {Type: phaseNightWolf, Steps: step(roleWerewolf, skillKill), NextPhase: phaseNightWitch},
			phaseNightWitch:   {Type: phaseNightWitch, Steps: []PhaseStep{{Role: roleWitch, Skill: skillAntidote, AllowDeadTarget: true}, {Role: roleWitch, Skill: skillPoison}}, NextPhase: phaseNightSeer},
			phaseNightSeer:    {Type: phaseNightSeer, Steps: step(roleSeer, skillCheck), NextPhase: phaseNightResolve},
			phaseNightResolve: {Type: phaseNightResolve, NextPhase: phaseDay},
			phaseNightHunter:  {Type: phaseNightHunter, Steps: []PhaseStep{{Role: roleHunter, Skill: skillShoot, Group: "shoot"}, {Role: roleHunter, Skill: SkillSkip, Group: "shoot"}}, NextPhase: phaseDay},
			phaseDay:          {Type: phaseDay, NextPhase: phaseVote},
			phaseVote:         {Type: phaseVote, Steps: []PhaseStep{{Role: RoleUnspecified, Skill: skillVote, Required: true, Multiple: true}}, NextPhase: phaseNightGuard},
			phaseDayHunter:    {Type: phaseDayHunter, Steps: []PhaseStep{{Role: roleHunter, Skill: skillShoot, Group: "shoot"}, {Role: roleHunter, Skill: SkillSkip, Group: "shoot"}}, NextPhase: phaseNightGuard},
		},
	}
}
