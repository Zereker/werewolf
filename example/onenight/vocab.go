// vocab.go is One Night's vocabulary: ten phases, eleven roles, thirteen
// skills, eleven events.
//
// This is the **third** rules package. The first two are werewolf (the root
// package) and the mission-based games (missions/). The kernel has types
// only, and all the values live here -- it does not know there is a role
// called the troublemaker, nor a phase called NIGHT_ROBBER.
//
// # Where the rules come from
//
// This one could not follow the first two. Werewolf is based on the Chinese
// Wikipedia article, the mission-based rules on the English Wikipedia article
// for The Resistance (game), and **One Night has no article of its own on
// English Wikipedia** -- it redirects to Ultimate Werewolf, which covers it in
// two sentences with no night order, no role abilities and no victory
// details.
//
// So this package is based on **the publisher's official rulebook from Bezier
// Games**, cross-checked against the official rules as restated by
// ultraboardgames.com. The per-rule findings are in the RULES comments, and
// anywhere this deviates from the source, the reason is written down below.
//
// # Why the package is not called onuw
//
// "One Night Ultimate Werewolf" is a Bezier Games trademark. Game rules
// themselves are generally not copyrightable; what is protected is the name,
// the artwork and the specific wording. This package implements the play, so
// it uses the descriptive name onenight rather than the trademark, and is
// unaffiliated with the publisher.
package onenight

import "github.com/Zereker/hiddenrole"

// Ten phases: nine night steps, plus the day discussion and the vote.
//
// The biggest difference from the first two packages: **this is a straight
// line, not a cycle**. The whole game is one night, one discussion and one
// vote, ending at VOTE. Round is 1 from start to finish.
//
// The night order is part of the rules rather than an implementation detail:
// the robber acts before the troublemaker, so the troublemaker can move the
// card the robber just stole; the insomniac acts last, so what they see is the
// result of every swap. Get the order wrong and it becomes a different game.
const (
	PhaseNightWerewolf    hiddenrole.PhaseType = "NIGHT_WEREWOLF"     // wolves recognise each other; a lone wolf may peek at one centre card
	PhaseNightMinion      hiddenrole.PhaseType = "NIGHT_MINION"       // the minion sees the wolves (who do not see the minion)
	PhaseNightMason       hiddenrole.PhaseType = "NIGHT_MASON"        // the masons recognise each other
	PhaseNightSeer        hiddenrole.PhaseType = "NIGHT_SEER"         // look at one player's card, or two centre cards
	PhaseNightRobber      hiddenrole.PhaseType = "NIGHT_ROBBER"       // swap cards with one player, and look at the new one
	PhaseNightTroublemake hiddenrole.PhaseType = "NIGHT_TROUBLEMAKER" // swap two other players' cards, without looking
	PhaseNightDrunk       hiddenrole.PhaseType = "NIGHT_DRUNK"        // swap with a centre card, without looking
	PhaseNightInsomniac   hiddenrole.PhaseType = "NIGHT_INSOMNIAC"    // look at your own card as it now stands
	PhaseDay              hiddenrole.PhaseType = "DAY"                // discussion
	PhaseVote             hiddenrole.PhaseType = "VOTE"               // everyone votes at once
)

// Eleven roles.
//
// The fundamental difference from the first two packages: **a role has two
// layers**.
//
//	the card you were dealt   decides what you do at night   never changes
//	the card in your hand now decides which side you score for   gets swapped around
//
// A robber who steals the werewolf card does **not** become a wolf and does
// not wake with them -- what they do at night is decided by the card they were
// dealt; but when the game is scored they count as the wolf team. This is the
// whole game's pivot, and the thing that most sets it apart from werewolf and
// the mission-based games: in those, "which role are you" has one answer from
// beginning to end.
//
// The kernel's RoleType carries the first layer (fixed at seating time, which
// is exactly the card you were dealt); the second layer is a game-long piece
// of this package's own state (varCard), see cards.go.
const (
	// The wolf team.
	RoleWerewolf hiddenrole.RoleType = "WEREWOLF" // recognise each other at night; a sole wolf may peek at one centre card
	RoleMinion   hiddenrole.RoleType = "MINION"   // sees the wolves; the wolves do not see them

	// The village team.
	RoleMason        hiddenrole.RoleType = "MASON"        // the two masons recognise each other; with only one, the other card is in the centre
	RoleSeer         hiddenrole.RoleType = "SEER"         // look at one player, or two centre cards
	RoleRobber       hiddenrole.RoleType = "ROBBER"       // swap cards with one player and look at the new one
	RoleTroublemaker hiddenrole.RoleType = "TROUBLEMAKER" // swap two other players' cards, without looking
	RoleDrunk        hiddenrole.RoleType = "DRUNK"        // swap with a centre card, **without looking**
	RoleInsomniac    hiddenrole.RoleType = "INSOMNIAC"    // look at your own card as it now stands
	RoleVillager     hiddenrole.RoleType = "VILLAGER"     // no ability
	RoleHunter       hiddenrole.RoleType = "HUNTER"       // if they are eliminated, so is whoever they voted for

	// Independent.
	RoleTanner hiddenrole.RoleType = "TANNER" // wins only by being eliminated
)

