// victory.go 胜负判定的接口。
//
// 内核不知道什么叫「赢」——它只知道有人可能会赢，以及那个结论长什么样
// （一个 Camp 标签）。具体条件由规则给，狼人杀的那一套见根包 victory.go。

package engine

// VictoryChecker 判定这一刻胜负是否已分。
//
// 返回 (false, CampUnspecified) 表示还没分出胜负。
// winner 可以是任何自定义阵营——Camp 的底层是字符串，内核不预设取值，
// 只负责把结论原样报出去。
//
// 与 Resolver 一样：只能读 GameView，在引擎持锁期间被调用，实现中不要
// 回调 Engine 的任何方法——后果是挂住，不是报错。
// 详见 doc.go「扩展点不能回头找引擎」。
type VictoryChecker interface {
	CheckVictory(view GameView) (over bool, winner Camp)
}

// WithVictoryChecker 换掉内置的胜负判定。
//
// 换掉之后 Config.VictoryMode 就不再起作用了——那个字段只喂给
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

// neverEnds 内核的缺省判定：永远不结束。
//
// 内核不知道什么叫「赢」，所以缺省只能是「不知道」。做成一个不结束的
// 判定而不是留 nil：一台只装了内核的引擎应该能推进阶段、只是永不分出胜负，
// 而不是在第一次 Start 就空指针崩掉。规则包一定会用 WithVictoryChecker
// 换掉它（见 werewolf.Options）。
type neverEnds struct{}

func (neverEnds) CheckVictory(GameView) (bool, Camp) { return false, CampUnspecified }
