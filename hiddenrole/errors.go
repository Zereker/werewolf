package hiddenrole

import (
	"errors"
	"fmt"
)

// ErrorCode classifies an error.
//
// Like the other enums it is a string underneath -- error codes show up in
// logs and in JSON, and the name itself is both the most stable and the most
// readable representation.
type ErrorCode string

const (
	CodeUnspecified        ErrorCode = ""
	CodePlayerNotFound     ErrorCode = "PLAYER_NOT_FOUND"     // no such player
	CodePlayerDead         ErrorCode = "PLAYER_DEAD"          // the player is dead
	CodeTargetNotFound     ErrorCode = "TARGET_NOT_FOUND"     // no such target
	CodeTargetDead         ErrorCode = "TARGET_DEAD"          // the target is dead
	CodeSkillNotAllowed    ErrorCode = "SKILL_NOT_ALLOWED"    // the skill is not allowed in this phase
	CodeGameNotStarted     ErrorCode = "GAME_NOT_STARTED"     // the game has not started
	CodeGameEnded          ErrorCode = "GAME_ENDED"           // the game is over
	CodeInvalidPhase       ErrorCode = "INVALID_PHASE"        // no such phase
	CodeMessageNotAllowed  ErrorCode = "MESSAGE_NOT_ALLOWED"  // speaking is not allowed in this phase
	CodePlayerExists       ErrorCode = "PLAYER_EXISTS"        // that player id is taken
	CodeInvalidPlayerID    ErrorCode = "INVALID_PLAYER_ID"    // malformed player id
	CodeInvalidRole        ErrorCode = "INVALID_ROLE"         // this role cannot be assigned to a player
	CodeGameAlreadyStarted ErrorCode = "GAME_ALREADY_STARTED" // the game has already started
	CodeInvalidBoard       ErrorCode = "INVALID_BOARD"        // the board setup is invalid
	CodeInvalidSnapshot    ErrorCode = "INVALID_SNAPSHOT"     // the snapshot is invalid or of an incompatible version
	CodeInvalidConfig      ErrorCode = "INVALID_CONFIG"       // the game config is invalid
	CodeInvalidEffectLog   ErrorCode = "INVALID_EFFECT_LOG"   // the effect log is invalid and cannot be replayed
)

// String implements fmt.Stringer.
func (v ErrorCode) String() string {
	if v == CodeUnspecified {
		return "UNSPECIFIED"
	}
	return string(v)
}

// GameError is this package's error type.
type GameError struct {
	Code    ErrorCode
	Message string

	// sentinel is the predefined sentinel this error corresponds to, for
	// errors.Is to match against.
	//
	// An error carrying context (one built by WrapError) has to both print
	// the specific details and still be recognisable by errors.Is as a member
	// of its class. Without this link a caller can only dispatch on the error
	// code as a string, and the predefined Err* variables look usable while
	// never actually matching anything.
	sentinel error
}

// Error implements error.
func (e *GameError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Code.String()
}

// Unwrap returns the predefined sentinel, so errors.Is can see through an
// error that carries context.
func (e *GameError) Unwrap() error { return e.sentinel }