// Thirteen skills.
//
// Actions like "look at two centre cards" and "swap with a given centre card"
// **do not point at a player**, and the kernel's target validation only knows
// player IDs (every entry of SkillUse.Targets is passed to getPlayer). So a
// centre card's index can only be encoded into the skill name -- three pairs
// out of three cards, three singles, and six skills doing what is really two
// things. This is the first scar this package recorded; see SCARS.md.
const (
	SkillPeekCenter0 hiddenrole.SkillType = "PEEK_CENTER_0" // a lone wolf peeks at centre card 0
	SkillPeekCenter1 hiddenrole.SkillType = "PEEK_CENTER_1"
	SkillPeekCenter2 hiddenrole.SkillType = "PEEK_CENTER_2"

	SkillSeerPlayer   hiddenrole.SkillType = "SEER_PLAYER"    // the seer looks at one player
	SkillSeerCenter01 hiddenrole.SkillType = "SEER_CENTER_01" // the seer looks at centre cards 0 and 1
	SkillSeerCenter02 hiddenrole.SkillType = "SEER_CENTER_02"
	SkillSeerCenter12 hiddenrole.SkillType = "SEER_CENTER_12"

	SkillRob hiddenrole.SkillType = "ROB" // the robber swaps cards with one player

	SkillMeddle hiddenrole.SkillType = "MEDDLE" // the troublemaker swaps two other players' cards

	SkillDrinkCenter0 hiddenrole.SkillType = "DRINK_CENTER_0" // the drunk swaps with centre card 0
	SkillDrinkCenter1 hiddenrole.SkillType = "DRINK_CENTER_1"
	SkillDrinkCenter2 hiddenrole.SkillType = "DRINK_CENTER_2"

	SkillVote hiddenrole.SkillType = "VOTE" // point at one person
)

// Eleven events: the rules' names for what happened. The kernel recognises
// none of them.
//
// Same rule as the first two packages: a lone SWAPPED moves nobody's card;
// what actually changes state is the SET_VAR alongside it. Two effects, two
// things.
const (
	EventLoneWolf  hiddenrole.EventType = "LONE_WOLF"   // there is only one wolf in play
	EventPeeked    hiddenrole.EventType = "PEEKED"      // a centre card was looked at
	EventSeerLook  hiddenrole.EventType = "SEER_LOOK"   // the seer looked at cards
	EventRobbed    hiddenrole.EventType = "ROBBED"      // the robber swapped cards
	EventMeddled   hiddenrole.EventType = "MEDDLED"     // the troublemaker swapped two players' cards
	EventDrunkSwap hiddenrole.EventType = "DRUNK_SWAP"  // the drunk swapped with the centre
	EventInsomnia  hiddenrole.EventType = "INSOMNIA"    // the insomniac looked at their own card
	EventVoted     hiddenrole.EventType = "VOTED"       // one vote was cast
	EventNoOneDies hiddenrole.EventType = "NO_ONE_DIES" // one vote each, so nobody is eliminated
	EventLynched   hiddenrole.EventType = "LYNCHED"     // voted out
	EventHunterHit hiddenrole.EventType = "HUNTER_HIT"  // the hunter took down whoever they voted for
)

// Two camps. The values share their names with werewolf's and the
// mission-based games', and mean and resolve entirely differently -- which is
// itself evidence that the kernel does not interpret values.
const (
	CampVillage hiddenrole.Camp = "VILLAGE"
	CampWolf    hiddenrole.Camp = "WOLF"

	// CampTanner is a side of its own: the tanner helps neither the villagers
	// nor the wolves, and only wants to die.
	CampTanner hiddenrole.Camp = "TANNER"
)
