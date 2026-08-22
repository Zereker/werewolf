package engine

import (
	"time"
)

// 超时常量
//
// 引擎本身不计时——阶段何时结束完全由调用方决定（调用 EndPhase）。
// 这些常量与 PhaseConfig.Timeout 是给调用方参考的建议值，
// 调用方据此设置自己的定时器。
const (
	// DefaultPhaseTimeout 未给出 PhaseConfig.Timeout 时的兜底建议值。
	//
	// 这是给调用方参考的建议值，引擎不据此计时——什么时候调 EndPhase
	// 完全由调用方决定。各阶段的建议值是板子数据，见规则包。
	DefaultPhaseTimeout = 30 * time.Second
)

// Config 阶段机的配置：从哪儿开始、阶段怎么流转、建议多久。
//
// 只有阶段机需要的三样东西。规则开关（狼人杀的「女巫能否自救」之类）
// 不在这里——内核不该认得那些概念，它们住在规则包自己的结构体上。
//
// 名字曾经是 GameConfig。改掉是因为它名不副实：它配的是阶段机，
// 不是「一局游戏」。
type Config struct {
	// 起始阶段。Start 之后进入的第一个阶段，为空时默认 NIGHT_GUARD。
	StartPhase PhaseType

	// 阶段配置
	Phases map[PhaseType]*PhaseConfig

	// DefaultTimeout 未给出 PhaseConfig.Timeout 时的建议超时。
	// 建议值，引擎不据此计时，详见超时常量的说明；
	// 用 Config.PhaseTimeout(phase) 取某个阶段的最终建议值。
	DefaultTimeout time.Duration
}

// PhaseTimeout 某个阶段的建议超时。
//
// 阶段自己没配就用 DefaultTimeout，DefaultTimeout 也没配就用
// DefaultPhaseTimeout。这两个字段此前只写不读——调用方要拿引擎实际
// 在用的建议值，只能自己再造一份配置比对。
func (c *Config) PhaseTimeout(phase PhaseType) time.Duration {
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
	Type      PhaseType     // 阶段类型
	Steps     []PhaseStep   // 步骤列表
	Timeout   time.Duration // 超时时间（建议值，引擎不据此计时）
	NextPhase PhaseType     // 下一阶段（默认出口，可被 GOTO_PHASE 效果改写）

	// EndsRound 结算完这个阶段之后是新的一回合：回合数加一，
	// 回合级的状态（RoundVar 与 PlayerRoundVar）全部清空。
	//
	// 这件事此前由内核自己猜：「绕回 StartPhase 就算新回合」。狼人杀里
	// 那个猜测恰好成立（夜→昼→夜），别的规则里就不一定——阿瓦隆每提名
	// 一次就绕一圈，于是引擎的「回合」成了提名计数器，与那套规则自己说的
	// 「第几轮任务」最多差五倍，还被 PlayerView.Round 原样发给玩家。
	//
	// 一局游戏的「一回合」是什么，只有规则知道。内核不再猜，改成读这个字段：
	// 声明在哪个阶段上，就是「这个阶段结算完，这一回合结束」。
	//
	// Validate 会检查至少有一个阶段声明了它——一个都没有的话回合数永远
	// 停在 1、回合状态永不重置，那是个必然出错的配置，应当在建局时就被
	// 拒绝，而不是跑到半局才让人发现。
	EndsRound bool
}

// PhaseStep 阶段步骤。步骤的先后由切片顺序决定。
type PhaseStep struct {
	Role  RoleType  // 哪个角色
	Skill SkillType // 使用什么技能

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

	// AllowDeadTarget 这个技能能否指向已出局的玩家。
	//
	// 默认不能——指着一具尸体用技能几乎总是提交错了。女巫的解药是例外：
	// 她要救的正是今晚已经被判死的那个人。
	//
	// 此前这条例外按技能名字硬判写在内核的校验里，
	// 也就是内核认得「解药」。现在它是规则声明出来的数据。
	AllowDeadTarget bool
}

