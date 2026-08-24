// knowledge.go records who saw what during the night.
//
// # Why it is recorded rather than recomputed on demand
//
// Information in this ruleset has a **timestamp**. The seer looked at
// player 3's card in step 5, and the robber swapped player 3's card away in
// step 6 -- what the seer saw is still the card they saw then, not the one
// there now. Recomputing from the board on demand always produces "now",
// which is always wrong.
//
// The first two rules packages do not have this problem: werewolf's seer gets
// a result that stays meaningful (it is a camp, and camps do not change), and
// the missions package's Merlin sees a list of bad guys that holds all game.
// Here, for the first time, "what they know" and "what is true now" come
// apart.
//
// So every look leaves a record on the looker, and afterwards only the record
// is read. A record is game-long state owned by one player -- exactly one
// cell of the variable-scope 2x2 table.

package onenight

import (
	"sort"
	"strconv"
	"strings"

	"github.com/Zereker/hiddenrole"
)

const (
	// learnSelfKey is "what I saw my own card to be". The robber and the
	// insomniac write it.
	learnSelfKey = "learn.self"

	// learnPlayerPrefix is "what I saw someone's card to be at the time",
	// followed by the ID of the player looked at.
	learnPlayerPrefix = "learn.player."

	// learnCenterPrefix is "what I saw a centre card to be at the time",
	// followed by its index.
	learnCenterPrefix = "learn.center."
)

// learnSelf records what the viewer saw their own card to be.
func learnSelf(viewerID string, role hiddenrole.RoleType) *hiddenrole.Effect {
	return hiddenrole.NewSetVarEffect(hiddenrole.ScopeGame.Of(viewerID), learnSelfKey, string(role))
}

// learnPlayer records what the viewer saw another player's card to be.
func learnPlayer(viewerID, targetID string, role hiddenrole.RoleType) *hiddenrole.Effect {
	return hiddenrole.NewSetVarEffect(
		hiddenrole.ScopeGame.Of(viewerID), learnPlayerPrefix+targetID, string(role))
}

// learnCenter records what the viewer saw a centre card to be.
func learnCenter(viewerID string, i int, role hiddenrole.RoleType) *hiddenrole.Effect {
	return hiddenrole.NewSetVarEffect(
		hiddenrole.ScopeGame.Of(viewerID), learnCenterPrefix+strconv.Itoa(i), string(role))
}

// knowledgeOf is everything this player saw during the night, under the same
// keys the learn* functions wrote.
//
// It reads the player's own game-long state, so it **has** to be projected
// explicitly by a RoleInfoProvider to reach the player -- the kernel
// deliberately does not hand Vars to players (see hiddenrole.PlayerInfo),
// which is exactly the class of judgement this library takes off a caller's
// hands.
func knowledgeOf(view hiddenrole.GameView, playerID string) map[string]string {
	p, ok := view.Player(playerID)
	if !ok {
		return nil
	}

	out := make(map[string]string, len(p.Vars))
	for k, v := range p.Vars {
		if strings.HasPrefix(k, "learn.") {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// teammatesByDealt returns the other players whose dealt card puts them on
// the same side, sorted by ID.
//
// "The same side" is decided by the card they were **dealt**, not the card in
// their hand now: the wolves recognise each other in the very first step, when
// no swap has happened yet. A robber who later steals the werewolf card is not
// recognised either -- they were not on the list at that moment.
func teammatesByDealt(view hiddenrole.GameView, playerID string, roles ...hiddenrole.RoleType) []string {
	want := make(map[hiddenrole.RoleType]bool, len(roles))
	for _, r := range roles {
		want[r] = true
	}

	var out []string
	for _, p := range view.AllPlayers() {
		if p.ID != playerID && want[p.Role] {
			out = append(out, p.ID)
		}
	}
	sort.Strings(out)
	return out
}
