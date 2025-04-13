package game

import "errors"

var (
	// ErrInvalidAction is returned when action is invalid
	ErrInvalidAction = errors.New("invalid action")
	// ErrInvalidPhase is returned when phase is invalid
	ErrInvalidPhase = errors.New("invalid phase")
	// ErrInvalidPlayer is returned when player is invalid
	ErrInvalidPlayer = errors.New("invalid player")
	// ErrInvalidRole is returned when role is invalid
	ErrInvalidRole = errors.New("invalid role")
	// ErrInvalidSkill is returned when skill is invalid
	ErrInvalidSkill = errors.New("invalid skill")
	// ErrInvalidTarget is returned when target is invalid
	ErrInvalidTarget = errors.New("invalid target")
	// ErrInvalidCamp is returned when camp is invalid
	ErrInvalidCamp = errors.New("invalid camp")
	// ErrInvalidGameState is returned when game state is invalid
	ErrInvalidGameState = errors.New("invalid game state")
	// ErrInvalidGameRules is returned when game rules is invalid
	ErrInvalidGameRules = errors.New("invalid game rules")
	// ErrInvalidGameManager is returned when game manager is invalid
	ErrInvalidGameManager = errors.New("invalid game manager")
	// ErrNotYourTurn 不是你的回合
	ErrNotYourTurn = errors.New("不是你的回合")
	// ErrGameNotStarted 游戏未开始
	ErrGameNotStarted = errors.New("游戏未开始")
	// ErrGameOver 游戏已结束
	ErrGameOver = errors.New("游戏已结束")
	// ErrPlayerDead 玩家已死亡
	ErrPlayerDead = errors.New("玩家已死亡")
	// ErrTargetDead 目标已死亡
	ErrTargetDead = errors.New("目标已死亡")
	// ErrTargetProtected 目标被保护
	ErrTargetProtected = errors.New("目标被保护")
)
