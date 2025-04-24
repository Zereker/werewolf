package phase

import (
	"testing"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/player"
	"github.com/Zereker/werewolf/pkg/game/role"
	"github.com/Zereker/werewolf/pkg/game/skill"
)

func TestDayPhase_GetName(t *testing.T) {
	phase := NewDayPhase()
	if got := phase.GetName(); got != game.PhaseDay {
		t.Errorf("GetName() = %v, want %v", got, game.PhaseDay)
	}
}

func TestDayPhase_Handle(t *testing.T) {
	tests := []struct {
		name        string
		setupAction func() *game.Action
		wantErr     bool
	}{
		{
			name: "村民说话",
			setupAction: func() *game.Action {
				villagerRole, _ := role.New(game.RoleTypeVillager)
				return &game.Action{
					Caster: player.New(villagerRole),
					Skill:  skill.NewSpeakSkill(),
				}
			},
			wantErr: false,
		},
		{
			name: "狼人说话",
			setupAction: func() *game.Action {
				werewolfRole, _ := role.New(game.RoleTypeWerewolf)
				return &game.Action{
					Caster: player.New(werewolfRole),
					Skill:  skill.NewSpeakSkill(),
				}
			},
			wantErr: false,
		},
		{
			name: "村民遗言",
			setupAction: func() *game.Action {
				villagerRole, _ := role.New(game.RoleTypeVillager)
				return &game.Action{
					Caster: player.New(villagerRole),
					Skill:  skill.NewLastWordsSkill(),
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phase := NewDayPhase()
			action := tt.setupAction()
			if err := phase.Handle(action); (err != nil) != tt.wantErr {
				t.Errorf("Handle() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDayPhase_GetPhaseResult(t *testing.T) {
	tests := []struct {
		name         string
		setupActions func() []*game.Action
		checkResult  func(*testing.T, *game.PhaseResult[game.SkillResultMap])
	}{
		{
			name: "正常发言",
			setupActions: func() []*game.Action {
				villagerRole1, _ := role.New(game.RoleTypeVillager)
				villagerRole2, _ := role.New(game.RoleTypeVillager)
				werewolfRole, _ := role.New(game.RoleTypeWerewolf)

				return []*game.Action{
					{
						Caster: player.New(villagerRole1),
						Skill:  skill.NewSpeakSkill(),
					},
					{
						Caster: player.New(villagerRole2),
						Skill:  skill.NewSpeakSkill(),
					},
					{
						Caster: player.New(werewolfRole),
						Skill:  skill.NewSpeakSkill(),
					},
				}
			},
			checkResult: func(t *testing.T, result *game.PhaseResult[game.SkillResultMap]) {
				// ... 检查逻辑
			},
		},
		{
			name: "遗言",
			setupActions: func() []*game.Action {
				villagerRole, _ := role.New(game.RoleTypeVillager)
				return []*game.Action{
					{
						Caster: player.New(villagerRole),
						Skill:  skill.NewLastWordsSkill(),
					},
				}
			},
			checkResult: func(t *testing.T, result *game.PhaseResult[game.SkillResultMap]) {
				// ... 检查逻辑
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phase := NewDayPhase()
			actions := tt.setupActions()
			for _, action := range actions {
				if err := phase.Handle(action); err != nil {
					t.Errorf("Handle() error = %v", err)
				}
			}
			result := phase.GetPhaseResult()
			tt.checkResult(t, result)
		})
	}
}

func TestDayPhase_Reset(t *testing.T) {
	phase := NewDayPhase()
	villagerRole, _ := role.New(game.RoleTypeVillager)
	player1 := player.New(villagerRole)

	// 添加一些动作
	action := &game.Action{
		Caster: player1,
		Target: nil,
		Skill:  skill.NewSpeakSkill(),
	}
	phase.Handle(action)

	// 重置
	phase.Reset()

	// 验证重置后的状态
	if len(phase.actions) != 0 {
		t.Error("Actions should be empty after reset")
	}
	if len(phase.skillResults) != 0 {
		t.Error("Skill results should be empty after reset")
	}
}
