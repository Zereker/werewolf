// cards.go 「发到手的那张牌」与「现在手上那张牌」。
//
// 这是一夜狼人与前两套规则包最根本的差别，也是这一整个包存在的理由：
// 前两套里「你是什么角色」自始至终只有一个答案，这一套里有两个。
//
//	发到手的那张牌   决定夜里你做什么   一局之内不变   ← 内核的 RoleType
//	现在手上那张牌   决定结算时你算哪边  夜里被换来换去  ← 本包的一项整局状态
//
// 抢劫者抢到狼人牌之后不会变成狼、不跟狼一起醒——他夜里做的事由发到手的牌
// 决定；但天亮结算时他算狼队。酒鬼把自己的牌与中央的换掉，而且**不看**，
// 于是连他自己都不知道自己现在算哪边。
//
// 内核的 RoleType 入座时定死、没有写入路径，正好承担第一层。第二层是一项
// 「整局有效、属于某个玩家」的状态，规则自己管。
//
// **这一条原本被列为内核的抽象缺口**（docs/DESIGN.md §8.1「身份入座定死」，
// 猜想是挡掉换牌类游戏）。写下来才发现猜错了：换牌类游戏要的不是「可写的
// RoleType」，而是**两层身份**——而内核给一层、规则给一层，正好够用。
// 见 SCARS.md 疤 1。

package onenight

import "github.com/Zereker/werewolf/engine"

const (
	// varCard 这名玩家现在手上是哪张牌。整局有效、属于某个玩家。
	//
	// 入座时由 RoleSetup 发放，值等于发到手的牌；此后被抢劫者、捣蛋鬼、
	// 酒鬼改写。
	varCard = "card"

	// varCenter0/1/2 中央三张牌。整局有效、不属于任何玩家。
	//
	// 任务制那一套撞出来的那一格（整局·无主）在这里又一次派上用场：中央牌不
	// 属于任何人，也不该每回合清空。没有那一格的话，三张公共牌只能挂到
	// 某个玩家名下当账本。
	varCenter0 = "center.0"
	varCenter1 = "center.1"
	varCenter2 = "center.2"
)

// centerKeys 中央三张牌的键，按下标排列。
var centerKeys = [CenterCount]string{varCenter0, varCenter1, varCenter2}

// dealt 这名玩家**发到手**的那张牌，一局之内不变。
//
// 夜里谁做什么由它决定，不由 card 决定：抢劫者抢到狼人牌之后不会跟狼一起
// 醒。内核的 RoleType 就是它，这个函数只是把「为什么读 Role 而不读 card」
// 这件事写在名字里。
func dealt(view engine.GameView, playerID string) engine.RoleType {
	p, ok := view.Player(playerID)
	if !ok {
		return engine.RoleUnspecified
	}
	return p.Role
}

// card 这名玩家**现在手上**的那张牌。结算时算哪边由它决定。
//
// 没写过则退回发到手的那张——RoleSetup 会在入座时发放，这一路只是兜底。
func card(view engine.GameView, playerID string) engine.RoleType {
	if v := view.Var(engine.ScopeGame.Of(playerID), varCard); v != "" {
		return engine.RoleType(v)
	}
	return dealt(view, playerID)
}

// setCard 把某人手上的牌改成某张。
func setCard(playerID string, role engine.RoleType) *engine.Effect {
	return engine.NewSetVarEffect(engine.ScopeGame.Of(playerID), varCard, string(role))
}

// centerCard 中央第 i 张牌，下标越界则为空。
func centerCard(view engine.GameView, i int) engine.RoleType {
	if i < 0 || i >= CenterCount {
		return engine.RoleUnspecified
	}
	return engine.RoleType(view.Var(engine.ScopeGame, centerKeys[i]))
}

// setCenterCard 把中央第 i 张牌改成某张。
func setCenterCard(i int, role engine.RoleType) *engine.Effect {
	if i < 0 || i >= CenterCount {
		return nil
	}
	return engine.NewSetVarEffect(engine.ScopeGame, centerKeys[i], string(role))
}

// swapCards 交换两名玩家手上的牌。
//
// 捣蛋鬼与抢劫者都走这一条，区别只在谁看得到结果——那是信息边界的事，
// 不是状态的事。
func swapCards(view engine.GameView, a, b string) []*engine.Effect {
	return []*engine.Effect{
		setCard(a, card(view, b)),
		setCard(b, card(view, a)),
	}
}

// swapWithCenter 把某人手上的牌与中央第 i 张交换。
func swapWithCenter(view engine.GameView, playerID string, i int) []*engine.Effect {
	held := card(view, playerID)
	return []*engine.Effect{
		setCard(playerID, centerCard(view, i)),
		setCenterCard(i, held),
	}
}

// CampOf 一张牌属于哪一边。
//
// 它是**给宿主翻牌用的**，不是给内核用的：内核认得一个标准键 VarCamp，
// 会把它的值搬进 SelfInfo.Camp——而这一套规则**不能让它搬**。酒鬼不知道
// 自己现在拿的是什么牌，把当前阵营填进他自己的视图等于直接告诉他。
// 所以本包整局都不写 VarCamp，阵营在翻牌那一刻由宿主自己算。
// 见 SCARS.md 疤 4。
func CampOf(role engine.RoleType) engine.Camp {
	switch role {
	case RoleWerewolf, RoleMinion:
		return CampWolf
	case RoleTanner:
		return CampTanner
	default:
		return CampVillage
	}
}

// isWolfCard 这张牌是不是狼人牌。
//
// 只认 WEREWOLF：爪牙属狼队，但他**不是狼**——「有没有狼人出局」这条
// 胜负判定数的是狼人牌，不是狼队。
func isWolfCard(role engine.RoleType) bool { return role == RoleWerewolf }
