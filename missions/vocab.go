// vocab.go 任务制社会推理的词汇表：四个阶段、八个角色、六个技能、九个事件。
//
// 与根包的 vocab.go 是同一件事的另一份填法——内核只有类型，取值全在
// 规则包里。这个文件的存在本身就是「内核不知道自己在跑哪个游戏」的证据：
// 它和狼人杀那份没有一个取值相同，而内核一行都不用改。
//
// # 为什么包名叫 missions
//
// 本包实现的是 The Resistance 与它的 Avalon 变体那套玩法：五轮任务，
// 每轮先提名队伍、再全员表决、通过了才由队员秘密投成败。这两个名字都是
// 商标。玩法规则本身一般不受著作权保护，受保护的是名称、美术与具体文字
// 表述，所以包名取玩法的核心结构（missions，任务制）而不是商标本身，
// 与出版方无关。
//
// 角色名保留：梅林、派西维尔、莫德雷德、莫甘娜、奥伯伦都是亚瑟王传说里
// 的人物，属于公有领域，不是谁的商标。
//
// # 规则来源
//
// 以英文维基百科 The Resistance (game) 条目为基准：
// https://en.wikipedia.org/wiki/The_Resistance_(game)
//
// 中文条目「抵抗组织」在梅林那一条上有误——它写「知道邪恶阵营的玩家都有谁，
// 但不知道谁是谁」，那半句是从派西维尔那条串过来的。梅林**能**逐个认出坏人
// 是谁，这是整个游戏成立的前提：正因为他知道得太准，一开口就会暴露，
// 才需要藏。照中文条目实现，梅林会退化成「知道场上有几个坏人」，
// 派西维尔与刺客的整套机制随之失去意义。本包从英文条目。
package missions

import "github.com/Zereker/hiddenrole"

// 四个阶段。
//
// 一轮任务要走三个阶段：队长提名 -> 全员表决 -> 上任务的人各投成败。
// 表决没通过就绕回提名，换下一个队长——阶段环因此是三个节点的循环，
// 第四个（刺杀）只在好人凑满三次成功时才进得去。
const (
	PhasePropose  hiddenrole.PhaseType = "PROPOSE"   // 队长提名任务队伍
	PhaseTeamVote hiddenrole.PhaseType = "TEAM_VOTE" // 全员表决是否接受这支队伍
	PhaseMission  hiddenrole.PhaseType = "MISSION"   // 队伍成员各投成功/失败
	PhaseAssassin hiddenrole.PhaseType = "ASSASSIN"  // 刺客指认梅林
)

// 八个角色：三好五坏里的「坏」是可选的，最小一局只需要忠臣与爪牙。
const (
	// 好人
	RoleLoyalServant hiddenrole.RoleType = "LOYAL_SERVANT" // 亚瑟的忠臣，无特殊能力
	RoleMerlin       hiddenrole.RoleType = "MERLIN"        // 认得每一个坏人（莫德雷德除外），但一暴露就会被刺杀
	RolePercival     hiddenrole.RoleType = "PERCIVAL"      // 看到梅林与莫甘娜两个人，但分不清谁是谁

	// 坏人
	RoleMinion   hiddenrole.RoleType = "MINION"   // 莫德雷德的爪牙，无特殊能力
	RoleAssassin hiddenrole.RoleType = "ASSASSIN" // 好人凑满三次成功后，由他指认梅林
	RoleMorgana  hiddenrole.RoleType = "MORGANA"  // 在派西维尔眼里与梅林长得一样
	RoleMordred  hiddenrole.RoleType = "MORDRED"  // 梅林看不见他
	RoleOberon   hiddenrole.RoleType = "OBERON"   // 既不认识同伙，也不被同伙认识
)

// 六个技能。
const (
	SkillPropose hiddenrole.SkillType = "PROPOSE" // 队长提名一人上任务；一支队伍提交多次
	SkillApprove hiddenrole.SkillType = "APPROVE" // 表决：接受这支队伍
	SkillReject  hiddenrole.SkillType = "REJECT"  // 表决：否决这支队伍

	SkillMissionSuccess hiddenrole.SkillType = "MISSION_SUCCESS" // 任务：投成功
	SkillMissionFail    hiddenrole.SkillType = "MISSION_FAIL"    // 任务：投失败（只有坏人能投）

	SkillAssassinate hiddenrole.SkillType = "ASSASSINATE" // 刺杀：指认梅林
)

// 九个事件：规则给「发生了什么」起的名字。内核一个都不认得。
const (
	EventProposed         hiddenrole.EventType = "PROPOSED"          // 某人被提名上任务
	EventTeamApproved     hiddenrole.EventType = "TEAM_APPROVED"     // 队伍表决通过
	EventTeamRejected     hiddenrole.EventType = "TEAM_REJECTED"     // 队伍被否决
	EventLeaderChanged    hiddenrole.EventType = "LEADER_CHANGED"    // 队长轮转到下一位
	EventMissionSucceeded hiddenrole.EventType = "MISSION_SUCCEEDED" // 任务成功
	EventMissionFailed    hiddenrole.EventType = "MISSION_FAILED"    // 任务失败（附失败票数）
	EventHammerReached    hiddenrole.EventType = "HAMMER_REACHED"    // 连续五次否决，坏人直接获胜
	EventAssassinated     hiddenrole.EventType = "ASSASSINATED"      // 刺客指认了某人
	EventVote             hiddenrole.EventType = "VOTE"              // 某人的表决态度（公开）
	EventFailRejected     hiddenrole.EventType = "FAIL_REJECTED"     // 好人试图投失败，被否决（只发给他本人）
)

// 两个阵营。
const (
	CampGood hiddenrole.Camp = "GOOD"
	CampEvil hiddenrole.Camp = "EVIL"
)
