package missions

import (
	"testing"

	"github.com/Zereker/hiddenrole"
)

// A five-player game: Merlin, Percival, a loyal servant / the assassin,
// Morgana.
func fivePlayer(t *testing.T) *hiddenrole.Engine {
	t.Helper()
	e := MustNew()
	for id, role := range map[string]hiddenrole.RoleType{
		"a": RoleMerlin, "b": RolePercival, "c": RoleLoyalServant,
		"d": RoleAssassin, "e": RoleMorgana,
	} {
		if err := e.AddPlayer(id, role); err != nil {
			t.Fatalf("AddPlayer(%s): %v", id, err)
		}
	}
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return e
}

// TestFirstMission plays one mission through: nominate -> approve -> succeed.
func TestFirstMission(t *testing.T) {
	e := fivePlayer(t)

	if got := e.Status().Phase; got != PhasePropose {
		t.Fatalf("opening phase = %v, want PROPOSE", got)
	}
	if n := MissionSize(5, 1); n != 2 {
		t.Fatalf("mission 1 of a five-player game takes 2, the table says %d", n)
	}

	// The leader (seat 0 = "a") nominates two players.
	for _, target := range []string{"a", "b"} {
		if err := e.SubmitSkillUse(&hiddenrole.SkillUse{
			PlayerID: "a", Skill: SkillPropose, Targets: []string{target},
		}); err != nil {
			t.Fatalf("nominating %s: %v", target, err)
		}
	}
	if _, err := e.EndPhase(); err != nil {
		t.Fatalf("EndPhase(PROPOSE): %v", err)
	}
	if got := e.Status().Phase; got != PhaseTeamVote {
		t.Fatalf("phase = %v, want TEAM_VOTE", got)
	}

	// Everyone approves.
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		if err := e.SubmitSkillUse(&hiddenrole.SkillUse{PlayerID: id, Skill: SkillApprove}); err != nil {
			t.Fatalf("%s voting: %v", id, err)
		}
	}
	if _, err := e.EndPhase(); err != nil {
		t.Fatalf("EndPhase(TEAM_VOTE): %v", err)
	}
	if got := e.Status().Phase; got != PhaseMission {
		t.Fatalf("phase = %v, want MISSION", got)
	}

	// The team votes success.
	for _, id := range []string{"a", "b"} {
		if err := e.SubmitSkillUse(&hiddenrole.SkillUse{PlayerID: id, Skill: SkillMissionSuccess}); err != nil {
			t.Fatalf("%s casting a mission vote: %v", id, err)
		}
	}
	effects, err := e.EndPhase()
	if err != nil {
		t.Fatalf("EndPhase(MISSION): %v", err)
	}

	var succeeded bool
	for _, ef := range effects {
		if ef.Type == EventMissionSucceeded {
			succeeded = true
		}
	}
	if !succeeded {
		t.Fatalf("mission 1 should have succeeded, but there is no MISSION_SUCCEEDED among the effects: %v", typesOf(effects))
	}
	if e.Status().Over {
		t.Fatal("the game ended after a single win")
	}
}

