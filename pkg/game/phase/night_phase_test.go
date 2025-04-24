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
			wantErr: true,
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
			wantErr: false,
		},
		{
			name: "预言家查验",
			setupAction: func() *game.Action {
				seerRole, _ := role.New(game.RoleTypeSeer)
				werewolfRole, _ := role.New(game.RoleTypeWerewolf)
				target := player.New(werewolfRole)
				return &game.Action{
					Caster: player.New(seerRole),
					Target: target,
					Skill:  skill.NewCheckSkill(),
				}
			},
			wantErr: false,
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
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phase := NewNightPhase()
			action := tt.setupAction()
			if err := phase.Handle(action); (err != nil) != tt.wantErr {
				t.Errorf("Handle() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNightPhase_GetPhaseResult(t *testing.T) {
	tests := []struct {
		name           string
		setupActions   func() []*game.Action
		expectedDeaths int
		checkResult    func(*testing.T, *game.PhaseResult[game.SkillResultMap])
	}{
		{
			name: "狼人杀人场景",
			setupActions: func() []*game.Action {
				werewolfRole, _ := role.New(game.RoleTypeWerewolf)
				villagerRole, _ := role.New(game.RoleTypeVillager)
				target := player.New(villagerRole)
				return []*game.Action{
					{
						Caster: player.New(werewolfRole),
						Target: target,
						Skill:  skill.NewKillSkill(),
					},
				}
			},
			expectedDeaths: 1,
			checkResult: func(t *testing.T, result *game.PhaseResult[game.SkillResultMap]) {
				if len(result.Deaths) != 1 {
					t.Errorf("Expected 1 death, got %d", len(result.Deaths))
				}

				// 检查死亡玩家是否是村民
				if len(result.Deaths) > 0 {
					deadPlayer := result.Deaths[0]
					if deadPlayer.GetRole().GetName() != game.RoleTypeVillager {
						t.Errorf("Expected dead player to be villager, got %s", deadPlayer.GetRole().GetName())
					}
				}
			},
		},
		{
			name: "狼人杀人被女巫救场景",
			setupActions: func() []*game.Action {
				werewolfRole, _ := role.New(game.RoleTypeWerewolf)
				villagerRole, _ := role.New(game.RoleTypeVillager)
				witchRole, _ := role.New(game.RoleTypeWitch)

				target := player.New(villagerRole)
				return []*game.Action{
					{
						Caster: player.New(werewolfRole),
						Target: target,
						Skill:  skill.NewKillSkill(),
					},
					{
						Caster: player.New(witchRole),
						Target: target,
						Skill:  skill.NewAntidoteSkill(),
					},
				}
			},
			expectedDeaths: 0,
			checkResult: func(t *testing.T, result *game.PhaseResult[game.SkillResultMap]) {
				if len(result.Deaths) != 0 {
					t.Errorf("Expected no deaths due to witch save, got %d", len(result.Deaths))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phase := NewNightPhase()
			actions := tt.setupActions()
			for _, action := range actions {
				err := phase.Handle(action)
				if err != nil {
					t.Errorf("Handle() error = %v", err)
				}
			}

			result := phase.GetPhaseResult()
			if len(result.Deaths) != tt.expectedDeaths {
				t.Errorf("Expected %d deaths, got %d", tt.expectedDeaths, len(result.Deaths))
			}

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

	action := &game.Action{
		Caster: werewolf,
		Target: villager,
		Skill:  skill.NewKillSkill(),
	}
	phase.Handle(action)

	phase.Reset()

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
