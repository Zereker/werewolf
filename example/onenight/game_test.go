package onenight

import (
	"testing"

	"github.com/Zereker/hiddenrole"
)

// game is one game for tests, plus a few assertion helpers.
type game struct {
	t *testing.T
	e *hiddenrole.Engine
}

// newGame starts a game: seats maps player ID to the card they were dealt,
// and center is the three centre cards.
func newGame(t *testing.T, center [CenterCount]hiddenrole.RoleType, seats ...seat) *game {
	t.Helper()

	e, err := hiddenrole.NewEngine(GameConfig(), Options(center)...)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	for _, s := range seats {
		if err := e.AddPlayer(s.id, s.role); err != nil {
			t.Fatalf("AddPlayer(%s): %v", s.id, err)
		}
	}
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return &game{t: t, e: e}
}

type seat struct {
	id   string
	role hiddenrole.RoleType
}

func at(id string, role hiddenrole.RoleType) seat { return seat{id, role} }

// use submits a skill, failing the test on error.
func (g *game) use(playerID string, skill hiddenrole.SkillType, targets ...string) {
	g.t.Helper()
	err := g.e.SubmitSkillUse(&hiddenrole.SkillUse{
		PlayerID: playerID, Skill: skill, Targets: targets,
	})
	if err != nil {
		g.t.Fatalf("%s submitting %v: %v", playerID, skill, err)
	}
}

// end ends the current phase and asserts it landed on want.
func (g *game) end(want hiddenrole.PhaseType) {
	g.t.Helper()
	if _, err := g.e.EndPhase(); err != nil {
		g.t.Fatalf("EndPhase: %v", err)
	}
	if got := g.e.Status().Phase; got != want {
		g.t.Fatalf("phase = %v, want %v", got, want)
	}
}

// advance keeps going until it reaches a given phase.
//
// Easier to count than end: end means "end the current phase and land on
// want", and chaining several is easy to get off by one.
func (g *game) advance(to hiddenrole.PhaseType) {
	g.t.Helper()
	for i := 0; i < 20; i++ {
		if g.e.Status().Phase == to {
			return
		}
		if _, err := g.e.EndPhase(); err != nil {
			g.t.Fatalf("advancing to %v: %v", to, err)
		}
	}
	g.t.Fatalf("20 steps and still not at %v, currently at %v", to, g.e.Status().Phase)
}

// toVote runs all the way to the vote, taking no night actions on the way.
func (g *game) toVote() {
	g.t.Helper()
	for _, p := range []hiddenrole.PhaseType{
		PhaseNightMinion, PhaseNightMason, PhaseNightSeer, PhaseNightRobber,
		PhaseNightTroublemake, PhaseNightDrunk, PhaseNightInsomniac,
		PhaseDay, PhaseVote,
	} {
		g.end(p)
	}
}

// card is the card this player now holds.
func (g *game) card(playerID string) hiddenrole.RoleType {
	g.t.Helper()
	return card(g.e.View(), playerID)
}

// info is the role information in this player's view.
func (g *game) info(playerID string) map[string]string {
	g.t.Helper()
	v := g.e.PlayerView(playerID)
	if v == nil {
		g.t.Fatalf("%s has no view", playerID)
	}
	return v.RoleInfo
}

