// nightstate.go 狼人杀的回合状态：刀口、被守、被救、被毒。
//
// 这四样此前是 RoundContext 上的字段与三张 map[string]bool，也就是说
// 「一局狼人杀的夜里会发生哪几种标记」被写进了内核。换一套规则——任务制那一套
// 没有夜里的刀口，血染钟楼的标记有十几种——它们一个都用不上，而新规则
// 要表达自己的状态又只能回头去改内核的结构体。
//
// 现在它们只是键名。存储用内核的四格作用域（见 engine.VarScope），读写走通用原语，
// 内核不知道「刀口」是什么意思。这一层整体属于狼人杀规则包。

package werewolf

import (
	"strconv"

	"github.com/Zereker/werewolf/engine"
)

// 狼人杀在回合级状态里用到的键名。
//
// 前缀按角色分，避免与扩展角色自己定的键撞名——这不是内核强制的，
// 只是这个规则包自己的约定，扩展没有义务照做。
const (
	// RoundVarKillTarget 今晚狼人选定的击杀目标。整局唯一，不属于任何玩家。
	RoundVarKillTarget = "wolf.kill_target"

	// PlayerRoundVarProtected 今晚被守卫守护。
	PlayerRoundVarProtected = "guard.protected"

	// PlayerRoundVarSaved 今晚被女巫的解药救下。
	PlayerRoundVarSaved = "witch.saved"

	// PlayerRoundVarPoisoned 今晚被女巫的毒药毒杀。
	PlayerRoundVarPoisoned = "witch.poisoned"
)

// 守卫的守护记录用到的键名。
//
// 记「哪一回合守了谁」而不是「最后一次成功守护的目标」：后者不会因为
// 守卫弃权而失效，一旦命中就把那个目标永久锁死。是否构成连守由
// lastProtected 按回合号判定。
const (
	PlayerVarLastProtectedTarget = "guard.last_target"
	PlayerVarLastProtectedRound  = "guard.last_round"
)

// nightKillTarget 今晚的刀口，没有则为空。
func nightKillTarget(view GameView) string {
	return view.Var(ScopeRound, RoundVarKillTarget)
}

// isProtected 该玩家今晚是否被守护。
func isProtected(view GameView, playerID string) bool {
	return view.Var(ScopeRound.Of(playerID), PlayerRoundVarProtected) != ""
}

// isSaved 该玩家今晚是否被解药救下。
func isSaved(view GameView, playerID string) bool {
	return view.Var(ScopeRound.Of(playerID), PlayerRoundVarSaved) != ""
}

// isPoisoned 该玩家今晚是否被毒。
func isPoisoned(view GameView, playerID string) bool {
	return view.Var(ScopeRound.Of(playerID), PlayerRoundVarPoisoned) != ""
}

// lastProtected 守卫上一回合守护的目标，无则为空。
//
// 「上一回合」是严格的：空守一晚、或那次守护被判无效，都返回空。
// 记的是回合号而不是「最后一次成功守护」，否则守卫一旦弃权，那个目标
// 就被永久锁死了。
//
// 这条判定此前在内核里（gameState.lastProtectedTarget + GameView 上一个
// 专门的方法），而「守卫不能连守」是狼人杀的规则，不是状态机的事。
func lastProtected(view GameView, guardID string) string {
	round, err := strconv.Atoi(view.Var(ScopeGame.Of(guardID), PlayerVarLastProtectedRound))
	if err != nil || round != view.Round()-1 {
		return ""
	}
	return view.Var(ScopeGame.Of(guardID), PlayerVarLastProtectedTarget)
}

// markProtected 记下「本回合守卫守了谁」，供下回合判断连守。
func markProtected(view GameView, guardID, targetID string) []*Effect {
	return []*Effect{
		engine.NewSetVarEffect(ScopeGame.Of(guardID), PlayerVarLastProtectedTarget, targetID),
		engine.NewSetVarEffect(ScopeGame.Of(guardID), PlayerVarLastProtectedRound, strconv.Itoa(view.Round())),
	}
}

// NightKillTarget 今晚被狼人选中的刀口，没有则为空。
//
// 拆包之后它是包级函数而不是 Engine 的方法：Engine 住在内核里，
// 而「刀口」是狼人杀的概念，本包没有办法给别人的类型加方法。
//
// 等价写法：e.Var(ScopeRound, werewolf.RoundVarKillTarget)。
func NightKillTarget(e *Engine) string {
	return e.Var(ScopeRound, RoundVarKillTarget)
}
