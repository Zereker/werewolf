// board.go 狼人杀的默认板子：九个阶段怎么排、每个阶段谁行动。
//
// 这些是**数据**，不是引擎逻辑——内核只认 PhaseConfig 这个形状，不知道
// 「NIGHT_WITCH 之后是 NIGHT_SEER」。换一套规则整个换掉即可。
//
// VictoryMode 也在这里：屠边与屠城是狼人杀的两种打法，内核只知道
// 「有人可能赢」，不知道怎么算赢。

package werewolf

import (
	"time"

	"github.com/Zereker/werewolf/engine"
)

// 各阶段的建议超时。板子数据，不是引擎逻辑——引擎不据此计时，
// 什么时候调 EndPhase 完全由调用方决定。
//
// 内核只留一个兜底值（DefaultPhaseTimeout）：「白天要 60 秒」是狼人杀
// 的说法，「狼人阶段要 30 秒因为要协商」更是。
const (
	DayPhaseTimeout   = 60 * time.Second // 白天阶段：发言时间较长
	VotePhaseTimeout  = 30 * time.Second // 投票阶段
	NightPhaseTimeout = 15 * time.Second // 夜晚子阶段
	WolfPhaseTimeout  = 30 * time.Second // 狼人阶段：需要协商

	// HunterPhaseTimeout 猎人开枪阶段。
	// 与夜晚子阶段同为 15 秒，但白天的开枪阶段也用它——开枪是一个
	// 即时决定，不需要白天那 60 秒的发言时间，与「白天」无关。
	HunterPhaseTimeout = 15 * time.Second
)

// hunterShootGroup 猎人「开枪 / 不开枪」这一对互斥备选的组名。
const hunterShootGroup = "hunter-shoot"

// VictoryMode 胜负判定方式
// 底层是字符串，与其余枚举一致。此前是 int + iota——那意味着零值恰好是
// 屠边，而「没填」与「选了屠边」在结构体里长得一模一样。现在零值是空串，
// Validate 会把它拦下来。
type VictoryMode string

const (
	// VictoryModeSideWipe 屠边（默认）：狼人需淘汰「所有平民」或「所有神职」之一。
	// 依据维基「狼人殺」条目：「狼人陣營需要淘汰所有平民或神職人員以獲取勝利」。
	VictoryModeSideWipe VictoryMode = "SIDE_WIPE"

	// VictoryModeTownWipe 屠城：好人存活数 <= 狼人存活数即狼人胜利。
	// 不区分神职与平民，适合无神职或角色板子简单的场合。
	VictoryModeTownWipe VictoryMode = "TOWN_WIPE"
)

// String 实现 fmt.Stringer。
func (m VictoryMode) String() string {
	if m == "" {
		return "UNSPECIFIED"
	}
	return string(m)
}

// DefaultGameConfig 默认游戏配置
func DefaultGameConfig() *GameConfig {
	return &GameConfig{
		StartPhase:     PhaseNightGuard,
		DefaultTimeout: engine.DefaultPhaseTimeout,
		Phases: map[PhaseType]*PhaseConfig{
			// 白天和投票阶段
			PhaseDay:       StandardDayPhase(),
			PhaseVote:      StandardVotePhase(),
			PhaseDayHunter: DayHunterPhase(),
			// 夜晚子阶段
			PhaseNightGuard:   NightGuardPhase(),
			PhaseNightWolf:    NightWolfPhase(),
			PhaseNightWitch:   NightWitchPhase(),
			PhaseNightSeer:    NightSeerPhase(),
			PhaseNightResolve: NightResolvePhase(),
			PhaseNightHunter:  NightHunterPhase(),
		},
	}
}

// StandardDayPhase 标准白天阶段配置
func StandardDayPhase() *PhaseConfig {
	return &PhaseConfig{
		Type: PhaseDay,
		// 白天只有发言，而发言走 SendMessage 而非技能通道，
		// 因此这里没有玩家技能步骤
		Steps: []PhaseStep{
			{Role: RoleGod, Skill: SkillAnnounce},
		},
		Timeout:   DayPhaseTimeout,
		NextPhase: PhaseVote,
	}
}

