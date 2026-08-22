// roleinfo.go 角色专属信息：某个角色的玩家额外看得到什么。
//
// 内核只知道通用信息——谁活着、轮到谁行动。「女巫看得到今晚的刀口」
// 「盗贼看得到两张底牌」这类是角色自己的规则，由规则回答。

package engine

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