// TestGame_FullNightAndVote plays one whole game: cards swap at night, the
// table votes in the day, and an outcome is decided.
//
// This was this rules package's first test, and what it checks is whether the
// thing can run on this kernel at all.
func TestGame_FullNightAndVote(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleVillager, RoleWerewolf, RoleVillager},
		at("w", RoleWerewolf), at("s", RoleSeer), at("r", RoleRobber),
		at("t", RoleTroublemaker), at("v", RoleVillager))

	// The wolf: is there a second one? No -- the other card is in the centre,
	// so this one may look at centre card 0.
	g.use("w", SkillPeekCenter0)
	g.end(PhaseNightMinion)
	g.end(PhaseNightMason)
	g.end(PhaseNightSeer)

	// The seer looks at the wolf.
	g.use("s", SkillSeerPlayer, "w")
	g.end(PhaseNightRobber)

	// The robber steals the wolf's card -- which puts them on the wolf team,
	// but they will **not** do anything wolfish for the rest of the night.
	g.use("r", SkillRob, "w")
	g.end(PhaseNightTroublemake)

	if g.card("r") != RoleWerewolf {
		t.Fatalf("the robber stole the wolf's card and should now hold WEREWOLF, got %v", g.card("r"))
	}
	if g.card("w") != RoleRobber {
		t.Fatalf("the wolf's card was stolen, so they should now hold ROBBER, got %v", g.card("w"))
	}

	// The troublemaker swaps the villager's and the seer's cards; neither
	// knows.
	g.use("t", SkillMeddle, "v", "s")
	g.end(PhaseNightDrunk)
	g.end(PhaseNightInsomniac)
	g.end(PhaseDay)
	g.end(PhaseVote)

	// What the seer saw is still the card at **that moment**, not the one
	// there now.
	if got := g.info("s")["learn.player.w"]; got != string(RoleWerewolf) {
		t.Errorf("the seer saw WEREWOLF at the time, now reading %q", got)
	}
	if g.card("w") == RoleWerewolf {
		t.Error("the wolf's card was already stolen; this test did not exercise information going stale")
	}

	// Everyone votes for the robber -- who now holds the werewolf card, so the
	// village wins.
	for _, id := range []string{"w", "s", "t", "v"} {
		g.use(id, SkillVote, "r")
	}
	g.use("r", SkillVote, "w")
	g.end(hiddenrole.PhaseEnd)

	st := g.e.Status()
	if !st.Over {
		t.Fatal("the game should end once the vote resolves")
	}
	if !Won(st.Winner, CampVillage) {
		t.Errorf("the eliminated player held the werewolf card, so the village should win, got %v", st.Winner)
	}
	if p, _ := g.e.PlayerInfo("r"); p.Alive {
		t.Error("whoever got the most votes should be eliminated")
	}
}

// voteAll has everyone vote for the same person.
func (g *game) voteAll(target string) {
	g.t.Helper()
	for _, p := range g.e.View().AllPlayers() {
		want := target
		if p.ID == target {
			// You may not vote for yourself; pick anyone else.
			for _, q := range g.e.View().AllPlayers() {
				if q.ID != target {
					want = q.ID
					break
				}
			}
		}
		g.use(p.ID, SkillVote, want)
	}
}

// TestVictory_WolfDies: at least one werewolf eliminated -> the village wins.
func TestVictory_WolfDies(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("w", RoleWerewolf), at("v1", RoleVillager), at("v2", RoleVillager))

	g.toVote()
	g.voteAll("w")
	g.end(hiddenrole.PhaseEnd)

	if got := g.e.Status().Winner; !Won(got, CampVillage) {
		t.Errorf("a wolf was eliminated, so the village should win, got %v", got)
	}
	if Won(g.e.Status().Winner, CampWolf) {
		t.Error("a wolf was eliminated, so the wolves should not win")
	}
}

// TestVictory_NoWolfDies: a wolf in play and no wolf eliminated -> the wolves
// win.
func TestVictory_NoWolfDies(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("w", RoleWerewolf), at("v1", RoleVillager), at("v2", RoleVillager))

	g.toVote()
	g.voteAll("v1")
	g.end(hiddenrole.PhaseEnd)

	if got := g.e.Status().Winner; !Won(got, CampWolf) {
		t.Errorf("no wolf was eliminated, so the wolves should win, got %v", got)
	}
}

// TestVictory_NoWolfInPlayAndNobodyDies: no wolf in play and nobody eliminated
// -> the village wins.
//
// "Nobody is eliminated" is reached by everyone getting exactly one vote --
// written in the official rules, not a special case of a tie. Three players
// each voting for a different person gives exactly one vote each.
func TestVictory_NoWolfInPlayAndNobodyDies(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleWerewolf, RoleVillager, RoleVillager},
		at("v1", RoleVillager), at("v2", RoleSeer), at("v3", RoleVillager))

	g.toVote()
	g.use("v1", SkillVote, "v2")
	g.use("v2", SkillVote, "v3")
	g.use("v3", SkillVote, "v1")
	g.end(hiddenrole.PhaseEnd)

	for _, p := range g.e.View().AllPlayers() {
		if !p.Alive {
			t.Fatalf("with one vote each nobody should be eliminated, yet %s was", p.ID)
		}
	}
	if got := g.e.Status().Winner; !Won(got, CampVillage) {
		t.Errorf("no wolf in play and nobody eliminated, so the village should win, got %v", got)
	}
}

