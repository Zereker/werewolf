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
					Caster: player.New("", werewolfRole),
					Target: player.New("", villagerRole),
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
				p := player.New("", villagerRole)
				p.SetProtected(true)
				return &game.Action{
					Caster: player.New("", werewolfRole),
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
					Caster: player.New("", witchRole),
					Target: player.New("", werewolfRole),
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
				target := player.New("", werewolfRole)
				return &game.Action{
					Caster: player.New("", seerRole),
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
					Caster: player.New("", guardRole),
					Target: player.New("", villagerRole),
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
				villager := player.New("v1", role.NewVillager())
				werewolf := player.New("w1", role.NewWerewolf())

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
				villager := player.New("v1", role.NewVillager())
				werewolf := player.New("w1", role.NewWerewolf())
				witch := player.New("witch1", role.NewWitch())

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
				werewolf := player.New("w1", role.NewWerewolf())
				witch := player.New("witch1", role.NewWitch())

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
				villager := player.New("v1", role.NewVillager())
				werewolf := player.New("w1", role.NewWerewolf())
				guard := player.New("g1", role.NewGuard())

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
				villager := player.New("v1", role.NewVillager())
				werewolf := player.New("w1", role.NewWerewolf())
				witch := player.New("witch1", role.NewWitch())

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
				werewolf := player.New("w1", role.NewWerewolf())
				seer := player.New("s1", role.NewSeer())

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
		{
			name: "守卫不能连续保护同一个人",
			setupActions: func() []*game.Action {
				villager := player.New("v1", role.NewVillager())
				guard := player.New("g1", role.NewGuard())

				// 模拟守卫已经在上一轮保护过该村民
				villager.SetProtected(true)

				return []*game.Action{
					{
						Caster: guard,
						Target: villager,
						Skill:  skill.NewProtectSkill(),
					},
				}
			},
			expectedDeaths: 0,
			checkResult: func(t *testing.T, result *game.PhaseResult[game.SkillResultMap]) {
				protectResult := result.ExtraData[game.SkillTypeProtect]
				if protectResult == nil {
					t.Error("应该有守卫技能的结果")
				}
				if protectResult.Success {
					t.Error("守卫不应该能够连续保护同一个人")
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

	werewolf := player.New("", werewolfRole)
	villager := player.New("", villagerRole)

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
