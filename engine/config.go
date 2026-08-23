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
	// 那个猜测恰好成立（夜→昼→夜），别的规则里就不一定——任务制那一套每提名
	// 一次就绕一圈，于是引擎的「回合」成了提名计数器，与那套规则自己说的
	// 「第几轮任务」最多差五倍，还被 PlayerView.Round 原样发给玩家。
	//
	// 一局游戏的「一回合」是什么，只有规则知道。内核不再猜，改成读这个字段：
	// 声明在哪个阶段上，就是「这个阶段结算完，这一回合结束」。
	//
	// Validate 会检查至少有一个阶段声明了它——一个都没有的话回合数永远
	// 停在 1、回合状态永不重置，那是个必然出错的配置，应当在建局时就被
	// 拒绝，而不是跑到半局才让人发现。
	//
	// 它**只管计数**。回合级变量什么时候清空是另一件事，
	// 由下面的 ClearsRoundVars 声明——两者常常标在相邻的两个阶段上，
	// 但它们是两件事。
	EndsRound bool

	// ClearsRoundVars **进入**这个阶段之前，回合级变量全部清空。
	//
	// 读法是「这个阶段从干净的局面开始」——与 EndsRound 相反，它说的是
	// 自己开始时的样子，不是自己结束时的效果。
	//
	// 「回合数」与「变量寿命」此前焊在一起：EndsRound 一次做两件事。
	// 狼人杀里它们恰好重合（夜间标记活到下一个夜晚，而那正是一回合），
	// 所以一直看不出问题。任务制那一套里不重合：
	//
	//	队伍标记活到「下一次提名开始」   一轮任务里可能提名五次
	//	回合数跟着「第几轮任务」走       否则报给玩家的数没有意义
	//
	// 于是任务制那一套只能在提名解析器里手工清一遍——那是内核少给了一档寿命，
	// 规则替它补。现在两件事各自声明，每个阶段只说关于自己的事。
	//
	// Validate 会检查至少有一个阶段声明了它：一个都没有的话回合级变量
	// 永不清空，狼人杀里女巫用掉的那瓶解药会一夜又一夜救同一个人。
	ClearsRoundVars bool
}

// PhaseStep 阶段步骤。步骤的先后由切片顺序决定。
type PhaseStep struct {
	// Role 哪个角色。RoleUnspecified 表示「所有角色」，
	// RoleSystem 表示「这一步没有玩家承担」。
	Role RoleType

	// Skill 这一步要提交什么技能。
	//
	// **留空表示「这个角色该醒了，但他没有行动」**——只接收信息，不提交
	// 任何东西。一夜狼人的爪牙睁眼看谁是狼、守夜人互认、失眠者看自己的牌
	// 都是这一类：没有目标，没有状态变更，只是「轮到你知道一件事了」。
	//
	// 它与 RoleSystem 是一对镜像：那个是「这一步没有玩家」，这个是
	// 「这一步有玩家，但他不行动」。两个加起来，「阶段里的一步」这四种
	// 组合才齐全。
	//
	// 留空的步骤：不出现在 AllowedSkills 里（他没有可提交的东西）、
	// 不进入就绪判定（没有东西可满足），但**出现在 PhaseInfo.ActiveRoles
	// 里**——主持人得知道该叫醒谁，那正是这种步骤存在的全部理由。
	//
	// 此前表达不了这件事，规则包只能挂一个 SkillSkip 当占位，而 SKIP 的
	// 意思是「主动放弃行动」——他不是放弃，他本来就没有行动可放弃。
	Skill SkillType

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

	// Targets 技能目标。绝大多数技能只有一个，少数一次指定一组。
	//
	// 此前这里是 `TargetID string`——一个目标。那个形状是被样本量为一固定
	// 下来的：狼人杀的九个技能恰好每个都只有一个目标。missions 包的「提名任务
	// 队伍」一次要指定 2-5 个人，只能拆成多次提交，代价是就绪判定说不清
	// 「还差几个人没提」——它只知道队长提交过没有，于是提名了 1 人（需要
	// 2 人）之后就报 Ready=true。那与「AllowedSkills 对没资格的人说他能行动」
	// 是同一类问题：内核对玩家说了不实的话。
	//
	// 单目标的技能写 Targets: []string{"x"}，读用 Target()。
	Targets []string

	// 以下字段由 Engine 在提交时填充，调用方无需设置
	Phase PhaseType
	Round int
}

