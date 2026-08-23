// types.go is the kernel's vocabulary: phases, roles, skills, camps, categories.
//
// Only the **types** live here, never the values. The kernel does not know
// which phases or roles exist -- "NIGHT_WITCH" and "WEREWOLF" belong to the
// werewolf rules package (see vocab.go in the root package).
//
// Everything is a string underneath, not a number. The numbers were a
// protobuf legacy: back then these types were generated from .proto files and
// their numeric tags were part of the wire format. Once protobuf was removed
// the numbers were pure overhead -- snapshots are written by name, logs are
// printed by name, so every type needed a lookup table and a pair of JSON
// methods, a hundred-odd lines whose only job was translating a value back
// into what it already was.
//
// With the name as the value all of that disappears: JSON is readable on its
// own, String() is one line, and a rules package defines its own values with
// RoleType("KNIGHT") without ever colliding with anyone else's numbering.
//
// The zero value is the empty string and means "unspecified".

package hiddenrole

// PhaseType is a phase of play. The values are defined by the rules.
type PhaseType string

// Three phases the kernel owns itself: they are the state machine's lifecycle,
// not a step in anybody's rules.
//
// A rules package's phase cycle starts at Config.StartPhase and terminates at
// PhaseEnd; PhaseStart is the "not started yet" state itself, and AddPlayer is
// only allowed while the game is in it.
const (
	PhaseUnspecified PhaseType = ""
	PhaseStart       PhaseType = "START" // not started yet
	PhaseEnd         PhaseType = "END"   // already over
)

// String implements fmt.Stringer.
func (v PhaseType) String() string {
	if v == PhaseUnspecified {
		return "UNSPECIFIED"
	}
	return string(v)
}

// RoleType is a role. The values are defined by the rules.
type RoleType string

const (
	// RoleUnspecified is unspecified. On a PhaseStep it means "every role".
	RoleUnspecified RoleType = ""

	// RoleSystem means "no player carries this step".
	//
	// It is not an identity, it is a **marker**: a phase step declaring it is
	// a broadcast (something is to be announced), not a wait for someone to
	// act. Seating it is rejected, and readiness does not count it.
	//
	// It used to be called RoleGod, with the value "GOD". That name implied
	// the identity of a host -- but a host is a werewolf concept, the
	// mission-based games have nobody hosting at all, and Blood on the
	// Clocktower calls theirs a storyteller. What the kernel recognises is
	// not "who is hosting", it is "this step waits for nobody". If you want a
	// role literally named god, name it in your rules package (that is
	// exactly what werewolf.RoleGod is).
	RoleSystem RoleType = "SYSTEM"
)

// String implements fmt.Stringer.
func (v RoleType) String() string {
	if v == RoleUnspecified {
		return "UNSPECIFIED"
	}
	return string(v)
}

// SkillType is a skill. The values are defined by the rules.
type SkillType string

const (
	// SkillUnspecified is unspecified.
	SkillUnspecified SkillType = ""

	// SkillSkip declines to act. Every turn-based game has this move, so the
	// kernel provides one shared name for it instead of letting each rules
	// package invent its own.
	//
	// **It carries no kernel privilege.** validateSkillUse used to have a
	// branch reading "skipping needs no target, let it through" -- that branch
	// was empty: a submission with no target already passes target validation
	// (the loop never runs), and a submission that *does* carry a target
	// **should** be validated. Its only real effect was to make the kernel
	// recognise one specific skill, which is precisely what this library sets
	// out to eliminate.
	SkillSkip SkillType = "SKIP"

	// SkillAnnounce is a broadcast, paired with RoleSystem. The content is up
	// to the caller.
	SkillAnnounce SkillType = "ANNOUNCE"
)

// String implements fmt.Stringer.
func (v SkillType) String() string {
	if v == SkillUnspecified {
		return "UNSPECIFIED"
	}
	return string(v)
}

// Camp labels one side. It is what a victory check resolves to.
//
// The kernel **presumes no values**: villagers and werewolves are the two
// sides of werewolf, the mission-based games have good and evil, and Blood on
// the Clocktower additionally has travellers who are scored separately. The
// kernel only knows that there are some number of sides, one of which may
// win, and that each player may belong to one of them (VarCamp) -- not which
// one, nor what it means.
type Camp string

// CampUnspecified means no side has won yet, or this player belongs to no side.
const CampUnspecified Camp = ""

// String implements fmt.Stringer.
func (v Camp) String() string {
	if v == CampUnspecified {
		return "UNSPECIFIED"
	}
	return string(v)
}

// VarCamp is the canonical key under which a player's camp lives in Vars.
//
// This is the one key the kernel recognises: its value is copied into the Camp
// field of PlayerInfo and SelfInfo, so that "which side is this player on"
// does not have to be dug out of Vars by every caller. The value is handed out
// by the rules (see RoleSetup); the kernel neither checks nor interprets it.
//
// There is only this one. Sub-divisions within a camp -- "special roles" vs
// "plain villagers" -- exist only because werewolf needs them for its
// wipe-out-one-side victory check; the kernel does not recognise them, and a
// rules package can simply define its own key (see werewolf.VarCategory).
const VarCamp = "camp"
