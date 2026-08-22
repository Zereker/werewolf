// types.go 游戏词汇表：阶段、阵营、角色、技能。
//
// 这几个枚举此前由 protobuf 生成。但这个库从来没有真的序列化过 protobuf——
// 没有 Marshal、没有 gRPC——protobuf 在这里的全部作用就是声明枚举，
// 代价却是一个外部依赖、一份构建期的 protoc、以及每个使用者都得
// 多 import 一个包才能写出 RoleWerewolf 这样的名字。
//
// 取值刻意与 protobuf 时代的编号一致：编号不再进存储（快照按名字写），
// 但保持一致省掉了一次无谓的数据迁移。

package werewolf

import "fmt"

// PhaseType 游戏阶段
type PhaseType int32

const (
	PhaseUnspecified  PhaseType = 0
	PhaseStart        PhaseType = 10
	PhaseNight        PhaseType = 20 // 夜晚（整体，用于批量模式）
	PhaseNightGuard   PhaseType = 21 // 守卫阶段
	PhaseNightWolf    PhaseType = 22 // 狼人阶段
	PhaseNightWitch   PhaseType = 23 // 女巫阶段
	PhaseNightSeer    PhaseType = 24 // 预言家阶段
	PhaseNightResolve PhaseType = 25 // 夜晚结算阶段（处理击杀、猎人触发等）
	PhaseNightHunter  PhaseType = 26 // 猎人阶段（被动触发）
	PhaseDay          PhaseType = 30
	PhaseDayHunter    PhaseType = 31 // 白天猎人阶段（被投票出局后触发）
	PhaseVote         PhaseType = 40
	PhaseEnd          PhaseType = 50
)

// String 实现 fmt.Stringer。
//
// 输出沿用枚举的全名，日志与错误信息里一眼能看出是哪一类。
func (v PhaseType) String() string {
	if s, ok := phaseTypeNames[v]; ok {
		return s
	}
	return fmt.Sprintf("PhaseType(%d)", int32(v))
}

// phaseTypeNames 全部取值到名字的映射，遍历它即可枚举所有取值。
var phaseTypeNames = map[PhaseType]string{
	PhaseUnspecified:  "UNSPECIFIED",
	PhaseStart:        "START",
	PhaseNight:        "NIGHT",
	PhaseNightGuard:   "NIGHT_GUARD",
	PhaseNightWolf:    "NIGHT_WOLF",
	PhaseNightWitch:   "NIGHT_WITCH",
	PhaseNightSeer:    "NIGHT_SEER",
	PhaseNightResolve: "NIGHT_RESOLVE",
	PhaseNightHunter:  "NIGHT_HUNTER",
	PhaseDay:          "DAY",
	PhaseDayHunter:    "DAY_HUNTER",
	PhaseVote:         "VOTE",
	PhaseEnd:          "END",
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

// RoleType 角色类型
type RoleType int32

const (
	RoleUnspecified RoleType = 0
	RoleGod         RoleType = 1 // 上帝（系统角色，用于发送公告）
	RoleWerewolf    RoleType = 2
	RoleSeer        RoleType = 3
	RoleWitch       RoleType = 4
	RoleHunter      RoleType = 5
	RoleVillager    RoleType = 6
	RoleGuard       RoleType = 7
)

// String 实现 fmt.Stringer。
//
// 输出沿用枚举的全名，日志与错误信息里一眼能看出是哪一类。
func (v RoleType) String() string {
	if s, ok := roleTypeNames[v]; ok {
		return s
	}
	return fmt.Sprintf("RoleType(%d)", int32(v))
}

// roleTypeNames 全部取值到名字的映射，遍历它即可枚举所有取值。
var roleTypeNames = map[RoleType]string{
	RoleUnspecified: "UNSPECIFIED",
	RoleGod:         "GOD",
	RoleWerewolf:    "WEREWOLF",
	RoleSeer:        "SEER",
	RoleWitch:       "WITCH",
	RoleHunter:      "HUNTER",
	RoleVillager:    "VILLAGER",
	RoleGuard:       "GUARD",
}

// SkillType 技能类型
type SkillType int32

const (
	SkillUnspecified SkillType = 0
	SkillKill        SkillType = 1  // 狼人击杀
	SkillCheck       SkillType = 2  // 预言家查验
	SkillProtect     SkillType = 3  // 守卫保护
	SkillAntidote    SkillType = 4  // 女巫解药
	SkillPoison      SkillType = 5  // 女巫毒药
	SkillVote        SkillType = 6  // 投票
	SkillSpeak       SkillType = 7  // 发言
	SkillShoot       SkillType = 8  // 猎人开枪
	SkillAnnounce    SkillType = 9  // 上帝公告
	SkillSkip        SkillType = 10 // 跳过行动（主动放弃技能使用）
)

// String 实现 fmt.Stringer。
//
// 输出沿用枚举的全名，日志与错误信息里一眼能看出是哪一类。
func (v SkillType) String() string {
	if s, ok := skillTypeNames[v]; ok {
		return s
	}
	return fmt.Sprintf("SkillType(%d)", int32(v))
}

// skillTypeNames 全部取值到名字的映射，遍历它即可枚举所有取值。
var skillTypeNames = map[SkillType]string{
	SkillUnspecified: "UNSPECIFIED",
	SkillKill:        "KILL",
	SkillCheck:       "CHECK",
	SkillProtect:     "PROTECT",
	SkillAntidote:    "ANTIDOTE",
	SkillPoison:      "POISON",
	SkillVote:        "VOTE",
	SkillSpeak:       "SPEAK",
	SkillShoot:       "SHOOT",
	SkillAnnounce:    "ANNOUNCE",
	SkillSkip:        "SKIP",
}
