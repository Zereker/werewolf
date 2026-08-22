// victory.go 胜负判定：内置两种，也可以由第三方替换。
//
// 为什么要能替换：狼人杀的变体里，第三方阵营（丘比特的情侣、白狼王带走
// 全场之类）有自己的胜利条件，而内置的两种判定只认好人与狼人两边。
// 判定写死在引擎里的话，这类板子做不出来——不是「做起来麻烦」，是
// 根本没有地方表达。
//
// 与 Resolver 同构：拿只读的 GameView，返回结论，不碰状态。

package werewolf

// VictoryChecker 判定这一刻胜负是否已分。
//
// 返回 (false, CampUnspecified) 表示还没分出胜负。
// winner 可以是自定义阵营（Camp 从 1000 起），引擎只负责把它原样报出去。
//
// 与 Resolver 一样：只能读 GameView，在引擎持锁期间被调用，
// 实现中不要回调 Engine 的任何方法。
type VictoryChecker interface {
	CheckVictory(view GameView) (over bool, winner Camp)
}

// WithVictoryChecker 换掉内置的胜负判定。
//
// 换掉之后 GameConfig.VictoryMode 就不再起作用了——那个字段只喂给
// 内置判定。想在内置规则之上再加一条（比如「情侣双双存活即情侣胜」），
// 把 DefaultVictoryChecker 包起来，先问自己的条件再问它。
func WithVictoryChecker(checker VictoryChecker) EngineOption {
	return func(e *Engine) error {
		if checker == nil {
			return WrapError(CodeInvalidConfig, "victory checker must not be nil")
		}
		e.victory = checker
		return nil
	}
}

// DefaultVictoryChecker 内置判定，按 GameConfig.VictoryMode 分屠边与屠城。
//
// 导出它是为了让扩展能包装复用：第三方阵营的胜利条件通常是
// 「先看我这一条，不成立再走原来的」。
type DefaultVictoryChecker struct {
	Mode VictoryMode
}

// CheckVictory 实现 VictoryChecker。
//
// 好人阵营的胜利条件与判定方式无关：「將狼人淘汰以獲取勝利」。
// 狼人阵营的胜利条件取决于 Mode：
//
//	VictoryModeSideWipe（屠边）「需要淘汰所有平民或神職人員」
//	VictoryModeTownWipe（屠城）好人存活数 <= 狼人存活数
//
// 屠边判定只对开局就存在的类别生效：没有神职的板子不会因
// 「神职全灭」在开局瞬间判负，平民同理。
//
// 「平民」「神職人員」说的都是好人阵营的那一半。狼队也可以有自己的神
// （隐狼，经 AddCustomPlayer 标成 RoleCategoryGod），把它们一起计进总数
// 会让一名活着的隐狼把「好人的神已经死光」这个事实一直挡住。
func (c DefaultVictoryChecker) CheckVictory(view GameView) (bool, Camp) {
	n := tally(view)

	// 狼人全死，好人胜利（两种判定方式一致）
	if n.evilAlive == 0 {
		return true, CampGood
	}

	// 好人全灭，狼人胜利（兜底，避免无神职无平民的板子永不结束）
	if n.goodAlive == 0 {
		return true, CampEvil
	}

	switch c.Mode {
	case VictoryModeTownWipe:
		if n.goodAlive <= n.evilAlive {
			return true, CampEvil
		}

	default: // VictoryModeSideWipe
		// 屠神 / 屠民：开局有这个类别，且已经全部出局
		if n.wipedOut(RoleCategoryGod) || n.wipedOut(RoleCategoryVillager) {
			return true, CampEvil
		}
	}

	return false, CampUnspecified
}

// census 一次点名的结果：各阵营存活数，以及好人阵营各类别的总数与存活数。
type census struct {
	goodAlive int
	evilAlive int

	// total / alive 只统计好人阵营，理由见 DefaultVictoryChecker.CheckVictory
	total map[RoleCategory]int
	alive map[RoleCategory]int
}

// wipedOut 该类别开局就存在，且现在已经全部出局。
func (c census) wipedOut(cat RoleCategory) bool {
	return c.total[cat] > 0 && c.alive[cat] == 0
}

// tally 点一次名。
//
// 走的是 GameView 而不是内部状态——判定既然可以由第三方替换，
// 它能看到的东西就该和第三方一样多，不多也不少。
func tally(view GameView) census {
	c := census{
		total: make(map[RoleCategory]int, 2),
		alive: make(map[RoleCategory]int, 2),
	}

	for _, p := range view.AllPlayers() {
		camp, category := campOf(p), categoryOf(p)
		good := camp == CampGood
		if good {
			c.total[category]++
		}
		if !p.Alive {
			continue
		}
		if good {
			c.alive[category]++
			c.goodAlive++
		} else if camp == CampEvil {
			c.evilAlive++
		}
	}

	return c
}
