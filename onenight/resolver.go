// resolver.go 十个阶段各自的结算。
//
// 夜晚的每一条能力都由**发到手的那张牌**决定谁能用（见 cards.go）——
// 抢劫者抢到狼人牌之后不会跟狼一起醒。内核的行动者判定正好是按 RoleType
// 算的，而 RoleType 就是发到手的那张牌，因此这一条不需要规则做任何事。

package onenight

import (
	"sort"
	"strings"

	"github.com/Zereker/werewolf/engine"
)

// noopResolver 这个阶段不产生任何状态变更。
//
// 白天只是讨论，主持人看够了就推进。爪牙、守夜人、失眠者那三个阶段也走
// 这一条：他们只接收信息、不做任何动作，信息由 RoleInfoProvider 送达
// （见 boundary.go）。
type noopResolver struct{}

func (noopResolver) Resolve([]*engine.SkillUse, engine.GameView) []*engine.Effect { return nil }

// firstUse 取某个技能集合里第一条有效提交，没有则返回 nil。
//
// 夜晚能力全是「至多一次」：内核允许重复提交，取第一条是本包的口径。
func firstUse(uses []*engine.SkillUse, skills ...engine.SkillType) *engine.SkillUse {
	want := make(map[engine.SkillType]bool, len(skills))
	for _, s := range skills {
		want[s] = true
	}
	for _, u := range uses {
		if want[u.Skill] {
			return u
		}
	}
	return nil
}

// centerIndexes 从技能名末尾读出中央牌的下标。
//
// 「看两张中央牌」「与某张中央牌交换」指向的不是玩家，而内核的目标校验只认
// 玩家 ID——SkillUse.Targets 里的每一项都会被拿去 getPlayer，对不上就是
// ErrTargetNotFound。于是下标只能编进技能名里，再在这里读回来。
// 这是本包的第一条疤，见 SCARS.md 疤 1。
func centerIndexes(skill engine.SkillType) []int {
	name := string(skill)
	i := strings.LastIndex(name, "_")
	if i < 0 {
		return nil
	}
	var out []int
	for _, c := range name[i+1:] {
		if c < '0' || c >= '0'+CenterCount {
			return nil
		}
		out = append(out, int(c-'0'))
	}
	return out
}

// ==================== 夜晚 ====================

// werewolfResolver 狼人阶段。
//
// 狼互认是纯信息，走 RoleInfoProvider。这里只处理**独狼**那一条：场上只有
// 一只狼时，他可以看一张中央牌。规则没说「必须只有一只才能看」是内核该管
// 的事——所以由这里判断，不是内核。
type werewolfResolver struct{}

func (werewolfResolver) Resolve(uses []*engine.SkillUse, view engine.GameView) []*engine.Effect {
	use := firstUse(uses, SkillPeekCenter0, SkillPeekCenter1, SkillPeekCenter2)
	if use == nil {
		return nil
	}
	if len(dealtWith(view, RoleWerewolf)) != 1 {
		// 不止一只狼，看牌这个选项根本不存在。内核拦不住这一条——
		// 「场上有几只狼」是规则的判断。
		return nil
	}
	idx := centerIndexes(use.Skill)
	if len(idx) != 1 {
		return nil
	}
	return []*engine.Effect{
		engine.NewEffect(EventLoneWolf, use.PlayerID, ""),
		learnCenter(use.PlayerID, idx[0], centerCard(view, idx[0])),
	}
}

// seerResolver 预言家阶段：看一名玩家的牌，或者两张中央牌。
type seerResolver struct{}

func (seerResolver) Resolve(uses []*engine.SkillUse, view engine.GameView) []*engine.Effect {
	use := firstUse(uses, SkillSeerPlayer, SkillSeerCenter01, SkillSeerCenter02, SkillSeerCenter12)
	if use == nil {
		return nil
	}

	if use.Skill == SkillSeerPlayer {
		target := use.Target()
		if target == "" || target == use.PlayerID {
			return nil // 看自己没有意义，规则也不允许
		}
		return []*engine.Effect{
			engine.NewEffect(EventSeerLook, use.PlayerID, target),
			learnPlayer(use.PlayerID, target, card(view, target)),
		}
	}

	out := []*engine.Effect{engine.NewEffect(EventSeerLook, use.PlayerID, "")}
	for _, i := range centerIndexes(use.Skill) {
		out = append(out, learnCenter(use.PlayerID, i, centerCard(view, i)))
	}
	return out
}

// robberResolver 抢劫者阶段：与一名玩家换牌，并看新牌。
//
// 「换完之后看」这个次序是规则的一部分：他知道自己现在是什么，但对方不知道。
type robberResolver struct{}

func (robberResolver) Resolve(uses []*engine.SkillUse, view engine.GameView) []*engine.Effect {
	use := firstUse(uses, SkillRob)
	if use == nil {
		return nil
	}
	target := use.Target()
	if target == "" || target == use.PlayerID {
		return nil
	}

	got := card(view, target)
	out := []*engine.Effect{engine.NewEffect(EventRobbed, use.PlayerID, target)}
	out = append(out, swapCards(view, use.PlayerID, target)...)
	return append(out, learnSelf(use.PlayerID, got))
}

// troublemakerResolver 捣蛋鬼阶段：交换另外两名玩家的牌，自己不看。
//
// 三个人都不知道发生了什么——捣蛋鬼没看，被换的两个人也没被告知。
// 这是这个游戏里信息最不对称的一手。
type troublemakerResolver struct{}

