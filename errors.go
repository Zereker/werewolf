package werewolf

import (
	"errors"
	"fmt"

	pb "github.com/Zereker/werewolf/proto"
)

// GameError 游戏错误（实现 error 接口）
type GameError struct {
	Code    pb.ErrorCode
	Message string

	// sentinel 本错误对应的预定义哨兵，供 errors.Is 比对。
	//
	// 带上下文的错误（WrapError 出来的那些）需要既能打印出具体信息，
	// 又能被 errors.Is 认出是哪一类。少了这一环，调用方只能拿错误码
	// 做字符串式的分流，而预定义的那批 Err* 变量看着能用、实际永远不命中。
	sentinel error
}

// Error 实现 error 接口
func (e *GameError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Code.String()
}

// Unwrap 返回本错误对应的预定义哨兵，让 errors.Is 能穿透带上下文的错误。
func (e *GameError) Unwrap() error { return e.sentinel }

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

	// 玩家与开局校验
	ErrPlayerExists       = &GameError{Code: pb.ErrorCode_ERROR_CODE_PLAYER_EXISTS, Message: "player already exists"}
	ErrInvalidPlayerID    = &GameError{Code: pb.ErrorCode_ERROR_CODE_INVALID_PLAYER_ID, Message: "player id must not be empty"}
	ErrInvalidRole        = &GameError{Code: pb.ErrorCode_ERROR_CODE_INVALID_ROLE, Message: "role cannot be assigned to a player"}
	ErrGameAlreadyStarted = &GameError{Code: pb.ErrorCode_ERROR_CODE_GAME_ALREADY_STARTED, Message: "game already started"}
	ErrInvalidBoard       = &GameError{Code: pb.ErrorCode_ERROR_CODE_INVALID_BOARD, Message: "invalid board"}
	ErrNoWerewolf         = &GameError{Code: pb.ErrorCode_ERROR_CODE_INVALID_BOARD, Message: "board must contain at least one werewolf", sentinel: ErrInvalidBoard}
	ErrNoGoodPlayer       = &GameError{Code: pb.ErrorCode_ERROR_CODE_INVALID_BOARD, Message: "board must contain at least one good player", sentinel: ErrInvalidBoard}

	// 快照与效果流
	ErrInvalidSnapshot  = &GameError{Code: pb.ErrorCode_ERROR_CODE_INVALID_SNAPSHOT, Message: "invalid snapshot"}
	ErrNilSnapshot      = &GameError{Code: pb.ErrorCode_ERROR_CODE_INVALID_SNAPSHOT, Message: "snapshot must not be nil", sentinel: ErrInvalidSnapshot}
	ErrInvalidEffectLog = &GameError{Code: pb.ErrorCode_ERROR_CODE_INVALID_EFFECT_LOG, Message: "invalid effect log"}

	// 配置
	ErrInvalidConfig = &GameError{Code: pb.ErrorCode_ERROR_CODE_INVALID_CONFIG, Message: "invalid game config"}
)

// IsErrorCode 检查错误是否匹配指定错误码。
//
// 走 errors.As 而非裸类型断言：调用方用 fmt.Errorf("...: %w", err)
// 包一层上下文是最常见的写法，裸断言在那之后就再也不命中了。
func IsErrorCode(err error, code pb.ErrorCode) bool {
	return ErrorCode(err) == code
}

// ErrorCode 从错误获取错误码，不是本库的错误时返回 UNSPECIFIED。
func ErrorCode(err error) pb.ErrorCode {
	var gameErr *GameError
	if errors.As(err, &gameErr) {
		return gameErr.Code
	}
	return pb.ErrorCode_ERROR_CODE_UNSPECIFIED
}

// WrapError 构造一个带上下文的错误。
//
// 错误码对应的预定义哨兵会被挂上，因此
// errors.Is(err, ErrPlayerExists) 对 WrapError 出来的错误同样成立。
func WrapError(code pb.ErrorCode, format string, args ...interface{}) *GameError {
	return &GameError{
		Code:     code,
		Message:  fmt.Sprintf(format, args...),
		sentinel: sentinelFor(code),
	}
}

// sentinelByCode 错误码到预定义哨兵的映射。
//
// 一个码下若有多个更具体的哨兵（INVALID_BOARD 下的缺狼与缺好人），
// 映射指向那一类的通用哨兵，具体的那几个再把它挂成自己的 sentinel，
// 于是 errors.Is(ErrNoWerewolf, ErrInvalidBoard) 也成立。
var sentinelByCode = map[pb.ErrorCode]error{
	pb.ErrorCode_ERROR_CODE_PLAYER_NOT_FOUND:     ErrPlayerNotFound,
	pb.ErrorCode_ERROR_CODE_PLAYER_DEAD:          ErrPlayerDead,
	pb.ErrorCode_ERROR_CODE_TARGET_NOT_FOUND:     ErrTargetNotFound,
	pb.ErrorCode_ERROR_CODE_TARGET_DEAD:          ErrTargetDead,
	pb.ErrorCode_ERROR_CODE_SKILL_NOT_ALLOWED:    ErrSkillNotAllowed,
	pb.ErrorCode_ERROR_CODE_GAME_NOT_STARTED:     ErrGameNotStarted,
	pb.ErrorCode_ERROR_CODE_GAME_ENDED:           ErrGameEnded,
	pb.ErrorCode_ERROR_CODE_INVALID_PHASE:        ErrInvalidPhase,
	pb.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED:  ErrMessageNotAllowed,
	pb.ErrorCode_ERROR_CODE_PLAYER_EXISTS:        ErrPlayerExists,
	pb.ErrorCode_ERROR_CODE_INVALID_PLAYER_ID:    ErrInvalidPlayerID,
	pb.ErrorCode_ERROR_CODE_INVALID_ROLE:         ErrInvalidRole,
	pb.ErrorCode_ERROR_CODE_GAME_ALREADY_STARTED: ErrGameAlreadyStarted,
	pb.ErrorCode_ERROR_CODE_INVALID_BOARD:        ErrInvalidBoard,
	pb.ErrorCode_ERROR_CODE_INVALID_SNAPSHOT:     ErrInvalidSnapshot,
	pb.ErrorCode_ERROR_CODE_INVALID_CONFIG:       ErrInvalidConfig,
	pb.ErrorCode_ERROR_CODE_INVALID_EFFECT_LOG:   ErrInvalidEffectLog,
}

// sentinelFor 返回错误码对应的哨兵，没有对应哨兵时返回 nil。
func sentinelFor(code pb.ErrorCode) error {
	return sentinelByCode[code]
}