// SkillUse 技能使用记录
//
// 玩家发言不走技能通道，由 Engine.SendMessage 处理，可见性也在那里按阶段路由。
type SkillUse struct {
	PlayerID string    // 使用技能的玩家
	Skill    SkillType // 技能类型
	TargetID string    // 技能目标（单人）

	// 以下字段由 Engine 在提交时填充，调用方无需设置
	Phase PhaseType
	Round int
}

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
func (c *Config) Validate() error {
	if c == nil {
		return WrapError(CodeInvalidConfig, "config must not be nil")
	}
	if len(c.Phases) == 0 {
		return WrapError(CodeInvalidConfig, "config contains no phases")
	}

	// 起始阶段必须显式给出：内核没有默认板子，也就没有默认的第一个阶段。
	// 此前留空会退回 NIGHT_GUARD——那是狼人杀的第一个阶段。
	if c.StartPhase == PhaseUnspecified {
		return WrapError(CodeInvalidConfig,
			"config must declare StartPhase: the kernel has no default")
	}
	if !c.hasRoundBoundary() {
		return WrapError(CodeInvalidConfig,
			"no phase declares EndsRound: the round would never advance and "+
				"round-scoped state would never reset")
	}
	if _, ok := c.Phases[c.StartPhase]; !ok {
		return WrapError(CodeInvalidPhase,
			"start phase %v is not present in config", c.StartPhase)
	}

	for phaseType, pc := range c.Phases {
		if pc == nil {
			return WrapError(CodeInvalidPhase,
				"phase %v has a nil config", phaseType)
		}
		if pc.Type != phaseType {
			return WrapError(CodeInvalidPhase,
				"phase %v is registered under key %v", pc.Type, phaseType)
		}

		// 悬空的 NextPhase 会让游戏走到一半无声结束。
		//
		// UNSPECIFIED 一并拒绝：想表达「到此结束」有 PhaseEnd，
		// 留空只可能是漏填，而漏填的后果与悬空完全一样。
		if pc.NextPhase == PhaseUnspecified {
			return WrapError(CodeInvalidPhase,
				"phase %v has no NextPhase (use PhaseEnd to end the game)", phaseType)
		}
		if pc.NextPhase != PhaseEnd {
			if _, ok := c.Phases[pc.NextPhase]; !ok {
				return WrapError(CodeInvalidPhase,
					"phase %v points to %v which is not present in config", phaseType, pc.NextPhase)
			}
		}

		if err := validateSteps(phaseType, pc.Steps); err != nil {
			return err
		}
	}

	return nil
}

// validateSteps 校验一个阶段的步骤声明。
//
// 重复声明会让 AllowedSkills 返回重复项、PhaseReadiness 重复计数
// 「还差谁行动」。RoleUnspecified 表示「所有角色」，因此它与任何
// 具体角色声明同一个技能都构成重复——键相同的那半只是同一个问题里
// 比较显眼的一半。
func validateSteps(phaseType PhaseType, steps []PhaseStep) error {
	type stepKey struct {
		role  RoleType
		skill SkillType
	}

	seen := make(map[stepKey]bool, len(steps))
	allRoles := make(map[SkillType]bool, len(steps))
	groupRole := make(map[string]RoleType, len(steps))
	for _, step := range steps {
		// 互斥备选组是「同一个人几选一」，跨角色的组没有意义：
		// 就绪判定会逐个行动者去看他提交了组里的哪一个，
		// 而预言家永远不会提交女巫的技能。
		if step.Group != "" {
			if role, ok := groupRole[step.Group]; ok && role != step.Role {
				return WrapError(CodeInvalidPhase,
					"phase %v group %q spans roles %v and %v",
					phaseType, step.Group, role, step.Role)
			}
			groupRole[step.Group] = step.Role
		}

		key := stepKey{role: step.Role, skill: step.Skill}
		if seen[key] {
			return WrapError(CodeInvalidPhase,
				"phase %v declares %v/%v twice", phaseType, step.Role, step.Skill)
		}
		seen[key] = true
		if step.Role == RoleUnspecified {
			allRoles[step.Skill] = true
		}
	}

	for _, step := range steps {
		if step.Role != RoleUnspecified && allRoles[step.Skill] {
			return WrapError(CodeInvalidPhase,
				"phase %v declares %v for all roles and for %v separately",
				phaseType, step.Skill, step.Role)
		}
	}

	return nil
}

// startPhase 返回起始阶段。
//
// 一定有值：Validate 强制配置给出它。此前留空会退回 NIGHT_GUARD——
// 那是狼人杀的第一个阶段，内核没有资格替任何规则挑一个默认值。
func (c *Config) startPhase() PhaseType {
	return c.StartPhase
}

// endsRound 结算完这个阶段之后是不是新的一回合。
//
// 认不得的阶段一律为否：回合边界宁可不推进，也不能凭空多推一次——
// 多推一次会把本回合的标记全部清空，规则会看到一个从未发生过的
// 「新回合」。
func (c *Config) endsRound(phase PhaseType) bool {
	pc := c.Phases[phase]
	return pc != nil && pc.EndsRound
}

// hasRoundBoundary 配置里有没有阶段声明自己是回合的终点。
//
// 一个都没有的话，回合数永远停在 1、回合级状态永不重置——狼人杀里这意味着
// 女巫用掉的那瓶解药会一夜又一夜地把同一个人救回来，一次性道具变成永久道具。
// 这类配置必然出错，应当在建局时就被拒绝，而不是跑到半局才让人发现。
//
// 这道检查是把回合边界交给规则之后**换来的**：内核自己猜的时候没法检查
// 「猜得对不对」，规则声明出来之后反而查得动。
func (c *Config) hasRoundBoundary() bool {
	for _, pc := range c.Phases {
		if pc != nil && pc.EndsRound {
			return true
		}
	}
	return false
}
