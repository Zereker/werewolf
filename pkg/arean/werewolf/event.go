package werewolf

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Zereker/werewolf/pkg/game"
)

// EventType 事件类型
type EventType string

const (
	// 系统事件（系统 -> 用户）
	EventSystemGameStart   EventType = "system_game_start"   // 游戏开始
	EventSystemGameEnd     EventType = "system_game_end"     // 游戏结束
	EventSystemPhaseStart  EventType = "system_phase_start"  // 阶段开始
	EventSystemPhaseEnd    EventType = "system_phase_end"    // 阶段结束
	EventSystemPlayerDeath EventType = "system_player_death" // 玩家死亡
	EventSystemSkillResult EventType = "system_skill_result" // 技能使用结果
	EventSystemVoteResult  EventType = "system_vote_result"  // 投票结果
	EventSystemGameResult  EventType = "system_game_result"  // 游戏结果

	// 用户事件（用户 -> 系统）
	EventUserSkill   EventType = "user_skill"   // 玩家使用技能
	EventUserSpeak   EventType = "user_speak"   // 玩家发言
	EventUserVote    EventType = "user_vote"    // 玩家投票
	EventUserReady   EventType = "user_ready"   // 玩家准备
	EventUserUnready EventType = "user_unready" // 玩家取消准备
)

// Event 游戏事件
type Event struct {
	ID        string      `json:"id"`        // 事件ID
	Type      EventType   `json:"type"`      // 事件类型
	PlayerID  int64       `json:"player_id"` // 玩家ID
	TargetID  int64       `json:"target_id"` // 目标ID（如果有）
	Timestamp time.Time   `json:"timestamp"` // 事件发生时间
	Data      interface{} `json:"data"`      // 额外数据
}

// 系统事件数据结构
type SystemGameStartData struct {
	Players []PlayerInfo `json:"players"` // 玩家信息列表
	Phase   PhaseInfo    `json:"phase"`   // 初始阶段信息
}

type SystemGameEndData struct {
	Winner    game.Camp `json:"winner"`    // 获胜阵营
	Round     int       `json:"round"`     // 结束回合
	Timestamp time.Time `json:"timestamp"` // 结束时间
}

type SystemPhaseData struct {
	Phase     game.PhaseType `json:"phase"`     // 阶段类型
	Round     int            `json:"round"`     // 当前回合
	Timestamp time.Time      `json:"timestamp"` // 阶段时间
	Duration  int            `json:"duration"`  // 阶段持续时间（秒）
}

type SystemSkillResultData struct {
	SkillType game.SkillType `json:"skill_type"` // 技能类型
	Success   bool           `json:"success"`    // 是否成功
	Message   string         `json:"message"`    // 结果消息
	Effect    interface{}    `json:"effect"`     // 技能效果
}

type SystemVoteResultData struct {
	Round      int           `json:"round"`      // 投票轮次
	VoteCount  map[int64]int `json:"vote_count"` // 每个玩家获得的票数
	Eliminated []int64       `json:"eliminated"` // 被淘汰的玩家ID
}

// 用户事件数据结构
type UserSkillData struct {
	SkillType game.SkillType `json:"skill_type"` // 技能类型
	TargetID  int64          `json:"target_id"`  // 目标ID
}

type UserSpeakData struct {
	Message string `json:"message"` // 发言内容
}

type UserVoteData struct {
	TargetID int64 `json:"target_id"` // 投票目标ID
}

// 辅助数据结构
type PlayerInfo struct {
	ID      int64     `json:"id"`       // 玩家ID
	Role    game.Role `json:"role"`     // 玩家角色
	IsAlive bool      `json:"is_alive"` // 是否存活
}

type PhaseInfo struct {
	Type      game.PhaseType `json:"type"`       // 阶段类型
	Round     int            `json:"round"`      // 当前回合
	StartTime time.Time      `json:"start_time"` // 开始时间
	Duration  int            `json:"duration"`   // 持续时间（秒）
}

// NewEvent 创建新事件
func NewEvent(eventType EventType, playerID int64, targetID int64, data interface{}) *Event {
	return &Event{
		ID:        generateEventID(),
		Type:      eventType,
		PlayerID:  playerID,
		TargetID:  targetID,
		Timestamp: time.Now(),
		Data:      data,
	}
}

// generateEventID 生成事件ID
func generateEventID() string {
	return fmt.Sprintf("evt_%s", uuid.New().String())
}
