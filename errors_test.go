package werewolf

import (
	"errors"
	"fmt"
	"testing"
)

func TestGameError_Error(t *testing.T) {
	// With message
	err := &GameError{
		Code:    CodePlayerNotFound,
		Message: "player p1 not found",
	}
	if err.Error() != "player p1 not found" {
		t.Errorf("expected 'player p1 not found', got '%s'", err.Error())
	}

	// Without message (uses code string)
	err2 := &GameError{
		Code: CodePlayerDead,
	}
	if err2.Error() != "PLAYER_DEAD" {
		t.Errorf("expected 'PLAYER_DEAD', got '%s'", err2.Error())
	}
}

func TestHasCode(t *testing.T) {
	gameErr := &GameError{
		Code:    CodeSkillNotAllowed,
		Message: "skill not allowed",
	}

	if !HasCode(gameErr, CodeSkillNotAllowed) {
		t.Error("expected HasCode to return true for matching code")
	}
	if HasCode(gameErr, CodePlayerDead) {
		t.Error("expected HasCode to return false for non-matching code")
	}

	// Non-GameError
	if HasCode(nil, CodePlayerNotFound) {
		t.Error("expected HasCode to return false for nil")
	}
}

func TestCodeOf(t *testing.T) {
	gameErr := &GameError{
		Code: CodeGameEnded,
	}

	code := CodeOf(gameErr)
	if code != CodeGameEnded {
		t.Errorf("expected GAME_ENDED, got %v", code)
	}

	// Non-GameError returns UNSPECIFIED
	code2 := CodeOf(nil)
	if code2 != CodeUnspecified {
		t.Errorf("expected UNSPECIFIED, got %v", code2)
	}
}

func TestWrapError(t *testing.T) {
	err := WrapError(CodeInvalidPhase, "invalid phase: %s", "START")

	if err.Code != CodeInvalidPhase {
		t.Errorf("expected INVALID_PHASE, got %v", err.Code)
	}
	if err.Message != "invalid phase: START" {
		t.Errorf("expected 'invalid phase: START', got '%s'", err.Message)
	}
}

func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		err  *GameError
		code ErrorCode
		msg  string
	}{
		{ErrPlayerNotFound, CodePlayerNotFound, "player not found"},
		{ErrPlayerDead, CodePlayerDead, "player is dead"},
		{ErrTargetNotFound, CodeTargetNotFound, "target not found"},
		{ErrTargetDead, CodeTargetDead, "target is dead"},
		{ErrSkillNotAllowed, CodeSkillNotAllowed, "skill not allowed in this phase"},
		{ErrGameNotStarted, CodeGameNotStarted, "game not started"},
		{ErrGameEnded, CodeGameEnded, "game has ended"},
		{ErrInvalidPhase, CodeInvalidPhase, "invalid phase"},
	}

	for _, tt := range tests {
		if tt.err.Code != tt.code {
			t.Errorf("expected code %v, got %v", tt.code, tt.err.Code)
		}
		if tt.err.Message != tt.msg {
			t.Errorf("expected message '%s', got '%s'", tt.msg, tt.err.Message)
		}
		// Test Error() returns message
		if tt.err.Error() != tt.msg {
			t.Errorf("expected Error() '%s', got '%s'", tt.msg, tt.err.Error())
		}
	}
}

// TestErrorCode_ThroughWrappedError 调用方包一层上下文之后仍要能判断错误。
//
// 裸类型断言在 fmt.Errorf("...: %w", err) 之后就再也不命中了，
// 而这是库使用者最常见的写法，HasCode / ErrorCode 又是本库导出的
// 唯一错误判定入口。
func TestCodeOf_ThroughWrappedError(t *testing.T) {
	wrapped := fmt.Errorf("submit failed: %w", ErrPlayerNotFound)

	if !errors.Is(wrapped, ErrPlayerNotFound) {
		t.Fatal("errors.Is 应当命中")
	}
	if !HasCode(wrapped, CodePlayerNotFound) {
		t.Error("HasCode 应当穿透包装")
	}
	if got := CodeOf(wrapped); got != CodePlayerNotFound {
		t.Errorf("CodeOf: 期望 PLAYER_NOT_FOUND，实际 %v", got)
	}

	if got := CodeOf(errors.New("plain")); got != CodeUnspecified {
		t.Errorf("非本库错误应返回 UNSPECIFIED，实际 %v", got)
	}
}

// TestWrapError_MatchesSentinel 带上下文的错误要能被预定义哨兵认出来。
//
// 预定义的那批 Err* 变量此前有几个从未出现在任何返回路径上：
// 实际返回的是 WrapError 出来的富文本错误，errors.Is 比对永远落空，
// 而读 errors.go 的人会理所当然地以为它们能用。
func TestWrapError_MatchesSentinel(t *testing.T) {
	engine := MustNewEngine(nil)
	mustAdd(t, engine, "w1", RoleWerewolf)

	err := engine.AddPlayer("w1", RoleVillager)
	if !errors.Is(err, ErrPlayerExists) {
		t.Errorf("重复加玩家应当命中 ErrPlayerExists，实际 %v", err)
	}
	if err := engine.AddPlayer("x", RoleGod); !errors.Is(err, ErrInvalidRole) {
		t.Errorf("非法角色应当命中 ErrInvalidRole，实际 %v", err)
	}

	// 同一错误码下的具体哨兵，也要能被该类的通用哨兵认出
	if !errors.Is(ErrNoWerewolf, ErrInvalidBoard) {
		t.Error("ErrNoWerewolf 应当属于 ErrInvalidBoard 这一类")
	}

	// 快照版本不符
	snap := &Snapshot{Version: SnapshotVersion + 1}
	if _, err := RestoreEngine(nil, snap); !errors.Is(err, ErrInvalidSnapshot) {
		t.Errorf("版本不符应当命中 ErrInvalidSnapshot，实际 %v", err)
	}
}
