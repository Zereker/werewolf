package engine

import (
	"errors"
	"fmt"
)

// ErrorCode 错误码，用于把错误分类。
//
// 与其他枚举一样，底层是字符串——错误码会出现在日志与 JSON 里，
// 名字本身就是最稳定、也最能读懂的表示。
type ErrorCode string

const (
	CodeUnspecified        ErrorCode = ""
	CodePlayerNotFound     ErrorCode = "PLAYER_NOT_FOUND"     // 玩家未找到
	CodePlayerDead         ErrorCode = "PLAYER_DEAD"          // 玩家已死亡
	CodeTargetNotFound     ErrorCode = "TARGET_NOT_FOUND"     // 目标未找到
	CodeTargetDead         ErrorCode = "TARGET_DEAD"          // 目标已死亡
	CodeSkillNotAllowed    ErrorCode = "SKILL_NOT_ALLOWED"    // 技能不允许在此阶段使用
	CodeGameNotStarted     ErrorCode = "GAME_NOT_STARTED"     // 游戏未开始
	CodeGameEnded          ErrorCode = "GAME_ENDED"           // 游戏已结束
	CodeInvalidPhase       ErrorCode = "INVALID_PHASE"        // 无效阶段
	CodeMessageNotAllowed  ErrorCode = "MESSAGE_NOT_ALLOWED"  // 当前阶段不允许发言
	CodePlayerExists       ErrorCode = "PLAYER_EXISTS"        // 玩家ID已存在
	CodeInvalidPlayerID    ErrorCode = "INVALID_PLAYER_ID"    // 玩家ID非法
	CodeInvalidRole        ErrorCode = "INVALID_ROLE"         // 该角色不能作为玩家身份
	CodeGameAlreadyStarted ErrorCode = "GAME_ALREADY_STARTED" // 游戏已开始
	CodeInvalidBoard       ErrorCode = "INVALID_BOARD"        // 板子配置不合法
	CodeInvalidSnapshot    ErrorCode = "INVALID_SNAPSHOT"     // 快照不合法或版本不兼容
	CodeInvalidConfig      ErrorCode = "INVALID_CONFIG"       // 游戏配置不合法
	CodeInvalidEffectLog   ErrorCode = "INVALID_EFFECT_LOG"   // 效果流不合法，无法回放
)

// String 实现 fmt.Stringer。
func (v ErrorCode) String() string {
	if v == CodeUnspecified {
		return "UNSPECIFIED"
	}
	return string(v)
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
	ErrPlayerExists        = &GameError{Code: CodePlayerExists, Message: "player already exists"}
	ErrInvalidPlayerID     = &GameError{Code: CodeInvalidPlayerID, Message: "player id must not be empty"}
	ErrInvalidRole         = &GameError{Code: CodeInvalidRole, Message: "role cannot be assigned to a player"}
	ErrGameAlreadyStarted  = &GameError{Code: CodeGameAlreadyStarted, Message: "game already started"}
	ErrInvalidBoard        = &GameError{Code: CodeInvalidBoard, Message: "invalid board"}
	ErrBoardAlreadyDecided = &GameError{Code: CodeInvalidBoard, Message: "board is already decided before the game starts", sentinel: ErrInvalidBoard}

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
// 于是 errors.Is(ErrBoardAlreadyDecided, ErrInvalidBoard) 也成立。
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

// sentinelFor 返回错误码对应的哨兵，没有对应哨兵时返回 nil。
func sentinelFor(code ErrorCode) error {
	return sentinelByCode[code]
}
