package werewolf

import (
	"github.com/Zereker/werewolf/engine"
	"testing"
)

// 本文件是给「按脚本推进一局游戏」的测试用的辅助函数。
//
// 这些包装存在的理由只有一个：**建局与推进出错时必须当场终止**。
// 早期测试大量写成 eng.Start() / eng.AddPlayer(...) 而不看返回值，
// 一旦这些调用开始返回错误（板子不合法、重复 ID、配置残缺），测试不会
// 在出错处停下，而是带着一个没开局的引擎继续往下跑——最后要么在某个
// 无关的断言上失败、要么直接在空切片上取下标 panic，两种都极难定位。
//
// 按脚本推进一整局的用例现在统一走 rules_test.go 的 newRuleGame / ruleGame，
// 那套辅助连阶段流转一起断言。这里只剩下不适合 newRuleGame 的场景仍在使用：
// 期待 Start()/AddPlayer() 报错的、断言开局前状态或内部字段的、以及并发用例。

// mustAdd 添加玩家，失败即终止
func mustAdd(t *testing.T, e *Engine, id string, role RoleType) {
	t.Helper()
	if err := e.AddPlayer(id, role); err != nil {
		t.Fatalf("AddPlayer(%q, %v): %v", id, role, err)
	}
}

// mustStart 开局，失败即终止
func mustStart(t *testing.T, e *Engine) {
	t.Helper()
	if err := e.Start(); err != nil {
		t.Fatalf("Start(): %v", err)
	}
}

// mustSubmit 提交技能，失败即终止
func mustSubmit(t *testing.T, e *Engine, use *SkillUse) {
	t.Helper()
	if err := e.SubmitSkillUse(use); err != nil {
		t.Fatalf("SubmitSkillUse(player=%s skill=%v target=%s) 于 %v: %v",
			use.PlayerID, use.Skill, use.TargetID, e.Phase(), err)
	}
}

// checkVictory 按默认规则算一次胜负。
//
// 拆包之后测试拿不到引擎内部装的那个判定器了，改成显式构造一个同样的——
// 这反而更实在：测的是「这个局面按屠边规则算下来是什么结果」，
// 而不是「引擎里那个对象说了什么」。
func checkVictory(e *Engine) (bool, Camp) {
	return checkVictoryWith(e, DefaultRules())
}

// checkVictoryWith 按给定规则算一次胜负。
func checkVictoryWith(e *Engine, rules Rules) (bool, Camp) {
	return DefaultVictoryChecker{Mode: rules.VictoryMode}.CheckVictory(e.View())
}

// witchKill 从 RolePhaseInfo 里取出女巫可见的刀口。
//
// 刀口从 RolePhaseInfo 的一等字段变成了角色自己填的 RoleInfo——
// 内置角色在信息这件事上不再比第三方角色多一等待遇。
func witchKill(ri *engine.RolePhaseInfo) string {
	for _, info := range ri.RoleInfo {
		if t := info[RoleInfoKillTarget]; t != "" {
			return t
		}
	}
	return ""
}

// ==================== 解析器的单元测试辅助 ====================
//
// 解析器收的是 GameView，而规则包在内核之外，拿不到内核的内部状态。
// engine.Board 是内核给规则包留的入口：手工摆一副局面，转成视图喂给
// 解析器，再把产出的效果折回去看局面变成了什么样——走的是与引擎完全
// 相同的那个写入点。

// board 一副手工摆出来的局面。
type board = engine.Board

// newBoard 摆一副局面。
func newBoard(seats ...engine.PlayerInfo) board {
	return board{Round: 1, Players: seats}
}

// seatOf 拼一名玩家。vars 是键值交替的可变参数。
func seatOf(id string, role RoleType, vars ...string) engine.PlayerInfo {
	out := engine.Seat(id, role, true, vars...)
	if setup, ok := builtinRoleSetup[role]; ok {
		// 内置角色的初始状态（阵营、类别、女巫的药）由 RoleSetup 发，
		// 直接摆局面时得自己补上，否则女巫手里没有药
		for k, v := range setup.Setup(id, role) {
			if out.Vars == nil {
				out.Vars = map[string]string{}
			}
			if _, exists := out.Vars[k]; !exists {
				out.Vars[k] = v
			}
		}
	}
	return out
}

// withKill 记下今晚的刀口。
func withKill(b board, target string) board {
	b.RoundVars = map[string]string{RoundVarKillTarget: target}
	return b
}

// markSeat 给某名玩家加上本回合的标记。
func markSeat(b board, id string, keys ...string) board {
	for i, p := range b.Players {
		if p.ID == id {
			b.Players[i] = engine.Mark(p, keys...)
		}
	}
	return b
}

// roundVarOfBoard 读某名玩家本回合的一项标记。
func roundVarOfBoard(b board, id, key string) string {
	if p, ok := b.Player(id); ok {
		return p.RoundVar(key)
	}
	return ""
}

// protectedInEngine 这名玩家今晚被守了吗（从引擎读）。
func protectedInEngine(e *Engine, id string) bool {
	p, ok := e.PlayerInfo(id)
	return ok && p.RoundVar(PlayerRoundVarProtected) != ""
}

// mustSeat 取出一名玩家，不存在即终止。
func mustSeat(t *testing.T, b board, id string) engine.PlayerInfo {
	t.Helper()
	p, ok := b.Player(id)
	if !ok {
		t.Fatalf("玩家不存在: %s", id)
	}
	return p
}
