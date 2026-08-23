package hiddenrole

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

// TestErrorCode_ThroughWrappedError: an error must still be classifiable
// after the caller wraps it in context.
//
// A bare type assertion stops matching after fmt.Errorf("...: %w", err), which
// is the most common thing a user of this library does, and HasCode / CodeOf
// are the only error-classification entry points this library exports.
func TestCodeOf_ThroughWrappedError(t *testing.T) {
	wrapped := fmt.Errorf("submit failed: %w", ErrPlayerNotFound)

	if !errors.Is(wrapped, ErrPlayerNotFound) {
		t.Fatal("errors.Is should match")
	}
	if !HasCode(wrapped, CodePlayerNotFound) {
		t.Error("HasCode should see through the wrapper")
	}
	if got := CodeOf(wrapped); got != CodePlayerNotFound {
		t.Errorf("CodeOf: want PLAYER_NOT_FOUND, got %v", got)
	}

	if got := CodeOf(errors.New("plain")); got != CodeUnspecified {
		t.Errorf("an error from elsewhere should give UNSPECIFIED, got %v", got)
	}
}

// TestWrapError_MatchesSentinel: an error carrying context must still be
// recognised by its predefined sentinel.
//
// Several of the predefined Err* variables used to appear on no return path
// at all: what was actually returned was a rich WrapError, errors.Is never
// matched, and anyone reading errors.go would reasonably assume they worked.
func TestWrapError_MatchesSentinel(t *testing.T) {
	engine := newTestEngine(t)
	mustAdd(t, engine, "w1", roleWerewolf)

	err := engine.AddPlayer("w1", roleVillager)
	if !errors.Is(err, ErrPlayerExists) {
		t.Errorf("adding a duplicate player should match ErrPlayerExists, got %v", err)
	}
	if err := engine.AddPlayer("x", RoleSystem); !errors.Is(err, ErrInvalidRole) {
		t.Errorf("an invalid role should match ErrInvalidRole, got %v", err)
	}

	// A specific sentinel under one code must also be recognised by that
	// class's general sentinel.
	if !errors.Is(ErrBoardAlreadyDecided, ErrInvalidBoard) {
		t.Error("ErrBoardAlreadyDecided should belong to the ErrInvalidBoard class")
	}

	// A snapshot version mismatch.
	snap := &Snapshot{Version: SnapshotVersion + 1}
	if _, err := RestoreEngine(testConfig(), snap); !errors.Is(err, ErrInvalidSnapshot) {
		t.Errorf("a version mismatch should match ErrInvalidSnapshot, got %v", err)
	}
}