// Target 单目标技能的那一个目标，没有则为空串。
//
// 绝大多数技能只有一个目标，这个方法让它们不必每次都写 Targets[0] 并自己判空。
// 多目标的技能直接读 Targets。
func (u *SkillUse) Target() string {
	if u == nil || len(u.Targets) == 0 {
		return ""
	}
	return u.Targets[0]
}

// Validate 检查配置自身是否自洽。
//
// 阶段流转图是使用者可以替换的数据，一旦出现悬空的 NextPhase，
// 引擎会在推进到那里时静默地把游戏判为结束——这类问题必须在构造时暴露，
// 而不是等到第三回合突然收场。
//
// 这里只校验配置的形状。两类问题校验不到，各有归处：
//   - 「每个阶段是否都有 Resolver」依赖运行期注册，由 Engine.Start 校验；
//   - 绕道带来的动态流转（Resolver 产出 NewDetourEffect 指向的
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
	// 回合边界只对**会转圈**的阶段图是必需的，见 loops()。
	if c.loops() {
		if !c.hasRoundBoundary() {
			return WrapError(CodeInvalidConfig,
				"no phase declares EndsRound: the round would never advance")
		}
		if !c.hasVarReset() {
			return WrapError(CodeInvalidConfig,
				"no phase declares ClearsRoundVars: round-scoped state would never reset")
		}
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

// clearsRoundVars 进入这个阶段之前要不要清空回合级变量。
func (c *Config) clearsRoundVars(phase PhaseType) bool {
	pc := c.Phases[phase]
	return pc != nil && pc.ClearsRoundVars
}

// loops 阶段图会不会转圈：沿默认出口走下去，能不能回到走过的阶段。
//
// 这道判断存在的理由是**一夜狼人撞出来的**：那一套规则整局只有一个夜晚、
// 一次讨论、一次投票，走到 VOTE 就结束——阶段图是一条**直线**，回合数
// 从头到尾是 1，而那恰恰是对的。
//
// 而 hasRoundBoundary / hasVarReset 那两道检查此前是无条件的，于是内核为了
// 防一类配置错误，逼一个正确的配置去撒谎：只好把 EndsRound 挂在 VOTE 上，
// 虽然它之后没有下一个回合。配置因此在骗读代码的人。
//
// 现在的口径是：**转圈的图才需要回合边界**。理由是那两道检查真正防的是
// 「回合级状态永远不清」——而不转圈的图里每个阶段只经过一次，第二个回合
// 根本不存在，风险也就不存在。
//
// 只沿 NextPhase 走。GOTO_PHASE 与绕道队列都能在运行时把流转拐到别处，
// 但那是规则的运行期决定，静态配置里看不见——这里判的是**声明出来的**
// 阶段图，与 Validate 其余各条同一个口径。
func (c *Config) loops() bool {
	// 不转圈的走法**至多**经过每个阶段一次，因此 len(Phases) 步之内一定
	// 走到 END（或者走进一个配置里没有的阶段，那由别的检查报出来）。
	// 走满了还没到头，只可能是在转圈。
	//
	// 这么写而不是拿一张 seen 表记走过哪些：两者答案完全一样，而步数封顶
	// 同时保证了**这个函数一定会停**。Validate 是建局路径上的第一道关，
	// 它自己绝不能因为一份写坏的配置而挂住。
	phase := c.StartPhase
	for i := 0; i <= len(c.Phases); i++ {
		if phase == PhaseUnspecified || phase == PhaseEnd {
			return false
		}
		pc, ok := c.Phases[phase]
		if !ok || pc == nil {
			return false // 图断了，别的检查会报出来
		}
		phase = pc.NextPhase
	}
	return true
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

// hasVarReset 配置里有没有阶段声明自己从干净的局面开始。
//
// 一个都没有的话回合级变量永不清空——狼人杀里女巫用掉的那瓶解药会一夜又
// 一夜救同一个人，一次性道具变成永久道具。与 hasRoundBoundary 一样，
// 这是把决定权交给规则之后**换来的**：内核自己焊死的时候没法检查。
func (c *Config) hasVarReset() bool {
	for _, pc := range c.Phases {
		if pc != nil && pc.ClearsRoundVars {
			return true
		}
	}
	return false
}
