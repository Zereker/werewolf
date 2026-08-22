package avalon

import (
	"strconv"

	"github.com/Zereker/werewolf/engine"
)

// gamestate.go 阿瓦隆的整局进度存在哪。
//
// 五样东西：现在第几轮任务、成功了几次、失败了几次、连续否决了几次、
// 队长轮到谁。它们全都**整局有效、且不属于任何玩家**——正好是内核第四种
// 变量作用域（GameVar）。
//
// # 这个文件曾经是一处绕法
//
// 内核此前只有三种作用域，缺「整局有效 + 无主」这一格（SCARS.md 疤 4）。
// 于是这五个数只能挂到「ID 字典序最小的那名玩家」的 PlayerVar 上当账本：
// 全局事实记在某个人名下，那个玩家的 PlayerView 里凭空多出五个与他无关的
// 字段，「谁是账本」还得靠约定维持。
//
// 内核补上第四格之后，账本整个删掉了——现在是 GameVar，读写各一行。
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

// gameNum 读一个整局计数，没有则为 0。
func gameNum(view engine.GameView, key string) int {
	n, err := strconv.Atoi(view.GameVar(key))
	if err != nil {
		return 0
	}
	return n
}

// setGameNum 写一个整局计数。
func setGameNum(_ engine.GameView, key string, n int) *engine.Effect {
	return engine.NewSetGameVarEffect(key, strconv.Itoa(n))
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
