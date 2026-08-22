package engine

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

	// AlivePlayers 返回所有存活玩家，按 ID 排序。
	//
	// 有序是规则可以依赖的：规则产出的效果顺序必须由局面唯一决定，
	// 否则回放与快照比对失去确定性。
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

	// Var 返回某个作用域下的一项自定义状态，没有则为空串。
	//
	// 作用域是一张 2×2 的表（见 VarScope）：
	//
	//	Var(ScopeGame, "score")            整局·无主
	//	Var(ScopeGame.Of(id), "antidote")  整局·某人
	//	Var(ScopeRound, "kill")            本回合·无主
	//	Var(ScopeRound.Of(id), "guarded")  本回合·某人
	//
	// 规则把自己的状态全放在这里，内置角色与第三方角色同一条路。
	// 写入走 NewSetVarEffect，玩家的初始状态由 RoleSetup 发放。
	Var(scope VarScope, key string) string

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

func (v stateView) Var(scope VarScope, key string) string {
	return v.s.varOf(scope, key)
}

func (v stateView) Round() int { return v.s.currentRound() }

func (v stateView) Phase() PhaseType { return v.s.currentPhase() }
