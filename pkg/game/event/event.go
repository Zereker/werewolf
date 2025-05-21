package event

import (
	"time"
)

// Type 事件类型
type Type string

const (
	SystemGameStart        Type = "system_game_start"   // 游戏开始
	SystemGameEnd          Type = "system_game_end"     // 游戏结束
	SystemPhaseStart       Type = "system_phase_start"  // 阶段开始
	SystemPhaseEnd         Type = "system_phase_end"    // 阶段结束
	EventSystemPlayerDeath Type = "system_player_death" // 玩家死亡
	SystemSkillResult      Type = "system_skill_result" // 技能使用结果
	EventSystemVoteResult  Type = "system_vote_result"  // 投票结果
	EventSystemGameResult  Type = "system_game_result"  // 游戏结果

	// UserSkill 用户事件
	UserSkill Type = "user_skill" // 玩家使用技能

	// Witch specific events
	SystemWitchNotification Type = "system_witch_notification" // Sent to Witch with werewolf target info
	UserSkillWitchResponse  Type = "user_skill_witch_response"  // Witch's response (use antidote/poison/skip)
)

// Event 游戏事件
type Event[T any] struct {
	ID        string    `json:"id"`        // 事件ID
	Type      Type      `json:"type"`      // 事件类型
	PlayerID  string    `json:"player_id"` // 发送者ID
	Receivers []string  `json:"receivers"` // 接收者ID列表
	Timestamp time.Time `json:"timestamp"` // 事件发生时间
	Data      T         `json:"data"`      // 事件数据
}

// Visibility 事件可见性
type Visibility int

const (
	// VisibilityAll 所有人可见
	VisibilityAll Visibility = iota
	// VisibilityRole 同角色可见
	VisibilityRole
	// VisibilityTeam 同阵营可见
	VisibilityTeam
	// VisibilityPrivate 仅相关玩家可见
	VisibilityPrivate
)

// SystemGameStartData 游戏开始事件数据
type SystemGameStartData struct {
	Players []PlayerInfo
	Phase   PhaseInfo
	Role    string
}

// SystemGameEndData 游戏结束事件数据
type SystemGameEndData struct {
	Winner    string
	Round     int
	Timestamp time.Time
	Players   []PlayerInfo
}

// SystemPhaseData 阶段事件数据
type SystemPhaseData struct {
	Phase     string
	Round     int
	Timestamp time.Time
}

// UserSkillData 用户技能事件数据
type UserSkillData struct {
	TargetID  string
	SkillType string
	Content   string // 遗言内容
}

// SystemWitchNotificationData informs the Witch about the night's events.
type SystemWitchNotificationData struct {
	WerewolfTargetID string // PlayerID of the player targeted by werewolves. Empty if none or multiple and rules hide it.
	CanUseAntidote   bool   // Does the Witch still have Antidote?
	CanUsePoison     bool   // Does the Witch still have Poison?
}

// UserSkillWitchResponseData is the Witch's decision.
type UserSkillWitchResponseData struct {
	UseAntidote      bool   // True if Witch wants to use Antidote
	AntidoteTargetID string // PlayerID to use Antidote on (could be WerewolfTargetID or another player)
	UsePoison        bool   // True if Witch wants to use Poison
	PoisonTargetID   string // PlayerID to use Poison on
}

// UserSpeakData 用户发言事件数据
type UserSpeakData struct {
	Message string
}

// UserVoteData 用户投票事件数据
type UserVoteData struct {
	TargetID string
}

// PlayerInfo 玩家信息
type PlayerInfo struct {
	ID      string
	Role    string
	IsAlive bool
}

// PhaseInfo 阶段信息
type PhaseInfo struct {
	Type      string
	Round     int
	StartTime time.Time
	Duration  int
}

// PhaseStartData 阶段开始数据
type PhaseStartData struct {
	Phase   string `json:"phase"`
	Round   int    `json:"round"`
	Message string `json:"message"`
}

// SkillResultData 技能结果数据
type SkillResultData struct {
	SkillType string      `json:"skill_type"`
	Message   string      `json:"message"`
	PlayerID  string      `json:"player_id,omitempty"`
	Options   interface{} `json:"options,omitempty"`
}

// VoteResultData 投票结果数据
type VoteResultData struct {
	Votes      map[string]string `json:"votes"`       // 玩家ID -> 被投票玩家ID
	Executed   string            `json:"executed"`    // 被处决的玩家ID
	IsTie      bool              `json:"is_tie"`      // 是否平票
	TiePlayers []string          `json:"tie_players"` // 平票的玩家ID列表
}

// GameResultData 游戏结果数据
type GameResultData struct {
	Winner    string            `json:"winner"`    // 获胜阵营
	Survivors []string          `json:"survivors"` // 存活的玩家ID列表
	Roles     map[string]string `json:"roles"`     // 玩家ID -> 角色
}
