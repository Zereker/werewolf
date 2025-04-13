package game

// Camp 阵营
type Camp int

const (
	CampGood Camp = iota
	CampBad
)

type Phase string

const (
	PhaseNight Phase = "night" // 夜晚
	PhaseDay   Phase = "day"   // 白天发言阶段
	PhaseVote  Phase = "vote"  // 投票阶段
)

// Player 玩家接口
type Player interface {
	GetRole() Role
	GetCamp() Camp

	IsAlive() bool
	SetAlive(alive bool)
	IsProtected() bool
	SetProtected(protected bool)

	AddSkill(skill Skill)
	UseSkill(phase Phase, target Player, skill Skill) error
}

// RoleType 角色类型
type RoleType string

const (
	RoleTypeWerewolf RoleType = "werewolf" // 狼人
	RoleTypeSeer     RoleType = "seer"     // 预言家
	RoleTypeWitch    RoleType = "witch"    // 女巫
	RoleTypeHunter   RoleType = "hunter"   // 猎人
	RoleTypeVillager RoleType = "villager" // 村民
	RoleTypeGuard    RoleType = "guard"    // 守卫
)

// Role 角色接口
type Role interface {
	// GetName 获取技能名称
	GetName() string
	// GetCamp 获取角色所属阵营
	GetCamp() Camp
	// GetAvailableSkills 获取角色可用的技能类型
	GetAvailableSkills() []string
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
	Put(currentPhase Phase, caster Player, target Player) error
	// Reset 重置技能状态
	Reset()
}
