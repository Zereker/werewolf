// victory.go 谁赢了。
//
// 这一节是本包撞得最狠的一处，见 SCARS.md 疤 5：**这套规则可以有不止一个
// 赢家**，而内核的 VictoryChecker 只能返回一个 Camp。

package onenight

import (
	"sort"
	"strings"

	"github.com/Zereker/werewolf/engine"
)

// 胜负条件，取自官方规则书：
//
//	村民队赢   至少一名狼人出局（同时有非狼出局也不影响）
//	           或者：场上根本没有狼人（三张狼牌都在中央）且无人出局
//	狼队赢     场上至少有一名狼人，且没有狼人出局
//	皮匠赢     只有他自己出局才赢。他出局且无狼出局 → 狼不赢；
//	           他出局且有狼出局 → 村民也赢
//
// 「狼人出局」数的是**狼人牌**，不是狼队：爪牙属狼队，但他不是狼，
// 他出局不算「狼人出局」。
//
// # 与来源的一处分歧
//
// 广为流传的一条爪牙细则是「场上没有狼人时，爪牙只要自己不死、且至少死了
// 一名村民就赢」。它在出版方的规则引文里**找不到**，只出现在二手复述里。
// 本包从官方引文，不实现那一条——与阿瓦隆包在梅林那条上的做法一致：
// 来源打架时说明理由，从更权威的那一份。
//
// 后果是一个边角局面没有赢家：狼牌全在中央、爪牙在场、且有人出局。
// 这一条有测试钉住（TestVictory_NoWolfInPlayAndSomeoneDies），
// 换来源时会立刻看见。

// checkVictory 判定胜负。
//
// 只在投票结束之后才有答案——这一套规则里，**中途永远不结束**。
func checkVictory(view engine.GameView) (bool, engine.Camp) {
	if view.Phase() != PhaseVote {
		return false, engine.CampUnspecified
	}

	players := view.AllPlayers()
	var wolfInPlay, anyDied, wolfDied, tannerDied bool
	for _, p := range players {
		held := card(view, p.ID)
		if isWolfCard(held) {
			wolfInPlay = true
		}
		if p.Alive {
			continue
		}
		anyDied = true
		if isWolfCard(held) {
			wolfDied = true
		}
		if held == RoleTanner {
			tannerDied = true
		}
	}

	// 投票还没结算完（没人死也没有「无人出局」的结论）时不下判断。
	if !anyDied && !votingSettled(view) {
		return false, engine.CampUnspecified
	}

	var winners []engine.Camp
	if wolfDied || (!wolfInPlay && !anyDied) {
		winners = append(winners, CampVillage)
	}
	if tannerDied {
		winners = append(winners, CampTanner)
	}
	if wolfInPlay && !wolfDied && !tannerDied {
		winners = append(winners, CampWolf)
	}

	return true, joinCamps(winners)
}

// votingSettled 投票阶段是不是已经结算过了。
//
// 「无人出局」是一个合法结局（每人各得一票），它与「还没投票」在局面上
// 长得一模一样——两种情况都是没有人出局。用一项整局状态把它们分开。
func votingSettled(view engine.GameView) bool {
	return view.Var(engine.ScopeGame, varVoteSettled) != ""
}

// varVoteSettled 投票已结算的标记。
const varVoteSettled = "vote.settled"

// markVoteSettled 投票结算完就记一笔，供胜负判定区分「无人出局」与「还没投」。
func markVoteSettled() *engine.Effect {
	return engine.NewSetVarEffect(engine.ScopeGame, varVoteSettled, engine.VarPresent)
}

// joinCamps 把若干个赢家拼成一个 Camp。
//
// 内核的 VictoryChecker 返回 (bool, Camp) ——**一个** Camp。而这套规则里
// 皮匠可以和村民一起赢：他出局、同时也有狼出局，两边都赢。
//
// Camp 的底层是字符串、内核不解释取值，所以把几个拼成一个是能跑的：
// "TANNER+VILLAGE"。但那是一个**字符串编码**，不是一个类型——调用方要知道
// 拆开的规矩，而那条规矩内核不知道、也没地方写。见 SCARS.md 疤 5。
//
// 按字典序拼，结果因此是确定的。
func joinCamps(winners []engine.Camp) engine.Camp {
	if len(winners) == 0 {
		return CampNobody
	}
	out := make([]string, 0, len(winners))
	for _, c := range winners {
		out = append(out, string(c))
	}
	sort.Strings(out)
	return engine.Camp(strings.Join(out, "+"))
}

// CampNobody 没有任何一边达成胜利条件。
//
// 这不是「还没结束」（那是 engine.CampUnspecified），是「结束了，没人赢」。
// 一个真实存在的边角局面：狼牌全在中央、爪牙在场、且有人出局。
const CampNobody engine.Camp = "NOBODY"

// Winners 把 checkVictory 拼出来的 Camp 拆回一组。
//
// 这个函数的存在本身就是疤 5 的证据：内核给不出「一组赢家」，于是编码与
// 解码的规矩只能由规则包自己带着。
func Winners(c engine.Camp) []engine.Camp {
	if c == engine.CampUnspecified || c == CampNobody {
		return nil
	}
	parts := strings.Split(string(c), "+")
	out := make([]engine.Camp, 0, len(parts))
	for _, p := range parts {
		out = append(out, engine.Camp(p))
	}
	return out
}

// Won 某一边是不是赢家之一。
func Won(c engine.Camp, want engine.Camp) bool {
	for _, w := range Winners(c) {
		if w == want {
			return true
		}
	}
	return false
}
