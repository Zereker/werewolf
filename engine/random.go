package engine

import (
	"hash/fnv"
	"math/rand"
)

// random.go 规则怎么摇随机数，而回放还能重现。

// Rand 的设计：**不存 PRNG 状态，从局面推导随机流**。
//
// 对照过的实现（boardgame.io）把 PRNG 的内部状态存进游戏状态，每次取数之后
// 写回去。那是被它的形态逼的：它的 move 是任意代码，一局之内可以在任何地方
// 取任意多次随机数，不记住进度就重现不了。
//
// 我们不需要那么做，因为这里的约束更强：**结算是局面的纯函数**。
// Resolver 拿到的是某一刻的只读局面，产出效果；同一个局面进去，必须同样的
// 效果出来（这条早就有测试守着，见规则包的 EffectOrderIsDeterminedByTheBoard）。
// 于是只要随机流本身由局面唯一决定，重现就是自然结果——回放走到同一个局面，
// 摇出来的就是同一串数。
//
// 流由三样东西决定：整局的种子、当前回合、当前阶段。种子进快照，
// 回合与阶段本来就在局面里。因此**不需要额外存任何 PRNG 进度**，
// 快照格式也不必为随机多一个可变字段。
//
// 代价说清楚：同一个回合的同一个阶段被结算两次，摇出来的是同一串数。
// 对这个引擎来说那正是要的——同一个局面必须得出同一个结果。

// randStream 由局面推导出一条随机流。
//
// 用 FNV 把 (种子, 回合, 阶段) 混成一个 int64 做 PRNG 的种子：阶段是字符串，
// 直接相加会让不同阶段撞在一起（"DAY"+1 与 "DA"+2 之类）。
func randStream(seed int64, round int, phase PhaseType) *rand.Rand {
	h := fnv.New64a()
	var buf [8]byte
	put := func(v uint64) {
		for i := 0; i < 8; i++ {
			buf[i] = byte(v >> (8 * i))
		}
		_, _ = h.Write(buf[:])
	}
	put(uint64(seed))
	put(uint64(round))
	_, _ = h.Write([]byte(phase))
	return rand.New(rand.NewSource(int64(h.Sum64()))) //nolint:gosec // 游戏用随机，不是密码学用途
}
