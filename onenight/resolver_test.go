package onenight

import (
	"testing"

	"github.com/Zereker/hiddenrole"
)

// TestSeer_LooksAtTwoCenterCards: the seer looks at two centre cards.
//
// This takes the "index encoded into the skill name" workaround -- a centre
// card is not a player, and the kernel's target validation only knows player
// IDs. See SCARS.md, scar 1.
func TestSeer_LooksAtTwoCenterCards(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleWerewolf, RoleTanner, RoleVillager},
		at("s", RoleSeer), at("v1", RoleVillager), at("v2", RoleVillager))

	g.advance(PhaseNightSeer)
	g.use("s", SkillSeerCenter02)
	g.advance(PhaseNightRobber)

	info := g.info("s")
	if got := info["learn.center.0"]; got != string(RoleWerewolf) {
		t.Errorf("centre card 0 is the werewolf card, read %q", got)
	}
	if got := info["learn.center.2"]; got != string(RoleVillager) {
		t.Errorf("centre card 2 is the villager card, read %q", got)
	}
	if _, ok := info["learn.center.1"]; ok {
		t.Error("they looked at 0 and 2 only and should know nothing of card 1")
	}
}

// TestLoneWolf_MayPeekOnlyWhenAlone: a centre card may be peeked at only with
// a single wolf in play.
//
// "How many wolves are in play" is the rules' judgement and the kernel cannot
// block it -- it only knows this phase allows PEEK. So with two wolves the
// submission is accepted and then dropped at resolution.
func TestLoneWolf_MayPeekOnlyWhenAlone(t *testing.T) {
	t.Run("a lone wolf may look", func(t *testing.T) {
		g := newGame(t,
			[CenterCount]hiddenrole.RoleType{RoleWerewolf, RoleVillager, RoleVillager},
			at("w", RoleWerewolf), at("v1", RoleVillager), at("v2", RoleVillager))

		g.use("w", SkillPeekCenter0)
		g.advance(PhaseNightMinion)

		if got := g.info("w")["learn.center.0"]; got != string(RoleWerewolf) {
			t.Errorf("a lone wolf should see centre card 0, got %q", got)
		}
	})

	t.Run("two wolves may not", func(t *testing.T) {
		g := newGame(t,
			[CenterCount]hiddenrole.RoleType{RoleVillager, RoleVillager, RoleVillager},
			at("w1", RoleWerewolf), at("w2", RoleWerewolf), at("v", RoleVillager))

		g.use("w1", SkillPeekCenter0)
		g.advance(PhaseNightMinion)

		if got := g.info("w1")["learn.center.0"]; got != "" {
			t.Errorf("with two wolves in play the option does not exist, yet %q was seen", got)
		}
	})
}

// TestRobber_LearnsWhatHeTook: the robber knows what they took, and the
// player robbed does not.
func TestRobber_LearnsWhatHeTook(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("r", RoleRobber), at("w", RoleWerewolf), at("v", RoleVillager))

	g.advance(PhaseNightRobber)
	g.use("r", SkillRob, "w")
	g.advance(PhaseNightTroublemake)

	if got := g.info("r")["learn.self"]; got != string(RoleWerewolf) {
		t.Errorf("the robber should know they took the werewolf card, got %q", got)
	}
	if got := g.info("w")["learn.self"]; got != "" {
		t.Errorf("the player robbed should not know what they now hold, got %q", got)
	}
}

// TestNightActions_AreAllOptional: night abilities are all optional, and the
// phase advances without a submission.
//
// The rules word every one of them as "you **may**...". The configuration
// therefore does not mark them Required, so PhaseReadiness lists them under
// Optional rather than Pending.
func TestNightActions_AreAllOptional(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("s", RoleSeer), at("r", RoleRobber), at("v", RoleVillager))

	g.advance(PhaseNightSeer)
	rd := g.e.PhaseReadiness()
	if !rd.Ready {
		t.Errorf("night abilities are optional, so the phase should be ready from the start, Pending=%v", rd.Pending)
	}
	if len(rd.Optional) == 0 {
		t.Error("the seer may act and has not, so they should appear in Optional")
	}
}

