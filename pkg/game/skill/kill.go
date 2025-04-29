package skill

import (
	"fmt"

	"github.com/Zereker/werewolf/pkg/game"
)

// Kill 狼人击杀技能
type Kill struct {
	name     game.SkillType
	phase    game.PhaseType
	priority int
	hasUsed  bool
}

// NewKillSkill creates new kill skill
func NewKillSkill() *Kill {
	return &Kill{
		name:     game.SkillTypeKill,
		phase:    game.PhaseNight,
		priority: PriorityKill,
	}
}

// GetName returns skill name
func (k *Kill) GetName() game.SkillType {
	return k.name
}

// GetPhase returns skill phase
func (k *Kill) GetPhase() game.PhaseType {
	return k.phase
}

// Check 检查技能是否可以使用
func (k *Kill) Check(phase game.PhaseType, caster game.Player, target game.Player) error {
	// 检查阶段
	if phase != k.phase {
		return fmt.Errorf("kill skill cannot be used in %s phase", phase)
	}

	// 检查是否已使用
	if k.hasUsed {
		return fmt.Errorf("kill skill has already been used")
	}

	// 检查施法者是否存活
	if !caster.IsAlive() {
		return fmt.Errorf("dead werewolf cannot kill")
	}

	// 检查目标
	if target == nil {
		return fmt.Errorf("kill skill requires a target")
	}

	// 检查目标是否存活
	if !target.IsAlive() {
		return fmt.Errorf("target is already dead")
	}

	return nil
}

// Put 使用技能
func (k *Kill) Put(caster game.Player, target game.Player, option game.PutOption) {
	k.hasUsed = true

	if target.IsProtected() {
		return
	}

	target.SetAlive(false)
}

// Exec 执行技能，包含检查和执行两个步骤
func (k *Kill) Exec(phase game.PhaseType, caster game.Player, target game.Player, option game.PutOption) error {
	if err := k.Check(phase, caster, target); err != nil {
		return err
	}
	k.Put(caster, target, option)
	return nil
}

// Reset resets skill state
func (k *Kill) Reset() {
	k.hasUsed = false
}

func (k *Kill) GetPriority() int {
	return k.priority
}
