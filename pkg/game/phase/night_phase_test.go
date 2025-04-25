package phase

import (
	"testing"

	"github.com/stretchr/testify/assert"

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
				villager := player.New(role.NewVillager())
				werewolf := player.New(role.NewWerewolf())

				return []*game.Action{
					{
						Caster: werewolf,
						Target: villager,
						Skill:  skill.NewKillSkill(),
					},
				}
			},
			expectedDeaths: 1,
			checkResult: func(t *testing.T, result *game.PhaseResult[game.SkillResultMap]) {
				if len(result.Deaths) != 1 {
					t.Errorf("期望1个玩家死亡，实际有%d个", len(result.Deaths))
				}
				if len(result.Deaths) > 0 && result.Deaths[0].GetRole().GetName() != game.RoleTypeVillager {
					t.Error("期望死亡的是村民")
				}
			},
		},
		{
			name: "狼人杀人被女巫救场景",
			setupActions: func() []*game.Action {
				villager := player.New(role.NewVillager())
				werewolf := player.New(role.NewWerewolf())
				witch := player.New(role.NewWitch())

				return []*game.Action{
					{
						Caster: werewolf,
						Target: villager,
						Skill:  skill.NewKillSkill(),
					},
					{
						Caster: witch,
						Target: villager,
						Skill:  skill.NewAntidoteSkill(),
					},
				}
			},
			expectedDeaths: 0,
			checkResult: func(t *testing.T, result *game.PhaseResult[game.SkillResultMap]) {
				if len(result.Deaths) != 0 {
					t.Error("期望没有玩家死亡，因为女巫救人")
				}
			},
		},
		{
			name: "女巫毒杀场景",
			setupActions: func() []*game.Action {
				werewolf := player.New(role.NewWerewolf())
				witch := player.New(role.NewWitch())

				return []*game.Action{
					{
						Caster: witch,
						Target: werewolf,
						Skill:  skill.NewPoisonSkill(),
					},
				}
			},
			expectedDeaths: 1,
			checkResult: func(t *testing.T, result *game.PhaseResult[game.SkillResultMap]) {
				if len(result.Deaths) != 1 {
					t.Errorf("期望1个玩家死亡，实际有%d个", len(result.Deaths))
				}
				if len(result.Deaths) > 0 && result.Deaths[0].GetRole().GetName() != game.RoleTypeWerewolf {
					t.Error("期望死亡的是狼人")
				}
			},
		},
		{
			name: "守卫保护场景",
			setupActions: func() []*game.Action {
				villager := player.New(role.NewVillager())
				werewolf := player.New(role.NewWerewolf())
				guard := player.New(role.NewGuard())

				return []*game.Action{
					{
						Caster: guard,
						Target: villager,
						Skill:  skill.NewProtectSkill(),
					},
					{
						Caster: werewolf,
						Target: villager,
						Skill:  skill.NewKillSkill(),
					},
				}
			},
			expectedDeaths: 0,
			checkResult: func(t *testing.T, result *game.PhaseResult[game.SkillResultMap]) {
				if len(result.Deaths) != 0 {
					t.Error("期望没有玩家死亡，因为守卫保护")
				}
			},
		},
		{
			name: "复杂场景：狼人杀人+女巫救+女巫毒",
			setupActions: func() []*game.Action {
				villager := player.New(role.NewVillager())
				werewolf := player.New(role.NewWerewolf())
				witch := player.New(role.NewWitch())

				return []*game.Action{
					{
						Caster: werewolf,
						Target: villager,
						Skill:  skill.NewKillSkill(),
					},
					{
						Caster: witch,
						Target: villager,
						Skill:  skill.NewAntidoteSkill(),
					},
					{
						Caster: witch,
						Target: werewolf,
						Skill:  skill.NewPoisonSkill(),
					},
				}
			},
			expectedDeaths: 1,
			checkResult: func(t *testing.T, result *game.PhaseResult[game.SkillResultMap]) {
				if len(result.Deaths) != 1 {
					t.Errorf("期望1个玩家死亡，实际有%d个", len(result.Deaths))
				}
				if len(result.Deaths) > 0 && result.Deaths[0].GetRole().GetName() != game.RoleTypeWerewolf {
					t.Error("期望死亡的是狼人")
				}

				// 检查技能结果
				if result.ExtraData[game.SkillTypeKill] == nil {
					t.Error("应该有狼人杀人的技能结果")
				}
				if result.ExtraData[game.SkillTypeAntidote] == nil {
					t.Error("应该有女巫救人的技能结果")
				}
				if result.ExtraData[game.SkillTypePoison] == nil {
					t.Error("应该有女巫毒人的技能结果")
				}
			},
		},
		{
			name: "预言家查验场景",
			setupActions: func() []*game.Action {
				werewolf := player.New(role.NewWerewolf())
				seer := player.New(role.NewSeer())

				return []*game.Action{
					{
						Caster: seer,
						Target: werewolf,
						Skill:  skill.NewCheckSkill(),
					},
				}
			},
			expectedDeaths: 0,
			checkResult: func(t *testing.T, result *game.PhaseResult[game.SkillResultMap]) {
				if len(result.Deaths) != 0 {
					t.Error("预言家查验不应导致玩家死亡")
				}

				// 检查查验结果
				checkResult := result.ExtraData[game.SkillTypeCheck]
				if checkResult == nil {
					t.Error("应该有预言家查验的技能结果")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phase := NewNightPhase()

			// 获取并执行测试用例的动作
			actions := tt.setupActions()
			for _, action := range actions {
				err := phase.Handle(action)
				if err != nil {
					t.Errorf("Handle() error = %v", err)
				}
			}

			result := phase.GetPhaseResult()

			// 检查死亡人数
			if len(result.Deaths) != tt.expectedDeaths {
				t.Errorf("期望%d人死亡，实际有%d人死亡", tt.expectedDeaths, len(result.Deaths))
			}

			// 执行测试用例特定的检查
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

func TestNightPhase_Handle(t *testing.T) {
	tests := []struct {
		name         string
		setupPhase   func() *NightPhase
		setupPlayers func() []game.Player
		setupAction  func([]game.Player) *game.Action
		wantErr      string
	}{
		{
			name: "狼人成功杀人",
			setupPhase: func() *NightPhase {
				return NewNightPhase()
			},
			setupPlayers: func() []game.Player {
				werewolf := player.New(role.NewWerewolf())
				villager := player.New(role.NewVillager())
				return []game.Player{werewolf, villager}
			},
			setupAction: func(players []game.Player) *game.Action {
				return &game.Action{
					Skill:  skill.NewKillSkill(),
					Caster: players[0],
					Target: players[1],
				}
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phase := tt.setupPhase()
			players := tt.setupPlayers()
			action := tt.setupAction(players)

			err := phase.Handle(action)

			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
