// vocab.go 狼人杀的词汇表：九个阶段、六个角色、十个技能、两个阵营、三种类别。
//
// 内核只有类型，取值全在这里——它不知道有「女巫」这个角色，也不知道
// 「NIGHT_WITCH」这个阶段。换一套规则，这个文件整个换掉即可。
//
// 类型经 alias 再导出（见 alias.go），因此使用者写 werewolf.RoleWitch
// 与 werewolf.RoleType 都成立，不必显式 import 内核包。

package werewolf

// 九个阶段。
const (
	PhaseNight        PhaseType = "NIGHT" // 夜晚（整体，用于批量模式）
	PhaseNightGuard   PhaseType = "NIGHT_GUARD"
	PhaseNightWolf    PhaseType = "NIGHT_WOLF"
	PhaseNightWitch   PhaseType = "NIGHT_WITCH"
	PhaseNightSeer    PhaseType = "NIGHT_SEER"
	PhaseNightResolve PhaseType = "NIGHT_RESOLVE" // 夜晚结算（击杀、猎人触发等）
	PhaseNightHunter  PhaseType = "NIGHT_HUNTER"  // 猎人阶段（被动触发）
	PhaseDay          PhaseType = "DAY"
	PhaseDayHunter    PhaseType = "DAY_HUNTER" // 白天猎人阶段（被投票出局后触发）
	PhaseVote         PhaseType = "VOTE"
)

// PhaseStart 与 PhaseEnd 由内核拥有（还没开局 / 已经结束），
// 经 alias 再导出，见 alias.go。

// 六个角色，外加上帝这个系统角色。
const (
	RoleWerewolf RoleType = "WEREWOLF"
	RoleSeer     RoleType = "SEER"
	RoleWitch    RoleType = "WITCH"
	RoleHunter   RoleType = "HUNTER"
	RoleVillager RoleType = "VILLAGER"
	RoleGuard    RoleType = "GUARD"
)

// 十个技能。
const (
	SkillKill     SkillType = "KILL"     // 狼人击杀
	SkillCheck    SkillType = "CHECK"    // 预言家查验
	SkillProtect  SkillType = "PROTECT"  // 守卫保护
	SkillAntidote SkillType = "ANTIDOTE" // 女巫解药
	SkillPoison   SkillType = "POISON"   // 女巫毒药
	SkillVote     SkillType = "VOTE"     // 投票
	SkillSpeak    SkillType = "SPEAK"    // 发言
	SkillShoot    SkillType = "SHOOT"    // 猎人开枪
)

// RoleGod、SkillSkip、SkillAnnounce 由内核拥有（主持人不是玩家、弃权是
// 通用动作），经 alias 再导出，见 alias.go。

// 狼人杀的事件类型：规则给「发生了什么」起的名字。
//
// 内核不认得它们——一个 KILL 效果单独发出去，谁都不会死；真正改状态的是
// 旁边那条 SET_ALIVE。两个效果，两件事：前者给受众与效果流看，
// 后者给状态机看。
const (
	EventKill      EventType = "KILL"      // 狼人击杀
	EventProtect   EventType = "PROTECT"   // 守卫保护
	EventSave      EventType = "SAVE"      // 女巫救人
	EventPoison    EventType = "POISON"    // 女巫毒杀
	EventCheck     EventType = "CHECK"     // 预言家查验
	EventEliminate EventType = "ELIMINATE" // 投票出局
	EventShoot     EventType = "SHOOT"     // 猎人开枪
	EventSkip      EventType = "SKIP"      // 跳过行动
	EventVoteTied  EventType = "VOTE_TIED" // 投票平票或无人得票，本轮无人出局
)
