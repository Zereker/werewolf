package werewolf

import (
	"testing"
)

// 本文件是给「按脚本推进一局游戏」的测试用的辅助函数。
//
// 这些包装存在的理由只有一个：**建局与推进出错时必须当场终止**。
// 早期测试大量写成 engine.Start() / engine.AddPlayer(...) 而不看返回值，
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

// mustAddTo 直接向状态添加玩家，失败即终止。
// 供不经 Engine、直接测试 Resolver 与状态的单元测试使用。
func mustAddTo(t *testing.T, s *gameState, id string, role RoleType) {
	t.Helper()
	if err := s.addPlayer(id, role); err != nil {
		t.Fatalf("addPlayer(%q, %v): %v", id, role, err)
	}
}

// checkVictory 按引擎当前的判定器算一次胜负。
//
// 胜负判定从 gameState 上的方法改成了可替换的 VictoryChecker，
// 测试里问「现在分出胜负了吗」得走同一条路——否则测的就不是引擎在用的
// 那一套了。
func checkVictory(e *Engine) (bool, Camp) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.victory.CheckVictory(newStateView(e.state))
}
