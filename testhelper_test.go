package werewolf

import (
	"testing"

	pb "github.com/Zereker/werewolf/proto"
)

// 本文件是给「按脚本推进一局游戏」的测试用的辅助函数。
//
// 这些包装存在的理由只有一个：**建局与推进出错时必须当场终止**。
// 早期测试大量写成 engine.Start() / engine.AddPlayer(...) 而不看返回值，
// 一旦这些调用开始返回错误（板子不合法、重复 ID、配置残缺），测试不会
// 在出错处停下，而是带着一个没开局的引擎继续往下跑——最后要么在某个
// 无关的断言上失败、要么直接在空切片上取下标 panic，两种都极难定位。

// mustAdd 添加玩家，失败即终止
func mustAdd(t *testing.T, e *Engine, id string, role pb.RoleType) {
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

// mustEnd 结束当前阶段，失败即终止，返回本阶段产生的效果
func mustEnd(t *testing.T, e *Engine) []*Effect {
	t.Helper()
	effects, err := e.EndPhase()
	if err != nil {
		t.Fatalf("EndPhase() 于 %v: %v", e.GetCurrentPhase(), err)
	}
	return effects
}

// mustSubmit 提交技能，失败即终止
func mustSubmit(t *testing.T, e *Engine, use *SkillUse) {
	t.Helper()
	if err := e.SubmitSkillUse(use); err != nil {
		t.Fatalf("SubmitSkillUse(player=%s skill=%v target=%s) 于 %v: %v",
			use.PlayerID, use.Skill, use.TargetID, e.GetCurrentPhase(), err)
	}
}

// mustAddTo 直接向状态添加玩家，失败即终止。
// 供不经 Engine、直接测试 Resolver 与状态的单元测试使用。
func mustAddTo(t *testing.T, s *gameState, id string, role pb.RoleType) {
	t.Helper()
	if err := s.addPlayer(id, role); err != nil {
		t.Fatalf("addPlayer(%q, %v): %v", id, role, err)
	}
}
