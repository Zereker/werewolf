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

	// HunterPhaseTimeout 猎人开枪阶段超时。
	// 与夜晚子阶段同为 15 秒，但白天的开枪阶段也用它——开枪是一个
	// 即时决定，不需要白天那 60 秒的发言时间，与「白天」无关。
	HunterPhaseTimeout = 15 * time.Second
)

// hunterShootGroup 猎人「开枪 / 不开枪」这一对互斥备选的组名。
const hunterShootGroup = "hunter-shoot"

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

// String 实现 fmt.Stringer，让日志与错误信息里出现的是名字而不是 0/1。
func (m VictoryMode) String() string {
	switch m {
	case VictoryModeSideWipe:
		return "SIDE_WIPE"
	case VictoryModeTownWipe:
		return "TOWN_WIPE"
	default:
		return "UNKNOWN"
	}
}

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

	// 起始阶段。Start 之后进入的第一个阶段，为空时默认 NIGHT_GUARD。
	StartPhase pb.PhaseType

	// 阶段配置
	Phases map[pb.PhaseType]*PhaseConfig

	// DefaultTimeout 未给出 PhaseConfig.Timeout 时的建议超时。
	// 建议值，引擎不据此计时，详见超时常量的说明；
	// 用 GameConfig.PhaseTimeout(phase) 取某个阶段的最终建议值。
	DefaultTimeout time.Duration
}

// PhaseTimeout 某个阶段的建议超时。
//
// 阶段自己没配就用 DefaultTimeout，DefaultTimeout 也没配就用
// DefaultPhaseTimeout。这两个字段此前只写不读——调用方要拿引擎实际
// 在用的建议值，只能自己再造一份配置比对。
func (c *GameConfig) PhaseTimeout(phase pb.PhaseType) time.Duration {
	if pc := c.Phases[phase]; pc != nil && pc.Timeout > 0 {
		return pc.Timeout
	}
	if c.DefaultTimeout > 0 {
		return c.DefaultTimeout
	}
	return DefaultPhaseTimeout
}

// PhaseConfig 阶段配置
type PhaseConfig struct {
	Type      pb.PhaseType  // 阶段类型
	Steps     []PhaseStep   // 步骤列表
	Timeout   time.Duration // 超时时间（建议值，引擎不据此计时）
	NextPhase pb.PhaseType  // 下一阶段（声明式配置）
}

// PhaseStep 阶段步骤。步骤的先后由切片顺序决定。
type PhaseStep struct {
	Role  pb.RoleType  // 哪个角色
	Skill pb.SkillType // 使用什么技能

	// Required 该步骤是否必须完成，阶段才算就绪。
	//
	// 引擎不计时、也不会因此拒绝 EndPhase——它只据此回答
	// Engine.PhaseReadiness()「还差谁没行动」，让调用方决定是继续等待
	// 还是按超时推进。没有任何合格行动者时（例如守卫已出局），
	// 该步骤视为自动满足。
	Required bool

	// Multiple 是否要求全部合格行动者都行动。
	//
	// true：所有人都提交了才算完成（狼人商刀、全员投票）
	// false：任意一人提交即算完成
	// 仅影响就绪判断；重复提交如何取舍由各阶段的 Resolver 决定。
	Multiple bool

	// Group 互斥备选组。同一阶段内 Group 相同且非空的若干步骤是
	// 「几选一」的关系，行动者提交了其中任意一个，整组即算完成。
	//
	// 猎人的「开枪」与「不开枪」就是这样一对：没有这个字段，逐步骤
	// 独立判定会认为提交了 SKIP 的猎人仍欠着 SHOOT，一旦按文档字面
	// 把两步都标成 Required，这个阶段就永远不会就绪。
	//
	// 只影响就绪判断，不影响技能校验：一个阶段允许哪些技能仍由
	// 全部步骤共同决定。
	Group string
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
		StartPhase:             pb.PhaseType_PHASE_TYPE_NIGHT_GUARD,
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
		// 白天只有发言，而发言走 SendMessage 而非技能通道，
		// 因此这里没有玩家技能步骤
		Steps: []PhaseStep{
			{Role: pb.RoleType_ROLE_TYPE_GOD, Skill: pb.SkillType_SKILL_TYPE_ANNOUNCE},
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
			{Role: pb.RoleType_ROLE_TYPE_UNSPECIFIED, Skill: pb.SkillType_SKILL_TYPE_VOTE, Required: true, Multiple: true},
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
			// 开枪与不开枪是二选一，用同一个 Group 声明出来
			{Role: pb.RoleType_ROLE_TYPE_HUNTER, Skill: pb.SkillType_SKILL_TYPE_SHOOT, Group: hunterShootGroup},
			{Role: pb.RoleType_ROLE_TYPE_HUNTER, Skill: pb.SkillType_SKILL_TYPE_SKIP, Group: hunterShootGroup},
		},
		Timeout:   HunterPhaseTimeout,
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
			{Role: pb.RoleType_ROLE_TYPE_WEREWOLF, Skill: pb.SkillType_SKILL_TYPE_KILL, Required: true, Multiple: true},
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
			// 开枪与不开枪是二选一，用同一个 Group 声明出来
			{Role: pb.RoleType_ROLE_TYPE_HUNTER, Skill: pb.SkillType_SKILL_TYPE_SHOOT, Group: hunterShootGroup},
			{Role: pb.RoleType_ROLE_TYPE_HUNTER, Skill: pb.SkillType_SKILL_TYPE_SKIP, Group: hunterShootGroup},
		},
		Timeout:   HunterPhaseTimeout,
		NextPhase: pb.PhaseType_PHASE_TYPE_DAY,
	}
}

