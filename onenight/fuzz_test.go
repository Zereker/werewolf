package onenight

import (
	"math/rand"
	"testing"

	"github.com/Zereker/werewolf/engine"
	"github.com/Zereker/werewolf/internal/gamefuzz"
)

// TestFuzz_Invariants 随机对局，核对通用不变量。
//
// 不变量在 internal/gamefuzz 里，一条都不认识这套规则——它们问的全是内核
// 层面的事：存了再读回来一样吗、回放走到同一个局面吗、说不能行动的人是不是
// 真的不能行动。这里只负责摆局面与出招。
//
// 随机的不只是打法，还有**牌的分配**：谁拿到什么、哪三张留在中央。
// 这一套规则的分歧几乎全由发牌决定（场上有几只狼、皮匠在不在场、
// 狼牌是不是全在中央），只随机打法的话搜索空间会塌成一条线。
func TestFuzz_Invariants(t *testing.T) {
	gamefuzz.Run(t, gamefuzz.Config{
		Games:    300,
		MaxSteps: 40,
		WantEnd:  true,
		Setup:    setupRandom,
		Act:      actRandom,
		MustSee: []string{
			"场上有狼", "狼全在中央", "有皮匠", "有猎人", "三人局", "多人局",
		},
	})
}

// deck 随机摆一副牌：人数 + 3 张。
func setupRandom(rng *rand.Rand) gamefuzz.Game {
	n := MinPlayers + rng.Intn(4) // 3~6 人
	pool := []engine.RoleType{
		RoleWerewolf, RoleWerewolf, RoleMinion, RoleMason, RoleMason,
		RoleSeer, RoleRobber, RoleTroublemaker, RoleDrunk, RoleInsomniac,
		RoleVillager, RoleVillager, RoleHunter, RoleTanner,
	}
	rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })

	seats := make([]gamefuzz.Seat, 0, n)
	for i := 0; i < n; i++ {
		seats = append(seats, gamefuzz.Seat{ID: playerID(i), Role: pool[i]})
	}
	var center [CenterCount]engine.RoleType
	copy(center[:], pool[n:n+CenterCount])

	// 标签：盯住随机化有没有退化。这套规则最要紧的分歧全在发牌上。
	var labels []string
	wolvesInPlay := 0
	for _, s := range seats {
		switch s.Role {
		case RoleWerewolf:
			wolvesInPlay++
		case RoleTanner:
			labels = append(labels, "有皮匠")
		case RoleHunter:
			labels = append(labels, "有猎人")
		}
	}
	if wolvesInPlay > 0 {
		labels = append(labels, "场上有狼")
	} else {
		labels = append(labels, "狼全在中央")
	}
	if n == MinPlayers {
		labels = append(labels, "三人局")
	} else {
		labels = append(labels, "多人局")
	}

	return gamefuzz.Game{
		Config:  GameConfig(),
		Options: Options(center),
		Seats:   seats,
		Labels:  labels,
	}
}

func playerID(i int) string { return string(rune('a' + i)) }

// actRandom 这一步随便出一招。
//
// 泛泛地随机提交在多目标技能上几乎必然被拒（捣蛋鬼要正好两个人），
// 所以按技能挑目标个数。提交被拒不算错——规则本来就会丢掉不合法的提交，
// 那也是被测的行为之一。
func actRandom(e *engine.Engine, rng *rand.Rand) {
	players := e.View().AllPlayers()
	ids := make([]string, 0, len(players))
	for _, p := range players {
		ids = append(ids, p.ID)
	}

	for _, p := range players {
		skills := e.AllowedSkills(p.ID)
		if len(skills) == 0 || rng.Intn(4) == 0 {
			continue // 四分之一的机会不动——夜晚能力本来就是可选的
		}
		skill := skills[rng.Intn(len(skills))]

		var targets []string
		switch skill {
		case SkillMeddle:
			// 「另外两名」：不含自己，且两人不同
			others := without(ids, p.ID)
			if len(others) < 2 {
				continue
			}
			rng.Shuffle(len(others), func(i, j int) { others[i], others[j] = others[j], others[i] })
			targets = others[:2]
		case SkillSeerPlayer, SkillRob, SkillVote:
			others := without(ids, p.ID)
			if len(others) == 0 {
				continue
			}
			targets = []string{others[rng.Intn(len(others))]}
		}

		_ = e.SubmitSkillUse(&engine.SkillUse{
			PlayerID: p.ID, Skill: skill, Targets: targets,
		})
	}
}

// without 名单里去掉某个人。
func without(ids []string, drop string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != drop {
			out = append(out, id)
		}
	}
	return out
}
