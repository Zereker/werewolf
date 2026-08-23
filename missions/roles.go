package missions

import (
	"sort"
	"strings"

	"github.com/Zereker/hiddenrole"
)

// roles.go 谁是好人、谁看得见谁。
//
// 这套规则整局没有一个人出局——这是它与狼人杀最大的结构差异。
// 内核的存活位在这一局里从头到尾没被写过，`SET_ALIVE` 一次都不产出。

// evilRoles 坏人阵营的全部角色。
var evilRoles = map[hiddenrole.RoleType]bool{
	RoleMinion:   true,
	RoleAssassin: true,
	RoleMorgana:  true,
	RoleMordred:  true,
	RoleOberon:   true,
}

func isEvil(role hiddenrole.RoleType) bool { return evilRoles[role] }

func campOf(role hiddenrole.RoleType) hiddenrole.Camp {
	if isEvil(role) {
		return CampEvil
	}
	return CampGood
}

// builtinRoleSetup 入座时发放阵营。
//
// 内核只认 VarCamp 这一个键（胜负判定数阵营用），别的都是规则自己的事。
// 这套规则不需要「神职/平民」那种细分——它的胜负只看任务成败与刺杀，
// 不数人头。
var builtinRoleSetup = func() map[hiddenrole.RoleType]hiddenrole.RoleSetup {
	out := map[hiddenrole.RoleType]hiddenrole.RoleSetup{}
	for _, r := range []hiddenrole.RoleType{
		RoleLoyalServant, RoleMerlin, RolePercival,
		RoleMinion, RoleAssassin, RoleMorgana, RoleMordred, RoleOberon,
	} {
		camp := campOf(r)
		out[r] = hiddenrole.RoleSetupFunc(func(string, hiddenrole.RoleType) map[string]string {
			return map[string]string{hiddenrole.VarCamp: string(camp)}
		})
	}
	return out
}()

// idsWithRole 场上担任这些角色的玩家，按 ID 排序。
func idsWithRole(view hiddenrole.GameView, roles ...hiddenrole.RoleType) []string {
	want := map[hiddenrole.RoleType]bool{}
	for _, r := range roles {
		want[r] = true
	}
	var out []string
	for _, p := range view.AllPlayers() {
		if want[p.Role] {
			out = append(out, p.ID)
		}
	}
	sort.Strings(out)
	return out
}

// teammates 「谁和我是一边的」。
//
// 只有坏人之间才是真正的同伙，而且**奥伯伦不在其中**：条目原文
// 「Oberon: Unknown to other evil players」——他既不认识同伙，
// 也不被同伙认识。这是一处天然不对称，内核明确支持。
//
// 梅林看得见坏人，但那不是「同一边」——他和他们是死敌。那份知识
// 属于角色专属信息，走 RoleInfo，见下面的 merlinInfo。
func teammates(playerID string, view hiddenrole.GameView) []string {
	self, ok := view.Player(playerID)
	if !ok || !isEvil(self.Role) || self.Role == RoleOberon {
		return nil
	}
	var out []string
	for _, p := range view.AllPlayers() {
		if p.ID == playerID || !isEvil(p.Role) || p.Role == RoleOberon {
			continue
		}
		out = append(out, p.ID)
	}
	return out
}

// RoleInfo 的键名。键名由角色自己定，内核不认得。
const (
	RoleInfoMerlinEvil        = "merlin.evil"                // 梅林看到的坏人名单
	RoleInfoPercivalCandidate = "percival.merlin_or_morgana" // 派西维尔看到的两个人
)

// merlinInfo 梅林认得每一个坏人——**除了莫德雷德**。
//
// 条目原文「Mordred: Unknown to Merlin」。注意梅林是能逐个认出他们是谁的，
// 不是只知道有几个（中文条目此处有误，见 vocab.go 的说明）。奥伯伦虽然
// 不被同伙认识，梅林照样看得见他。
func merlinInfo(_ string, view hiddenrole.GameView) map[string]string {
	var out []string
	for _, p := range view.AllPlayers() {
		if isEvil(p.Role) && p.Role != RoleMordred {
			out = append(out, p.ID)
		}
	}
	sort.Strings(out)
	return map[string]string{RoleInfoMerlinEvil: strings.Join(out, ",")}
}

// percivalInfo 派西维尔看到梅林与莫甘娜两个人，但分不清谁是谁。
//
// 条目原文「Percival secretly learns that two players are Merlin and Morgana,
// but does not know which player is which」。「分不清」这件事在实现上就是
// **把两个 ID 排序后一并给出**——不带任何区分标记。莫甘娜不在场时
// 只有梅林一个人，那一局派西维尔等于直接认出了梅林。
func percivalInfo(_ string, view hiddenrole.GameView) map[string]string {
	ids := idsWithRole(view, RoleMerlin, RoleMorgana)
	return map[string]string{RoleInfoPercivalCandidate: strings.Join(ids, ",")}
}

var builtinRoleInfo = map[hiddenrole.RoleType]hiddenrole.RoleInfoProvider{
	RoleMerlin:   hiddenrole.RoleInfoFunc(merlinInfo),
	RolePercival: hiddenrole.RoleInfoFunc(percivalInfo),
}
