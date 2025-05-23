package skill

import (
	"fmt"

	"github.com/Zereker/werewolf/pkg/game"
)

// LastWords 遗言技能
type LastWords struct {
	name     game.SkillType
	phase    game.PhaseType
	priority int
	hasUsed  bool
	content  string
}

func NewLastWordsSkill() *LastWords {
	return &LastWords{
		name:     game.SkillTypeLastWords,
		phase:    game.PhaseDay,
		priority: PriorityLastWords,
	}
}

func (l *LastWords) GetName() game.SkillType {
	return l.name
}

func (l *LastWords) GetPhase() game.PhaseType {
	return l.phase
}

func (l *LastWords) GetPriority() int {
	return l.priority
}

func (l *LastWords) Check(phase game.PhaseType, caster game.Player, target game.Player) error {
	if phase != l.phase {
		return fmt.Errorf("last words skill cannot be used in %s phase", phase)
	}

	if l.hasUsed {
		return fmt.Errorf("last words skill has already been used")
	}

	if target != nil {
		return fmt.Errorf("last words skill does not require a target")
	}

	return nil
}

// Put 使用技能
func (l *LastWords) Put(caster game.Player, target game.Player, result *game.SkillResult) {
	l.hasUsed = true

	result.Success = true
	result.Message = fmt.Sprintf("玩家 %s 的遗言：%s", caster.GetID(), l.content)
}

func (l *LastWords) Reset() {
	l.hasUsed = false
}

func (l *LastWords) GetContent() string {
	return l.content
}
