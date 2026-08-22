// roleinfo.go 角色专属信息：某个角色的玩家额外看得到什么。
//
// 引擎只知道通用信息——谁活着、轮到谁行动、同阵营的是哪些人。
// 「女巫看得到今晚的刀口」「盗贼看得到两张底牌」「丘比特看得到自己连的
// 那一对」这类是角色自己的规则，本来就该由角色自己回答。
//
// 此前它们是引擎里一个认得所有内置角色的 switch：
//
//	switch role {
//	case RoleWerewolf: ri.Teammates = ...
//	case RoleWitch:    ri.KillTarget = ...
//	}
//
// 第三方角色因此发不出任何专属信息——加一个盗贼就得改引擎，这不合理。
// 内置女巫现在走的也是这条路（builtinWitchInfo），它没有特权。

package werewolf

// RoleInfoProvider 回答「这个玩家额外该知道什么」。
//
// 与 Resolver、VictoryChecker 同构：拿只读的 GameView，返回结论，不碰状态。
// 在引擎持锁期间被调用，实现中不要回调 Engine 的任何方法。
//
// 返回 nil 或空表示没有额外信息。键名由角色自己定，会原样出现在
// PlayerView.RoleInfo 与 RolePhaseInfo.RoleInfo 里。
type RoleInfoProvider interface {
	RoleInfo(playerID string, view GameView) map[string]string
}

// RoleInfoFunc 让普通函数满足 RoleInfoProvider。
type RoleInfoFunc func(playerID string, view GameView) map[string]string

// RoleInfo 实现 RoleInfoProvider。
func (f RoleInfoFunc) RoleInfo(playerID string, view GameView) map[string]string {
	return f(playerID, view)
}

// WithRoleInfo 给某个角色注册专属信息的提供者。
//
//	engine, _ := werewolf.NewEngine(cfg,
//		werewolf.WithResolver(phaseThief, thiefResolver{}),
//		werewolf.WithRoleInfo(roleThief, werewolf.RoleInfoFunc(
//			func(id string, view werewolf.GameView) map[string]string {
//				return map[string]string{"spare_cards": view.RoundVar("thief.spares")}
//			})))
//
// 同一个角色重复注册以最后一次为准，因此也可以用来换掉内置的那些。
func WithRoleInfo(role RoleType, provider RoleInfoProvider) EngineOption {
	return func(e *Engine) error {
		if provider == nil {
			return WrapError(CodeInvalidConfig,
				"role info provider for %v must not be nil", role)
		}
		e.roleInfo[role] = provider
		return nil
	}
}

// 女巫专属信息在 RoleInfo 里的键名。
//
// RoleInfoAntidote / RoleInfoPoison 是药剂存量的**投射**，存储在
// Vars 里（VarWitchAntidote / VarWitchPoison）。存储与投射分开是刻意的：
// 存储只有 Vars 一种，谁都能写；要给玩家看成什么样由角色自己决定。
// 它们此前是 SelfInfo 上两个具名 bool 字段，等于内置女巫在面向玩家的
// 视图上比第三方角色多一等公民的待遇。
const (
	RoleInfoKillTarget = "kill_target"
	RoleInfoAntidote   = "antidote"
	RoleInfoPoison     = "poison"
)

// builtinRoleInfo 内置角色的专属信息。
//
// 做成表而不是 switch：加内置角色时只需在这里加一行，第三方经
// WithRoleInfo 注册的走的是同一张表、同一条读取路径，没有先后之分。
var builtinRoleInfo = map[RoleType]RoleInfoProvider{
	RoleWitch: RoleInfoFunc(builtinWitchInfo),
}

// builtinWitchInfo 女巫看得到自己的药剂存量，以及解药尚在手时的刀口。
//
// 刀口按规则「解藥未使用時可以得知狼人的殺害對象」给。已出局的女巫
// 不再是行动者，天亮公布之前不该拿到今晚的刀口——但药剂存量照给：
// 那是她自己的东西，死了也不该凭空看不见。
func builtinWitchInfo(playerID string, view GameView) map[string]string {
	self, ok := view.Player(playerID)
	if !ok {
		return nil
	}

	info := make(map[string]string, 3)
	if v := view.PlayerVar(playerID, VarWitchAntidote); v != "" {
		info[RoleInfoAntidote] = v
	}
	if v := view.PlayerVar(playerID, VarWitchPoison); v != "" {
		info[RoleInfoPoison] = v
	}

	if self.Alive && info[RoleInfoAntidote] != "" {
		if target := nightKillTarget(view); target != "" {
			info[RoleInfoKillTarget] = target
		}
	}

	if len(info) == 0 {
		return nil
	}
	return info
}

// roleInfoFor 算出某个玩家的角色专属信息。调用前需持有 e.mu。
func (e *Engine) roleInfoFor(playerID string, role RoleType) map[string]string {
	provider, ok := e.roleInfo[role]
	if !ok {
		return nil
	}
	info := provider.RoleInfo(playerID, newStateView(e.state))
	if len(info) == 0 {
		return nil
	}
	return info
}
