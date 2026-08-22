// types.go 游戏词汇表：阶段、阵营、角色、技能。
//
// 这几个枚举的底层是**字符串**，不是编号。
//
// 编号是 protobuf 时代留下的：那时它们由 .proto 生成，编号进了线格式。
// protobuf 拆掉之后编号就只剩负担了——快照按名字写（编号对不上号，
// 存档要给人看，也可能被别的语言读），日志按名字打，于是每个类型都得
// 额外挂一张「编号到名字」的对照表和一对 JSON 方法，一百多行代码只为
// 把值翻译回它本来的样子。
//
// 名字直接就是值之后，那些全部消失：JSON 天然可读、String() 是一行、
// 第三方定义自己的角色只需 RoleType("KNIGHT")，不再需要「自定义取值
// 从 1000 起」这类避让约定——字符串本来就不会撞号。
//
// 零值是空串，语义即「未指定」。

package werewolf

// PhaseType 游戏阶段。
//
// 第三方可以定义自己的阶段，取值随意，不会与内置的撞号。
type PhaseType string

const (
	PhaseUnspecified  PhaseType = ""
	PhaseStart        PhaseType = "START"
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
	PhaseEnd          PhaseType = "END"
)

// String 实现 fmt.Stringer。
func (v PhaseType) String() string {
	if v == PhaseUnspecified {
		return "UNSPECIFIED"
	}
	return string(v)
}

// Camp 一个「边」的标签，胜负判定的结果就是它。
//
// 内核**不预设任何取值**：好人与狼人是狼人杀的两边（见 wolfcamp.go），
// 阿瓦隆是正义与邪恶，血染钟楼还有单独结算的旅行者。内核只知道
// 「有若干个边，其中一个可能会赢」，具体是哪些边由规则定义。
//
// 底层是字符串而不是编号：这个值要出现在日志、指标、快照与
// VictoryChecker 的返回值里，名字本身就是最稳定的表示，也省掉了
// 一张「编号到名字」的对照表。
type Camp string

// CampUnspecified 还没分出胜负，或者这名玩家不属于任何一边。
const CampUnspecified Camp = ""

// String 实现 fmt.Stringer。
func (v Camp) String() string {
	if v == CampUnspecified {
		return "UNSPECIFIED"
	}
	return string(v)
}

// RoleType 角色类型。
//
// 第三方定义自己的角色只需一个自己的字符串，比如 RoleType("KNIGHT")。
type RoleType string

const (
	RoleUnspecified RoleType = ""
	RoleGod         RoleType = "GOD" // 上帝（系统角色，用于发送公告）
	RoleWerewolf    RoleType = "WEREWOLF"
	RoleSeer        RoleType = "SEER"
	RoleWitch       RoleType = "WITCH"
	RoleHunter      RoleType = "HUNTER"
	RoleVillager    RoleType = "VILLAGER"
	RoleGuard       RoleType = "GUARD"
)

// String 实现 fmt.Stringer。
func (v RoleType) String() string {
	if v == RoleUnspecified {
		return "UNSPECIFIED"
	}
	return string(v)
}

// SkillType 技能类型。
type SkillType string

const (
	SkillUnspecified SkillType = ""
	SkillKill        SkillType = "KILL"     // 狼人击杀
	SkillCheck       SkillType = "CHECK"    // 预言家查验
	SkillProtect     SkillType = "PROTECT"  // 守卫保护
	SkillAntidote    SkillType = "ANTIDOTE" // 女巫解药
	SkillPoison      SkillType = "POISON"   // 女巫毒药
	SkillVote        SkillType = "VOTE"     // 投票
	SkillSpeak       SkillType = "SPEAK"    // 发言
	SkillShoot       SkillType = "SHOOT"    // 猎人开枪
	SkillAnnounce    SkillType = "ANNOUNCE" // 上帝公告
	SkillSkip        SkillType = "SKIP"     // 跳过行动（主动放弃技能使用）
)

// String 实现 fmt.Stringer。
func (v SkillType) String() string {
	if v == SkillUnspecified {
		return "UNSPECIFIED"
	}
	return string(v)
}
