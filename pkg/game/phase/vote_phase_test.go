package phase

import (
	"testing"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/player"
	"github.com/Zereker/werewolf/pkg/game/role"
	"github.com/Zereker/werewolf/pkg/game/skill"
)

func TestVotePhase_GetName(t *testing.T) {
	phase := NewVotePhase()
	if got := phase.GetName(); got != game.PhaseVote {
		t.Errorf("GetName() = %v, want %v", got, game.PhaseVote)
	}
}

func TestVotePhase_Handle(t *testing.T) {
	tests := []struct {
		name        string
		setupAction func() *game.Action
		wantErr     bool
	}{
		{
			name: "活人投票",
			setupAction: func() *game.Action {
				villagerRole1, _ := role.New(game.RoleTypeVillager)
				villagerRole2, _ := role.New(game.RoleTypeVillager)
				return &game.Action{
					Caster: player.New(villagerRole1),
					Target: player.New(villagerRole2),
					Skill:  skill.NewVoteSkill(),
				}
			},
			wantErr: false,
		},
		{
			name: "死亡玩家不能投票",
			setupAction: func() *game.Action {
				villagerRole2, _ := role.New(game.RoleTypeVillager)
				werewolfRole, _ := role.New(game.RoleTypeWerewolf)
				p := player.New(villagerRole2)
				p.SetAlive(false)
				return &game.Action{
					Caster: p,
					Target: player.New(werewolfRole),
					Skill:  skill.NewVoteSkill(),
				}
			},
			wantErr: true,
		},
		{
			name: "不能投票给死亡玩家",
			setupAction: func() *game.Action {
				villagerRole1, _ := role.New(game.RoleTypeVillager)
				werewolfRole, _ := role.New(game.RoleTypeWerewolf)
				p := player.New(werewolfRole)
				p.SetAlive(false)
				return &game.Action{
					Caster: player.New(villagerRole1),
					Target: p,
					Skill:  skill.NewVoteSkill(),
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phase := NewVotePhase()
			action := tt.setupAction()
			if err := phase.Handle(action); (err != nil) != tt.wantErr {
				t.Errorf("Handle() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVotePhase_Reset(t *testing.T) {
	phase := NewVotePhase()
	villagerRole, _ := role.New(game.RoleTypeVillager)
	werewolfRole, _ := role.New(game.RoleTypeWerewolf)

	villager := player.New(villagerRole)
	werewolf := player.New(werewolfRole)

	// 添加一些投票动作
	action := &game.Action{
		Caster: villager,
		Target: werewolf,
		Skill:  skill.NewVoteSkill(),
	}
	phase.Handle(action)

	// 重置
	phase.Start()

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

func TestVotePhase_GetPhaseResult(t *testing.T) {
	tests := []struct {
		name           string
		setupActions   func() []*game.Action
		expectedDeaths int
		checkResult    func(*testing.T, *game.PhaseResult[game.SkillResultMap])
	}{
		{
			name: "一致投票出局狼人",
			setupActions: func() []*game.Action {
				// 创建角色
				villager1 := player.New(role.NewVillager())
				villager2 := player.New(role.NewVillager())
				villager3 := player.New(role.NewVillager())
				werewolf := player.New(role.NewWerewolf())

				return []*game.Action{
					{
						Caster: villager1,
						Target: werewolf,
						Skill:  skill.NewVoteSkill(),
					},
					{
						Caster: villager2,
						Target: werewolf,
						Skill:  skill.NewVoteSkill(),
					},
					{
						Caster: villager3,
						Target: werewolf,
						Skill:  skill.NewVoteSkill(),
					},
					{
						Caster: werewolf,
						Target: villager1,
						Skill:  skill.NewVoteSkill(),
					},
				}
			},
			expectedDeaths: 1,
			checkResult: func(t *testing.T, result *game.PhaseResult[game.SkillResultMap]) {
				// 检查死亡人数
				if len(result.Deaths) != 1 {
					t.Errorf("期望1个玩家死亡，实际有%d个", len(result.Deaths))
				}

				// 检查投票结果
				voteResult := result.ExtraData[game.SkillTypeVote]
				if voteResult == nil {
					t.Fatal("投票结果不应为空")
				}

				// 检查死亡玩家是否是狼人
				if len(result.Deaths) > 0 {
					deadPlayer := result.Deaths[0]
					if deadPlayer.GetRole().GetName() != game.RoleTypeWerewolf {
						t.Errorf("期望死亡玩家是狼人，实际是%s", deadPlayer.GetRole().GetName())
					}
				}

				// 检查投票记录
				voteData := voteResult.Data.(map[string]interface{})
				votes := voteData["votes"].(map[game.Player]game.Player)
				if len(votes) != 4 {
					t.Errorf("期望4条投票记录，实际有%d条", len(votes))
				}

				// 检查票数统计
				voteCount := voteData["voteCount"].(map[game.Player]int)
				for player, count := range voteCount {
					if player.GetRole().GetName() == game.RoleTypeWerewolf && count != 3 {
						t.Errorf("期望狼人获得3票，实际获得%d票", count)
					}
				}
			},
		},
		{
			name: "平票无人出局",
			setupActions: func() []*game.Action {
				// 创建角色
				villager1 := player.New(role.NewVillager())
				villager2 := player.New(role.NewVillager())
				wolf1 := player.New(role.NewWerewolf())
				wolf2 := player.New(role.NewWerewolf())

				return []*game.Action{
					{
						Caster: villager1,
						Target: wolf1,
						Skill:  skill.NewVoteSkill(),
					},
					{
						Caster: villager2,
						Target: wolf1,
						Skill:  skill.NewVoteSkill(),
					},
					{
						Caster: wolf1,
						Target: wolf2,
						Skill:  skill.NewVoteSkill(),
					},
					{
						Caster: wolf2,
						Target: wolf2,
						Skill:  skill.NewVoteSkill(),
					},
				}
			},
			expectedDeaths: 0,
			checkResult: func(t *testing.T, result *game.PhaseResult[game.SkillResultMap]) {
				// 检查无人死亡
				if len(result.Deaths) != 0 {
					t.Errorf("期望无人死亡，实际有%d人死亡", len(result.Deaths))
				}

				// 检查投票结果
				voteResult := result.ExtraData[game.SkillTypeVote]
				if voteResult == nil {
					t.Fatal("投票结果不应为空")
				}

				// 检查投票记录
				voteData := voteResult.Data.(map[string]interface{})
				votes := voteData["votes"].(map[game.Player]game.Player)
				if len(votes) != 4 {
					t.Errorf("期望4条投票记录，实际有%d条", len(votes))
				}

				// 检查票数统计
				voteCount := voteData["voteCount"].(map[game.Player]int)
				for _, count := range voteCount {
					if count > 2 {
						t.Errorf("平票情况下单个玩家不应获得超过2票，实际获得%d票", count)
					}
				}
			},
		},
		{
			name: "单人投票无效",
			setupActions: func() []*game.Action {
				villager := player.New(role.NewVillager())
				werewolf := player.New(role.NewWerewolf())

				return []*game.Action{
					{
						Caster: villager,
						Target: werewolf,
						Skill:  skill.NewVoteSkill(),
					},
				}
			},
			expectedDeaths: 0,
			checkResult: func(t *testing.T, result *game.PhaseResult[game.SkillResultMap]) {
				// 检查无人死亡
				if len(result.Deaths) != 0 {
					t.Errorf("单人投票应该无效，期望无人死亡，实际有%d人死亡", len(result.Deaths))
				}

				// 检查投票记录
				voteResult := result.ExtraData[game.SkillTypeVote]
				if voteResult == nil {
					t.Fatal("投票结果不应为空")
				}

				voteData := voteResult.Data.(map[string]interface{})
				votes := voteData["votes"].(map[game.Player]game.Player)
				if len(votes) != 1 {
					t.Errorf("期望1条投票记录，实际有%d条", len(votes))
				}
			},
		},
		{
			name: "全员弃权",
			setupActions: func() []*game.Action {
				return []*game.Action{} // 空的动作列表表示没有人投票
			},
			expectedDeaths: 0,
			checkResult: func(t *testing.T, result *game.PhaseResult[game.SkillResultMap]) {
				// 检查无人死亡
				if len(result.Deaths) != 0 {
					t.Errorf("全员弃权应该无人死亡，实际有%d人死亡", len(result.Deaths))
				}

				// 检查投票结果
				voteResult := result.ExtraData[game.SkillTypeVote]
				if voteResult == nil {
					t.Fatal("投票结果不应为空")
				}

				// 检查投票记录为空
				voteData := voteResult.Data.(map[string]interface{})
				votes := voteData["votes"].(map[game.Player]game.Player)
				if len(votes) != 0 {
					t.Errorf("期望0条投票记录，实际有%d条", len(votes))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phase := NewVotePhase()

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
