package werewolf

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/Zereker/hiddenrole"
)

// canonical 把一批效果压成一个可比较的字符串。
//
// Data 是 map，按键排序之后再拼——否则这个函数自己就是不确定的，
// 会把「顺序稳定」误报成不稳定。
func canonical(effects []*hiddenrole.Effect) string {
	var sb strings.Builder
	for _, ef := range effects {
		if ef == nil {
			sb.WriteString("<nil>\n")
			continue
		}
		keys := make([]string, 0, len(ef.Data))
		for k := range ef.Data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		data := make([]string, 0, len(keys))
		for _, k := range keys {
			data = append(data, fmt.Sprintf("%s=%v", k, ef.Data[k]))
		}
		fmt.Fprintf(&sb, "%v|%s|%s|canceled=%v|reason=%s|{%s}\n",
			ef.Type, ef.SourceID, ef.TargetID, ef.Canceled, ef.Reason,
			strings.Join(data, ","))
	}
	return sb.String()
}

// resolverRuns 同一个解析器、同一个局面、同一批提交，反复跑多少次
//
// Go 的 map 迭代顺序是每次 range 都重新随机的。一个解析器只要在产出
// 效果的路径上迭代了 map，几十次里几乎不可能次次同序。
const resolverRuns = 60

// TestResolvers_EffectOrderIsDeterminedByTheBoard 效果的顺序必须由局面唯一决定。
//
// 这是内核写在 Resolver 文档里的硬性要求，而它此前没有任何东西守着：
// 随机对局的「回放必须与原引擎同步」那条不变量抓不到它——回放走的是
// **已录制的**效果流，会忠实重放当时那个顺序，无论那个顺序是怎么来的。
//
// 违反它的写法很自然：统计完票数之后 `for target := range votes` 一把
// 产出效果。平时看不出任何异常，只在「从同一个快照重跑两次，结果不一样」
// 的时候才现形——而那种 bug 极难复现。
//
// 这个测试反过来做：同一个局面跑 60 次，要求逐字节一致。
func TestResolvers_EffectOrderIsDeterminedByTheBoard(t *testing.T) {
	rules := DefaultRules()

	// 一副人多、且各角色都有事可做的局面。人多是有意的：map 迭代的
	// 随机性在键少的时候可能碰巧同序。
	full := newBoard()
	full.Players = append(full.Players, []hiddenrole.PlayerInfo{
		seatOf("w1", RoleWerewolf), seatOf("w2", RoleWerewolf), seatOf("w3", RoleWerewolf),
		seatOf("s", RoleSeer), seatOf("wi", RoleWitch), seatOf("g", RoleGuard),
		seatOf("h", RoleHunter),
		seatOf("v1", RoleVillager), seatOf("v2", RoleVillager),
		seatOf("v3", RoleVillager), seatOf("v4", RoleVillager), seatOf("v5", RoleVillager),
	}...)

	// 一批四散的投票：三方并列最高票，逼出平票分支
	spreadVotes := func(skill SkillType) []*SkillUse {
		return []*SkillUse{
			{PlayerID: "w1", Skill: skill, Targets: []string{"v1"}},
			{PlayerID: "w2", Skill: skill, Targets: []string{"v2"}},
			{PlayerID: "w3", Skill: skill, Targets: []string{"v3"}},
			{PlayerID: "s", Skill: skill, Targets: []string{"v1"}},
			{PlayerID: "wi", Skill: skill, Targets: []string{"v2"}},
			{PlayerID: "g", Skill: skill, Targets: []string{"v3"}},
			{PlayerID: "h", Skill: skill, Targets: []string{"v4"}},
			{PlayerID: "v1", Skill: skill, Targets: []string{"v5"}},
		}
	}
	// 一批有唯一最高票的投票
	clearVotes := func(skill SkillType) []*SkillUse {
		return []*SkillUse{
			{PlayerID: "w1", Skill: skill, Targets: []string{"v1"}},
			{PlayerID: "w2", Skill: skill, Targets: []string{"v1"}},
			{PlayerID: "w3", Skill: skill, Targets: []string{"v2"}},
			{PlayerID: "s", Skill: skill, Targets: []string{"v3"}},
			{PlayerID: "wi", Skill: skill, Targets: []string{"v4"}},
		}
	}

	cases := []struct {
		name  string
		r     hiddenrole.Resolver
		uses  []*SkillUse
		board board
	}{
		{"投票·平票", NewVoteResolver(), spreadVotes(SkillVote), full},
		{"投票·唯一最高票", NewVoteResolver(), clearVotes(SkillVote), full},
		{"狼刀·分歧", NewWolfResolver(), spreadVotes(SkillKill), full},
		{"狼刀·一致", NewWolfResolver(), clearVotes(SkillKill), full},
		{"白天发言", NewDayResolver(), []*SkillUse{
			{PlayerID: "v1", Skill: SkillSpeak}, {PlayerID: "v2", Skill: SkillSpeak},
			{PlayerID: "v3", Skill: SkillSpeak},
		}, full},
		{"守卫", NewGuardResolver(rules), []*SkillUse{
			{PlayerID: "g", Skill: SkillProtect, Targets: []string{"s"}},
		}, full},
		{"预言家", NewSeerResolver(), []*SkillUse{
			{PlayerID: "s", Skill: SkillCheck, Targets: []string{"w1"}},
		}, full},
		{"女巫·双开药", NewWitchResolver(rules), []*SkillUse{
			{PlayerID: "wi", Skill: SkillAntidote, Targets: []string{"v1"}},
			{PlayerID: "wi", Skill: SkillPoison, Targets: []string{"w1"}},
		}, withKill(full, "v1")},
		{"猎人·开枪", NewHunterResolver(), []*SkillUse{
			{PlayerID: "h", Skill: SkillShoot, Targets: []string{"w1"}},
		}, full},
		{"夜晚结算·有刀口", NewNightResolveResolver(rules), nil, withKill(full, "v1")},
		{"夜晚结算·刀口被守", NewNightResolveResolver(rules),
			nil, markSeat(withKill(full, "v1"), "v1", PlayerRoundVarProtected)},
		// 多人同时中毒：毒杀名单在实现里是从局面里筛出来的，只毒一个人的
		// 用例里它只有一个元素，顺序天然稳定——那样的用例守不住任何东西。
		{"夜晚结算·多人中毒", NewNightResolveResolver(rules), nil,
			markSeat(markSeat(markSeat(markSeat(markSeat(withKill(full, "v1"),
				"v1", PlayerRoundVarPoisoned),
				"v2", PlayerRoundVarPoisoned),
				"v3", PlayerRoundVarPoisoned),
				"v4", PlayerRoundVarPoisoned),
				"v5", PlayerRoundVarPoisoned)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := canonical(tc.r.Resolve(tc.uses, tc.board.View()))
			for i := 1; i < resolverRuns; i++ {
				got := canonical(tc.r.Resolve(tc.uses, tc.board.View()))
				if got != want {
					t.Fatalf("第 %d/%d 次产出的效果与第一次不同——"+
						"效果顺序没有由局面唯一决定，回放与快照比对会失去确定性。\n"+
						"--- 第一次 ---\n%s--- 第 %d 次 ---\n%s",
						i+1, resolverRuns, want, i+1, got)
				}
			}
		})
	}
}

