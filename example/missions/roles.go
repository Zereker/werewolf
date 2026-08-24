package missions

import (
	"sort"
	"strings"

	"github.com/Zereker/hiddenrole"
)

// roles.go covers who is good and who can see whom.
//
// Nobody is eliminated in this ruleset for the whole game -- the biggest
// structural difference from werewolf. The kernel's alive bit is never written
// here, and not one SET_ALIVE is ever produced.

// evilRoles is every role on the evil side.
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

// builtinRoleSetup hands out the camp at seating time.
//
// The kernel recognises one key, VarCamp (the victory check reads camps); the
// rest is the rules' business. This ruleset needs no "special role vs plain
// villager" sub-division -- its outcome depends on missions and the
// assassination, and it counts nobody.
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

// idsWithRole is the players holding any of these roles, sorted by ID.
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

// teammates answers "who is on my side".
//
// Only the evil players are truly each other's fellows, and **Oberon is not
// among them**: the article's wording is "Oberon: Unknown to other evil
// players" -- he neither knows his fellows nor is known to them. A natural
// asymmetry, and one the kernel supports explicitly.
//
// Merlin can see the bad guys, but that is not "the same side" -- he is their
// mortal enemy. That knowledge is role information and goes through RoleInfo;
// see merlinInfo below.
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

// The RoleInfo keys. The keys are the role's own and the kernel does not
// recognise them.
const (
	RoleInfoMerlinEvil        = "merlin.evil"                // the bad guys Merlin sees
	RoleInfoPercivalCandidate = "percival.merlin_or_morgana" // the two people Percival sees
)

// merlinInfo: Merlin knows every bad guy -- **except Mordred**.
//
// The article's wording is "Mordred: Unknown to Merlin". Note that Merlin can
// identify them individually, not merely count them (the Chinese article is
// wrong here; see vocab.go). Oberon, though unknown to his own side, is still
// visible to Merlin.
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

// percivalInfo: Percival sees Merlin and Morgana as two people without
// telling them apart.
//
// The article's wording is "Percival secretly learns that two players are
// Merlin and Morgana, but does not know which player is which". "Cannot tell
// them apart" in implementation terms means **handing over both IDs sorted**,
// with nothing distinguishing them. Without Morgana in play there is only
// Merlin, and in that game Percival has effectively identified him outright.
func percivalInfo(_ string, view hiddenrole.GameView) map[string]string {
	ids := idsWithRole(view, RoleMerlin, RoleMorgana)
	return map[string]string{RoleInfoPercivalCandidate: strings.Join(ids, ",")}
}

var builtinRoleInfo = map[hiddenrole.RoleType]hiddenrole.RoleInfoProvider{
	RoleMerlin:   hiddenrole.RoleInfoFunc(merlinInfo),
	RolePercival: hiddenrole.RoleInfoFunc(percivalInfo),
}
