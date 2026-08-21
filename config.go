package werewolf

import (
	"time"

	pb "github.com/Zereker/werewolf/proto"
)

// 超时常量
//
// 引擎本身不计时——阶段何时结束完全由调用方决定（调用 EndPhase）。
// 这些常量与 PhaseConfig.Timeout 是给调用方参考的建议值，
// 调用方据此设置自己的定时器。
const (
	DefaultPhaseTimeout = 30 * time.Second // 默认阶段超时
	DayPhaseTimeout     = 60 * time.Second // 白天阶段超时（发言时间较长）
	VotePhaseTimeout    = 30 * time.Second // 投票阶段超时
	NightPhaseTimeout   = 15 * time.Second // 夜晚子阶段超时
	WolfPhaseTimeout    = 30 * time.Second // 狼人阶段超时（需要协商）
)

// VictoryMode 胜负判定方式
type VictoryMode int

const (
	// VictoryModeSideWipe 屠边（默认）：狼人需淘汰「所有平民」或「所有神职」之一。
	// 依据维基「狼人殺」条目：「狼人陣營需要淘汰所有平民或神職人員以獲取勝利」。
	VictoryModeSideWipe VictoryMode = iota

	// VictoryModeTownWipe 屠城：好人存活数 <= 狼人存活数即狼人胜利。
	// 不区分神职与平民，适合无神职或角色板子简单的场合。
	VictoryModeTownWipe
)

// GameConfig 游戏配置
type GameConfig struct {
	// 规则变体
	WitchCanSaveSelf       bool // 女巫能否自救
	WitchCanUseBothPotions bool // 女巫能否在同一夜同时使用解药和毒药
	GuardCanProtectSelf    bool // 守卫能否自守
	GuardCanRepeat         bool // 守卫能否连续守同一人
	SameGuardKillIsEmpty   bool // 守卫守住刀口时是否空刀（守护是否生效）
	GuardSaveTogetherDies  bool // 同守同救（守卫守护 + 女巫解药）目标是否依然死亡

	// 胜负判定方式
	VictoryMode VictoryMode

	// 阶段配置
	Phases map[pb.PhaseType]*PhaseConfig

	// 超时配置（建议值，引擎不据此计时，详见超时常量的说明）
	DefaultTimeout time.Duration
}

// PhaseConfig 阶段配置
type PhaseConfig struct {
	Type      pb.PhaseType  // 阶段类型
	Steps     []PhaseStep   // 步骤列表
	Timeout   time.Duration // 超时时间（建议值，引擎不据此计时）
	NextPhase pb.PhaseType  // 下一阶段（声明式配置）
}

// PhaseStep 阶段步骤
//
// 步骤的先后由切片顺序决定。「是否允许多名玩家参与」「未行动如何处理」
// 由各阶段的 Resolver 自行编码（如狼人按票数取共识、守卫只取首个提交）。
type PhaseStep struct {
	Role  pb.RoleType  // 哪个角色
	Skill pb.SkillType // 使用什么技能
}

// SkillUse 技能使用记录
//
// 玩家发言不走技能通道，由 Engine.SendMessage 处理，可见性也在那里按阶段路由。
type SkillUse struct {
	PlayerID string       // 使用技能的玩家
	Skill    pb.SkillType // 技能类型
	TargetID string       // 技能目标（单人）

	// 以下字段由 Engine 在提交时填充，调用方无需设置
	Phase pb.PhaseType
	Round int
}

