// rolesetup.go 角色的初始状态：这个角色入座时带着什么。
//
// 引擎此前在 addCustomPlayer 里写着：
//
//	if role == RoleWitch {
//	    player.HasAntidote = true
//	    player.HasPoison = true
//	}
//
// 这是引擎里最后一处因具体角色而分叉的逻辑。它的代价不是这三行本身，
// 而是第三方角色**没有任何办法**给自己发初始状态——骑士开局带一次决斗、
// 摄梦人开局带两条命，都得改引擎才能表达，而加一个角色不该要求改引擎。
//
// 现在初始状态与角色行为、胜利条件、专属信息一样，由角色自己回答。
// 内置女巫走的是同一张表（builtinRoleSetup），它没有特权。

package werewolf

// RoleSetup 回答「这个角色入座时带着什么状态」。
//
// 与 Resolver、VictoryChecker、RoleInfoProvider 同构：不碰状态，
// 只返回结论。返回的键值会原样写进该玩家的 Vars，之后经
// GameView.PlayerVar 读、NewSetPlayerVarEffect 改。
//
// 入座发生在开局之前，此时还没有局面可看，因此签名里没有 GameView：
// 初始状态只能由角色本身决定，不能取决于谁先入座、场上还有谁。
// 需要看局面的初始化（丘比特连情侣、盗贼选底牌）是一个阶段，
// 用 Resolver 做。
//
// 返回 nil 或空表示这个角色不带任何初始状态。
type RoleSetup interface {
	Setup(playerID string, role RoleType) map[string]string
}

// RoleSetupFunc 让普通函数满足 RoleSetup。
type RoleSetupFunc func(playerID string, role RoleType) map[string]string

// Setup 实现 RoleSetup。
func (f RoleSetupFunc) Setup(playerID string, role RoleType) map[string]string {
	return f(playerID, role)
}

// WithRoleSetup 给某个角色注册初始状态。
//
//	const roleKnight werewolf.RoleType = 1001
//
//	engine, _ := werewolf.NewEngine(cfg,
//		werewolf.WithResolver(phaseKnight, knightResolver{}),
//		werewolf.WithRoleSetup(roleKnight, werewolf.RoleSetupFunc(
//			func(id string, role werewolf.RoleType) map[string]string {
//				return map[string]string{"knight.duel": "1"}
//			})))
//
// 同一个角色重复注册以最后一次为准，因此也可以用来换掉内置的那些
// （比如发一个开局就用掉解药的女巫）。
//
// 回放与恢复都不需要再传一遍：初始状态落在效果流的入座那一条上
// （ReplayEngine）与快照的 Vars 里（RestoreEngine）。这与解析器不同，
// 解析器是规则、必须由调用方带着；初始状态是事实，记下来就够了。
func WithRoleSetup(role RoleType, setup RoleSetup) EngineOption {
	return func(e *Engine) error {
		if setup == nil {
			return WrapError(CodeInvalidConfig,
				"role setup for %v must not be nil", role)
		}
		e.roleSetup[role] = setup
		return nil
	}
}

// 女巫药剂在 Vars 里的键名。
//
// 值为 "1" 表示尚在手中，用掉即删除（空值在 applyEffect 里等同删除）。
// 它们此前是 PlayerState 上的两个 bool 字段——那意味着引擎认得「女巫」
// 这个概念，也意味着第三方的「女巫类」角色改不动自己的药。
const (
	VarWitchAntidote = "witch.antidote"
	VarWitchPoison   = "witch.poison"
)

// VarPresent 是布尔型 Vars 约定的「有」值。
//
// Vars 的值是字符串，空串在写入点等同删除，因此「有/没有」这类状态
// 只需要一个非空值。内置角色统一用它，扩展没有义务照做。
const VarPresent = "1"

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
var builtinRoleSetup = map[RoleType]RoleSetup{
	RoleWerewolf: sideSetup(CampEvil, RoleCategoryWolf),
	RoleSeer:     sideSetup(CampGood, RoleCategoryGod),
	RoleHunter:   sideSetup(CampGood, RoleCategoryGod),
	RoleGuard:    sideSetup(CampGood, RoleCategoryGod),
	RoleVillager: sideSetup(CampGood, RoleCategoryVillager),
	RoleWitch:    RoleSetupFunc(builtinWitchSetup),
}

// sideSetup 只发阵营与类别的初始状态。
func sideSetup(camp Camp, category RoleCategory) RoleSetup {
	return RoleSetupFunc(func(string, RoleType) map[string]string {
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

// setupFor 算出某个玩家的初始状态。调用前需持有 e.mu。
func (e *Engine) setupFor(playerID string, role RoleType) map[string]string {
	setup, ok := e.roleSetup[role]
	if !ok {
		return nil
	}
	return setup.Setup(playerID, role)
}
