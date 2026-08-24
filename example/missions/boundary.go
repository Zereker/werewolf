package missions

import "github.com/Zereker/hiddenrole"

// boundary.go covers who should be told about something, and who hears whom
// speak.
//
// The information boundary here is far simpler than werewolf's: **almost
// everything that happens on the table is public**. Who was nominated, who
// voted for or against, whether a mission succeeded, how many fail votes there
// were -- the whole table sees all of it. Only two things are private: the
// identity information dealt at the start (which goes through RoleInfo and
// Teammates, not through events), and "who voted failure" -- and the latter is
// implemented by **not producing that event at all**.
func audience(event *hiddenrole.Event, view hiddenrole.GameView) ([]string, bool) {
	// A vetoed action is only the actor's business.
	//
	// This has to come **before** the type-based split: a good player's
	// mistaken fail vote is rejected as the rules' own FAIL_REJECTED type, and
	// bucketing by type alone would broadcast it -- naming them on the spot.
	if event.Canceled || event.Type == EventFailRejected {
		return actorOnly(event.SourceID, view), true
	}

	switch event.Type {
	case EventProposed, EventVote, EventTeamApproved, EventTeamRejected,
		EventLeaderChanged, EventMissionSucceeded, EventMissionFailed,
		EventHammerReached, EventAssassinated:
		return allIDs(view), true
	}
	return nil, false
}

func actorOnly(id string, view hiddenrole.GameView) []string {
	if id == "" {
		return nil
	}
	if _, ok := view.Player(id); !ok {
		return nil
	}
	return []string{id}
}

func allIDs(view hiddenrole.GameView) []string {
	out := make([]string, 0, len(view.AllPlayers()))
	for _, p := range view.AllPlayers() {
		out = append(out, p.ID)
	}
	return out
}

// speech is who hears whom speak.
//
// Discussion is public throughout this ruleset -- there is nothing like
// werewolf's separate night channel for the wolves. This is also a point in
// the kernel's favour: swapping the SpeechProvider for "everyone hears
// everything" is one function, and the kernel need not know that a "night"
// exists at all.
func speech(_ string, view hiddenrole.GameView) []string { return allIDs(view) }
