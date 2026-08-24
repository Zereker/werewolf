// vocab.go is the mission-based game's vocabulary: four phases, eight roles,
// six skills, nine events.
//
// It is the same thing as the root package's vocab.go filled in differently --
// the kernel has types only, and the values all live in a rules package. This
// file existing is itself the evidence that the kernel does not know which
// game it is running: it shares not one value with werewolf's, and the kernel
// needed not one line changed.
//
// # Why the package is called missions
//
// This package implements the play of The Resistance and its Avalon variant:
// five missions, each one nominated by a leader, voted on by everyone, and --
// if approved -- resolved by the team members voting success or failure in
// secret. Both of those names are trademarks. Game rules themselves are
// generally not copyrightable; what is protected is the name, the artwork and
// the specific wording, so the package is named after the play's core
// structure (missions) rather than the trademark, and is unaffiliated with
// the publisher.
//
// The role names stay: Merlin, Percival, Mordred, Morgana and Oberon are
// figures from Arthurian legend and are in the public domain, not anybody's
// trademark.
//
// # Where the rules come from
//
// Based on the English Wikipedia article for The Resistance (game):
// https://en.wikipedia.org/wiki/The_Resistance_(game)
//
// The Chinese article gets Merlin wrong -- it says he "knows who the evil
// players are but not which is which", which is half a sentence carried over
// from Percival. Merlin **can** identify each bad guy individually, and that
// is the premise the whole game rests on: precisely because he knows so
// exactly, one word gives him away, and so he has to hide. Implemented the
// Chinese way, Merlin degenerates into "knows how many bad guys there are",
// and the whole Percival-and-assassin apparatus loses its point. This package
// follows the English article.
package missions

import "github.com/Zereker/hiddenrole"

// Four phases.
//
// One mission takes three of them: the leader nominates, everyone votes, and
// then the team members vote the mission up or down. A rejected vote loops
// back to nomination with the next leader -- so the phase cycle is a loop of
// three nodes, and the fourth (the assassination) is only entered once the
// good side has three successes.
const (
	PhasePropose  hiddenrole.PhaseType = "PROPOSE"   // the leader nominates a team
	PhaseTeamVote hiddenrole.PhaseType = "TEAM_VOTE" // everyone votes on whether to accept it
	PhaseMission  hiddenrole.PhaseType = "MISSION"   // the team members vote success or failure
	PhaseAssassin hiddenrole.PhaseType = "ASSASSIN"  // the assassin names Merlin
)

// Eight roles. Of the good-and-evil split, the named evil roles are optional;
// the smallest game needs only loyal servants and minions.
const (
	// The good side.
	RoleLoyalServant hiddenrole.RoleType = "LOYAL_SERVANT" // a loyal servant of Arthur, no special ability
	RoleMerlin       hiddenrole.RoleType = "MERLIN"        // knows every bad guy (except Mordred), and is assassinated the moment he is exposed
	RolePercival     hiddenrole.RoleType = "PERCIVAL"      // sees Merlin and Morgana as two people without telling them apart

	// The evil side.
	RoleMinion   hiddenrole.RoleType = "MINION"   // a minion of Mordred, no special ability
	RoleAssassin hiddenrole.RoleType = "ASSASSIN" // once the good side has three successes, he names Merlin
	RoleMorgana  hiddenrole.RoleType = "MORGANA"  // appears identical to Merlin in Percival's eyes
	RoleMordred  hiddenrole.RoleType = "MORDRED"  // Merlin cannot see him
	RoleOberon   hiddenrole.RoleType = "OBERON"   // neither knows his fellows nor is known to them
)

// Six skills.
const (
	SkillPropose hiddenrole.SkillType = "PROPOSE" // the leader nominates one player; a team is submitted several times
	SkillApprove hiddenrole.SkillType = "APPROVE" // vote: accept this team
	SkillReject  hiddenrole.SkillType = "REJECT"  // vote: reject this team

	SkillMissionSuccess hiddenrole.SkillType = "MISSION_SUCCESS" // mission: vote success
	SkillMissionFail    hiddenrole.SkillType = "MISSION_FAIL"    // mission: vote failure (only the evil side may)

	SkillAssassinate hiddenrole.SkillType = "ASSASSINATE" // the assassination: name Merlin
)

// Nine events: the rules' names for what happened. The kernel recognises none
// of them.
const (
	EventProposed         hiddenrole.EventType = "PROPOSED"          // somebody was nominated for the team
	EventTeamApproved     hiddenrole.EventType = "TEAM_APPROVED"     // the team was approved
	EventTeamRejected     hiddenrole.EventType = "TEAM_REJECTED"     // the team was rejected
	EventLeaderChanged    hiddenrole.EventType = "LEADER_CHANGED"    // leadership passed to the next player
	EventMissionSucceeded hiddenrole.EventType = "MISSION_SUCCEEDED" // the mission succeeded
	EventMissionFailed    hiddenrole.EventType = "MISSION_FAILED"    // the mission failed (with the number of fail votes)
	EventHammerReached    hiddenrole.EventType = "HAMMER_REACHED"    // five consecutive rejections; the evil side wins outright
	EventAssassinated     hiddenrole.EventType = "ASSASSINATED"      // the assassin named somebody
	EventVote             hiddenrole.EventType = "VOTE"              // one player's vote on a team (public)
	EventFailRejected     hiddenrole.EventType = "FAIL_REJECTED"     // a good player tried to vote failure and was vetoed (sent to them alone)
)

// Two camps.
const (
	CampGood hiddenrole.Camp = "GOOD"
	CampEvil hiddenrole.Camp = "EVIL"
)