// ==================== 配置校验 ====================

// Validate 检查配置自身是否自洽。
//
// 阶段流转图是使用者可以替换的数据，一旦出现悬空的 NextPhase，
// 引擎会在推进到那里时静默地把游戏判为结束——这类问题必须在构造时暴露，
// 而不是等到第三回合突然收场。
//
// 这里只校验配置的形状。两类问题校验不到，各有归处：
//   - 「每个阶段是否都有 Resolver」依赖运行期注册，由 Engine.Start 校验；
//   - 死亡技能的动态流转（Resolver 产出 NewAbilityTriggerEffect 指向的
//     阶段）是运行期才知道的边，由引擎在入队前检查——目标阶段不在配置里
//     的触发会被就地否决并记 Error 日志，而不是把游戏带进一个空阶段。
func (c *GameConfig) Validate() error {
	if c == nil {
		return WrapError(pb.ErrorCode_ERROR_CODE_INVALID_CONFIG, "config must not be nil")
	}
	if len(c.Phases) == 0 {
		return WrapError(pb.ErrorCode_ERROR_CODE_INVALID_CONFIG, "config contains no phases")
	}

	start := c.startPhase()
	if _, ok := c.Phases[start]; !ok {
		return WrapError(pb.ErrorCode_ERROR_CODE_INVALID_PHASE,
			"start phase %v is not present in config", start)
	}

	for phaseType, pc := range c.Phases {
		if pc == nil {
			return WrapError(pb.ErrorCode_ERROR_CODE_INVALID_PHASE,
				"phase %v has a nil config", phaseType)
		}
		if pc.Type != phaseType {
			return WrapError(pb.ErrorCode_ERROR_CODE_INVALID_PHASE,
				"phase %v is registered under key %v", pc.Type, phaseType)
		}

		// 悬空的 NextPhase 会让游戏走到一半无声结束。
		//
		// UNSPECIFIED 一并拒绝：想表达「到此结束」有 PHASE_TYPE_END，
		// 留空只可能是漏填，而漏填的后果与悬空完全一样。
		if pc.NextPhase == pb.PhaseType_PHASE_TYPE_UNSPECIFIED {
			return WrapError(pb.ErrorCode_ERROR_CODE_INVALID_PHASE,
				"phase %v has no NextPhase (use PHASE_TYPE_END to end the game)", phaseType)
		}
		if pc.NextPhase != pb.PhaseType_PHASE_TYPE_END {
			if _, ok := c.Phases[pc.NextPhase]; !ok {
				return WrapError(pb.ErrorCode_ERROR_CODE_INVALID_PHASE,
					"phase %v points to %v which is not present in config", phaseType, pc.NextPhase)
			}
		}

		if err := validateSteps(phaseType, pc.Steps); err != nil {
			return err
		}
	}

	if c.VictoryMode < VictoryModeSideWipe || c.VictoryMode > VictoryModeTownWipe {
		return WrapError(pb.ErrorCode_ERROR_CODE_INVALID_CONFIG,
			"unknown victory mode %d", int(c.VictoryMode))
	}

	return nil
}

// validateSteps 校验一个阶段的步骤声明。
//
// 重复声明会让 AllowedSkills 返回重复项、PhaseReadiness 重复计数
// 「还差谁行动」。ROLE_TYPE_UNSPECIFIED 表示「所有角色」，因此它与任何
// 具体角色声明同一个技能都构成重复——键相同的那半只是同一个问题里
// 比较显眼的一半。
func validateSteps(phaseType pb.PhaseType, steps []PhaseStep) error {
	type stepKey struct {
		role  pb.RoleType
		skill pb.SkillType
	}

	seen := make(map[stepKey]bool, len(steps))
	allRoles := make(map[pb.SkillType]bool, len(steps))
	groupRole := make(map[string]pb.RoleType, len(steps))
	for _, step := range steps {
		// 互斥备选组是「同一个人几选一」，跨角色的组没有意义：
		// 就绪判定会逐个行动者去看他提交了组里的哪一个，
		// 而预言家永远不会提交女巫的技能。
		if step.Group != "" {
			if role, ok := groupRole[step.Group]; ok && role != step.Role {
				return WrapError(pb.ErrorCode_ERROR_CODE_INVALID_PHASE,
					"phase %v group %q spans roles %v and %v",
					phaseType, step.Group, role, step.Role)
			}
			groupRole[step.Group] = step.Role
		}

		key := stepKey{role: step.Role, skill: step.Skill}
		if seen[key] {
			return WrapError(pb.ErrorCode_ERROR_CODE_INVALID_PHASE,
				"phase %v declares %v/%v twice", phaseType, step.Role, step.Skill)
		}
		seen[key] = true
		if step.Role == pb.RoleType_ROLE_TYPE_UNSPECIFIED {
			allRoles[step.Skill] = true
		}
	}

	for _, step := range steps {
		if step.Role != pb.RoleType_ROLE_TYPE_UNSPECIFIED && allRoles[step.Skill] {
			return WrapError(pb.ErrorCode_ERROR_CODE_INVALID_PHASE,
				"phase %v declares %v for all roles and for %v separately",
				phaseType, step.Skill, step.Role)
		}
	}

	return nil
}

// startPhase 返回起始阶段，未配置时用默认值
func (c *GameConfig) startPhase() pb.PhaseType {
	if c.StartPhase == pb.PhaseType_PHASE_TYPE_UNSPECIFIED {
		return pb.PhaseType_PHASE_TYPE_NIGHT_GUARD
	}
	return c.StartPhase
}
