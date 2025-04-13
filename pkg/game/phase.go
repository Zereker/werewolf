package game

// Phase represents game phase
type Phase string

const (
	PhaseNight Phase = "night" // Night phase
	PhaseDay   Phase = "day"   // Day phase
	PhaseVote  Phase = "vote"  // Vote phase
)

// Action represents game action
type Action string

const (
	ActionWerewolfKill Action = "werewolf_kill" // Werewolf kill
	ActionSeerCheck    Action = "seer_check"    // Seer check
	ActionWitchSave    Action = "witch_save"    // Witch save
	ActionWitchPoison  Action = "witch_poison"  // Witch poison
	ActionVote         Action = "vote"          // Vote
)

// Skill represents game skill
type Skill interface {
	GetName() string
	Put(phase Phase, caster Player, target Player) error
	Reset()
}