// TestVictory_TannerDiesAlone: the tanner eliminated with no wolf eliminated
// -> the tanner wins alone and the wolves do not.
func TestVictory_TannerDiesAlone(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("w", RoleWerewolf), at("tn", RoleTanner), at("v", RoleVillager))

	g.toVote()
	g.voteAll("tn")
	g.end(hiddenrole.PhaseEnd)

	got := g.e.Status().Winner
	if !Won(got, CampTanner) {
		t.Errorf("the tanner was eliminated and should win, got %v", got)
	}
	if Won(got, CampWolf) {
		t.Errorf("with the tanner eliminated the wolves should not win, got %v", got)
	}
	if Won(got, CampVillage) {
		t.Errorf("no wolf was eliminated, so the village should not win, got %v", got)
	}
}

// TestVictory_TannerAndWolfBothDie: the tanner and a wolf both eliminated ->
// the tanner and the village both win.
//
// **This is an answer the kernel cannot give**: VictoryChecker returns one
// Camp, and here there are two winners. This package packs them into one
// string ("TANNER+VILLAGE"), and the encoding and decoding rules have to be
// carried by the rules package. See SCARS.md, scar 5.
func TestVictory_TannerAndWolfBothDie(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("w", RoleWerewolf), at("tn", RoleTanner),
		at("v1", RoleVillager), at("v2", RoleVillager))

	g.toVote()
	// The wolf and the tanner each get two votes and tie at the top, so both
	// are eliminated. Nobody may vote for themselves, hence the routing:
	g.use("v1", SkillVote, "w")
	g.use("tn", SkillVote, "w") // wolf: 2 votes
	g.use("v2", SkillVote, "tn")
	g.use("w", SkillVote, "tn") // tanner: 2 votes
	g.end(hiddenrole.PhaseEnd)

	for _, id := range []string{"w", "tn"} {
		if p, _ := g.e.PlayerInfo(id); p.Alive {
			t.Fatalf("on a tie everyone at the top should be eliminated, yet %s is alive", id)
		}
	}

	got := g.e.Status().Winner
	if !Won(got, CampTanner) {
		t.Errorf("the tanner was eliminated and should win, got %v", got)
	}
	if !Won(got, CampVillage) {
		t.Errorf("a wolf was eliminated, so the village should also win, got %v", got)
	}
	if len(Winners(got)) != 2 {
		t.Errorf("there should be two winners, got %v", Winners(got))
	}
}

// TestHunter_TakesHisVoteWithHim: when the hunter is eliminated, so is
// whoever they voted for.
//
// "The hunter" is decided by **the card in their hand now**: what is revealed
// in the morning is the card in hand, so whoever holds the hunter card is the
// hunter, and the player dealt the hunter card who had it swapped away is
// not.
func TestHunter_TakesHisVoteWithHim(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("h", RoleHunter), at("w", RoleWerewolf),
		at("v1", RoleVillager), at("v2", RoleVillager))

	g.toVote()
	g.use("v1", SkillVote, "h")
	g.use("v2", SkillVote, "h")
	g.use("w", SkillVote, "h") // the hunter takes three votes and is eliminated
	g.use("h", SkillVote, "w") // they voted for the wolf, who goes down with them
	g.end(hiddenrole.PhaseEnd)

	if p, _ := g.e.PlayerInfo("h"); p.Alive {
		t.Fatal("the hunter got the most votes and should be eliminated")
	}
	if p, _ := g.e.PlayerInfo("w"); p.Alive {
		t.Fatal("an eliminated hunter should take whoever they voted for with them")
	}
	if got := g.e.Status().Winner; !Won(got, CampVillage) {
		t.Errorf("the hunter took the wolf down, so the village should win, got %v", got)
	}
}

