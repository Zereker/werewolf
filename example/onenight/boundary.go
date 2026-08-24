// boundary.go covers who sees what.
//
// The information asymmetry here is denser than in either earlier package,
// and it points a **different way**: the asymmetry in werewolf and the
// mission-based games is fixed for the whole game (the wolves know each other,
// Merlin knows the bad guys), and here it is **momentary** -- you saw
// something at some step, the board changed afterwards, your information is
// stale, and you do not know that it is. The whole game is built on that.

package onenight

import (
	"github.com/Zereker/hiddenrole"
)

// roleInfoFor is what a given role additionally sees.
//
// Three kinds:
//
//	mutual recognition   wolves, masons, the minion -- by the card they were **dealt**, since they act before any swap
//	what they saw        the seer, the robber, a lone wolf, the insomniac -- recorded on themselves, see knowledge.go
//	nothing at all       villagers, the troublemaker (who did not look either), the drunk (who knows even less), the tanner, the hunter
func roleInfoFor(role hiddenrole.RoleType) hiddenrole.RoleInfoProvider {
	return hiddenrole.RoleInfoFunc(func(playerID string, view hiddenrole.GameView) map[string]string {
		out := knowledgeOf(view, playerID)
		if out == nil {
			out = map[string]string{}
		}

		switch role {
		case RoleWerewolf:
			// Wolves recognise each other. With a single wolf the list is
			// empty -- which is exactly what they should know (an empty list
			// means "I am the lone wolf" and may peek at a centre card).
			addList(out, "wolves", teammatesByDealt(view, playerID, RoleWerewolf))

		case RoleMinion:
			// The minion sees the wolves and the wolves do not see them.
			// **Asymmetric**, and one-way.
			addList(out, "wolves", teammatesByDealt(view, playerID, RoleWerewolf))

		case RoleMason:
			// The masons recognise each other. With only one, the list is
			// empty, which means the other card is in the centre.
			addList(out, "masons", teammatesByDealt(view, playerID, RoleMason))
		}

		if len(out) == 0 {
			return nil
		}
		return out
	})
}

// addList puts a list into the information map, empty lists included --
// "the list is empty" is itself information.
func addList(out map[string]string, key string, ids []string) {
	out[key] = joinIDs(ids)
}

// joinIDs renders a list as one string; an empty list is the empty string.
func joinIDs(ids []string) string {
	s := ""
	for i, id := range ids {
		if i > 0 {
			s += ","
		}
		s += id
	}
	return s
}

// teammates answers "who is on whose side".
//
// It holds between wolves only, and **excludes the minion**: the minion sees
// the wolves, and the wolves do not see the minion. The kernel allows
// asymmetry precisely for this -- the missions package's Oberon is the other
// direction (he neither knows his fellows nor is known to them).
func teammates() hiddenrole.TeammateProvider {
	return hiddenrole.TeammateFunc(func(playerID string, view hiddenrole.GameView) []string {
		if dealt(view, playerID) != RoleWerewolf {
			return nil
		}
		return teammatesByDealt(view, playerID, RoleWerewolf)
	})
}

// audience answers who should be told about something.
//
// Everything that happens at night is told **only to the person it happened
// to**: whose card the seer looked at, who the robber robbed, which two the
// troublemaker swapped -- none of it should reach the table. The daytime vote
// and elimination are public.
//
// The kernel's state primitives (SET_VAR / SET_ALIVE) are never sent out and
// that is not configurable, so "player 3 now holds the werewolf card" cannot
// leak through an omission here.
func audience() hiddenrole.AudienceProvider {
	return hiddenrole.AudienceFunc(func(event *hiddenrole.Event, view hiddenrole.GameView) ([]string, bool) {
		switch event.Type {
		// Public: votes, eliminations, and nobody being eliminated.
		case EventVoted, EventLynched, EventNoOneDies, EventHunterHit,
			hiddenrole.EventGameStarted, hiddenrole.EventGameEnded:
			return allIDs(view), true

		// Only the person it happened to. The troublemaker's case matters
		// most: the two players swapped must not know either.
		case EventSeerLook, EventRobbed, EventMeddled, EventDrunkSwap,
			EventInsomnia, EventLoneWolf, EventPeeked:
			if event.SourceID == "" {
				return nil, true
			}
			return []string{event.SourceID}, true
		}
		return nil, false
	})
}

// speech is the audible range: everyone, throughout.
//
// This ruleset has one discussion step and it is public -- there is no wolf
// night chat as in werewolf. Installing this provider does not change
// anything; it is here to **turn off the kernel's default**. The kernel
// defaults to "the eliminated may not speak", and here elimination happens at
// the very last moment with nothing left to say, so the default is irrelevant
// to this ruleset. Saying so beats relying on it.
func speech() hiddenrole.SpeechProvider {
	return hiddenrole.SpeechFunc(func(_ string, view hiddenrole.GameView) []string {
		return allIDs(view)
	})
}

// allIDs is everyone at the table, sorted by ID.
func allIDs(view hiddenrole.GameView) []string {
	players := view.AllPlayers()
	out := make([]string, 0, len(players))
	for _, p := range players {
		out = append(out, p.ID)
	}
	return out
}