// StandardVotePhase 标准投票阶段配置
func StandardVotePhase() *PhaseConfig {
	return &PhaseConfig{
		Type: PhaseVote,
		Steps: []PhaseStep{
			{Role: RoleGod, Skill: SkillAnnounce},
			{Role: engine.RoleUnspecified, Skill: SkillVote, Required: true, Multiple: true},
		},
		Timeout:   VotePhaseTimeout,
		NextPhase: PhaseNightGuard, // 进入下一夜

		// 投票结算完，这一回合（这一昼夜）到此为止。
		//
		// 此前这件事是内核猜出来的——「绕回 StartPhase 就算新回合」，
		// 而狼人杀的 StartPhase 恰好是 NIGHT_GUARD，猜对了。别的规则
		// 就不一定：阿瓦隆每提名一次绕一圈，那个猜测会把「回合」变成
		// 提名计数器。现在由板子自己说。
		//
		// 被投出去的是猎人时，投票之后还有一个 DAY_HUNTER 要结算——
		// 内核会等待结算队列排空再落回合边界，因此这里声明它是安全的。
		EndsRound: true,
	}
}

// DayHunterPhase 白天猎人阶段配置（被投票出局后触发）
func DayHunterPhase() *PhaseConfig {
	return &PhaseConfig{
		Type: PhaseDayHunter,
		Steps: []PhaseStep{
			{Role: RoleGod, Skill: SkillAnnounce},
			// 开枪与不开枪是二选一，用同一个 Group 声明出来
			{Role: RoleHunter, Skill: SkillShoot, Group: hunterShootGroup},
			{Role: RoleHunter, Skill: SkillSkip, Group: hunterShootGroup},
		},
		Timeout:   HunterPhaseTimeout,
		NextPhase: PhaseNightGuard, // 猎人行动后进入下一夜

		// 白天猎人阶段是投票之后的延长赛，它结算完同样是这一回合的终点。
		// 夜间猎人阶段（NightHunterPhase）不声明：那是在天亮之前触发的，
		// 后面还有一整个白天。
		EndsRound: true,
	}
}

// NightGuardPhase 守卫阶段配置
func NightGuardPhase() *PhaseConfig {
	return &PhaseConfig{
		Type: PhaseNightGuard,
		Steps: []PhaseStep{
			{Role: RoleGod, Skill: SkillAnnounce},
			{Role: RoleGuard, Skill: SkillProtect},
		},
		Timeout:   NightPhaseTimeout,
		NextPhase: PhaseNightWolf,
	}
}

// NightWolfPhase 狼人阶段配置
func NightWolfPhase() *PhaseConfig {
	return &PhaseConfig{
		Type: PhaseNightWolf,
		Steps: []PhaseStep{
			{Role: RoleGod, Skill: SkillAnnounce},
			{Role: RoleWerewolf, Skill: SkillKill, Required: true, Multiple: true},
		},
		Timeout:   WolfPhaseTimeout,
		NextPhase: PhaseNightWitch,
	}
}

// NightWitchPhase 女巫阶段配置
func NightWitchPhase() *PhaseConfig {
	return &PhaseConfig{
		Type: PhaseNightWitch,
		Steps: []PhaseStep{
			{Role: RoleGod, Skill: SkillAnnounce},
			{Role: RoleWitch, Skill: SkillAntidote},
			{Role: RoleWitch, Skill: SkillPoison},
		},
		Timeout:   NightPhaseTimeout,
		NextPhase: PhaseNightSeer,
	}
}

// NightSeerPhase 预言家阶段配置
func NightSeerPhase() *PhaseConfig {
	return &PhaseConfig{
		Type: PhaseNightSeer,
		Steps: []PhaseStep{
			{Role: RoleGod, Skill: SkillAnnounce},
			{Role: RoleSeer, Skill: SkillCheck},
		},
		Timeout:   NightPhaseTimeout,
		NextPhase: PhaseNightResolve,
	}
}

// NightResolvePhase 夜晚结算阶段配置（处理击杀、猎人触发等）
func NightResolvePhase() *PhaseConfig {
	return &PhaseConfig{
		Type: PhaseNightResolve,
		Steps: []PhaseStep{
			{Role: RoleGod, Skill: SkillAnnounce},
		},
		Timeout:   NightPhaseTimeout,
		NextPhase: PhaseDay, // 默认进入白天，如有猎人死亡则动态改为猎人阶段
	}
}

// NightHunterPhase 夜晚猎人阶段配置（被动触发）
func NightHunterPhase() *PhaseConfig {
	return &PhaseConfig{
		Type: PhaseNightHunter,
		Steps: []PhaseStep{
			{Role: RoleGod, Skill: SkillAnnounce},
			// 开枪与不开枪是二选一，用同一个 Group 声明出来
			{Role: RoleHunter, Skill: SkillShoot, Group: hunterShootGroup},
			{Role: RoleHunter, Skill: SkillSkip, Group: hunterShootGroup},
		},
		Timeout:   HunterPhaseTimeout,
		NextPhase: PhaseDay,
	}
}

// ==================== 配置校验 ====================
