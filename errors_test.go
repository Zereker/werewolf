package werewolf

import (
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

func TestNewGameError(t *testing.T) {
	err := NewGameError(pb.ErrorCode_ERROR_CODE_TARGET_NOT_FOUND, "target xyz not found")

	if err.Code != pb.ErrorCode_ERROR_CODE_TARGET_NOT_FOUND {
		t.Errorf("expected ERROR_CODE_TARGET_NOT_FOUND, got %v", err.Code)
	}
	if err.Message != "target xyz not found" {
		t.Errorf("expected 'target xyz not found', got '%s'", err.Message)
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

	code := GetErrorCode(gameErr)
	if code != pb.ErrorCode_ERROR_CODE_GAME_ENDED {
		t.Errorf("expected ERROR_CODE_GAME_ENDED, got %v", code)
	}

	// Non-GameError returns UNSPECIFIED
	code2 := GetErrorCode(nil)
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
