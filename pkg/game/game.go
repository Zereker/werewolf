package game

// Camp represents player's camp
type Camp int

const (
	CampGood Camp = iota // Good camp
	CampBad              // Bad camp
)

// PhaseType represents werewolf phase
type PhaseType string

const (
	PhaseNight PhaseType = "night" // Night phase
	PhaseDay   PhaseType = "day"   // Day phase
	PhaseVote  PhaseType = "vote"  // Vote phase
)

// Phase interface defines werewolf phase behavior
type Phase interface {
	// GetName returns phase name
	GetName() PhaseType
	// GetNextPhase returns next phase
	GetNextPhase() Phase
	// SetNextPhase
	SetNextPhase(next Phase)
	// ValidateAction validates if action is legal
	ValidateAction(skill Skill) error
}

// Player interface defines player behavior
type Player interface {
	GetID() int64
	GetCamp() Camp
	GetRole() Role

	IsAlive() bool
	SetAlive(alive bool)

	IsProtected() bool
	SetProtected(protected bool)

	UseSkill(phase PhaseType, target Player, skill Skill) error
}

// RoleType represents role type
type RoleType string

const (
	RoleTypeWerewolf RoleType = "werewolf" // Werewolf
	RoleTypeSeer     RoleType = "seer"     // Seer
	RoleTypeWitch    RoleType = "witch"    // Witch
	RoleTypeHunter   RoleType = "hunter"   // Hunter
	RoleTypeVillager RoleType = "villager" // Villager
	RoleTypeGuard    RoleType = "guard"    // Guard
)

// Role interface defines role behavior
type Role interface {
	// GetName returns role name
	GetName() string
	// GetCamp returns role's camp
	GetCamp() Camp
	// GetAvailableSkills returns available skill types
	GetAvailableSkills() []SkillType
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
	GetName() string
	// Put 使用技能
	Put(phase PhaseType, caster Player, target Player) error
	// Reset 重置技能状态
	Reset()
	// IsUsed 技能是否已使用
	IsUsed() bool
	// UseInPhase 技能使用阶段
	UseInPhase() PhaseType
}
