package game

import (
	"context"

	"github.com/Zereker/werewolf/pkg/game/event"
)

// Camp represents player's camp
type Camp int

const (
	CampGood Camp = iota // Good camp
	CampEvil             // Bad camp
	CampNone
)

func (c Camp) String() string {
	switch c {
	case CampGood:
		return "good"
	case CampEvil:
		return "evil"
	default:
		return ""
	}
}

// PhaseType represents werewolf phase
type PhaseType string

const (
	PhaseNight PhaseType = "night" // Night phase
	PhaseDay   PhaseType = "day"   // Day phase
	PhaseVote  PhaseType = "vote"  // Vote phase
	PhaseStart PhaseType = "start" // Game start phase
	PhaseEnd   PhaseType = "end"   // Game end phase
)

// Action 表示一个技能行为
type Action struct {
	// Caster 技能施放者
	Caster Player
	// Target 技能目标
	Target Player
	// Skill 使用的技能
	Skill Skill
}

// Phase 阶段接口
type Phase interface {
	// Start 开始阶段
	Start(ctx context.Context) error
	// GetName 获取阶段类型
	GetName() PhaseType
	// GetRound 获取回合数
	GetRound() int
	// HandleAction processes a player's action within the phase.
	// player: The player performing the action.
	// actionData: The payload of the action from server.Message.Payload.
	// results: A channel to send back asynchronous errors or signals.
	// Returns an error for immediate validation issues.
	HandleAction(player Player, actionData interface{}, results chan<- error) error
	// IsComplete checks if the phase has completed its action collection or conditions.
	// The runtimeWrapper is an interface to avoid direct dependency on werewolf.Runtime in this package.
	// Concrete phase implementations will need to type-assert it to access specific runtime methods.
	IsComplete(runtimeWrapper interface{}) bool
}

// PhaseResult 阶段结果
type PhaseResult[T any] struct {
	Deaths    []Player // 死亡玩家列表
	ExtraData T        // 额外数据（如投票结果）
}

// SkillResultMap 技能结果映射
type SkillResultMap map[SkillType]*SkillResult

type UserSkillResultMap map[Player]*SkillResult

const SystemPlayerID = "System"

// Player interface defines player behavior
type Player interface {
	GetID() string
	GetRole() Role

	IsAlive() bool
	SetAlive(alive bool)

	IsProtected() bool
	SetProtected(protected bool)

	Write(event event.Event[any]) error // 写入事件
	// Read was removed as player input is now handled by server pushing actions to runtime.
}

// RoleType represents role type
type RoleType string

func (r RoleType) String() string {
	return string(r)
}

const (
	RoleTypeWerewolf RoleType = "werewolf" // Werewolf
	RoleTypeSeer     RoleType = "seer"     // Seer
	RoleTypeWitch    RoleType = "witch"    // Witch
	RoleTypeHunter   RoleType = "hunter"   // Hunter
	RoleTypeVillager RoleType = "villager" // Villager
	RoleTypeGuard    RoleType = "guard"    // Guard
)

// 角色优先级常量
const (
	PriorityLowest = 99

	PriorityWerewolfNight = 1              // 狼人优先级最高
	PrioritySeerNight     = 2              // 预言家第二
	PriorityGuardNight    = 3              // 守卫第三
	PriorityWitchNight    = 4              // 女巫最后
	PriorityHunterNight   = 5              // 猎人（死亡时触发）
	PriorityVillagerNight = PriorityLowest // 村民在夜晚没有行动

	PriorityHunterDay   = 1              // 猎人死亡时优先级最高
	PriorityVillagerDay = 2              // 村民发言优先级第二
	PriorityWerewolfDay = PriorityLowest // 狼人在白天没有特殊行动
	PrioritySeerDay     = PriorityLowest // 预言家在白天没有特殊行动
	PriorityGuardDay    = PriorityLowest // 守卫在白天没有特殊行动
	PriorityWitchDay    = PriorityLowest // 女巫在白天没有特殊行动

	PriorityVote = 1 // 所有角色在投票阶段优先级相同
)

// Role interface defines role behavior
type Role interface {
	// GetName returns role name
	GetName() RoleType
	// GetCamp returns role's camp
	GetCamp() Camp
	// GetAvailableSkills returns available skills in specific phase
	GetAvailableSkills(phase PhaseType) []Skill
	// GetPriority returns role's action priority in specific phase
	GetPriority(phase PhaseType) int
}

// SkillType 技能类型
type SkillType string

const (
	SkillTypeKill      SkillType = "kill"      // 狼人杀人
	SkillTypeCheck     SkillType = "check"     // 预言家查验
	SkillTypeAntidote  SkillType = "antidote"  // 女巫解药
	SkillTypePoison    SkillType = "poison"    // 女巫毒药
	SkillTypeHunter    SkillType = "hunter"    // 猎人开枪
	SkillTypeSpeak     SkillType = "speak"     // 发言
	SkillTypeVote      SkillType = "vote"      // 投票
	SkillTypeProtect   SkillType = "protect"   // 守护
	SkillTypeLastWords SkillType = "lastWords" // 遗言
)

// SkillResult 技能结果
type SkillResult struct {
	// Success 是否成功
	Success bool
	// Message 结果描述
	Message string
	// Data 额外数据
	Data interface{}
}

// Skill 技能接口
type Skill interface {
	// GetName 获取技能名称
	GetName() SkillType
	// GetPhase 获取技能使用阶段
	GetPhase() PhaseType
	// GetPriority 获取技能优先级
	GetPriority() int
	// Check 检查技能条件
	Check(phase PhaseType, caster Player, target Player) error
	// Put 使用技能，result 用于填充技能执行的结果
	Put(caster Player, target Player, result *SkillResult)
	// Reset 重置技能状态
	Reset()
}
