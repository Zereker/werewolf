package hiddenrole

// The vocabulary the kernel's own tests use.
//
// The kernel does not know the words "witch" or "werewolf", but a test still
// has to lay out some concrete board. The values defined here exist only in
// the tests -- they are **test fixtures**, not kernel API.
//
// The names follow werewolf's so that these tests still read like something a
// person would say; to the kernel they are no different from
// RoleType("KNIGHT"), and the tests pass just as well with any other strings.
// That is itself one of the properties under test: the state machine
// recognises no concrete value.
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

	// testKillTarget is the key of one round variable. The kernel does not
	// know it is "tonight's kill", only that somebody wrote something into
	// this round's state.
	testKillTarget = "test.kill_target"

	// Three keys of the form "marked on a player this round". The kernel
	// does not know they mean guarded, healed and poisoned, only that
	// somebody wrote something round-scoped onto a player.
	testMarkA = "test.mark_a"
	testMarkB = "test.mark_b"
	testMarkC = "test.mark_c"

	// testVarStock is a piece of state that follows a player for the whole
	// game.
	testVarStock = "test.stock"
)

// testConfig is a phase graph that is enough to work with: guard -> wolves
// -> witch -> seer -> resolution -> day -> vote -> back to guard.
//
// The kernel's tests need a valid Config, and the kernel has no default board
// of its own -- which phases exist is the rules' business. This one is laid
// out by hand, looks much like werewolf's, and belongs to these tests alone.
func testConfig() *Config {
	step := func(role RoleType, skill SkillType) []PhaseStep {
		return []PhaseStep{{Role: role, Skill: skill, Required: true}}
	}
	return &Config{
		StartPhase: phaseNightGuard,
		Phases: map[PhaseType]*PhaseConfig{
			phaseNightGuard:   {Type: phaseNightGuard, Steps: step(roleGuard, skillProtect), NextPhase: phaseNightWolf, ClearsRoundVars: true},
			phaseNightWolf:    {Type: phaseNightWolf, Steps: step(roleWerewolf, skillKill), NextPhase: phaseNightWitch},
			phaseNightWitch:   {Type: phaseNightWitch, Steps: []PhaseStep{{Role: roleWitch, Skill: skillAntidote, AllowDeadTarget: true}, {Role: roleWitch, Skill: skillPoison}}, NextPhase: phaseNightSeer},
			phaseNightSeer:    {Type: phaseNightSeer, Steps: step(roleSeer, skillCheck), NextPhase: phaseNightResolve},
			phaseNightResolve: {Type: phaseNightResolve, NextPhase: phaseDay},
			phaseNightHunter:  {Type: phaseNightHunter, Steps: []PhaseStep{{Role: roleHunter, Skill: skillShoot, Group: "shoot"}, {Role: roleHunter, Skill: SkillSkip, Group: "shoot"}}, NextPhase: phaseDay},
			phaseDay:          {Type: phaseDay, NextPhase: phaseVote},
			phaseVote:         {Type: phaseVote, Steps: []PhaseStep{{Role: RoleUnspecified, Skill: skillVote, Required: true, Multiple: true}}, NextPhase: phaseNightGuard, EndsRound: true},
			phaseDayHunter:    {Type: phaseDayHunter, Steps: []PhaseStep{{Role: roleHunter, Skill: skillShoot, Group: "shoot"}, {Role: roleHunter, Skill: SkillSkip, Group: "shoot"}}, NextPhase: phaseNightGuard, EndsRound: true},
		},
	}
}
