// rolesetup.go 狼人杀六个角色的初始状态。
//
// 阵营、类别、女巫的两瓶药，全都是入座时发放的初始状态，走的是同一张表、
// 同一条写入路径。第三方角色经 WithRoleSetup 注册，与它们没有先后之分。

package werewolf

import "github.com/Zereker/hiddenrole"

// 女巫药剂在 Vars 里的键名。
//
// 值为 "1" 表示尚在手中，用掉即删除（空值在 applyEffect 里等同删除）。
// 它们此前是 PlayerState 上的两个 bool 字段——那意味着引擎认得「女巫」
// 这个概念，也意味着第三方的「女巫类」角色改不动自己的药。
const (
	VarWitchAntidote = "witch.antidote"
	VarWitchPoison   = "witch.poison"
)

// builtinRoleSetup 内置六个角色的初始状态。
//
// 做成表而不是 if：加内置角色时只需在这里加一行，第三方经
// WithRoleSetup 注册的走同一张表、同一条写入路径，没有先后之分。
//
// 表里装的是阵营与类别——它们此前是内核状态的一部分，也是
// AddCustomPlayer 上的两个参数，于是「这个角色属于哪一边」的答案取决于
// 调用方每一处入座时记得填对。现在它写在角色自己身上。
//
// 没有登记的角色（含扩展角色）不属于任何阵营，也就不参与胜负计数。
// 这是刻意的：内核没有默认阵营可给，而「悄悄算作好人」比「不算」更难查。
var builtinRoleSetup = map[RoleType]hiddenrole.RoleSetup{
	RoleWerewolf: sideSetup(CampEvil, RoleCategoryWolf),
	RoleSeer:     sideSetup(CampGood, RoleCategoryGod),
	RoleHunter:   sideSetup(CampGood, RoleCategoryGod),
	RoleGuard:    sideSetup(CampGood, RoleCategoryGod),
	RoleVillager: sideSetup(CampGood, RoleCategoryVillager),
	RoleWitch:    hiddenrole.RoleSetupFunc(builtinWitchSetup),
}

// sideSetup 只发阵营与类别的初始状态。
func sideSetup(camp Camp, category RoleCategory) hiddenrole.RoleSetup {
	return hiddenrole.RoleSetupFunc(func(string, RoleType) map[string]string {
		return campVars(camp, category)
	})
}

// builtinWitchSetup 女巫是好人阵营的神职，开局有解药和毒药各一瓶。
func builtinWitchSetup(playerID string, role RoleType) map[string]string {
	out := campVars(CampGood, RoleCategoryGod)
	out[VarWitchAntidote] = VarPresent
	out[VarWitchPoison] = VarPresent
	return out
}
