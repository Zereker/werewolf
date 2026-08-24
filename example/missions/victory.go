package missions

import "github.com/Zereker/hiddenrole"

// victory.go decides how winning works.
//
// The outcome here is nothing like werewolf's: **it counts nobody**. No
// wipe-out of either kind, and nobody is ever eliminated. Only three things
// matter -- the mission score, the number of consecutive rejections, and the
// single strike once the good side reaches three.
//
// That is a point in the kernel's favour: a VictoryChecker is handed a
// GameView and returns (over, winner), and nothing anywhere assumes that
// winning means having killed everyone off.
type victoryChecker struct{}

func (victoryChecker) CheckVictory(view hiddenrole.GameView) (bool, hiddenrole.Camp) {
	// Five consecutive team rejections: the evil side wins outright.
	if rejects(view) >= HammerRejections {
		return true, CampEvil
	}

	// Three failed missions.
	if failures(view) >= 3 {
		return true, CampEvil
	}

	if successes(view) < 3 {
		return false, hiddenrole.CampUnspecified
	}

	// The good side has three successes, but still has to survive the
	// assassination.
	//
	// With no assassin in play (the smallest board) the good side wins
	// outright; with an assassin who has not yet struck, this must answer
	// "not over" -- otherwise the engine would decide the game before the
	// assassination phase. That phase is queued by the mission resolver
	// through the detour queue, and the kernel defers the victory check until
	// afterwards, so reporting "not over" truthfully is all that is needed
	// here.
	switch view.Var(hiddenrole.ScopeGame, varAssassinated) {
	case "hit":
		return true, CampEvil // Merlin was hit; the evil side snatches the win
	case "miss":
		return true, CampGood
	}
	if len(idsWithRole(view, RoleAssassin)) == 0 {
		return true, CampGood
	}
	return false, hiddenrole.CampUnspecified
}
