// rolesetup.go 角色的初始状态：这个角色入座时带着什么。
//
// 引擎此前在入座时写着 `if role == 某个具体角色 { 发点东西 }`——那是内核里
// 最后一处因具体角色而分叉的逻辑。它的代价不是那三行，而是第三方角色
// **没有任何办法**给自己发初始状态：骑士开局带一次决斗、摄梦人开局带两条命，
// 都得改引擎才能表达，而加一个角色不该要求改引擎。

package hiddenrole

// RoleSetup 回答「这个角色入座时带着什么状态」。
//
// 与 Resolver、VictoryChecker、RoleInfoProvider 同构：不碰状态，
// 只返回结论。返回的键值会原样写进该玩家的 Vars，之后经
// GameView.Var(ScopeGame.Of(id), key) 读、NewSetVarEffect 改。
//
// 入座发生在开局之前，此时还没有局面可看，因此签名里没有 GameView：
// 初始状态只能由角色本身决定，不能取决于谁先入座、场上还有谁。
// 需要看局面的初始化（丘比特连情侣、盗贼选底牌）是一个阶段，
// 用 Resolver 做。
//
// 返回 nil 或空表示这个角色不带任何初始状态。
//
// 它在引擎持锁期间被调用，实现中不要回调 Engine 的任何方法——后果是
// 挂住，不是报错。详见 doc.go「扩展点不能回头找引擎」。
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
//	const roleKnight = engine.RoleType("KNIGHT")
//
//	e, _ := engine.NewEngine(cfg,
//		engine.WithResolver(phaseKnight, knightResolver{}),
//		engine.WithRoleSetup(roleKnight, engine.RoleSetupFunc(
//			func(id string, role engine.RoleType) map[string]string {
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

// VarPresent 是布尔型 Vars 约定的「有」值。
//
// Vars 的值是字符串，空串在写入点等同删除，因此「有/没有」这类状态
// 只需要一个非空值。内置角色统一用它，扩展没有义务照做。
const VarPresent = "1"

// setupFor 算出某个玩家的初始状态。调用前需持有 e.mu。
func (e *Engine) setupFor(playerID string, role RoleType) map[string]string {
	setup, ok := e.roleSetup[role]
	if !ok {
		return nil
	}
	return setup.Setup(playerID, role)
}

// GameSetup 开局那一刻，规则要先把局面铺成什么样。
//
// 与 RoleSetup 是一对：那个管**一名玩家**入座时带着什么，这个管**整局**
// 开始时的初始状态。它拿到的是全员就座之后的局面，因此能做到 RoleSetup
// 做不到的事——比如「第一个队长是几号座位」，那取决于场上有谁。
//
// 典型用途：初始化整局计数器（GameVar），以及**指定第一个阶段的行动者**
// （SetActors）。后者是这个扩展点存在的直接原因：行动者集合通常由上一个
// 阶段的解析器算出来，而第一个阶段前面没有阶段。
//
// 它在 Start() 里被调用一次，产出的效果走与其余效果完全相同的写入点，
// 因此进效果流、能回放、进快照。
//
// 与其余扩展点一样：只能读 GameView，在引擎持锁期间被调用，实现中不要
// 回调 Engine 的任何方法——后果是挂住，不是报错。
// 详见 doc.go「扩展点不能回头找引擎」。
type GameSetup interface {
	Setup(view GameView) []*Effect
}

// GameSetupFunc 让普通函数满足 GameSetup。
type GameSetupFunc func(view GameView) []*Effect

// Setup 实现 GameSetup。
func (f GameSetupFunc) Setup(view GameView) []*Effect { return f(view) }