// DefaultGameConfig 默认游戏配置
func DefaultGameConfig() *GameConfig {
	return &GameConfig{
		WitchCanSaveSelf:       false,
		WitchCanUseBothPotions: false,
		GuardCanProtectSelf:    true,
		GuardCanRepeat:         false,
		SameGuardKillIsEmpty:   true,
		GuardSaveTogetherDies:  true,
		VictoryMode:            VictoryModeSideWipe,
		DefaultTimeout:         DefaultPhaseTimeout,
		Phases: map[pb.PhaseType]*PhaseConfig{
			// 白天和投票阶段
			pb.PhaseType_PHASE_TYPE_DAY:        StandardDayPhase(),
			pb.PhaseType_PHASE_TYPE_VOTE:       StandardVotePhase(),
			pb.PhaseType_PHASE_TYPE_DAY_HUNTER: DayHunterPhase(),
			// 夜晚子阶段
			pb.PhaseType_PHASE_TYPE_NIGHT_GUARD:   NightGuardPhase(),
			pb.PhaseType_PHASE_TYPE_NIGHT_WOLF:    NightWolfPhase(),
			pb.PhaseType_PHASE_TYPE_NIGHT_WITCH:   NightWitchPhase(),
			pb.PhaseType_PHASE_TYPE_NIGHT_SEER:    NightSeerPhase(),
			pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE: NightResolvePhase(),
			pb.PhaseType_PHASE_TYPE_NIGHT_HUNTER:  NightHunterPhase(),
		},
	}
}

// StandardDayPhase 标准白天阶段配置
func StandardDayPhase() *PhaseConfig {
	return &PhaseConfig{
		Type: pb.PhaseType_PHASE_TYPE_DAY,
		Steps: []PhaseStep{
			{Role: pb.RoleType_ROLE_TYPE_GOD, Skill: pb.SkillType_SKILL_TYPE_ANNOUNCE},
			// 白天主要是发言，所有存活玩家
			{Role: pb.RoleType_ROLE_TYPE_UNSPECIFIED, Skill: pb.SkillType_SKILL_TYPE_SPEAK},
		},
		Timeout:   DayPhaseTimeout,
		NextPhase: pb.PhaseType_PHASE_TYPE_VOTE,
	}
}

// StandardVotePhase 标准投票阶段配置
func StandardVotePhase() *PhaseConfig {
	return &PhaseConfig{
		Type: pb.PhaseType_PHASE_TYPE_VOTE,
		Steps: []PhaseStep{
			{Role: pb.RoleType_ROLE_TYPE_GOD, Skill: pb.SkillType_SKILL_TYPE_ANNOUNCE},
			{Role: pb.RoleType_ROLE_TYPE_UNSPECIFIED, Skill: pb.SkillType_SKILL_TYPE_VOTE},
		},
		Timeout:   VotePhaseTimeout,
		NextPhase: pb.PhaseType_PHASE_TYPE_NIGHT_GUARD, // 进入下一夜
	}
}

// DayHunterPhase 白天猎人阶段配置（被投票出局后触发）
func DayHunterPhase() *PhaseConfig {
	return &PhaseConfig{
		Type: pb.PhaseType_PHASE_TYPE_DAY_HUNTER,
		Steps: []PhaseStep{
			{Role: pb.RoleType_ROLE_TYPE_GOD, Skill: pb.SkillType_SKILL_TYPE_ANNOUNCE},
			{Role: pb.RoleType_ROLE_TYPE_HUNTER, Skill: pb.SkillType_SKILL_TYPE_SHOOT},
			{Role: pb.RoleType_ROLE_TYPE_HUNTER, Skill: pb.SkillType_SKILL_TYPE_SKIP},
		},
		Timeout:   NightPhaseTimeout,
		NextPhase: pb.PhaseType_PHASE_TYPE_NIGHT_GUARD, // 猎人行动后进入下一夜
	}
}

// NightGuardPhase 守卫阶段配置
func NightGuardPhase() *PhaseConfig {
	return &PhaseConfig{
		Type: pb.PhaseType_PHASE_TYPE_NIGHT_GUARD,
		Steps: []PhaseStep{
			{Role: pb.RoleType_ROLE_TYPE_GOD, Skill: pb.SkillType_SKILL_TYPE_ANNOUNCE},
			{Role: pb.RoleType_ROLE_TYPE_GUARD, Skill: pb.SkillType_SKILL_TYPE_PROTECT},
		},
		Timeout:   NightPhaseTimeout,
		NextPhase: pb.PhaseType_PHASE_TYPE_NIGHT_WOLF,
	}
}

