package missions

import "github.com/Zereker/hiddenrole"

// victory.go 怎么算赢。
//
// 本包的胜负与狼人杀完全不是一回事：**它不数人头**。没有屠边、没有屠城，
// 没有任何人出局。胜负只看三样——任务成败的比分、连续否决的次数、
// 以及好人凑满三次之后那一刀。
//
// 这一点对内核是个正面证据：VictoryChecker 只拿到 GameView、只返回
// (是否结束, 赢家)，没有任何地方假设「赢是因为把谁杀光了」。
type victoryChecker struct{}

func (victoryChecker) CheckVictory(view hiddenrole.GameView) (bool, hiddenrole.Camp) {
	// 连续五次组队被否决，坏人直接获胜
	if rejects(view) >= HammerRejections {
		return true, CampEvil
	}

	// 三次任务失败
	if failures(view) >= 3 {
		return true, CampEvil
	}

	if successes(view) < 3 {
		return false, hiddenrole.CampUnspecified
	}

	// 好人凑满三次成功，但还得过刺杀这一关。
	//
	// 场上没有刺客时（最小板子）直接判好人赢；有刺客而还没动手时
	// 必须回「还没结束」——否则引擎会在刺杀阶段之前就把这局判掉。
	// 刺杀阶段由任务解析器用绕道队列排进来，内核会把胜负判定推迟到
	// 那之后，这里只要如实报「还没完」即可。
	switch view.Var(hiddenrole.ScopeGame, varAssassinated) {
	case "hit":
		return true, CampEvil // 刺中梅林，坏人反败为胜
	case "miss":
		return true, CampGood
	}
	if len(idsWithRole(view, RoleAssassin)) == 0 {
		return true, CampGood
	}
	return false, hiddenrole.CampUnspecified
}
