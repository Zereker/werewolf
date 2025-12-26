package werewolf

import (
	"fmt"

	pb "github.com/Zereker/werewolf/proto"
)

// GameError 游戏错误（实现 error 接口）
type GameError struct {
	Code    pb.ErrorCode
	Message string
}

// Error 实现 error 接口
func (e *GameError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Code.String()
}

// NewGameError 创建游戏错误
func NewGameError(code pb.ErrorCode, message string) *GameError {
	return &GameError{
		Code:    code,
		Message: message,
	}
}

// 预定义错误
var (
	ErrPlayerNotFound    = &GameError{Code: pb.ErrorCode_ERROR_CODE_PLAYER_NOT_FOUND, Message: "player not found"}
	ErrPlayerDead        = &GameError{Code: pb.ErrorCode_ERROR_CODE_PLAYER_DEAD, Message: "player is dead"}
	ErrTargetNotFound    = &GameError{Code: pb.ErrorCode_ERROR_CODE_TARGET_NOT_FOUND, Message: "target not found"}
	ErrTargetDead        = &GameError{Code: pb.ErrorCode_ERROR_CODE_TARGET_DEAD, Message: "target is dead"}
	ErrSkillNotAllowed   = &GameError{Code: pb.ErrorCode_ERROR_CODE_SKILL_NOT_ALLOWED, Message: "skill not allowed in this phase"}
	ErrGameNotStarted    = &GameError{Code: pb.ErrorCode_ERROR_CODE_GAME_NOT_STARTED, Message: "game not started"}
	ErrGameEnded         = &GameError{Code: pb.ErrorCode_ERROR_CODE_GAME_ENDED, Message: "game has ended"}
	ErrInvalidPhase      = &GameError{Code: pb.ErrorCode_ERROR_CODE_INVALID_PHASE, Message: "invalid phase"}
	ErrMessageNotAllowed = &GameError{Code: pb.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED, Message: "message not allowed in this phase"}
)

// IsErrorCode 检查错误是否匹配指定错误码
func IsErrorCode(err error, code pb.ErrorCode) bool {
	if gameErr, ok := err.(*GameError); ok {
		return gameErr.Code == code
	}
	return false
}

// GetErrorCode 从错误获取错误码
func GetErrorCode(err error) pb.ErrorCode {
	if gameErr, ok := err.(*GameError); ok {
		return gameErr.Code
	}
	return pb.ErrorCode_ERROR_CODE_UNSPECIFIED
}

// WrapError 包装错误并添加上下文
func WrapError(code pb.ErrorCode, format string, args ...interface{}) *GameError {
	return &GameError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}
