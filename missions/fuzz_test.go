package missions

import (
	"math/rand"
	"testing"

	"github.com/Zereker/hiddenrole"
	"github.com/Zereker/hiddenrole/enginetest"
)

// TestFuzz_Invariants 随机对局，核对通用不变量。
//
// 不变量在 engine/enginetest 里，一条都不认识这套规则。这里只负责摆局面
// 与出招。
//
// 随机的是**人数与角色分配**：5~10 人，坏人数按 EvilCount 表定，
// 具体给谁哪个特殊角色随机。这套规则的分歧几乎全在角色配置上
// （有没有莫德雷德，梅林就看不看得见全部坏人；有没有奥伯伦，坏人之间
// 就认不认得全）。
//
// 本包的一局比另外两套长得多：一轮任务要走提名、表决、任务三个阶段，
// 表决否决还会绕回提名，五轮任务加上刺杀——所以 MaxSteps 给得宽。
func TestFuzz_Invariants(t *testing.T) {
	enginetest.RunFuzz(t, enginetest.FuzzSpec{
		Games:    1000, // 三套合计 5000 局；这一套的单局最长，给 1000
		MaxSteps: 400,
		WantEnd:  true,
		Setup:    setupRandom,
		Act:      actRandom,
		MustSee: []string{
			"五人局", "大局", "有莫德雷德", "无莫德雷德", "有奥伯伦",
		},
	})
}

// setupRandom 摆一副随机的板子。
func setupRandom(rng *rand.Rand) enginetest.Game {
	n := 5 + rng.Intn(6) // 5~10 人
	evil := EvilCount(n)

	// 坏人：刺客必有（没有他就没有刺杀阶段），其余从可选里挑。
	evilPool := []hiddenrole.RoleType{RoleMorgana, RoleMordred, RoleOberon}
	rng.Shuffle(len(evilPool), func(i, j int) { evilPool[i], evilPool[j] = evilPool[j], evilPool[i] })
	roles := []hiddenrole.RoleType{RoleAssassin}
	for i := 0; len(roles) < evil; i++ {
		if i < len(evilPool) && rng.Intn(2) == 0 {
			roles = append(roles, evilPool[i])
			continue
		}
		roles = append(roles, RoleMinion)
	}

	// 好人：梅林必有（没有他刺杀就没有意义），派西维尔一半机会。
	roles = append(roles, RoleMerlin)
	if rng.Intn(2) == 0 {
		roles = append(roles, RolePercival)
	}
	for len(roles) < n {
		roles = append(roles, RoleLoyalServant)
	}
	rng.Shuffle(len(roles), func(i, j int) { roles[i], roles[j] = roles[j], roles[i] })

	seats := make([]enginetest.Seat, 0, n)
	for i := 0; i < n; i++ {
		seats = append(seats, enginetest.Seat{ID: playerID(i), Role: roles[i]})
	}

	labels := []string{"大局"}
	if n == 5 {
		labels = []string{"五人局"}
	}
	has := func(want hiddenrole.RoleType) bool {
		for _, r := range roles {
			if r == want {
				return true
			}
		}
		return false
	}
	if has(RoleMordred) {
		labels = append(labels, "有莫德雷德")
	} else {
		labels = append(labels, "无莫德雷德")
	}
	if has(RoleOberon) {
		labels = append(labels, "有奥伯伦")
	}

	return enginetest.Game{
		Config:  DefaultConfig(),
		Options: Options(),
		Seats:   seats,
		Labels:  labels,
	}
}

func playerID(i int) string { return string(rune('a' + i)) }

// actRandom 这一步随便出一招。
//
// 提名要一次带一整支队伍，人数由 MissionSize 定——泛泛地随机单目标提交
// 在这里几乎必然被丢掉，对局会一直卡在提名阶段直到 hammer。
func actRandom(e *hiddenrole.Engine, rng *rand.Rand) {
	view := e.View()
	players := view.AllPlayers()
	ids := make([]string, 0, len(players))
	for _, p := range players {
		ids = append(ids, p.ID)
	}

	for _, p := range players {
		skills := e.AllowedSkills(p.ID)
		if len(skills) == 0 {
			continue
		}
		skill := skills[rng.Intn(len(skills))]

		var targets []string
		if skill == SkillPropose {
			need := MissionSize(len(ids), mission(view))
			if need == 0 || need > len(ids) {
				continue
			}
			pick := append([]string(nil), ids...)
			rng.Shuffle(len(pick), func(i, j int) { pick[i], pick[j] = pick[j], pick[i] })
			targets = pick[:need]
		}
		if skill == SkillAssassinate {
			targets = []string{ids[rng.Intn(len(ids))]}
		}

		_ = e.SubmitSkillUse(&hiddenrole.SkillUse{
			PlayerID: p.ID, Skill: skill, Targets: targets,
		})
	}
}
