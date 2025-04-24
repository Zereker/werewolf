package phase

import (
	"testing"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/player"
	"github.com/Zereker/werewolf/pkg/game/role"
	"github.com/Zereker/werewolf/pkg/game/skill"
)

func TestNightPhase_GetName(t *testing.T) {
	phase := NewNightPhase()
	if got := phase.GetName(); got != game.PhaseNight {
		t.Errorf("GetName() = %v, want %v", got, game.PhaseNight)
	}
}

func TestNightPhase_Handle(t *testing.T) {
	tests := []struct {
		name        string
		setupAction func() *game.Action
		wantErr     bool
	}{
		{
			name: "狼人杀人",
			setupAction: func() *game.Action {
				werewolfRole, _ := role.New(game.RoleTypeWerewolf)
				villagerRole, _ := role.New(game.RoleTypeVillager)
				return &game.Action{
					Caster: player.New(werewolfRole),
					Target: player.New(villagerRole),
					Skill:  skill.NewKillSkill(),
				}
			},
			wantErr: false,
		},
		{
			name: "目标被守卫保护无法击杀",
			setupAction: func() *game.Action {
				werewolfRole, _ := role.New(game.RoleTypeWerewolf)
				villagerRole, _ := role.New(game.RoleTypeVillager)
				p := player.New(villagerRole)
				p.SetProtected(true)
				return &game.Action{
					Caster: player.New(werewolfRole),
					Target: p,
					Skill:  skill.NewKillSkill(),
				}
			},
			wantErr:     true,
			targetAlive: true,
		},
		{
			name: "女巫使用毒药",
			setupAction: func() *game.Action {
				witchRole, _ := role.New(game.RoleTypeWitch)
				werewolfRole, _ := role.New(game.RoleTypeWerewolf)
				return &game.Action{
					Caster: player.New(witchRole),
					Target: player.New(werewolfRole),
					Skill:  skill.NewPoisonSkill(),
				}
			},
			wantErr:     false,
			targetAlive: false,
		},
		{
			name: "预言家查验",
			setupAction: func() *game.Action {
				seerRole, _ := role.New(game.RoleTypeSeer)
				werewolfRole, _ := role.New(game.RoleTypeWerewolf)
				return &game.Action{
					Caster: player.New(seerRole),
					Target: player.New(werewolfRole),
					Skill:  skill.NewCheckSkill(),
				}
			},
			wantErr:     false,
			targetAlive: true,
		},
		{
			name: "守卫保护",
			setupAction: func() *game.Action {
				guardRole, _ := role.New(game.RoleTypeGuard)
				villagerRole, _ := role.New(game.RoleTypeVillager)
				return &game.Action{
					Caster: player.New(guardRole),
					Target: player.New(villagerRole),
					Skill:  skill.NewProtectSkill(),
				}
			},
			wantErr:     false,
			targetAlive: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phase := NewNightPhase()
			action := tt.setupAction()
			if err := phase.Handle(action); (err != nil) != tt.wantErr {
				t.Errorf("Handle() error = %v, wantErr %v", err, tt.wantErr)
			}

			if action.Target.IsAlive() != tt.targetAlive {
				t.Errorf("Target alive status = %v, want %v", action.Target.IsAlive(), tt.targetAlive)
			}
		})
	}
}

func TestNightPhase_GetPhaseResult(t *testing.T) {
	tests := []struct {
		name         string
		setupActions func() []*game.Action
		checkResult  func(*testing.T, *game.PhaseResult[game.SkillResultMap])
	}{
		{
			name: "狼人杀人场景",
			setupActions: func() []*game.Action {
				werewolfRole, _ := role.New(game.RoleTypeWerewolf)
				villagerRole, _ := role.New(game.RoleTypeVillager)
				witchRole, _ := role.New(game.RoleTypeWitch)

				return []*game.Action{
					{
						Caster: player.New(werewolfRole),
						Target: player.New(villagerRole),
						Skill:  skill.NewKillSkill(),
					},
					{
						Caster: player.New(witchRole),
						Target: player.New(villagerRole),
						Skill:  skill.NewSaveSkill(),
					},
				}
			},
			checkResult: func(t *testing.T, result *game.PhaseResult[game.SkillResultMap]) {
				// ... 检查逻辑
			},
		},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phase := NewNightPhase()
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

func TestNightPhase_Reset(t *testing.T) {
	phase := NewNightPhase()
	werewolfRole, _ := role.New(game.RoleTypeWerewolf)
	villagerRole, _ := role.New(game.RoleTypeVillager)

	werewolf := player.New(werewolfRole)
	villager := player.New(villagerRole)

	// 添加一些动作
	action := &game.Action{
		Caster: werewolf,
		Target: villager,
		Skill:  skill.NewKillSkill(),
	}
	phase.Handle(action)

	// 重置
	phase.Reset()

	// 验证重置后的状态
	if len(phase.actions) != 0 {
		t.Error("Actions should be empty after reset")
	}
	if len(phase.deaths) != 0 {
		t.Error("Deaths should be empty after reset")
	}
	if len(phase.skillResults) != 0 {
		t.Error("Skill results should be empty after reset")
	}
}
