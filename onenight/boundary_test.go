package onenight

import (
	"sort"
	"strings"
	"testing"

	"github.com/Zereker/hiddenrole"
)

// TestBoundary_WolvesRecogniseEachOtherMinionSeesThemNotViceVersa
// The wolves recognise each other; the minion sees the wolves and the wolves
// do not see the minion.
//
// This is a **one-way** asymmetry, the opposite direction from the missions
// package's Oberon (isolated both ways). The kernel allows asymmetry precisely
// for cases like these.
func TestBoundary_WolvesRecogniseEachOtherMinionSeesThemNotViceVersa(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("w1", RoleWerewolf), at("w2", RoleWerewolf),
		at("m", RoleMinion), at("v", RoleVillager))

	if got := g.info("w1")["wolves"]; got != "w2" {
		t.Errorf("w1 should recognise w2, got %q", got)
	}
	if got := g.info("m")["wolves"]; got != "w1,w2" {
		t.Errorf("the minion should see both wolves, got %q", got)
	}
	// The wolves' information does not include the minion -- one-way.
	for _, id := range []string{"w1", "w2"} {
		if strings.Contains(g.info(id)["wolves"], "m") {
			t.Errorf("%s should not see the minion", id)
		}
	}
	// The teammate relation likewise holds between wolves only.
	if mates := g.e.Teammates("m"); len(mates) != 0 {
		t.Errorf("the minion is nobody's teammate, got %v", mates)
	}
	if got := g.info("v")["wolves"]; got != "" {
		t.Errorf("a villager should see nothing, got %q", got)
	}
}

// TestBoundary_LoneWolfSeesEmptyList: a lone wolf sees an empty list.
//
// "The list is empty" is itself information: it means "I am the lone wolf" and
// may go and look at a centre card. So an empty list still has to be
// delivered, not withheld for being empty.
func TestBoundary_LoneWolfSeesEmptyList(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleWerewolf, RoleVillager, RoleVillager},
		at("w", RoleWerewolf), at("v1", RoleVillager), at("v2", RoleVillager))

	info := g.info("w")
	got, ok := info["wolves"]
	if !ok {
		t.Fatal("a lone wolf should still get the wolves entry -- an empty list is information")
	}
	if got != "" {
		t.Errorf("with a single wolf in play the list should be empty, got %q", got)
	}
}

// TestBoundary_LoneMasonKnowsTheOtherIsInCenter: with a single mason the list
// is empty.
func TestBoundary_LoneMasonKnowsTheOtherIsInCenter(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleMason, RoleVillager, RoleVillager},
		at("m", RoleMason), at("v1", RoleVillager), at("v2", RoleVillager))

	if got, ok := g.info("m")["masons"]; !ok || got != "" {
		t.Errorf("the other mason is in the centre, so the list should be empty, got %q (present=%v)", got, ok)
	}
}

// TestBoundary_NightEventsGoOnlyToTheActor: what happens at night is told only
// to the person it happened to.
//
// The troublemaker's case matters most: the two players swapped must not know
// either.
func TestBoundary_NightEventsGoOnlyToTheActor(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("t", RoleTroublemaker), at("w", RoleWerewolf),
		at("v1", RoleVillager), at("v2", RoleVillager))

	cases := []struct {
		event *hiddenrole.Event
		want  []string
		why   string
	}{
		{hiddenrole.NewEffect(EventMeddled, "t", "").ToEvent(), []string{"t"}, "only the troublemaker knows"},
		{hiddenrole.NewEffect(EventSeerLook, "v1", "w").ToEvent(), []string{"v1"}, "only the seer knows"},
		{hiddenrole.NewEffect(EventDrunkSwap, "v2", "").ToEvent(), []string{"v2"}, "even the drunk only knows that they swapped"},
		{hiddenrole.NewEffect(EventLynched, "", "w").ToEvent(), []string{"t", "v1", "v2", "w"}, "elimination is public"},
		{hiddenrole.NewEffect(EventVoted, "v1", "w").ToEvent(), []string{"t", "v1", "v2", "w"}, "the vote is public"},
		{hiddenrole.NewEffect(EventNoOneDies, "", "").ToEvent(), []string{"t", "v1", "v2", "w"}, "nobody being eliminated is public"},
	}

	for _, c := range cases {
		got, known := g.e.AudienceOf(c.event)
		if !known {
			t.Errorf("%v should have a definite audience verdict (%s)", c.event.Type, c.why)
			continue
		}
		sort.Strings(got)
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("audience of %v = %v, want %v (%s)", c.event.Type, got, c.want, c.why)
		}
	}

	// An event the rules did not speak on comes back as "don't know", for the
	// caller to route.
	if _, known := g.e.AudienceOf(hiddenrole.NewEffect(hiddenrole.EventType("SOMETHING_ELSE"), "t", "").ToEvent()); known {
		t.Error("this package did not speak on this event, so the answer should be \"don't know\"")
	}
}

// TestBoundary_StatePrimitivesNeverReachPlayers: not one state primitive is
// sent out.
//
// It matters especially in this ruleset: "player 3 now holds the werewolf card"
// is a SET_VAR. It is the kernel's one non-configurable rule, and it holds
// whether or not this package installs an AudienceProvider.
func TestBoundary_StatePrimitivesNeverReachPlayers(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("w", RoleWerewolf), at("v1", RoleVillager), at("v2", RoleVillager))

	primitives := []*hiddenrole.Effect{
		setCard("v1", RoleWerewolf),
		setCenterCard(0, RoleWerewolf),
		learnSelf("v1", RoleWerewolf),
		hiddenrole.NewSetAliveEffect("v1", false),
	}
	for _, ef := range primitives {
		got, known := g.e.AudienceOf(ef.ToEvent())
		if !known {
			t.Errorf("%v should be a definite verdict, not an \"I don't know\"", ef.Type)
		}
		if len(got) != 0 {
			t.Errorf("%v is a state primitive and should go to nobody, got %v", ef.Type, got)
		}
	}
}

// TestBoundary_SpeechIsPublicAllGame: speech is audible to everyone,
// throughout.
func TestBoundary_SpeechIsPublicAllGame(t *testing.T) {
	g := newGame(t,
		[CenterCount]hiddenrole.RoleType{RoleVillager, RoleVillager, RoleVillager},
		at("w", RoleWerewolf), at("v1", RoleVillager), at("v2", RoleVillager))

	g.advance(PhaseDay)
	got := g.e.MessageReceivers("w")
	sort.Strings(got)
	if strings.Join(got, ",") != "v1,v2,w" {
		t.Errorf("speech should be audible to everyone, got %v", got)
	}
}

// TestCampOf: the camp computed when cards are revealed. The minion is on the
// wolf team but is not a werewolf card.
func TestCampOf(t *testing.T) {
	cases := []struct {
		role hiddenrole.RoleType
		want hiddenrole.Camp
	}{
		{RoleWerewolf, CampWolf},
		{RoleMinion, CampWolf},
		{RoleTanner, CampTanner},
		{RoleVillager, CampVillage},
		{RoleSeer, CampVillage},
		{RoleHunter, CampVillage},
	}
	for _, c := range cases {
		if got := CampOf(c.role); got != c.want {
			t.Errorf("CampOf(%v) = %v, want %v", c.role, got, c.want)
		}
	}
	// "A werewolf was eliminated" counts werewolf cards, not the wolf team --
	// the minion does not count.
	if isWolfCard(RoleMinion) {
		t.Error("the minion is on the wolf team but is not a werewolf card")
	}
	if !isWolfCard(RoleWerewolf) {
		t.Error("the werewolf card is a werewolf card")
	}
}
