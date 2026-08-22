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
	roleIdiot     = werewolf.RoleType(1000)
	eventRevealed = werewolf.EventType(1001) // 白痴翻牌
)

// idiotRule 白痴的规则，包在内置投票解析器外面。
//
// # 状态放在哪
//
// 「这个白痴翻过牌了吗」是第三方自己的状态，而引擎里没有地方放它——
// PlayerState 的字段是固定的，RoundContext 也是，applyEffect 对不认识的
// 效果类型直接忽略。所以只能由扩展自己拿着。
//
// 代价是它不会进快照：Engine.Snapshot() 导不出这个字段。恢复对局时必须由
// 扩展自己从 Engine.EffectLog() 里把它捡回来（见 restoreFrom）。
// 这是这个库当前最硌手的一处，下面的注释里有更完整的说明。
type idiotRule struct {
	inner    werewolf.Resolver
	revealed map[string]bool // 已经翻过牌的白痴
}

func newIdiotRule(inner werewolf.Resolver) *idiotRule {
	return &idiotRule{inner: inner, revealed: make(map[string]bool)}
}

// Resolve 先把已翻牌白痴的票剔掉，再让内置解析器数票；
// 如果数出来要放逐的正好是个还没翻牌的白痴，就把这次放逐拦下来。
func (r *idiotRule) Resolve(
	uses []*werewolf.SkillUse, view werewolf.GameView, cfg *werewolf.GameConfig,
) []*werewolf.Effect {
	// 失去投票权：引擎那边没法表达「某个人不能用某个技能」——
	// 阶段配置声明的是「角色 + 技能」，投票那一步写的是「全体」。
	// 所以只能在这里把票丢掉。玩家提交时不会被拒，是在结算时不算数。
	kept := make([]*werewolf.SkillUse, 0, len(uses))
	for _, u := range uses {
		if u.Skill == werewolf.SkillVote && r.revealed[u.PlayerID] {
			continue
		}
		kept = append(kept, u)
	}

	effects := r.inner.Resolve(kept, view, cfg)

	out := make([]*werewolf.Effect, 0, len(effects)+1)
	for _, ef := range effects {
		if ef.Type != werewolf.EventEliminate || ef.Canceled {
			out = append(out, ef)
			continue
		}
		p, ok := view.Player(ef.TargetID)
		if !ok || p.Role != roleIdiot || r.revealed[ef.TargetID] {
			out = append(out, ef) // 不是白痴，或者已经翻过牌了：照常出局
			continue
		}

		// 是个还没翻牌的白痴：把放逐否决掉，改成翻牌。
		//
		// 这里用 Cancel 而不是干脆不产出这个效果——被否决的效果仍会
		// 出现在 EndPhase 的返回值与效果流里，调用方（和回放）因此
		// 知道「投出来的是他，但他没死，原因是这个」。
		ef.Cancel("白痴翻牌，不出局")
		out = append(out, ef)
		out = append(out, werewolf.NewEffect(eventRevealed, ef.TargetID, "").
			WithData("role", "IDIOT"))

		r.revealed[ef.TargetID] = true
	}
	return out
}

// restoreFrom 从效果流把「谁翻过牌」重建出来。
//
// 引擎恢复对局时不会带上扩展的状态——它根本不知道有这么个状态。
// 好在效果流是完整的，扩展自己产出的效果也在里面，可以扫一遍捡回来。
//
// 这个方法的存在本身就是一处 API 缺口的证据：每一个有自身状态的第三方
// 角色都得重写一遍这段，而且必须记得在恢复时调用它——忘了的话对局
// 恢复出来是错的，还不会报错。
func (r *idiotRule) restoreFrom(log []*werewolf.Effect) {
	for _, ef := range log {
		if ef.Type == eventRevealed && !ef.Canceled {
			r.revealed[ef.SourceID] = true
		}
	}
}

func (r *idiotRule) hasRevealed(id string) bool { return r.revealed[id] }

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
