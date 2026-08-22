// idiot.go 一个第三方角色：白痴。
//
// 规则：被投票放逐时翻牌，不出局，但此后失去投票权。
//
// 选它而不是再写一遍狼王，是因为形状完全不同——狼王是「死了之后触发一个
// 技能」，引擎为这类事专门给了 NewAbilityTriggerEffect；白痴是
// **阻止一次死亡** 再 **改变这个人往后的能力**，引擎没有为它准备任何东西。
// 一个扩展点只在被最接近它设计意图的角色使用时好用，那不叫可扩展。

package main

import (
	"fmt"

	"github.com/Zereker/werewolf"
)

// 自定义取值从 1000 起，避免与后续内置枚举撞号。
const (
	roleIdiot     = werewolf.RoleType("IDIOT")
	eventRevealed = werewolf.EventType("IDIOT_REVEALED") // 白痴翻牌
)

// idiotRule 白痴的规则，包在内置投票解析器外面。
//
// # 它是无状态的
//
// Resolver 接口的要求是「只能读 GameView、只能通过返回 Effect 表达状态
// 变更」。「这个白痴翻过牌了吗」是会影响规则判定的状态，所以它必须住在
// 引擎里，而不是这个结构体的字段里——住在字段里的话，快照带不上它，
// 效果流也重建不出它，恢复出来的对局是错的。
//
// 引擎给角色的存放处是 PlayerVar：读走 GameView.PlayerVar，
// 写走 NewSetPlayerVarEffect。女巫的药、守卫的守护记录走的是同一条路，
// 内置角色在这件事上没有特权。
type idiotRule struct {
	inner werewolf.Resolver
}

// varRevealed 这个扩展在 PlayerVar 里用的键。
const varRevealed = "idiot.revealed"

func newIdiotRule(inner werewolf.Resolver) *idiotRule {
	return &idiotRule{inner: inner}
}

// Resolve 先把已翻牌白痴的票剔掉，再让内置解析器数票；
// 如果数出来要放逐的正好是个还没翻牌的白痴，就把这次放逐拦下来。
//
// 「拦下来」拦的是那条致死的原语（SET_ALIVE），不是 ELIMINATE 这个说法。
// 这一点值得记：状态机只认原语，ELIMINATE 只是投票规则给「发生了什么」
// 起的名字，光否决它人照样会死。反过来，拦原语与死因无关——同一段代码
// 能挡住狼刀、毒杀、枪口和任何第三方规则的死法，因为它们最终都要走
// 这一条。ELIMINATE 也一并否决，是为了让效果流与受众看到「投出来的
// 是他，但他没死」。
func (r *idiotRule) Resolve(
	uses []*werewolf.SkillUse, view werewolf.GameView,
) []*werewolf.Effect {
	// 失去投票权：引擎那边没法表达「某个人不能用某个技能」——
	// 阶段配置声明的是「角色 + 技能」，投票那一步写的是「全体」。
	// 所以只能在这里把票丢掉。玩家提交时不会被拒，是在结算时不算数。
	kept := make([]*werewolf.SkillUse, 0, len(uses))
	for _, u := range uses {
		if u.Skill == werewolf.SkillVote && revealed(view, u.PlayerID) {
			continue
		}
		kept = append(kept, u)
	}

	effects := r.inner.Resolve(kept, view)

	// 先看这一批里有没有「还没翻牌的白痴要死了」。判断依据是那条致死的
	// 原语，与内置解析器把它叫作什么无关。
	saved := make(map[string]bool)
	for _, ef := range effects {
		if ef.Canceled {
			continue
		}
		if alive, ok := ef.SetsAlive(); !ok || alive {
			continue
		}
		p, ok := view.Player(ef.TargetID)
		if !ok || p.Role != roleIdiot || revealed(view, ef.TargetID) {
			continue // 不是白痴，或者已经翻过牌了：照常出局
		}
		saved[ef.TargetID] = true
	}

	out := make([]*werewolf.Effect, 0, len(effects)+2*len(saved))
	revealedAt := make(map[string]bool, len(saved))
	for _, ef := range effects {
		if !saved[ef.TargetID] || ef.Canceled {
			out = append(out, ef)
			continue
		}

		// 用 Cancel 而不是干脆不产出这个效果——被否决的效果仍会出现在
		// EndPhase 的返回值与效果流里，调用方（和回放）因此知道
		// 「投出来的是他，但他没死，原因是这个」。
		ef.Cancel("白痴翻牌，不出局")
		out = append(out, ef)

		// 翻牌只发一次，哪怕这一批里既有 ELIMINATE 又有 SET_ALIVE
		if revealedAt[ef.TargetID] {
			continue
		}
		revealedAt[ef.TargetID] = true
		out = append(out,
			werewolf.NewEffect(eventRevealed, ef.TargetID, "").WithData("role", "IDIOT"),
			// 状态交给引擎保管：随快照走，回放能重建，这个 Resolver 保持无状态
			werewolf.NewSetPlayerVarEffect(ef.TargetID, varRevealed, "1"),
		)
	}
	return out
}

// revealed 这个白痴翻过牌了吗。
func revealed(view werewolf.GameView, id string) bool {
	return view.PlayerVar(id, varRevealed) != ""
}

// describe 把效果讲成一句话，包括这个扩展自己的事件类型。
func describe(ef *werewolf.Effect) string {
	name := ef.Type.String()
	if ef.Type == eventRevealed {
		name = "翻牌（白痴）"
	}
	s := ""
	if ef.SourceID != "" {
		s += ef.SourceID + " -> "
	}
	if ef.TargetID != "" {
		s += ef.TargetID + " "
	}
	s += name
	if ef.Canceled {
		s += fmt.Sprintf("（未生效：%s）", ef.Reason)
	}
	return s
}
