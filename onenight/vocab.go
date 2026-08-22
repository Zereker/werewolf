// vocab.go 一夜狼人的词汇表：十个阶段、十一个角色、十三个技能、十一个事件。
//
// 这是**第三套**规则包。前两套是狼人杀（根包）与阿瓦隆（avalon/）。
// 内核只有类型，取值全在这里——它不知道有「捣蛋鬼」这个角色，也不知道
// 「NIGHT_ROBBER」这个阶段。
//
// # 规则来源
//
// 这一次不能照前两套的规矩走。狼人杀以维基「狼人殺」条目为基准、阿瓦隆以
// 英文维基 The Resistance (game) 条目为基准，而**一夜狼人在英文维基上没有
// 独立条目**——它重定向到 Ultimate Werewolf，那篇只有两句带过，没有夜晚
// 次序、没有角色能力、没有胜负细则。
//
// 因此本包以**出版方 Bezier Games 的官方规则书**为基准，并用
// ultraboardgames.com 复述的官方规则交叉核对。逐条判定见 RULES 注释。
// 与来源不一致的地方在下面写明理由。
//
// # 为什么包名不叫 onuw
//
// 「One Night Ultimate Werewolf」是 Bezier Games 的商标。玩法规则本身一般
// 不受著作权保护，受保护的是名称、美术与具体文字表述。本包实现的是玩法，
// 因此用描述性的 onenight（一夜）而不是商标本身，与出版方无关。
package onenight

import "github.com/Zereker/werewolf/engine"

// 十个阶段：九个夜晚环节 + 白天讨论 + 投票。
//
// 与前两套最大的不同：**这是一条直线，不是环**。整局只有一个夜晚、一次
// 讨论、一次投票，走到 VOTE 就结束。Round 从头到尾是 1。
//
// 夜晚次序是规则的一部分而不是实现细节：抢劫者在捣蛋鬼之前动，所以捣蛋鬼
// 能把抢劫者刚抢来的牌再换走；失眠者最后动，所以他看到的是所有交换之后的
// 结果。次序错了，游戏就变成另一个游戏。
const (
	PhaseNightWerewolf    engine.PhaseType = "NIGHT_WEREWOLF"     // 狼人互认；独狼可看一张中央牌
	PhaseNightMinion      engine.PhaseType = "NIGHT_MINION"       // 爪牙看谁是狼（狼不知道爪牙是谁）
	PhaseNightMason       engine.PhaseType = "NIGHT_MASON"        // 守夜人互认
	PhaseNightSeer        engine.PhaseType = "NIGHT_SEER"         // 看一名玩家的牌，或两张中央牌
	PhaseNightRobber      engine.PhaseType = "NIGHT_ROBBER"       // 与一名玩家换牌，并看新牌
	PhaseNightTroublemake engine.PhaseType = "NIGHT_TROUBLEMAKER" // 交换另外两名玩家的牌，自己不看
	PhaseNightDrunk       engine.PhaseType = "NIGHT_DRUNK"        // 与一张中央牌交换，不看
	PhaseNightInsomniac   engine.PhaseType = "NIGHT_INSOMNIAC"    // 看自己现在的牌
	PhaseDay              engine.PhaseType = "DAY"                // 讨论
	PhaseVote             engine.PhaseType = "VOTE"               // 同时投票
)

// 十一个角色。
//
// 与前两套的根本不同：**角色分两层**。
//
//	发到手的那张牌   决定夜里你做什么   一局之内不变
//	现在手上那张牌   决定结算时你算哪边  夜里会被换来换去
//
// 抢劫者抢到狼人牌之后**不会**变成狼、不跟狼一起醒——他夜里做的事由发到手
// 的牌决定；但天亮结算时他算狼队。这一条是整个游戏的支点，也是它与狼人杀、
// 阿瓦隆最不一样的地方：那两套里「你是什么角色」自始至终只有一个答案。
//
// 内核的 RoleType 承担第一层（入座时定死，正好对应发到手的牌），
// 第二层是本包自己的一项整局状态（varCard），见 cards.go。
const (
	// 狼队
	RoleWerewolf engine.RoleType = "WEREWOLF" // 夜里互认；场上只有一只时可看一张中央牌
	RoleMinion   engine.RoleType = "MINION"   // 看得见狼，狼看不见他

	// 村民队
	RoleMason        engine.RoleType = "MASON"        // 两名守夜人互认；只有一名时另一张在中央
	RoleSeer         engine.RoleType = "SEER"         // 看一名玩家，或两张中央牌
	RoleRobber       engine.RoleType = "ROBBER"       // 与一名玩家换牌并看新牌
	RoleTroublemaker engine.RoleType = "TROUBLEMAKER" // 交换另外两名玩家的牌，自己不看
	RoleDrunk        engine.RoleType = "DRUNK"        // 与一张中央牌交换，**不看**
	RoleInsomniac    engine.RoleType = "INSOMNIAC"    // 看自己现在的牌
	RoleVillager     engine.RoleType = "VILLAGER"     // 无能力
	RoleHunter       engine.RoleType = "HUNTER"       // 他出局时，他投的那个人也出局

	// 独立
	RoleTanner engine.RoleType = "TANNER" // 只有自己出局才赢
)

