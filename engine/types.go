// types.go 内核的词汇表：阶段、角色、技能、阵营、类别。
//
// 这里只有**类型**，没有取值。内核不知道有哪些阶段、哪些角色——
// 「NIGHT_WITCH」「WEREWOLF」是狼人杀这套规则定义的，见根包 vocab.go。
//
// 底层都是字符串，不是编号。编号是 protobuf 时代留下的：那时它们由 .proto
// 生成，编号进了线格式。protobuf 拆掉之后编号只剩负担——快照按名字写，
// 日志按名字打，于是每个类型都得额外挂一张对照表和一对 JSON 方法，
// 一百多行代码只为把值翻译回它本来的样子。
//
// 名字直接就是值之后，那些全部消失：JSON 天然可读、String() 是一行、
// 规则包定义自己的取值只需 RoleType("KNIGHT")，不会与任何人撞号。
//
// 零值是空串，语义即「未指定」。

package engine

// PhaseType 游戏阶段。取值由规则定义。
type PhaseType string

// 三个由内核自己拥有的阶段：它们是状态机的生命周期，不是某套规则的环节。
//
// 规则的阶段环从 Config.StartPhase 开始、以 PhaseEnd 收尾；PhaseStart
// 是「还没开局」这个状态本身，AddPlayer 只在它里面被允许。
const (
	PhaseUnspecified PhaseType = ""
	PhaseStart       PhaseType = "START" // 还没开局
	PhaseEnd         PhaseType = "END"   // 已经结束
)

// String 实现 fmt.Stringer。
func (v PhaseType) String() string {
	if v == PhaseUnspecified {
		return "UNSPECIFIED"
	}
	return string(v)
}

// RoleType 角色类型。取值由规则定义。
type RoleType string

const (
	// RoleUnspecified 未指定。在 PhaseStep 上它表示「所有角色」。
	RoleUnspecified RoleType = ""

	// RoleGod 主持人。它不是一个玩家身份——入座会被拒，就绪判定不数它。
	// 声明了它的阶段步骤是「该念一段公告了」，不是「等某个人行动」。
	RoleGod RoleType = "GOD"
)

// String 实现 fmt.Stringer。
func (v RoleType) String() string {
	if v == RoleUnspecified {
		return "UNSPECIFIED"
	}
	return string(v)
}

// SkillType 技能类型。取值由规则定义。
type SkillType string

const (
	// SkillUnspecified 未指定。
	SkillUnspecified SkillType = ""

	// SkillSkip 主动放弃行动。任何回合制游戏都有这个动作，也是唯一
	// 不需要目标的技能——内核据此跳过目标校验。
	SkillSkip SkillType = "SKIP"

	// SkillAnnounce 主持人的公告，与 RoleGod 配对。
	SkillAnnounce SkillType = "ANNOUNCE"
)

// String 实现 fmt.Stringer。
func (v SkillType) String() string {
	if v == SkillUnspecified {
		return "UNSPECIFIED"
	}
	return string(v)
}

// Camp 一个「边」的标签，胜负判定的结果就是它。
//
// 内核**不预设任何取值**：好人与狼人是狼人杀的两边，阿瓦隆是正义与邪恶，
// 血染钟楼还有单独结算的旅行者。内核只知道「有若干个边，其中一个可能会赢」，
// 也知道每名玩家可能属于某一边（VarCamp），但不知道那是哪一边、意味着什么。
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

// VarCamp 阵营在玩家 Vars 里的标准键名。
//
// 内核认这一个键：它的值会填进 PlayerInfo 与 SelfInfo 上的 Camp 字段，
// 让「这名玩家站哪一边」不必每个使用者自己去 Vars 里翻。值由规则发放
// （见 RoleSetup），内核不检查也不解释。
//
// 只有这一个。「神职/平民」这种阵营之内的细分是狼人杀为了屠边判定才需要的，
// 内核不认得——规则包自己定一个键即可（见 werewolf.VarCategory）。
const VarCamp = "camp"
