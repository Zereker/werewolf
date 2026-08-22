package werewolf

import (
	"errors"
	"fmt"
)

// ErrorCode 错误码，用于把错误分类。
type ErrorCode int32

const (
	CodeUnspecified        ErrorCode = 0
	CodePlayerNotFound     ErrorCode = 1  // 玩家未找到
	CodePlayerDead         ErrorCode = 2  // 玩家已死亡
	CodeTargetNotFound     ErrorCode = 3  // 目标未找到
	CodeTargetDead         ErrorCode = 4  // 目标已死亡
	CodeSkillNotAllowed    ErrorCode = 5  // 技能不允许在此阶段使用
	CodeGameNotStarted     ErrorCode = 6  // 游戏未开始
	CodeGameEnded          ErrorCode = 7  // 游戏已结束
	CodeInvalidPhase       ErrorCode = 8  // 无效阶段
	CodeMessageNotAllowed  ErrorCode = 9  // 当前阶段不允许发言
	CodePlayerExists       ErrorCode = 10 // 玩家ID已存在
	CodeInvalidPlayerId    ErrorCode = 11 // 玩家ID非法
	CodeInvalidRole        ErrorCode = 12 // 该角色不能作为玩家身份
	CodeGameAlreadyStarted ErrorCode = 13 // 游戏已开始
	CodeInvalidBoard       ErrorCode = 14 // 板子配置不合法
	CodeInvalidSnapshot    ErrorCode = 15 // 快照不合法或版本不兼容
	CodeInvalidConfig      ErrorCode = 16 // 游戏配置不合法
	CodeInvalidEffectLog   ErrorCode = 17 // 效果流不合法，无法回放
)

// String 实现 fmt.Stringer，输出沿用枚举全名。
func (v ErrorCode) String() string {
	if s, ok := errorCodeNames[v]; ok {
		return s
	}
	return fmt.Sprintf("ErrorCode(%d)", int32(v))
}

// errorCodeNames 全部取值到名字的映射，遍历它即可枚举所有取值。
var errorCodeNames = map[ErrorCode]string{
	CodeUnspecified:        "UNSPECIFIED",
	CodePlayerNotFound:     "PLAYER_NOT_FOUND",
	CodePlayerDead:         "PLAYER_DEAD",
	CodeTargetNotFound:     "TARGET_NOT_FOUND",
	CodeTargetDead:         "TARGET_DEAD",
	CodeSkillNotAllowed:    "SKILL_NOT_ALLOWED",
	CodeGameNotStarted:     "GAME_NOT_STARTED",
	CodeGameEnded:          "GAME_ENDED",
	CodeInvalidPhase:       "INVALID_PHASE",
	CodeMessageNotAllowed:  "MESSAGE_NOT_ALLOWED",
	CodePlayerExists:       "PLAYER_EXISTS",
	CodeInvalidPlayerId:    "INVALID_PLAYER_ID",
	CodeInvalidRole:        "INVALID_ROLE",
	CodeGameAlreadyStarted: "GAME_ALREADY_STARTED",
	CodeInvalidBoard:       "INVALID_BOARD",
	CodeInvalidSnapshot:    "INVALID_SNAPSHOT",
	CodeInvalidConfig:      "INVALID_CONFIG",
	CodeInvalidEffectLog:   "INVALID_EFFECT_LOG",
}

// GameError 游戏错误（实现 error 接口）
type GameError struct {
	Code    ErrorCode
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

// 预定义错误
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

	// 玩家与开局校验
	ErrPlayerExists       = &GameError{Code: CodePlayerExists, Message: "player already exists"}
	ErrInvalidPlayerID    = &GameError{Code: CodeInvalidPlayerId, Message: "player id must not be empty"}
	ErrInvalidRole        = &GameError{Code: CodeInvalidRole, Message: "role cannot be assigned to a player"}
	ErrGameAlreadyStarted = &GameError{Code: CodeGameAlreadyStarted, Message: "game already started"}
	ErrInvalidBoard       = &GameError{Code: CodeInvalidBoard, Message: "invalid board"}
	ErrNoWerewolf         = &GameError{Code: CodeInvalidBoard, Message: "board must contain at least one werewolf", sentinel: ErrInvalidBoard}
	ErrNoGoodPlayer       = &GameError{Code: CodeInvalidBoard, Message: "board must contain at least one good player", sentinel: ErrInvalidBoard}

	// 快照与效果流
	ErrInvalidSnapshot  = &GameError{Code: CodeInvalidSnapshot, Message: "invalid snapshot"}
	ErrNilSnapshot      = &GameError{Code: CodeInvalidSnapshot, Message: "snapshot must not be nil", sentinel: ErrInvalidSnapshot}
	ErrInvalidEffectLog = &GameError{Code: CodeInvalidEffectLog, Message: "invalid effect log"}

	// 配置
	ErrInvalidConfig = &GameError{Code: CodeInvalidConfig, Message: "invalid game config"}
)

// HasCode 检查错误是否匹配指定错误码。
//
// 走 errors.As 而非裸类型断言：调用方用 fmt.Errorf("...: %w", err)
// 包一层上下文是最常见的写法，裸断言在那之后就再也不命中了。
func HasCode(err error, code ErrorCode) bool {
	return CodeOf(err) == code
}

// CodeOf 从错误取出错误码，不是本库的错误时返回 CodeUnspecified。
func CodeOf(err error) ErrorCode {
	var gameErr *GameError
	if errors.As(err, &gameErr) {
		return gameErr.Code
	}
	return CodeUnspecified
}

// WrapError 构造一个带上下文的错误。
//
// 错误码对应的预定义哨兵会被挂上，因此
// errors.Is(err, ErrPlayerExists) 对 WrapError 出来的错误同样成立。
func WrapError(code ErrorCode, format string, args ...interface{}) *GameError {
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
	CodeInvalidPlayerId:    ErrInvalidPlayerID,
	CodeInvalidRole:        ErrInvalidRole,
	CodeGameAlreadyStarted: ErrGameAlreadyStarted,
	CodeInvalidBoard:       ErrInvalidBoard,
	CodeInvalidSnapshot:    ErrInvalidSnapshot,
	CodeInvalidConfig:      ErrInvalidConfig,
	CodeInvalidEffectLog:   ErrInvalidEffectLog,
}

// sentinelFor 返回错误码对应的哨兵，没有对应哨兵时返回 nil。
func sentinelFor(code ErrorCode) error {
	return sentinelByCode[code]
}