// 十三个技能。
//
// 「看两张中央牌」「与某张中央牌交换」这类动作**指向的不是玩家**，而内核的
// 目标校验只认玩家 ID（SkillUse.Targets 会被逐个拿去 getPlayer）。于是中央
// 牌的下标只能编进技能名里——三张牌两两组合三种，单张三种，一共六个技能
// 干的其实是两件事。这是本包记下的第一条疤，见 SCARS.md。
const (
	SkillPeekCenter0 engine.SkillType = "PEEK_CENTER_0" // 独狼看中央第 0 张
	SkillPeekCenter1 engine.SkillType = "PEEK_CENTER_1"
	SkillPeekCenter2 engine.SkillType = "PEEK_CENTER_2"

	SkillSeerPlayer   engine.SkillType = "SEER_PLAYER"    // 预言家看一名玩家
	SkillSeerCenter01 engine.SkillType = "SEER_CENTER_01" // 预言家看中央第 0、1 张
	SkillSeerCenter02 engine.SkillType = "SEER_CENTER_02"
	SkillSeerCenter12 engine.SkillType = "SEER_CENTER_12"

	SkillRob engine.SkillType = "ROB" // 抢劫者与一名玩家换牌

	SkillMeddle engine.SkillType = "MEDDLE" // 捣蛋鬼交换另外两名玩家的牌

	SkillDrinkCenter0 engine.SkillType = "DRINK_CENTER_0" // 酒鬼与中央第 0 张交换
	SkillDrinkCenter1 engine.SkillType = "DRINK_CENTER_1"
	SkillDrinkCenter2 engine.SkillType = "DRINK_CENTER_2"

	SkillVote engine.SkillType = "VOTE" // 指认一人
)

// 十一个事件：规则给「发生了什么」起的名字。内核一个都不认得。
//
// 与前两套同一条规矩：一条 SWAPPED 单独发出去，谁的牌都不会动；真正改状态
// 的是旁边那条 SET_VAR。两个效果，两件事。
const (
	EventLoneWolf  engine.EventType = "LONE_WOLF"   // 场上只有一只狼
	EventPeeked    engine.EventType = "PEEKED"      // 看了一张中央牌
	EventSeerLook  engine.EventType = "SEER_LOOK"   // 预言家看了牌
	EventRobbed    engine.EventType = "ROBBED"      // 抢劫者换了牌
	EventMeddled   engine.EventType = "MEDDLED"     // 捣蛋鬼交换了两人的牌
	EventDrunkSwap engine.EventType = "DRUNK_SWAP"  // 酒鬼与中央换了牌
	EventInsomnia  engine.EventType = "INSOMNIA"    // 失眠者看了自己的牌
	EventVoted     engine.EventType = "VOTED"       // 一票投出
	EventNoOneDies engine.EventType = "NO_ONE_DIES" // 每人各得一票，无人出局
	EventLynched   engine.EventType = "LYNCHED"     // 被票出局
	EventHunterHit engine.EventType = "HUNTER_HIT"  // 猎人带走了他投的那个人
)

// 两个阵营。取值与狼人杀、阿瓦隆同名，含义与判定完全不同——
// 这一点本身就是内核不解释取值的证据。
const (
	CampVillage engine.Camp = "VILLAGE"
	CampWolf    engine.Camp = "WOLF"

	// CampTanner 皮匠自成一边：他既不帮村民也不帮狼，只想自己死。
	CampTanner engine.Camp = "TANNER"
)
