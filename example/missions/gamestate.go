package missions

import (
	"strconv"

	"github.com/Zereker/hiddenrole"
)

// gamestate.go is where this package keeps the game's progress.
//
// Five things: which mission it is, how many succeeded, how many failed, how
// many consecutive rejections there have been, and whose turn it is to lead.
// All five are **game-long and owned by nobody** -- exactly the kernel's
// fourth variable scope (whole game, unowned).
//
// # This file used to be a workaround
//
// The kernel used to have only three scopes and was missing the "whole game,
// unowned" cell (SCARS.md, scar 4). So these five numbers could only be filed
// under the lexicographically smallest player's own state as a ledger: global
// facts recorded under one person's name, five fields appearing out of nowhere
// in that player's PlayerView with nothing to do with them, and "who holds the
// ledger" upheld by convention.
//
// Once the kernel gained the fourth cell, the ledger was deleted outright --
// it is a game-scoped variable now, one line to read and one to write.
const (
	varMission = "missions.mission" // which mission this is, 1-5
	varSuccess = "missions.success" // how many have succeeded
	varFail    = "missions.fail"    // how many have failed
	varRejects = "missions.rejects" // how many consecutive rejections
	varLeader  = "missions.leader"  // the leader's seat number (an index into AllPlayers)

	// varAssassinated is the assassination's result: "hit" means Merlin was
	// named, "miss" means he was not. An empty string means the assassination
	// has not happened -- the victory check uses it to tell "the good side
	// won" from "the good side has three successes but has not yet survived
	// the assassination".
	varAssassinated = "missions.assassinated"
)

// Round-scoped state: these happen to want clearing once per nomination,
// which the kernel's existing scope covers.
const (
	varOnTeam   = "missions.on_team"  // per player: nominated for this round's mission
	varApproved = "missions.approved" // per round: this round's team was accepted
)

// gameNum reads a game-long counter, or 0 if it was never written.
func gameNum(view hiddenrole.GameView, key string) int {
	n, err := strconv.Atoi(view.Var(hiddenrole.ScopeGame, key))
	if err != nil {
		return 0
	}
	return n
}

// setGameNum writes a game-long counter.
func setGameNum(_ hiddenrole.GameView, key string, n int) *hiddenrole.Effect {
	return hiddenrole.NewSetVarEffect(hiddenrole.ScopeGame, key, strconv.Itoa(n))
}

// mission is which mission this is, 1-5. Before it is first written it counts
// as mission 1.
func mission(view hiddenrole.GameView) int {
	if n := gameNum(view, varMission); n > 0 {
		return n
	}
	return 1
}

func successes(view hiddenrole.GameView) int { return gameNum(view, varSuccess) }
func failures(view hiddenrole.GameView) int  { return gameNum(view, varFail) }
func rejects(view hiddenrole.GameView) int   { return gameNum(view, varRejects) }

// leaderID is the current leader.
//
// Leadership rotates by seat whether or not a team is approved -- the
// article's leader token passes on every round. The index is stored in the
// ledger and taken modulo the player count.
func leaderID(view hiddenrole.GameView) string {
	all := view.AllPlayers()
	if len(all) == 0 {
		return ""
	}
	return all[gameNum(view, varLeader)%len(all)].ID
}

// leaderAt is who leads at position n.
func leaderAt(view hiddenrole.GameView, n int) string {
	all := view.AllPlayers()
	if len(all) == 0 {
		return ""
	}
	return all[n%len(all)].ID
}

// onTeam reports whether this player is on this round's mission team.
func onTeam(view hiddenrole.GameView, playerID string) bool {
	return view.Var(hiddenrole.ScopeRound.Of(playerID), varOnTeam) != ""
}

// teamIDs is this round's mission team, sorted by ID.
//
// The ordering is something the rules have to guarantee: the order of the
// effects produced must be uniquely determined by the board, or replay and
// snapshot comparison lose their determinism. AllPlayers() is already sorted.
func teamIDs(view hiddenrole.GameView) []string {
	var out []string
	for _, p := range view.AllPlayers() {
		if onTeam(view, p.ID) {
			out = append(out, p.ID)
		}
	}
	return out
}

// approved reports whether this round's team was accepted.
func approved(view hiddenrole.GameView) bool {
	return view.Var(hiddenrole.ScopeRound, varApproved) != ""
}

// gameSetup lays out the board at the moment play begins.
//
// Two things: initialise the game-long counters explicitly (rather than
// relying on "unreadable means 0"), and **name the actors of the first
// nomination phase** -- the first leader.
//
// The latter is the direct reason the GameSetup extension point exists: an
// actor list is normally computed by the previous phase's resolver (the vote
// names the next leader, the nomination names the mission team), and the first
// phase has no previous phase. Without it, the opening nomination would fall
// back to computing by role -- that is, the whole table being told they may
// nominate, which is exactly what this scar set out to eliminate.
func gameSetup(view hiddenrole.GameView) []*hiddenrole.Effect {
	return []*hiddenrole.Effect{
		hiddenrole.NewSetVarEffect(hiddenrole.ScopeGame, varMission, "1"),
		hiddenrole.NewSetVarEffect(hiddenrole.ScopeGame, varLeader, "0"),
		hiddenrole.NewSetActorsEffect(PhasePropose, leaderAt(view, 0)),
	}
}
