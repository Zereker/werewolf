package missions

import (
	"math/rand"
	"testing"

	"github.com/Zereker/hiddenrole"
	"github.com/Zereker/hiddenrole/enginetest"
)

// TestFuzz_Invariants runs random games against the general invariants.
//
// The invariants live in hiddenrole/enginetest and none of them knows this
// ruleset. This file only lays out boards and takes turns.
//
// What is randomised is **the table size and the role assignment**: 5 to 10
// players, the number of evil players from the EvilCount table, and who gets
// which special role at random. Almost every branch in this ruleset comes from
// the role configuration (whether Mordred is in decides whether Merlin sees
// every bad guy; whether Oberon is in decides whether the evil side knows
// itself).
//
// A game here is far longer than in the other two packages: one mission takes
// nomination, vote and mission, a rejected vote loops back to nomination, and
// there are five missions plus the assassination -- so MaxSteps is generous.
func TestFuzz_Invariants(t *testing.T) {
	enginetest.RunFuzz(t, enginetest.FuzzSpec{
		Games:    1000, // 5000 games across the three packages; this one has the longest games, so 1000
		MaxSteps: 400,
		WantEnd:  true,
		Setup:    setupRandom,
		Act:      actRandom,
		MustSee: []string{
			"five players", "bigger table", "with Mordred", "without Mordred", "with Oberon",
		},
	})
}

// setupRandom lays out a random board.
func setupRandom(rng *rand.Rand) enginetest.Game {
	n := 5 + rng.Intn(6) // 5 to 10 players
	evil := EvilCount(n)

	// The evil side: the assassin is always in (without him there is no
	// assassination phase), and the rest are picked from the optional ones.
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

	// The good side: Merlin is always in (without him the assassination is
	// pointless), and Percival half the time.
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

	labels := []string{"bigger table"}
	if n == 5 {
		labels = []string{"five players"}
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
		labels = append(labels, "with Mordred")
	} else {
		labels = append(labels, "without Mordred")
	}
	if has(RoleOberon) {
		labels = append(labels, "with Oberon")
	}

	return enginetest.Game{
		Config:  DefaultConfig(),
		Options: Options(),
		Seats:   seats,
		Labels:  labels,
	}
}

func playerID(i int) string { return string(rune('a' + i)) }

// actRandom takes one random turn.
//
// A nomination carries a whole team at once, sized by MissionSize -- a
// generically random single-target submission would nearly always be thrown
// away here, and the game would sit in the nomination phase until the hammer.
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
