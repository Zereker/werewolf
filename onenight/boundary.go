// boundary.go 谁看到什么。
//
// 这一套规则的信息不对称比前两套都密，而且**方向不一样**：狼人杀与阿瓦隆
// 的不对称都是「一整局固定」的（狼互相认识、梅林认得坏人），这一套的不对称
// 是**一次性的**——你在某个环节看到了某样东西，之后局面变了，你的信息就过期
// 了，而且你不知道它过期了。整个游戏就建立在这上面。

package onenight

import (
	"github.com/Zereker/werewolf/engine"
)

// roleInfoFor 某个角色额外看到的东西。
//
// 三类：
//
//	互认    狼人、守夜人、爪牙——按**发到手**的牌算，因为他们在交换发生之前动
//	看到的  预言家、抢劫者、独狼、失眠者——记录在自己身上，见 knowledge.go
//	什么都没有  村民、捣蛋鬼（他自己也没看）、酒鬼（他更不知道）、皮匠、猎人
func roleInfoFor(role engine.RoleType) engine.RoleInfoProvider {
	return engine.RoleInfoFunc(func(playerID string, view engine.GameView) map[string]string {
		out := knowledgeOf(view, playerID)
		if out == nil {
			out = map[string]string{}
		}

		switch role {
		case RoleWerewolf:
			// 狼互认。只有一只狼时名单是空的——那正是他该知道的事
			// （空名单等于「我是独狼」，可以去看一张中央牌）。
			addList(out, "wolves", teammatesByDealt(view, playerID, RoleWerewolf))

		case RoleMinion:
			// 爪牙看得见狼，狼看不见他。**不对称**，而且是单向的。
			addList(out, "wolves", teammatesByDealt(view, playerID, RoleWerewolf))

		case RoleMason:
			// 守夜人互认。只有一名时名单是空的，意味着另一张在中央。
			addList(out, "masons", teammatesByDealt(view, playerID, RoleMason))
		}

		if len(out) == 0 {
			return nil
		}
		return out
	})
}

// addList 把一份名单塞进信息表，空名单也要塞——「名单是空的」本身是信息。
func addList(out map[string]string, key string, ids []string) {
	out[key] = joinIDs(ids)
}

// joinIDs 名单转成一个字符串，空名单是空串。
func joinIDs(ids []string) string {
	s := ""
	for i, id := range ids {
		if i > 0 {
			s += ","
		}
		s += id
	}
	return s
}

// teammates 「谁和谁是一边的」。
//
// 只有狼人之间成立，而且**不含爪牙**：爪牙看得见狼，狼看不见爪牙。
// 内核允许不对称正是为了这种情况——阿瓦隆的奥伯伦是另一个方向的例子
// （他既不认识同伙，也不被同伙认识）。
func teammates() engine.TeammateProvider {
	return engine.TeammateFunc(func(playerID string, view engine.GameView) []string {
		if dealt(view, playerID) != RoleWerewolf {
			return nil
		}
		return teammatesByDealt(view, playerID, RoleWerewolf)
	})
}

// audience 一件事该告诉谁。
//
// 夜里发生的每一件事都**只告诉当事人**：预言家看了谁的牌、抢劫者抢了谁、
// 捣蛋鬼换了哪两个人——全场都不该知道。白天的投票与出局是公开的。
//
// 内核的状态原语（SET_VAR / SET_ALIVE）永不外发，这一条不可配置，因此
// 「三号现在手上是狼人牌」这种事不会因为这里写漏而泄出去。
func audience() engine.AudienceProvider {
	return engine.AudienceFunc(func(event *engine.Event, view engine.GameView) ([]string, bool) {
		switch event.Type {
		// 公开：投票、出局、无人出局。
		case EventVoted, EventLynched, EventNoOneDies, EventHunterHit,
			engine.EventGameStarted, engine.EventGameEnded:
			return allIDs(view), true

		// 只有当事人知道。捣蛋鬼那条尤其重要：被换的两个人也不能知道。
		case EventSeerLook, EventRobbed, EventMeddled, EventDrunkSwap,
			EventInsomnia, EventLoneWolf, EventPeeked:
			if event.SourceID == "" {
				return nil, true
			}
			return []string{event.SourceID}, true
		}
		return nil, false
	})
}

// speech 发言的可听范围：全场，全程。
//
// 这一套规则只有一个讨论环节，而且是公开的——没有狼人杀那样的夜聊。
// 装这个 provider 不是为了改什么，是为了**关掉内核的默认**：内核默认
// 「出局的人不能发言」，而这一套里出局发生在最后一刻，之后也没有话要说，
// 默认与本规则无关。写出来比依赖默认清楚。
func speech() engine.SpeechProvider {
	return engine.SpeechFunc(func(_ string, view engine.GameView) []string {
		return allIDs(view)
	})
}

// allIDs 场上所有人，按 ID 排序。
func allIDs(view engine.GameView) []string {
	players := view.AllPlayers()
	out := make([]string, 0, len(players))
	for _, p := range players {
		out = append(out, p.ID)
	}
	return out
}
