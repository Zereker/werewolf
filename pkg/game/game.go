package game

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
	case CampNone:
		return "none"
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
)

// Action 表示一个技能行为
type Action struct {
	// Caster 技能施放者
	Caster Player
	// Target 技能目标
	Target Player
	// Skill 使用的技能
	Skill Skill

	// Content 发言内容
	Content string
}

// Phase 游戏阶段接口
type Phase interface {
	GetName() PhaseType

	Start()
	Handle(action Action) error
	IsCompleted() bool

	GetPhaseResult() *PhaseResult[SkillResultMap]
}

// PhaseResult 阶段结果
type PhaseResult[T any] struct {
	Deaths    []Player // 死亡玩家列表
	ExtraData T        // 额外数据（如投票结果）
}

// SkillResultMap 技能结果映射
type SkillResultMap map[SkillType]*SkillResult

// Player interface defines player behavior
type Player interface {
	GetRole() Role

	IsAlive() bool
	SetAlive(alive bool)

	IsProtected() bool
	SetProtected(protected bool)
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

// Role interface defines role behavior
type Role interface {
	// GetName returns role name
	GetName() RoleType
	// GetCamp returns role's camp
	GetCamp() Camp
	// GetAvailableSkills returns available skills
	GetAvailableSkills() []Skill
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

type PutOption struct {
	Content string
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
	// Put 使用技能
	Put(caster Player, target Player, option PutOption)
	// Reset 重置技能状态
	Reset()
}
