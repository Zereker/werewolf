package werewolf

import (
	"errors"
	"fmt"
	"testing"

	pb "github.com/Zereker/werewolf/proto"
)

func TestGameError_Error(t *testing.T) {
	// With message
	err := &GameError{
		Code:    pb.ErrorCode_ERROR_CODE_PLAYER_NOT_FOUND,
		Message: "player p1 not found",
	}
	if err.Error() != "player p1 not found" {
		t.Errorf("expected 'player p1 not found', got '%s'", err.Error())
	}

	// Without message (uses code string)
	err2 := &GameError{
		Code: pb.ErrorCode_ERROR_CODE_PLAYER_DEAD,
	}
	if err2.Error() != "ERROR_CODE_PLAYER_DEAD" {
		t.Errorf("expected 'ERROR_CODE_PLAYER_DEAD', got '%s'", err2.Error())
	}
}

func TestIsErrorCode(t *testing.T) {
	gameErr := &GameError{
		Code:    pb.ErrorCode_ERROR_CODE_SKILL_NOT_ALLOWED,
		Message: "skill not allowed",
	}

	if !IsErrorCode(gameErr, pb.ErrorCode_ERROR_CODE_SKILL_NOT_ALLOWED) {
		t.Error("expected IsErrorCode to return true for matching code")
	}
	if IsErrorCode(gameErr, pb.ErrorCode_ERROR_CODE_PLAYER_DEAD) {
		t.Error("expected IsErrorCode to return false for non-matching code")
	}

	// Non-GameError
	if IsErrorCode(nil, pb.ErrorCode_ERROR_CODE_PLAYER_NOT_FOUND) {
		t.Error("expected IsErrorCode to return false for nil")
	}
}

func TestGetErrorCode(t *testing.T) {
	gameErr := &GameError{
		Code: pb.ErrorCode_ERROR_CODE_GAME_ENDED,
	}

	code := ErrorCode(gameErr)
	if code != pb.ErrorCode_ERROR_CODE_GAME_ENDED {
		t.Errorf("expected ERROR_CODE_GAME_ENDED, got %v", code)
	}

	// Non-GameError returns UNSPECIFIED
	code2 := ErrorCode(nil)
	if code2 != pb.ErrorCode_ERROR_CODE_UNSPECIFIED {
		t.Errorf("expected ERROR_CODE_UNSPECIFIED, got %v", code2)
	}
}

func TestWrapError(t *testing.T) {
	err := WrapError(pb.ErrorCode_ERROR_CODE_INVALID_PHASE, "invalid phase: %s", "START")

	if err.Code != pb.ErrorCode_ERROR_CODE_INVALID_PHASE {
		t.Errorf("expected ERROR_CODE_INVALID_PHASE, got %v", err.Code)
	}
	if err.Message != "invalid phase: START" {
		t.Errorf("expected 'invalid phase: START', got '%s'", err.Message)
	}
}

func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		err  *GameError
		code pb.ErrorCode
		msg  string
	}{
		{ErrPlayerNotFound, pb.ErrorCode_ERROR_CODE_PLAYER_NOT_FOUND, "player not found"},
		{ErrPlayerDead, pb.ErrorCode_ERROR_CODE_PLAYER_DEAD, "player is dead"},
		{ErrTargetNotFound, pb.ErrorCode_ERROR_CODE_TARGET_NOT_FOUND, "target not found"},
		{ErrTargetDead, pb.ErrorCode_ERROR_CODE_TARGET_DEAD, "target is dead"},
		{ErrSkillNotAllowed, pb.ErrorCode_ERROR_CODE_SKILL_NOT_ALLOWED, "skill not allowed in this phase"},
		{ErrGameNotStarted, pb.ErrorCode_ERROR_CODE_GAME_NOT_STARTED, "game not started"},
		{ErrGameEnded, pb.ErrorCode_ERROR_CODE_GAME_ENDED, "game has ended"},
		{ErrInvalidPhase, pb.ErrorCode_ERROR_CODE_INVALID_PHASE, "invalid phase"},
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
// 而这是库使用者最常见的写法，IsErrorCode / ErrorCode 又是本库导出的
// 唯一错误判定入口。
func TestErrorCode_ThroughWrappedError(t *testing.T) {
	wrapped := fmt.Errorf("submit failed: %w", ErrPlayerNotFound)

	if !errors.Is(wrapped, ErrPlayerNotFound) {
		t.Fatal("errors.Is 应当命中")
	}
	if !IsErrorCode(wrapped, pb.ErrorCode_ERROR_CODE_PLAYER_NOT_FOUND) {
		t.Error("IsErrorCode 应当穿透包装")
	}
	if got := ErrorCode(wrapped); got != pb.ErrorCode_ERROR_CODE_PLAYER_NOT_FOUND {
		t.Errorf("ErrorCode: 期望 PLAYER_NOT_FOUND，实际 %v", got)
	}

	if got := ErrorCode(errors.New("plain")); got != pb.ErrorCode_ERROR_CODE_UNSPECIFIED {
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
	mustAdd(t, engine, "w1", pb.RoleType_ROLE_TYPE_WEREWOLF)

	err := engine.AddPlayer("w1", pb.RoleType_ROLE_TYPE_VILLAGER)
	if !errors.Is(err, ErrPlayerExists) {
		t.Errorf("重复加玩家应当命中 ErrPlayerExists，实际 %v", err)
	}
	if err := engine.AddPlayer("x", pb.RoleType_ROLE_TYPE_GOD); !errors.Is(err, ErrInvalidRole) {
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