// gameRuns 同一个脚本对局跑几遍
const gameRuns = 12

// TestFullGame_EffectLogIsReproducible 同一个脚本对局跑两遍，效果流必须逐字节一致。
//
// 上面那个测试罩住的是规则层的解析器。这一个罩住**整条栈**：解析器产出的
// 顺序、内核 applyEffects 的顺序、死亡触发排队的顺序、胜负判定的时机，
// 任何一处依赖了 map 迭代，两遍跑出来的效果流就对不上。
//
// 这是「同一个局面必须得出同一个结果」这条性质最直接的说法。它比回放那条
// 不变量强：回放走的是已录制的效果流，只能证明重放忠实，证明不了**生成**
// 是确定的。
func TestFullGame_EffectLogIsReproducible(t *testing.T) {
	// 一局走到底的脚本：守护、狼刀、女巫救人、预言家查验、结算、投票、
	// 猎人被投出后开枪——把会产生效果的路径尽量都踩一遍。
	play := func(t *testing.T) string {
		t.Helper()
		// 座两个女巫：一夜之内毒死两个人，夜晚结算那份毒杀名单才会有
		// 多个元素。只毒一个人的话名单里只有一项，顺序天然稳定，
		// 这条路上的顺序问题就漏过去了。
		g := newRuleGame(t, nil, seats(
			wolf("w1"), wolf("w2"), seer("s"), witch("wi"), witch("wi2"),
			guard("g"), hunter("h"), villagers("v1", "v2", "v3", "v4"),
		)...)

		g.mustUse("g", SkillProtect, "s")
		g.end(PhaseNightWolf)
		g.mustUse("w1", SkillKill, "v1")
		g.mustUse("w2", SkillKill, "v1")
		g.end(PhaseNightWitch)
		g.mustUse("wi", SkillPoison, "v2")
		g.mustUse("wi2", SkillPoison, "v3")
		g.end(PhaseNightSeer)
		g.mustUse("s", SkillCheck, "w1")
		g.end(PhaseNightResolve)
		g.endAny() // 夜晚结算：死亡在这里产生
		for i := 0; i < 6 && !g.e.Status().Over; i++ {
			for _, id := range g.e.AlivePlayerIDs() {
				for _, sk := range g.e.AllowedSkills(id) {
					target := ""
					if sk != SkillSkip && sk != SkillSpeak {
						target = "h" // 一路投猎人，逼出他出局时的开枪
					}
					_ = g.e.SubmitSkillUse(&SkillUse{PlayerID: id, Skill: sk, Targets: []string{target}})
					break
				}
			}
			g.endAny()
		}
		return canonical(g.e.EffectLog())
	}

	want := play(t)
	if want == "" {
		t.Fatal("这一局没有产生任何效果——脚本坏了，这个测试什么都没验到")
	}
	for i := 1; i < gameRuns; i++ {
		if got := play(t); got != want {
			t.Fatalf("第 %d/%d 遍的效果流与第一遍不同——同一个局面得出了不同的结果。\n"+
				"--- 第一遍 ---\n%s--- 第 %d 遍 ---\n%s", i+1, gameRuns, want, i+1, got)
		}
	}
}