func (troublemakerResolver) Resolve(uses []*engine.SkillUse, view engine.GameView) []*engine.Effect {
	use := firstUse(uses, SkillMeddle)
	if use == nil {
		return nil
	}
	if len(use.Targets) != 2 {
		return nil // 必须正好两个人
	}
	a, b := use.Targets[0], use.Targets[1]
	if a == b || a == use.PlayerID || b == use.PlayerID {
		return nil // 「另外两名」——不含自己，且两人不同
	}

	out := []*engine.Effect{engine.NewEffect(EventMeddled, use.PlayerID, "").
		WithData("a", a).WithData("b", b)}
	return append(out, swapCards(view, a, b)...)
}

// drunkResolver 酒鬼阶段：与一张中央牌交换，**不看**。
//
// 他因此不知道自己现在算哪边——这正是这个角色的全部内容。
type drunkResolver struct{}

func (drunkResolver) Resolve(uses []*engine.SkillUse, view engine.GameView) []*engine.Effect {
	use := firstUse(uses, SkillDrinkCenter0, SkillDrinkCenter1, SkillDrinkCenter2)
	if use == nil {
		return nil
	}
	idx := centerIndexes(use.Skill)
	if len(idx) != 1 {
		return nil
	}

	out := []*engine.Effect{engine.NewEffect(EventDrunkSwap, use.PlayerID, "").
		WithData("center", idx[0])}
	// 注意：只换，不 learn。
	return append(out, swapWithCenter(view, use.PlayerID, idx[0])...)
}

// insomniacResolver 失眠者阶段：看自己现在的牌。
//
// 他最后一个动，因此看到的是所有交换之后的结果。这条能力没有任何状态变更，
// 只有一条记录——而记录是必须的，见 learnSelf 的说明。
type insomniacResolver struct{}

func (insomniacResolver) Resolve(_ []*engine.SkillUse, view engine.GameView) []*engine.Effect {
	var out []*engine.Effect
	for _, id := range dealtWith(view, RoleInsomniac) {
		out = append(out,
			engine.NewEffect(EventInsomnia, id, ""),
			learnSelf(id, card(view, id)))
	}
	return out
}

// ==================== 投票 ====================

// voteResolver 全员同时投票。
//
// 规则（官方规则书）：
//   - 得票最多的人出局并翻牌
//   - 平票时并列最高的**全部**出局
//   - **每人恰好各得一票时无人出局**——这一条是规则明写的，不是平票的特例
//   - 猎人若出局，他投的那个人也出局
//
// 「猎人」按**现在手上那张牌**算，不按发到手的那张：天亮翻的是手上的牌，
// 抢到猎人牌的人就是猎人。
type voteResolver struct{}

func (voteResolver) Resolve(uses []*engine.SkillUse, view engine.GameView) []*engine.Effect {
	players := view.AllPlayers()

	// 一人一票，重复提交取第一条。
	votedBy := make(map[string]string, len(players))
	tally := make(map[string]int, len(players))
	var out []*engine.Effect
	for _, u := range uses {
		if u.Skill != SkillVote || votedBy[u.PlayerID] != "" {
			continue
		}
		target := u.Target()
		if target == "" || target == u.PlayerID {
			continue // 不能投自己
		}
		if _, ok := view.Player(target); !ok {
			continue
		}
		votedBy[u.PlayerID] = target
		tally[target]++
		out = append(out, engine.NewEffect(EventVoted, u.PlayerID, target))
	}

	// 投票结算过了。这一笔是给胜负判定用的：「无人出局」是一个合法结局，
	// 它与「还没投票」在局面上长得一模一样。
	out = append(out, markVoteSettled())

	// 每人恰好各得一票：无人出局。规则明写的一条，不是平票的特例。
	if allTiedAtOne(players, tally) {
		return append(out, engine.NewEffect(EventNoOneDies, "", ""))
	}

	doomed := topVoted(tally)
	if len(doomed) == 0 {
		return append(out, engine.NewEffect(EventNoOneDies, "", ""))
	}

	// 猎人带人：先把猎人的目标收进来，再一起结算。
	// 猎人自己也可能是被猎人带走的（两个猎人互相投），因此只走一轮——
	// 规则没有连锁开枪，这一点与狼人杀不同。
	dead := make(map[string]bool, len(doomed))
	for _, id := range doomed {
		dead[id] = true
	}
	for _, id := range doomed {
		if card(view, id) != RoleHunter {
			continue
		}
		hit := votedBy[id]
		if hit == "" || dead[hit] {
			continue
		}
		dead[hit] = true
		out = append(out, engine.NewEffect(EventHunterHit, id, hit))
	}

	for _, id := range sortedKeys(dead) {
		out = append(out,
			engine.NewEffect(EventLynched, "", id),
			engine.NewSetAliveEffect(id, false))
	}
	return out
}

// allTiedAtOne 是不是每一名玩家都恰好得了一票。
func allTiedAtOne(players []engine.PlayerInfo, tally map[string]int) bool {
	if len(players) == 0 {
		return false
	}
	for _, p := range players {
		if tally[p.ID] != 1 {
			return false
		}
	}
	return true
}

// topVoted 得票最多的那些人，按 ID 排序。零票不算。
func topVoted(tally map[string]int) []string {
	best := 0
	for _, n := range tally {
		if n > best {
			best = n
		}
	}
	if best == 0 {
		return nil
	}
	var out []string
	for id, n := range tally {
		if n == best {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// sortedKeys 一张集合的键，按字典序——效果流因此是确定的。
func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// dealtWith 发到手的牌是某个角色的那些玩家，按 ID 排序。
//
// 用 AllPlayers 而不是 AlivePlayers：这一局到投票之前没有人会出局，
// 而「谁能用某个能力」在这套规则里与生死无关。
func dealtWith(view engine.GameView, role engine.RoleType) []string {
	var out []string
	for _, p := range view.AllPlayers() {
		if p.Role == role {
			out = append(out, p.ID)
		}
	}
	sort.Strings(out)
	return out
}
