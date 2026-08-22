// resolver.go 阶段结算的接口。
//
// 内核不知道任何阶段该怎么结算——它只知道「到点了，问一下这个阶段的
// 解析器发生了什么」。狼人杀那七个解析器见根包 resolver.go。

package engine

// Resolver 冲突解析器接口。
//
// 实现者只能读 GameView、只能通过返回 Effect 表达状态变更——
// 这是引擎最重要的不变量，由签名保证而非靠约定。
//
// 注意：Resolve 在引擎持锁期间被调用，实现中不要回调 Engine 的任何方法
// ——后果是挂住，不是报错。详见 doc.go「扩展点不能回头找引擎」。
type Resolver interface {
	Resolve(uses []*SkillUse, view GameView) []*Effect
}
