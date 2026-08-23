// knowledge.go 谁在夜里看到了什么。
//
// # 为什么要记下来，而不是要用的时候再算一遍
//
// 这一套规则里的信息是**有时效的**。预言家在第 5 环节看了三号的牌，而抢劫者
// 在第 6 环节把三号的牌换走了——预言家看到的仍然是他当时看到的那一张，
// 不是现在那一张。要用的时候再从局面算，算出来的一律是「现在」，全错。
//
// 前两套规则包没有这个问题：狼人杀的预言家查验结果当场就有意义（查的是
// 阵营，阵营不变），missions 包的梅林看到的坏人名单整局不变。到了这一套，
// 「他知道什么」与「现在是什么」第一次分了家。
//
// 于是每一次「看」都在看的人身上留一条记录，事后只读记录。记录是整局有效、
// 属于某个玩家的状态——正好是变量作用域那张 2×2 表的一格。

package onenight

import (
	"sort"
	"strconv"
	"strings"

	"github.com/Zereker/werewolf/engine"
)

const (
	// learnSelfKey 「我看到自己现在是什么」。抢劫者与失眠者会写它。
	learnSelfKey = "learn.self"

	// learnPlayerPrefix 「我看到某个人当时是什么」，后面接被看的人的 ID。
	learnPlayerPrefix = "learn.player."

	// learnCenterPrefix 「我看到中央第几张当时是什么」，后面接下标。
	learnCenterPrefix = "learn.center."
)

// learnSelf 记下「我看到自己现在是什么」。
func learnSelf(viewerID string, role engine.RoleType) *engine.Effect {
	return engine.NewSetVarEffect(engine.ScopeGame.Of(viewerID), learnSelfKey, string(role))
}

// learnPlayer 记下「我看到某个人当时是什么」。
func learnPlayer(viewerID, targetID string, role engine.RoleType) *engine.Effect {
	return engine.NewSetVarEffect(
		engine.ScopeGame.Of(viewerID), learnPlayerPrefix+targetID, string(role))
}

// learnCenter 记下「我看到中央第几张当时是什么」。
func learnCenter(viewerID string, i int, role engine.RoleType) *engine.Effect {
	return engine.NewSetVarEffect(
		engine.ScopeGame.Of(viewerID), learnCenterPrefix+strconv.Itoa(i), string(role))
}

// knowledgeOf 这名玩家夜里看到的一切，键与 learn* 写进去的一致。
//
// 它读的是玩家自己的整局状态，因此**必须**由 RoleInfoProvider 显式投射才能
// 到达玩家——内核刻意不把 Vars 交给玩家（见 engine.PlayerInfo 的说明），
// 那正是这个库要替调用方收掉的那类判断。
func knowledgeOf(view engine.GameView, playerID string) map[string]string {
	p, ok := view.Player(playerID)
	if !ok {
		return nil
	}

	out := make(map[string]string, len(p.Vars))
	for k, v := range p.Vars {
		if strings.HasPrefix(k, "learn.") {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// teammatesByDealt 发到手的牌属于同一伙的其他人，按 ID 排序。
//
// 「同一伙」按**发到手**的牌算，不按现在手上那张：狼人在第一个环节互认，
// 那时候一次交换都还没发生。抢劫者后来抢走了狼人牌也不会被认出来——
// 他不在场上那一刻的名单里。
func teammatesByDealt(view engine.GameView, playerID string, roles ...engine.RoleType) []string {
	want := make(map[engine.RoleType]bool, len(roles))
	for _, r := range roles {
		want[r] = true
	}

	var out []string
	for _, p := range view.AllPlayers() {
		if p.ID != playerID && want[p.Role] {
			out = append(out, p.ID)
		}
	}
	sort.Strings(out)
	return out
}