// TestMerlinSeesEvilExceptMordred: Merlin knows every bad guy except Mordred.
func TestMerlinSeesEvilExceptMordred(t *testing.T) {
	e := MustNew()
	for id, role := range map[string]hiddenrole.RoleType{
		"a": RoleMerlin, "b": RolePercival, "c": RoleLoyalServant, "d": RoleLoyalServant,
		"e": RoleAssassin, "f": RoleMordred, "g": RoleOberon,
	} {
		if err := e.AddPlayer(id, role); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.Start(); err != nil {
		t.Fatal(err)
	}

	v := e.PlayerView("a")
	got := v.RoleInfo[RoleInfoMerlinEvil]
	// The assassin e and Oberon g are visible; Mordred f is not.
	if want := "e,g"; got != want {
		t.Errorf("the bad guys Merlin sees = %q, want %q (Mordred should be out, Oberon in)", got, want)
	}
}

// TestOberonIsAloneOnBothSides: Oberon neither knows his fellows nor is known
// to them.
func TestOberonIsAloneOnBothSides(t *testing.T) {
	e := MustNew()
	for id, role := range map[string]hiddenrole.RoleType{
		"a": RoleMerlin, "b": RoleLoyalServant, "c": RoleLoyalServant, "d": RoleLoyalServant,
		"e": RoleAssassin, "f": RoleMorgana, "g": RoleOberon,
	} {
		if err := e.AddPlayer(id, role); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.Start(); err != nil {
		t.Fatal(err)
	}

	if mates := e.Teammates("g"); len(mates) != 0 {
		t.Errorf("Oberon should know no fellows, got %v", mates)
	}
	for _, id := range []string{"e", "f"} {
		for _, m := range e.Teammates(id) {
			if m == "g" {
				t.Errorf("%s should not know Oberon, their fellows are %v", id, e.Teammates(id))
			}
		}
	}
	// The assassin and Morgana know each other.
	if mates := e.Teammates("e"); len(mates) != 1 || mates[0] != "f" {
		t.Errorf("the assassin's fellows = %v, want [f]", mates)
	}
}

// TestPercivalCannotTellMerlinFromMorgana: Percival sees two people and cannot
// tell which is which.
func TestPercivalCannotTellMerlinFromMorgana(t *testing.T) {
	e := fivePlayer(t) // a=Merlin e=Morgana
	v := e.PlayerView("b")
	got := v.RoleInfo[RoleInfoPercivalCandidate]
	if want := "a,e"; got != want {
		t.Errorf("Percival sees %q, want %q", got, want)
	}
	// "Cannot tell apart" is implemented as this one string carrying nothing
	// that distinguishes them.
	if len(v.RoleInfo) != 1 {
		t.Errorf("Percival should get nothing else: %v", v.RoleInfo)
	}
}

func typesOf(effects []*hiddenrole.Effect) []hiddenrole.EventType {
	out := make([]hiddenrole.EventType, 0, len(effects))
	for _, ef := range effects {
		out = append(out, ef.Type)
	}
	return out
}

// runMission plays one whole mission: nominate -> unanimous approval -> the
// team casting fails failure votes.
//
// The first fails members vote failure and the rest vote success.
func runMission(t *testing.T, e *hiddenrole.Engine, fails int, members ...string) []*hiddenrole.Effect {
	t.Helper()
	leader := leaderID(e.View())
	mustSubmit(t, e, &hiddenrole.SkillUse{PlayerID: leader, Skill: SkillPropose, Targets: members})
	mustEnd(t, e)

	for _, id := range e.AlivePlayerIDs() {
		mustSubmit(t, e, &hiddenrole.SkillUse{PlayerID: id, Skill: SkillApprove})
	}
	mustEnd(t, e)

	for i, id := range members {
		skill := SkillMissionSuccess
		if i < fails {
			skill = SkillMissionFail
		}
		mustSubmit(t, e, &hiddenrole.SkillUse{PlayerID: id, Skill: skill})
	}
	return mustEnd(t, e)
}

// TestFullGame_GoodWinsThreeThenSurvivesAssassination
// The good side wins three missions, the assassin names the wrong player, and
// the good side wins.
func TestFullGame_GoodWinsThreeThenSurvivesAssassination(t *testing.T) {
	e := fivePlayer(t) // a=Merlin b=Percival c=loyal servant d=assassin e=Morgana

	// Mission sizes in a five-player game: 2,3,2,3,3. Three successes, with
	// only good players on the teams.
	runMission(t, e, 0, "a", "b")
	runMission(t, e, 0, "a", "b", "c")
	last := runMission(t, e, 0, "a", "b")

	t.Logf("after three successes: phase=%v over=%v effects=%v", e.Status().Phase, e.Status().Over, typesOf(last))

	if e.Status().Over {
		t.Fatal("the assassination has not happened, so the game should not be over -- the victory check has to wait for it")
	}
	if e.Status().Phase != PhaseAssassin {
		t.Fatalf("phase = %v, want the detour queue to have routed to ASSASSIN", e.Status().Phase)
	}

	// The assassin names the wrong player (Percival; Merlin is a).
	mustSubmit(t, e, &hiddenrole.SkillUse{PlayerID: "d", Skill: SkillAssassinate, Targets: []string{"b"}})
	mustEnd(t, e)

	if !e.Status().Over {
		t.Fatal("the game should be over once the assassination resolves")
	}
	if got := e.Status().Winner; got != CampGood {
		t.Errorf("winner = %v, want GOOD (the assassin missed)", got)
	}
}

// TestFullGame_AssassinFindsMerlin: the good side wins three missions, the
// assassin names Merlin, and the evil side snatches the win.
func TestFullGame_AssassinFindsMerlin(t *testing.T) {
	e := fivePlayer(t)

	runMission(t, e, 0, "a", "b")
	runMission(t, e, 0, "a", "b", "c")
	runMission(t, e, 0, "a", "b")

	if e.Status().Phase != PhaseAssassin {
		t.Fatalf("phase = %v, want ASSASSIN", e.Status().Phase)
	}
	mustSubmit(t, e, &hiddenrole.SkillUse{PlayerID: "d", Skill: SkillAssassinate, Targets: []string{"a"}})
	mustEnd(t, e)

	if !e.Status().Over {
		t.Fatal("it should be over after the assassination")
	}
	if got := e.Status().Winner; got != CampEvil {
		t.Errorf("winner = %v, want EVIL (naming Merlin snatches the win)", got)
	}
}

// TestFullGame_EvilWinsThreeMissions: the evil side fails three missions and
// wins outright, with no assassination.
func TestFullGame_EvilWinsThreeMissions(t *testing.T) {
	e := fivePlayer(t) // d=assassin e=Morgana, both evil

	runMission(t, e, 1, "d", "e")
	runMission(t, e, 1, "d", "e", "a")
	runMission(t, e, 1, "d", "e")

	if !e.Status().Over {
		t.Fatal("it should be over after three failures")
	}
	if got := e.Status().Winner; got != CampEvil {
		t.Errorf("winner = %v, want EVIL", got)
	}
}

// TestHammer_FiveRejectionsEndTheGame: five consecutive team rejections hand
// the evil side an outright win.
func TestHammer_FiveRejectionsEndTheGame(t *testing.T) {
	e := fivePlayer(t)

	for i := 1; i <= HammerRejections; i++ {
		leader := leaderID(e.View())
		mustSubmit(t, e, &hiddenrole.SkillUse{PlayerID: leader, Skill: SkillPropose, Targets: []string{"a", "b"}})
		mustEnd(t, e)
		for _, id := range e.AlivePlayerIDs() {
			mustSubmit(t, e, &hiddenrole.SkillUse{PlayerID: id, Skill: SkillReject})
		}
		mustEnd(t, e)
		if i < HammerRejections {
			if e.Status().Over {
				t.Fatalf("it ended after only %d rejections, it should take %d", i, HammerRejections)
			}
			if got := e.Status().Phase; got != PhasePropose {
				t.Fatalf("a rejection should go straight back to PROPOSE, got %v", got)
			}
		}
	}

	if !e.Status().Over {
		t.Fatalf("it should be over after %d consecutive rejections", HammerRejections)
	}
	if got := e.Status().Winner; got != CampEvil {
		t.Errorf("winner = %v, want EVIL", got)
	}
}
