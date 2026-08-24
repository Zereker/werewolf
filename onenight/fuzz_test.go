package onenight

import (
	"math/rand"
	"testing"

	"github.com/Zereker/hiddenrole"
	"github.com/Zereker/hiddenrole/enginetest"
)

// TestFuzz_Invariants runs random games against the general invariants.
//
// The invariants live in hiddenrole/enginetest and none of them knows this
// ruleset -- they ask kernel-level questions only: does what was stored read
// back the same, does replay arrive at the same board, is somebody the engine
// says cannot act really unable to act. This file only lays out boards and
// takes turns.
//
// What is randomised is not only play but **the deal**: who gets what, and
// which three cards stay in the centre. Almost every branch in this ruleset is
// decided by the deal (how many wolves are in play, whether the tanner is in,
// whether all the wolf cards are in the centre), and randomising play alone
// would collapse the search space into a line.
func TestFuzz_Invariants(t *testing.T) {
	enginetest.RunFuzz(t, enginetest.FuzzSpec{
		Games:    2000, // 5000 games across the three packages; this one has the shortest games, so it runs more
		MaxSteps: 40,
		WantEnd:  true,
		Setup:    setupRandom,
		Act:      actRandom,
		MustSee: []string{
			"wolf in play", "all wolves in centre", "tanner in", "hunter in", "three players", "more players",
		},
	})
}

// deck lays out a random deal: one card per player, plus three.
func setupRandom(rng *rand.Rand) enginetest.Game {
	n := MinPlayers + rng.Intn(4) // 3 to 6 players
	pool := []hiddenrole.RoleType{
		RoleWerewolf, RoleWerewolf, RoleMinion, RoleMason, RoleMason,
		RoleSeer, RoleRobber, RoleTroublemaker, RoleDrunk, RoleInsomniac,
		RoleVillager, RoleVillager, RoleHunter, RoleTanner,
	}
	rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })

	seats := make([]enginetest.Seat, 0, n)
	for i := 0; i < n; i++ {
		seats = append(seats, enginetest.Seat{ID: playerID(i), Role: pool[i]})
	}
	var center [CenterCount]hiddenrole.RoleType
	copy(center[:], pool[n:n+CenterCount])

	// Labels, to watch for the randomisation degenerating. Every branch that
	// matters in this ruleset comes from the deal.
	var labels []string
	wolvesInPlay := 0
	for _, s := range seats {
		switch s.Role {
		case RoleWerewolf:
			wolvesInPlay++
		case RoleTanner:
			labels = append(labels, "tanner in")
		case RoleHunter:
			labels = append(labels, "hunter in")
		}
	}
	if wolvesInPlay > 0 {
		labels = append(labels, "wolf in play")
	} else {
		labels = append(labels, "all wolves in centre")
	}
	if n == MinPlayers {
		labels = append(labels, "three players")
	} else {
		labels = append(labels, "more players")
	}

	return enginetest.Game{
		Config:  GameConfig(),
		Options: Options(center),
		Seats:   seats,
		Labels:  labels,
	}
}

func playerID(i int) string { return string(rune('a' + i)) }

// actRandom takes one random turn.
//
// A generically random submission is nearly always rejected on a multi-target
// skill (the troublemaker needs exactly two players), so the number of targets
// is chosen per skill. A rejected submission is not a failure -- the rules drop
// illegal submissions by design, and that is one of the behaviours under test.
func actRandom(e *hiddenrole.Engine, rng *rand.Rand) {
	players := e.View().AllPlayers()
	ids := make([]string, 0, len(players))
	for _, p := range players {
		ids = append(ids, p.ID)
	}

	for _, p := range players {
		skills := e.AllowedSkills(p.ID)
		if len(skills) == 0 || rng.Intn(4) == 0 {
			continue // a one-in-four chance of doing nothing -- night abilities are optional anyway
		}
		skill := skills[rng.Intn(len(skills))]

		var targets []string
		switch skill {
		case SkillMeddle:
			// "two other players": not themselves, and not the same person twice
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

		_ = e.SubmitSkillUse(&hiddenrole.SkillUse{
			PlayerID: p.ID, Skill: skill, Targets: targets,
		})
	}
}

// without removes one player from a list.
func without(ids []string, drop string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != drop {
			out = append(out, id)
		}
	}
	return out
}
