package engine

import (
	"math/rand"
)

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

	// PlayerVar 返回某个玩家的一项自定义状态，没有则为空串。
	//
	// 第三方角色用它存放自身状态（白痴翻没翻牌、骑士的决斗用没用掉），
	// 写入走 NewSetPlayerVarEffect，初始值由 RoleSetup 发放。
	// 规则把角色私有的状态放在这里，内置角色与第三方角色同一条路。
	PlayerVar(playerID, key string) string

	// PlayerRoundVar 返回某个玩家在本回合的一项标记，没有则为空串。
	//
	// 三种作用域里的第三种：跟着玩家走一整局的用 PlayerVar，本回合有效
	// 且不属于任何人的用 RoundVar，「本回合标记了某人」用这个。
	// 今晚谁被守了、谁被救了、谁被毒了都是这一类。
	// 写入走 NewSetPlayerRoundVarEffect。
	PlayerRoundVar(playerID, key string) string

	// Rand 这一刻的随机流。
	//
	// 由 (Config.Seed, 当前回合, 当前阶段) 唯一决定：同一个局面拿到的永远是
	// 同一条流，因此回放能重现完全相同的结果，而 Resolver 仍然是局面的纯函数。
	//
	// 每次调用返回一条**新的、从头开始**的流——不要把它存起来跨阶段用，
	// 那会让结果依赖调用次序而不是局面。
	Rand() *rand.Rand

	// GameVar 返回整局的一项自定义状态，没有则为空串。
	//
	// 四种作用域里的第四种：**整局有效、不属于任何玩家**。比分、计数器、
	// 轮到谁这类「全局事实」属于这里。跟着某个玩家走一整局的用 PlayerVar，
	// 本回合有效且无主的用 RoundVar，「本回合标记了某人」用 PlayerRoundVar。
	//
	// 写入走 NewSetGameVarEffect。
	GameVar(key string) string

	// RoundVar 返回本回合的一项自定义状态，没有则为空串。
	//
	// 与 PlayerVar 的分工：那个跟着玩家走一整局，这个每回合自动清零，
	// 且不属于任何玩家——狼人杀的「今晚刀口」就是这一类。
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

func (v stateView) PlayerVar(playerID, key string) string {
	return v.s.playerVar(playerID, key)
}

func (v stateView) PlayerRoundVar(playerID, key string) string {
	return v.s.playerRoundVar(playerID, key)
}

func (v stateView) Rand() *rand.Rand {
	return randStream(v.s.Seed, v.s.currentRound(), v.s.currentPhase())
}

func (v stateView) GameVar(key string) string { return v.s.gameVar(key) }

func (v stateView) RoundVar(key string) string { return v.s.roundVar(key) }

func (v stateView) Round() int { return v.s.currentRound() }

func (v stateView) Phase() PhaseType { return v.s.currentPhase() }