// Predefined errors.
var (
	ErrPlayerNotFound    = &GameError{Code: CodePlayerNotFound, Message: "player not found"}
	ErrPlayerDead        = &GameError{Code: CodePlayerDead, Message: "player is dead"}
	ErrTargetNotFound    = &GameError{Code: CodeTargetNotFound, Message: "target not found"}
	ErrTargetDead        = &GameError{Code: CodeTargetDead, Message: "target is dead"}
	ErrSkillNotAllowed   = &GameError{Code: CodeSkillNotAllowed, Message: "skill not allowed in this phase"}
	ErrGameNotStarted    = &GameError{Code: CodeGameNotStarted, Message: "game not started"}
	ErrGameEnded         = &GameError{Code: CodeGameEnded, Message: "game has ended"}
	ErrInvalidPhase      = &GameError{Code: CodeInvalidPhase, Message: "invalid phase"}
	ErrMessageNotAllowed = &GameError{Code: CodeMessageNotAllowed, Message: "message not allowed in this phase"}

	// Player and start-of-game validation.
	ErrPlayerExists        = &GameError{Code: CodePlayerExists, Message: "player already exists"}
	ErrInvalidPlayerID     = &GameError{Code: CodeInvalidPlayerID, Message: "player id must not be empty"}
	ErrInvalidRole         = &GameError{Code: CodeInvalidRole, Message: "role cannot be assigned to a player"}
	ErrGameAlreadyStarted  = &GameError{Code: CodeGameAlreadyStarted, Message: "game already started"}
	ErrInvalidBoard        = &GameError{Code: CodeInvalidBoard, Message: "invalid board"}
	ErrBoardAlreadyDecided = &GameError{Code: CodeInvalidBoard, Message: "board is already decided before the game starts", sentinel: ErrInvalidBoard}

	// Snapshots and effect logs.
	ErrInvalidSnapshot  = &GameError{Code: CodeInvalidSnapshot, Message: "invalid snapshot"}
	ErrNilSnapshot      = &GameError{Code: CodeInvalidSnapshot, Message: "snapshot must not be nil", sentinel: ErrInvalidSnapshot}
	ErrInvalidEffectLog = &GameError{Code: CodeInvalidEffectLog, Message: "invalid effect log"}

	// Config.
	ErrInvalidConfig = &GameError{Code: CodeInvalidConfig, Message: "invalid game config"}
)

// HasCode reports whether an error carries the given code.
//
// It goes through errors.As rather than a bare type assertion: wrapping an
// error in context with fmt.Errorf("...: %w", err) is the most common thing a
// caller does, and a bare assertion stops matching the moment they do.
func HasCode(err error, code ErrorCode) bool {
	return CodeOf(err) == code
}

// CodeOf extracts the error code, returning CodeUnspecified for an error that
// did not come from this library.
func CodeOf(err error) ErrorCode {
	var gameErr *GameError
	if errors.As(err, &gameErr) {
		return gameErr.Code
	}
	return CodeUnspecified
}

// WrapError builds an error that carries context.
//
// The sentinel for the given code is attached, so errors.Is(err,
// ErrPlayerExists) holds for errors built by WrapError too.
func WrapError(code ErrorCode, format string, args ...interface{}) *GameError {
	return &GameError{
		Code:     code,
		Message:  fmt.Sprintf(format, args...),
		sentinel: sentinelFor(code),
	}
}

// sentinelByCode maps an error code to its predefined sentinel.
//
// Where one code has several more specific sentinels (no werewolves and no
// villagers both live under INVALID_BOARD), the map points at the general
// sentinel of that class and the specific ones attach it as their own
// sentinel, so errors.Is(ErrBoardAlreadyDecided, ErrInvalidBoard) holds as
// well.
var sentinelByCode = map[ErrorCode]error{
	CodePlayerNotFound:     ErrPlayerNotFound,
	CodePlayerDead:         ErrPlayerDead,
	CodeTargetNotFound:     ErrTargetNotFound,
	CodeTargetDead:         ErrTargetDead,
	CodeSkillNotAllowed:    ErrSkillNotAllowed,
	CodeGameNotStarted:     ErrGameNotStarted,
	CodeGameEnded:          ErrGameEnded,
	CodeInvalidPhase:       ErrInvalidPhase,
	CodeMessageNotAllowed:  ErrMessageNotAllowed,
	CodePlayerExists:       ErrPlayerExists,
	CodeInvalidPlayerID:    ErrInvalidPlayerID,
	CodeInvalidRole:        ErrInvalidRole,
	CodeGameAlreadyStarted: ErrGameAlreadyStarted,
	CodeInvalidBoard:       ErrInvalidBoard,
	CodeInvalidSnapshot:    ErrInvalidSnapshot,
	CodeInvalidConfig:      ErrInvalidConfig,
	CodeInvalidEffectLog:   ErrInvalidEffectLog,
}

// sentinelFor returns the sentinel for a code, or nil when there is none.
func sentinelFor(code ErrorCode) error {
	return sentinelByCode[code]
}
