// testview.go 手工构造一份 GameView。
//
// 规则包要单元测试自己的解析器时需要它：`Resolver.Resolve(uses, view)`
// 收的是一个 GameView，而规则包在内核之外，拿不到内核的内部状态。
// 没有这个入口，规则的解析器就只能整局跑起来才测得动——那测的是集成，
// 不是这个解析器本身。
//
// 名字不以 Test 开头，因为它是正经的公开 API，不是测试文件里的辅助。

package engine

// Board 一份手工摆出来的局面，用于构造 GameView。
type Board struct {
	// Players 场上的玩家。顺序无关，视图会按 ID 排序。
	Players []PlayerInfo

	// Round 当前回合数，从 1 起。为 0 时按 1 处理。
	Round int

	// Phase 当前阶段。
	Phase PhaseType

	// Vars 整局有效、不属于任何玩家的状态（ScopeGame）。
	Vars map[string]string

	// RoundVars 本回合有效、不属于任何玩家的状态（ScopeRound）。
	//
	// 有主的两格在 PlayerInfo 上（Vars / RoundVars），四格凑齐才摆得出
	// 任意一个局面——这里此前少了上面那格，理由与内核少那一格一样：
	// 狼人杀用不到，所以没人发现。
	RoundVars map[string]string
}

// Apply 把一批效果折进局面，返回改过的副本。
//
// 规则测试用它接住解析器的产出：`b = b.Apply(r.Resolve(uses, b.View()))`，
// 然后断言局面变成了什么样。走的是与引擎完全相同的那个写入点，因此
// 「效果没生效」这类问题在单元测试里就会暴露，不必整局跑起来。
//
// 被否决的效果与内核不认得的类型都不会改变任何东西——这正是它要验的。
func (b Board) Apply(effects []*Effect) Board {
	s := b.state()
	for _, ef := range effects {
		s.applyEffect(ef)
	}
	return boardOf(s)
}

// View 构造这份局面的只读视图。
//
// 返回的视图是快照式的：之后改动 Board 不会影响它。
func (b Board) View() GameView { return newStateView(b.state()) }

// Player 取出一名玩家，不存在时第二个返回值为 false。
func (b Board) Player(id string) (PlayerInfo, bool) {
	for _, p := range b.Players {
		if p.ID == id {
			return p, true
		}
	}
	return PlayerInfo{}, false
}

// Var 读某个作用域下的一项状态，四格都能读（见 VarScope）。
func (b Board) Var(scope VarScope, key string) string { return b.state().varOf(scope, key) }

// state 把局面还原成内部状态。
func (b Board) state() *gameState {
	round := b.Round
	if round < 1 {
		round = 1
	}
	s := newState()
	s.Round = round
	s.Phase = b.Phase
	s.Vars = copyVars(b.Vars)
	s.RoundCtx = newRoundContext()
	s.RoundCtx.Vars = copyVars(b.RoundVars)
	for _, p := range b.Players {
		s.players[p.ID] = &playerState{
			ID: p.ID, Role: p.Role, Alive: p.Alive,
			Vars: copyVars(p.Vars), RoundVars: copyVars(p.RoundVars),
		}
	}
	return s
}

// boardOf 把内部状态导回局面。
func boardOf(s *gameState) Board {
	b := Board{
		Round: s.Round, Phase: s.Phase,
		Vars: copyVars(s.Vars), RoundVars: copyVars(s.RoundCtx.Vars),
	}
	for _, id := range s.allPlayerIDs() {
		if info, ok := s.PlayerInfo(id); ok {
			b.Players = append(b.Players, info)
		}
	}
	return b
}

// Seat 拼一名玩家，供 Board 使用。vars 是键值交替的可变参数。
//
//	engine.Seat("wi", "WITCH", true, engine.VarCamp, "GOOD", "witch.antidote", "1")
//
// 键值个数不成对时，最后一个孤零零的键会被忽略——这是测试辅助，
// 不值得为一个写错的调用返回 error。
func Seat(id string, role RoleType, alive bool, vars ...string) PlayerInfo {
	p := PlayerInfo{ID: id, Role: role, Alive: alive}
	for i := 0; i+1 < len(vars); i += 2 {
		if p.Vars == nil {
			p.Vars = make(map[string]string, len(vars)/2)
		}
		p.Vars[vars[i]] = vars[i+1]
	}
	return p
}

// Mark 给一名玩家加上本回合的标记，返回改过的副本。
func Mark(p PlayerInfo, keys ...string) PlayerInfo {
	if len(keys) == 0 {
		return p
	}
	p.RoundVars = copyVars(p.RoundVars)
	if p.RoundVars == nil {
		p.RoundVars = make(map[string]string, len(keys))
	}
	for _, k := range keys {
		p.RoundVars[k] = VarPresent
	}
	return p
}