// TestNightAbilityFollowsDealtCard_NotHeldCard
// What you do at night is decided by the card you were **dealt**, not the one
// in your hand now.
//
// This is the ruleset's pivot and what most sets it apart from the first two.
// A robber who steals the werewolf card does not become a wolf and does not
// wake with them; and the wolf whose card was stolen **had already acted**
// that night.
func TestNightAbilityFollowsDealtCard_NotHeldCard(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("w", RoleWerewolf), at("r", RoleRobber),
		at("i", RoleInsomniac), at("v", RoleVillager))

	// The wolf phase: a single wolf in play, so they may look at a centre card.
	g.use("w", SkillPeekCenter1)

	// The robber steals the wolf's card. A submission only resolves when the
	// phase ends, so advance past it first.
	g.advance(PhaseNightRobber)
	g.use("r", SkillRob, "w")
	g.advance(PhaseNightTroublemake)

	if g.card("r") != RoleWerewolf {
		t.Fatalf("the robber should now hold the werewolf card, got %v", g.card("r"))
	}

	// The robber now holds the werewolf card, and their **teammate list** is
	// empty -- recognition happened before the theft, and goes by the card
	// dealt.
	if mates := g.e.Teammates("r"); len(mates) != 0 {
		t.Errorf("the robber should not appear in the wolves' teammate relation, got %v", mates)
	}
	// The other way round: the wolf's card was stolen and they still know they
	// were dealt the wolf.
	if got := g.e.PlayerView("w").Self.Role; got != RoleWerewolf {
		t.Errorf("the card dealt should not change, got %v", got)
	}

	g.advance(PhaseDay)

	// The insomniac acts last and sees the result of every swap.
	if got := g.info("i")["learn.self"]; got != string(RoleInsomniac) {
		t.Errorf("nobody touched the insomniac's card, so they should see INSOMNIAC, got %q", got)
	}
}

// TestDrunk_DoesNotKnowWhatHeHolds: the drunk swapped a card and does not know
// what they got.
//
// This is what the kernel's "no free-form state bag in a player-facing struct"
// rule is worth in this ruleset: the drunk's card is a piece of game-long
// state, and if the kernel handed Vars to players by default, this role would
// not work at all.
func TestDrunk_DoesNotKnowWhatHeHolds(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleWerewolf, RoleVillager, RoleVillager},
		at("d", RoleDrunk), at("v1", RoleVillager), at("v2", RoleVillager))

	g.advance(PhaseNightDrunk)
	g.use("d", SkillDrinkCenter0)
	g.advance(PhaseNightInsomniac)

	if g.card("d") != RoleWerewolf {
		t.Fatalf("the drunk should have taken centre card 0 (the werewolf card), got %v", g.card("d"))
	}

	view := g.e.PlayerView("d")
	for k, v := range view.RoleInfo {
		t.Errorf("the drunk should know nothing, yet sees %s=%s", k, v)
	}
	if view.Self.Camp != hiddenrole.CampUnspecified {
		t.Errorf("the drunk should not know which side they now count for, Self.Camp = %v", view.Self.Camp)
	}
	if view.Self.Role != RoleDrunk {
		t.Errorf("all they know is \"I was dealt the drunk\", got %v", view.Self.Role)
	}
}

// TestTroublemaker_VictimsAreNotTold: the two players the troublemaker swapped
// are not told.
func TestTroublemaker_VictimsAreNotTold(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("t", RoleTroublemaker), at("w", RoleWerewolf),
		at("v1", RoleVillager), at("v2", RoleVillager))

	g.advance(PhaseNightTroublemake)
	g.use("t", SkillMeddle, "w", "v1")
	g.advance(PhaseNightDrunk)

	if g.card("w") != RoleVillager || g.card("v1") != RoleWerewolf {
		t.Fatalf("the two cards should have been swapped, got w=%v v1=%v", g.card("w"), g.card("v1"))
	}
	for _, id := range []string{"w", "v1", "t"} {
		if got := g.info(id)["learn.self"]; got != "" {
			t.Errorf("%s should not know what they now hold, yet sees %q", id, got)
		}
	}
}
