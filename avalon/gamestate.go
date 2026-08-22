package avalon

import (
	"strconv"

	"github.com/Zereker/werewolf/engine"
)

// gamestate.go 阿瓦隆的整局进度存在哪。
//
// # 这个文件是一处绕法，不是设计
//
// 阿瓦隆要记五样东西：现在第几轮任务、成功了几次、失败了几次、
// 连续否决了几次、队长轮到谁。它们全都**整局有效、且不属于任何玩家**。
//
// 内核有三种变量作用域，摆成表少一格：
//
//	              无主          属于某个玩家
//	整局有效       （没有）       PlayerVar
//	本回合有效     RoundVar      PlayerRoundVar
//
// 唯一的「无主」作用域是 RoundVar，而它跨回合会被清空（绕回起始阶段
// 即新回合）。阿瓦隆每提名一次就绕一圈，这五个数一轮都活不过。
//
// 于是只能挂到某个玩家身上——本文件挑「ID 字典序最小的那位」当账本。
// 这么做能走通：PlayerVar 跟着玩家走一整局、进快照、能回放，AllPlayers()
// 有序所以选谁是确定的。但它显然不对：
//
//   - 整局进度是**全局事实**，却记在某个人的私有状态里；
//   - PlayerView 里那个玩家会莫名其妙多出五个跟他无关的字段；
//   - 「谁是账本」这件事得靠约定维持，第三方扩展一不小心就会覆盖它。
//
// 记在 SCARS.md 第 4 条。真正的解法是内核补上缺的那一格。
const (
	varMission = "avalon.mission" // 现在第几轮任务，1-5
	varSuccess = "avalon.success" // 已经成功几次
	varFail    = "avalon.fail"    // 已经失败几次
	varRejects = "avalon.rejects" // 连续被否决几次
	varLeader  = "avalon.leader"  // 队长是第几号座位（AllPlayers 的下标）

	// varAssassinated 刺杀结果："hit" 指中了梅林，"miss" 指没中。
	// 空串表示刺杀还没发生——胜负判定靠它区分「好人赢了」与「好人
	// 凑满三次、但还没过刺杀这一关」。
	varAssassinated = "avalon.assassinated"
)

// 回合级的状态：这些恰好每提名一轮就该清零，用得上内核现成的作用域。
const (
	varOnTeam   = "avalon.on_team"  // 玩家级：这一轮被提名上任务
	varApproved = "avalon.approved" // 回合级：这一轮的队伍表决通过了
)

// ledger 当账本的那名玩家。
//
// AllPlayers() 按 ID 排序，因此这个选择是确定的——回放与快照比对
// 依赖它。阿瓦隆没有出局机制，名单整局不变。
func ledger(view engine.GameView) string {
	all := view.AllPlayers()
	if len(all) == 0 {
		return ""
	}
	return all[0].ID
}

// gameNum 读一个整局计数，没有则为 0。
func gameNum(view engine.GameView, key string) int {
	id := ledger(view)
	if id == "" {
		return 0
	}
	n, err := strconv.Atoi(view.PlayerVar(id, key))
	if err != nil {
		return 0
	}
	return n
}

// setGameNum 写一个整局计数。
func setGameNum(view engine.GameView, key string, n int) *engine.Effect {
	return engine.NewSetPlayerVarEffect(ledger(view), key, strconv.Itoa(n))
}

// mission 当前是第几轮任务，1-5。开局还没写过时算第 1 轮。
func mission(view engine.GameView) int {
	if n := gameNum(view, varMission); n > 0 {
		return n
	}
	return 1
}

func successes(view engine.GameView) int { return gameNum(view, varSuccess) }
func failures(view engine.GameView) int  { return gameNum(view, varFail) }
func rejects(view engine.GameView) int   { return gameNum(view, varRejects) }

// leaderID 当前队长。
//
// 队长按座位顺序轮转，无论组队是否通过——条目里的 leader token 每轮
// 都往下传一位。下标存在账本里，取模玩家数。
func leaderID(view engine.GameView) string {
	all := view.AllPlayers()
	if len(all) == 0 {
		return ""
	}
	return all[gameNum(view, varLeader)%len(all)].ID
}

// onTeam 这名玩家这一轮在不在任务队伍里。
func onTeam(view engine.GameView, playerID string) bool {
	return view.PlayerRoundVar(playerID, varOnTeam) != ""
}

// teamIDs 这一轮的任务队伍，按 ID 排序。
//
// 有序是规则必须保证的：产出的效果顺序要由局面唯一决定，
// 否则回放与快照比对失去确定性。AllPlayers() 已经排好序。
func teamIDs(view engine.GameView) []string {
	var out []string
	for _, p := range view.AllPlayers() {
		if onTeam(view, p.ID) {
			out = append(out, p.ID)
		}
	}
	return out
}

// approved 这一轮的队伍表决通过了没有。
func approved(view engine.GameView) bool { return view.RoundVar(varApproved) != "" }