// NightWolfPhase 狼人阶段配置
func NightWolfPhase() *PhaseConfig {
	return &PhaseConfig{
		Type: pb.PhaseType_PHASE_TYPE_NIGHT_WOLF,
		Steps: []PhaseStep{
			{Role: pb.RoleType_ROLE_TYPE_GOD, Skill: pb.SkillType_SKILL_TYPE_ANNOUNCE},
			{Role: pb.RoleType_ROLE_TYPE_WEREWOLF, Skill: pb.SkillType_SKILL_TYPE_KILL},
		},
		Timeout:   WolfPhaseTimeout,
		NextPhase: pb.PhaseType_PHASE_TYPE_NIGHT_WITCH,
	}
}

// NightWitchPhase 女巫阶段配置
func NightWitchPhase() *PhaseConfig {
	return &PhaseConfig{
		Type: pb.PhaseType_PHASE_TYPE_NIGHT_WITCH,
		Steps: []PhaseStep{
			{Role: pb.RoleType_ROLE_TYPE_GOD, Skill: pb.SkillType_SKILL_TYPE_ANNOUNCE},
			{Role: pb.RoleType_ROLE_TYPE_WITCH, Skill: pb.SkillType_SKILL_TYPE_ANTIDOTE},
			{Role: pb.RoleType_ROLE_TYPE_WITCH, Skill: pb.SkillType_SKILL_TYPE_POISON},
		},
		Timeout:   NightPhaseTimeout,
		NextPhase: pb.PhaseType_PHASE_TYPE_NIGHT_SEER,
	}
}

// NightSeerPhase 预言家阶段配置
func NightSeerPhase() *PhaseConfig {
	return &PhaseConfig{
		Type: pb.PhaseType_PHASE_TYPE_NIGHT_SEER,
		Steps: []PhaseStep{
			{Role: pb.RoleType_ROLE_TYPE_GOD, Skill: pb.SkillType_SKILL_TYPE_ANNOUNCE},
			{Role: pb.RoleType_ROLE_TYPE_SEER, Skill: pb.SkillType_SKILL_TYPE_CHECK},
		},
		Timeout:   NightPhaseTimeout,
		NextPhase: pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE,
	}
}

// NightResolvePhase 夜晚结算阶段配置（处理击杀、猎人触发等）
func NightResolvePhase() *PhaseConfig {
	return &PhaseConfig{
		Type: pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE,
		Steps: []PhaseStep{
			{Role: pb.RoleType_ROLE_TYPE_GOD, Skill: pb.SkillType_SKILL_TYPE_ANNOUNCE},
		},
		Timeout:   NightPhaseTimeout,
		NextPhase: pb.PhaseType_PHASE_TYPE_DAY, // 默认进入白天，如有猎人死亡则动态改为猎人阶段
	}
}

// NightHunterPhase 夜晚猎人阶段配置（被动触发）
func NightHunterPhase() *PhaseConfig {
	return &PhaseConfig{
		Type: pb.PhaseType_PHASE_TYPE_NIGHT_HUNTER,
		Steps: []PhaseStep{
			{Role: pb.RoleType_ROLE_TYPE_GOD, Skill: pb.SkillType_SKILL_TYPE_ANNOUNCE},
			{Role: pb.RoleType_ROLE_TYPE_HUNTER, Skill: pb.SkillType_SKILL_TYPE_SHOOT},
			{Role: pb.RoleType_ROLE_TYPE_HUNTER, Skill: pb.SkillType_SKILL_TYPE_SKIP},
		},
		Timeout:   NightPhaseTimeout,
		NextPhase: pb.PhaseType_PHASE_TYPE_DAY,
	}
}
