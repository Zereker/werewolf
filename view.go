package werewolf

import ()

// GameView 只读的游戏视图。
//
// Resolver 拿到的是它而不是 *gameState：架构上「状态变更一律经由 Effect」
// 是这个引擎最重要的不变量，此前它只写在文档里，类型系统不设防——
// 任何 Resolver（包括第三方注册的）都能直接改状态、绕开整条 Effect 管线，
// 让可回放、可审计这些收益全部落空。现在这条约束写进了签名。
//
// 视图只提供事实，不提供判断：规则的判定属于 Resolver，
// 因此这里给的是「上一回合守了谁」而非「现在能不能守」。
type GameView interface {
	// Player 返回玩家信息的只读副本
	Player(id string) (PlayerInfo, bool)

	// AlivePlayers 返回所有存活玩家
	AlivePlayers() []PlayerInfo

	// AllPlayers 返回全部玩家（含已出局的），按 ID 排序。
	//
	// 胜负判定需要它：「开局有几个神职」得数上已经死掉的那些，
	// 只看存活的算不出「屠神」。
	AllPlayers() []PlayerInfo

	// AlivePlayerIDsByRole 返回指定角色的存活玩家 ID
	AlivePlayerIDsByRole(role RoleType) []string

	// RoundContext 返回本回合上下文的只读副本
	RoundContext() RoundContext

	// LastProtectedTarget 返回守卫在上一回合守护的目标，无则为空。
	//
	// 「上一回合」是严格的：空守一晚、或那次守护被判无效，都返回空。
	// 视图只给事实，是否允许再守由 Resolver 依配置判定。
	LastProtectedTarget(guardID string) string

	// PlayerVar 返回某个玩家的一项自定义状态，没有则为空串。
	//
	// 第三方角色用它存放自身状态（白痴翻没翻牌、骑士的决斗用没用掉），
	// 写入走 NewSetPlayerVarEffect。内置角色的药剂与守护记录是同一件事，
	// 只是它们在 PlayerInfo 上有专门的字段。
	PlayerVar(playerID, key string) string

	// RoundVar 返回本回合的一项自定义状态，没有则为空串。
	//
	// 与 PlayerVar 的分工：那个跟着玩家走一整局，这个每回合自动清零。
	// 内置的刀口、被守、被救、被毒都属于后者，只是它们在
	// RoundContext 上有专门的字段。
	RoundVar(key string) string

	// Round 返回当前回合数
	Round() int

	// Phase 返回当前阶段
	Phase() PhaseType
}

// stateView 是 GameView 的实现。
//
// 刻意做成不导出的包装类型而非直接让 *gameState 实现接口：
// 后者可以被类型断言还原成可变的状态对象，等于没有约束。
type stateView struct {
	s *gameState
}

func newStateView(s *gameState) GameView { return stateView{s: s} }

func (v stateView) Player(id string) (PlayerInfo, bool) {
	return v.s.PlayerInfo(id)
}

func (v stateView) AlivePlayers() []PlayerInfo {
	ids := v.s.getAlivePlayerIDs()
	out := make([]PlayerInfo, 0, len(ids))
	for _, id := range ids {
		if info, ok := v.s.PlayerInfo(id); ok {
			out = append(out, info)
		}
	}
	return out
}

func (v stateView) AllPlayers() []PlayerInfo {
	ids := v.s.allPlayerIDs()
	out := make([]PlayerInfo, 0, len(ids))
	for _, id := range ids {
		if info, ok := v.s.PlayerInfo(id); ok {
			out = append(out, info)
		}
	}
	return out
}

func (v stateView) AlivePlayerIDsByRole(role RoleType) []string {
	return v.s.getAlivePlayerIDsByRole(role)
}

func (v stateView) RoundContext() RoundContext {
	rc := v.s.RoundContext()
	if rc == nil {
		return RoundContext{}
	}
	return *rc
}

func (v stateView) LastProtectedTarget(guardID string) string {
	return v.s.lastProtectedTarget(guardID)
}

func (v stateView) PlayerVar(playerID, key string) string {
	return v.s.playerVar(playerID, key)
}

func (v stateView) RoundVar(key string) string { return v.s.roundVar(key) }

func (v stateView) Round() int { return v.s.currentRound() }

func (v stateView) Phase() PhaseType { return v.s.currentPhase() }
