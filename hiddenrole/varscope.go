package hiddenrole

// VarScope 变量作用域：一项自定义状态活多久、属于谁。
//
// 作用域是一张 2×2 的表——时间尺度（整局 / 本回合）乘以有没有主人
// （无主 / 属于某个玩家）：
//
//	            无主          属于某个玩家
//	整局有效     ScopeGame     ScopeGame.Of(id)
//	本回合有效   ScopeRound    ScopeRound.Of(id)
//
// 这张表此前只存在于注释里，代码里是八个互不相干的名字（四个构造器
// 加四个读法）。于是没有任何东西强制它完整：少写一格谁也不会发现，
// 事实上就是少了「整局·无主」那一格，直到任务制那一套撞上——比分、连续否决
// 数、队长轮到谁全是整局有效且不属于任何人，只能挂到某个玩家名下当账本。
//
// 现在四格由两个值叉乘一个方法得出，缺一格根本写不出来。
type VarScope struct {
	// perRound 为真表示本回合有效，回合边界处自动清空。
	perRound bool
	// owner 为空表示这项状态不属于任何玩家。
	owner string
}

// ScopeGame 整局有效、不属于任何玩家。比分、计数器、轮到谁属于这一格。
//
// 加 .Of(playerID) 变成「跟着某个玩家走一整局」：女巫的两瓶药、
// 骑士用掉的那次决斗、白痴翻过的牌。
var ScopeGame = VarScope{}

// ScopeRound 本回合有效、不属于任何玩家。今晚的刀口属于这一格。
//
// 加 .Of(playerID) 变成「本回合标记了某人」：今晚谁被守了、谁被救了、
// 谁被毒了。回合级的两格都在进入下一回合（或配置了 ClearsRoundVars
// 的阶段）时一起清空。
var ScopeRound = VarScope{perRound: true}

// Of 把作用域绑到某个玩家身上，时间尺度不变。
//
// 返回的是副本，ScopeGame 与 ScopeRound 这两个值本身不会被改动。
func (s VarScope) Of(playerID string) VarScope {
	s.owner = playerID
	return s
}

// String 供日志与调试打印，形如 game、round、game:p1、round:p1。
func (s VarScope) String() string {
	name := "game"
	if s.perRound {
		name = "round"
	}
	if s.owner == "" {
		return name
	}
	return name + ":" + s.owner
}
