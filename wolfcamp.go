// wolfcamp.go 狼人杀的阵营与角色类别。
//
// 这两样此前是内核状态的一部分：PlayerState 上两个字段、AddCustomPlayer
// 上两个参数、快照里两列。也就是说内核知道「一局游戏分好人与狼人两边、
// 好人还分神职与平民」——那是狼人杀的分法，不是所有社会推理游戏的分法。
//
// 现在它们只是玩家身上的两项状态（VarCamp / VarCategory），由角色的
// RoleSetup 在入座时发放，走的是与女巫的药完全相同的一条路。加一个
// 「第三阵营」的角色不需要改内核，登记它自己的 setup 就行。
//
// 这一层整体属于狼人杀规则包。

package werewolf

import "github.com/Zereker/hiddenrole"

// 狼人杀的两个阵营。
const (
	CampGood Camp = "GOOD" // 好人
	CampEvil Camp = "EVIL" // 狼人
)

// RoleCategory 角色在阵营之内的细分。
//
// 屠边判定需要区分「神职」与「平民」，而阵营只有好人/狼人两值，
// 表达不了这个维度，故单列一个类别。
//
// 它整个属于狼人杀：内核只认「这名玩家站哪一边」（hiddenrole.VarCamp），
// 不认得神职与平民。
type RoleCategory string

// String 实现 fmt.Stringer。
func (c RoleCategory) String() string {
	if c == RoleCategoryUnknown {
		return "UNKNOWN"
	}
	return string(c)
}

// VarCategory 角色类别在玩家 Vars 里的键名。规则包自己定的，内核不认。
const VarCategory = "category"

// 狼人杀的三种角色类别。
const (
	RoleCategoryUnknown  RoleCategory = ""         // 没有登记类别的角色
	RoleCategoryWolf     RoleCategory = "WOLF"     // 狼人阵营
	RoleCategoryGod      RoleCategory = "GOD"      // 神职：预言家、女巫、猎人、守卫
	RoleCategoryVillager RoleCategory = "VILLAGER" // 平民
)

// campOf 这名玩家属于哪一边，没有登记则为 CampUnspecified。
func campOf(p hiddenrole.PlayerInfo) Camp {
	return Camp(p.Vars[VarCamp])
}

// categoryOf 这名玩家是什么类别，没有登记则为 RoleCategoryUnknown。
func categoryOf(p hiddenrole.PlayerInfo) RoleCategory {
	return RoleCategory(p.Vars[VarCategory])
}

// campVars 把阵营与类别装成一份初始状态，供 RoleSetup 返回。
//
// 扩展角色照着写就行：
//
//	werewolf.WithRoleSetup(roleWolfKing, werewolf.RoleSetupFunc(
//		func(id string, role werewolf.RoleType) map[string]string {
//			return werewolf.CampVars(werewolf.CampEvil, werewolf.RoleCategoryWolf)
//		}))
func campVars(camp Camp, category RoleCategory) map[string]string {
	out := make(map[string]string, 2)
	if camp != hiddenrole.CampUnspecified {
		out[VarCamp] = string(camp)
	}
	if category != RoleCategoryUnknown {
		out[VarCategory] = string(category)
	}
	return out
}

// CampVars 同 campVars，导出供扩展角色的 RoleSetup 使用。
func CampVars(camp Camp, category RoleCategory) map[string]string {
	return campVars(camp, category)
}