// TestVote_IsRequiredOfEveryone: everyone must vote.
func TestVote_IsRequiredOfEveryone(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("w", RoleWerewolf), at("v1", RoleVillager), at("v2", RoleVillager))

	g.advance(PhaseVote)
	if rd := g.e.PhaseReadiness(); rd.Ready || len(rd.Pending) != 3 {
		t.Errorf("none of the three has voted, so three votes are owed, got Ready=%v Pending=%v", rd.Ready, rd.Pending)
	}

	g.use("w", SkillVote, "v1")
	if rd := g.e.PhaseReadiness(); len(rd.Pending) != 2 {
		t.Errorf("one vote is in, so two are still owed, got %v", rd.Pending)
	}
}

// TestVote_CannotVoteForSelf: you may not vote for yourself.
//
// The kernel stays out of this -- "may you vote for yourself" is the rules'
// judgement. The submission is accepted and dropped at resolution.
func TestVote_CannotVoteForSelf(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("w", RoleWerewolf), at("v1", RoleVillager), at("v2", RoleVillager))

	g.advance(PhaseVote)
	g.use("w", SkillVote, "w")
	g.use("v1", SkillVote, "w")
	g.use("v2", SkillVote, "w")
	g.end(hiddenrole.PhaseEnd)

	if p, _ := g.e.PlayerInfo("w"); p.Alive {
		t.Error("two votes for the wolf, so the wolf should be eliminated")
	}
}

// TestCenterIndexes: reading a centre card's index off a skill name.
func TestCenterIndexes(t *testing.T) {
	cases := []struct {
		skill hiddenrole.SkillType
		want  []int
	}{
		{SkillPeekCenter0, []int{0}},
		{SkillDrinkCenter2, []int{2}},
		{SkillSeerCenter01, []int{0, 1}},
		{SkillSeerCenter12, []int{1, 2}},
		{SkillSeerPlayer, nil},      // carries no index
		{SkillRob, nil},             // likewise
		{hiddenrole.SkillSkip, nil}, // has no underscore
	}
	for _, c := range cases {
		got := centerIndexes(c.skill)
		if len(got) != len(c.want) {
			t.Errorf("centerIndexes(%v) = %v, want %v", c.skill, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("centerIndexes(%v) = %v, want %v", c.skill, got, c.want)
				break
			}
		}
	}
}

// TestEffectOrderIsDeterminedByTheBoard: resolving the same board must produce
// effects in a stable order.
//
// Same rule as the first two packages: replaying and comparing effect logs
// rests entirely on it. The vote's elimination list is a map, and producing
// effects by iterating it directly would give a different order on every
// resolution of the same board.
func TestEffectOrderIsDeterminedByTheBoard(t *testing.T) {
	build := func() []*hiddenrole.Effect {
		g := newGame(t,
			[CenterCount]hiddenrole.RoleType{RoleVillager, RoleVillager, RoleVillager},
			at("a", RoleHunter), at("b", RoleWerewolf),
			at("c", RoleVillager), at("d", RoleVillager))
		g.advance(PhaseVote)
		g.use("c", SkillVote, "a")
		g.use("d", SkillVote, "a")
		g.use("b", SkillVote, "a")
		g.use("a", SkillVote, "b")
		out, err := g.e.EndPhase()
		if err != nil {
			t.Fatalf("EndPhase: %v", err)
		}
		return out
	}

	first := build()
	for i := 0; i < 20; i++ {
		again := build()
		if len(again) != len(first) {
			t.Fatalf("resolution %d produced %d effects, the first produced %d", i, len(again), len(first))
		}
		for j := range first {
			if again[j].Type != first[j].Type || again[j].TargetID != first[j].TargetID {
				t.Fatalf("resolution %d, effect %d differs from the first: %v/%v vs %v/%v",
					i, j, again[j].Type, again[j].TargetID, first[j].Type, first[j].TargetID)
			}
		}
	}
}
