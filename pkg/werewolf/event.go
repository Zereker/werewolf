package werewolf

import (
	"fmt"
	"time"

	"github.com/Zereker/werewolf/pkg/game"
)

// EventType 事件类型
type EventType string

const (
	EventSystemGameStart EventType = "system_game_start" // 游戏开始
	EventSystemGameEnd   EventType = "system_game_end"   // 游戏结束

	EventSystemPhaseStart EventType = "system_phase_start" // 阶段开始
	EventSystemPhaseEnd   EventType = "system_phase_end"   // 阶段结束

	EventSystemPlayerDeath EventType = "system_player_death" // 玩家死亡
	EventSystemSkillResult EventType = "system_skill_result" // 技能使用结果
	EventSystemVoteResult  EventType = "system_vote_result"  // 投票结果
	EventSystemGameResult  EventType = "system_game_result"  // 游戏结果

	EventUserSkill EventType = "user_skill" // 玩家使用技能
	EventUserSpeak EventType = "user_speak" // 玩家发言
	EventUserVote  EventType = "user_vote"  // 玩家投票
)

// EventVisibility 事件可见性
type EventVisibility int

const (
	// VisibilityAll 所有人可见
	VisibilityAll EventVisibility = iota
	// VisibilityRole 同角色可见
	VisibilityRole
	// VisibilityTeam 同阵营可见
	VisibilityTeam
	// VisibilityPrivate 仅相关玩家可见
	VisibilityPrivate
)

// Event 游戏事件
type Event struct {
	ID        string      `json:"id"`        // 事件ID
	Type      EventType   `json:"type"`      // 事件类型
	PlayerID  string      `json:"player_id"` // 发送者ID
	Receivers []string    `json:"receivers"` // 接收者ID列表
	Timestamp time.Time   `json:"timestamp"` // 事件发生时间
	Data      interface{} `json:"data"`      // 事件数据
}

func (e Event) String() string {
	return fmt.Sprintf(
		"Event{Type:%s, PlayerID:%s, Data:%s, Receivers:%v, Timestamp:%s}",
		e.Type, e.PlayerID, e.Data, e.Receivers, e.Timestamp.Format(time.RFC3339),
	)
}

type SystemGameStartData struct {
	Players []PlayerInfo
	Phase   PhaseInfo
	Role    string
}

func (s SystemGameStartData) String() string {
	// 统计各角色数量
	roleCount := make(map[string]int)
	for _, p := range s.Players {
		roleName := p.Role.GetName().String()
		roleCount[roleName]++
	}

	// 生成玩家列表
	playerList := ""
	for _, p := range s.Players {
		playerList += fmt.Sprintf("玩家：%s\n", p.ID)
	}

	// 生成角色分布
	roleList := ""
	for role, count := range roleCount {
		roleList += fmt.Sprintf("%s：%d人\n", role, count)
	}

	// 生成开场白
	return fmt.Sprintf(
		"——— 狼人杀游戏开始 ———\n"+
			"本局共有%d名玩家：\n%s"+
			"角色分布如下：\n%s"+
			"你是【%s】。\n"+
			"当前阶段：%s，第%d回合。\n"+
			"请遵守游戏规则，祝你好运！",
		len(s.Players),
		playerList,
		roleList,
		s.Role,
		s.Phase.Type, s.Phase.Round,
	)
}

type SystemGameEndData struct {
	Winner    game.Camp    `json:"winner"`    // 获胜阵营
	Round     int          `json:"round"`     // 结束回合
	Timestamp time.Time    `json:"timestamp"` // 结束时间
	Players   []PlayerInfo `json:"players"`   // 玩家信息列表
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
	Round    int
	Success  bool
	Message  string
	VoterID  string // 添加 VoterID 字段
	TargetID string // 添加 TargetID 字段
}

type UserSkillData struct {
	TargetID  string         `json:"target_id"`  // 目标ID
	SkillType game.SkillType `json:"skill_type"` // 技能类型
}

type UserSpeakData struct {
	Message string `json:"message"` // 发言内容
}

type UserVoteData struct {
	TargetID string `json:"target_id"` // 投票目标ID
}

type PlayerInfo struct {
	ID      string    `json:"id"`       // 玩家ID
	Role    game.Role `json:"role"`     // 玩家角色
	IsAlive bool      `json:"is_alive"` // 是否存活
}

type PhaseInfo struct {
	Type      game.PhaseType `json:"type"`       // 阶段类型
	Round     int            `json:"round"`      // 当前回合
	StartTime time.Time      `json:"start_time"` // 开始时间
	Duration  int            `json:"duration"`   // 持续时间（秒）
}
