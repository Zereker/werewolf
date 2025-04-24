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

func TestVotePhase_GetPhaseResult(t *testing.T) {
	tests := []struct {
		name           string
		setupActions   func() []*game.Action
		expectedDeaths int
		checkResult    func(*testing.T, *game.PhaseResult[game.SkillResultMap])
	}{
		{
			name: "一致投票出局",
			setupActions: func() []*game.Action {
				// 在每个测试用例中创建新的角色
				villagerRole1, _ := role.New(game.RoleTypeVillager)
				villagerRole2, _ := role.New(game.RoleTypeVillager)
				villagerRole3, _ := role.New(game.RoleTypeVillager)
				werewolfRole, _ := role.New(game.RoleTypeWerewolf)

				werewolf := player.New(werewolfRole)
				return []*game.Action{
					{
						Caster: player.New(villagerRole1),
						Target: werewolf,
						Skill:  skill.NewVoteSkill(),
					},
					{
						Caster: player.New(villagerRole2),
						Target: werewolf,
						Skill:  skill.NewVoteSkill(),
					},
					{
						Caster: player.New(villagerRole3),
						Target: werewolf,
						Skill:  skill.NewVoteSkill(),
					},
					{
						Caster: werewolf,
						Target: player.New(villagerRole1),
						Skill:  skill.NewVoteSkill(),
					},
				}
			},
			expectedDeaths: 1,
			checkResult: func(t *testing.T, result *game.PhaseResult[game.SkillResultMap]) {
				if len(result.Deaths) != 1 {
					t.Errorf("Expected 1 death, got %d", len(result.Deaths))
				}

				voteResult := result.ExtraData[game.SkillTypeVote]
				if voteResult == nil {
					t.Error("Vote result should not be nil")
					return
				}

				// 检查死亡的玩家是否是狼人
				if len(result.Deaths) > 0 {
					deadPlayer := result.Deaths[0]
					if deadPlayer.GetRole().GetName() != game.RoleTypeWerewolf {
						t.Errorf("Expected dead player to be werewolf, got %s", deadPlayer.GetRole().GetName())
					}
				}

				// 检查投票记录数量
				voteData := voteResult.Data.(map[string]interface{})
				votes := voteData["votes"].(map[string]string)
				if len(votes) != 4 {
					t.Errorf("Expected 4 vote records, got %d", len(votes))
				}
			},
		},
		{
			name: "平票无人出局",
			setupActions: func() []*game.Action {
				// 在每个测试用例中创建新的角色
				villagerRole1, _ := role.New(game.RoleTypeVillager)
				villagerRole2, _ := role.New(game.RoleTypeVillager)
				villagerRole3, _ := role.New(game.RoleTypeVillager)
				werewolfRole, _ := role.New(game.RoleTypeWerewolf)

				return []*game.Action{
					{
						Caster: player.New(villagerRole1),
						Target: player.New(werewolfRole),
						Skill:  skill.NewVoteSkill(),
					},
					{
						Caster: player.New(villagerRole2),
						Target: player.New(villagerRole3),
						Skill:  skill.NewVoteSkill(),
					},
					{
						Caster: player.New(villagerRole3),
						Target: player.New(villagerRole2),
						Skill:  skill.NewVoteSkill(),
					},
					{
						Caster: player.New(werewolfRole),
						Target: player.New(villagerRole1),
						Skill:  skill.NewVoteSkill(),
					},
				}
			},
			expectedDeaths: 0,
			checkResult: func(t *testing.T, result *game.PhaseResult[game.SkillResultMap]) {
				if len(result.Deaths) != 0 {
					t.Errorf("Expected no deaths, got %d", len(result.Deaths))
				}

				voteResult := result.ExtraData[game.SkillTypeVote]
				if voteResult == nil {
					t.Error("Vote result should not be nil")
					return
				}

				// 检查投票记录数量
				voteData := voteResult.Data.(map[string]interface{})
				votes := voteData["votes"].(map[string]string)
				if len(votes) != 4 {
					t.Errorf("Expected 4 vote records, got %d", len(votes))
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

			if len(result.Deaths) != tt.expectedDeaths {
				t.Errorf("Expected %d deaths, got %d", tt.expectedDeaths, len(result.Deaths))
			}

			tt.checkResult(t, result)
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

func TestVotePhase_MultipleVotes(t *testing.T) {
	tests := []struct {
		name         string
		setupActions func() []*game.Action
		checkResult  func(*testing.T, *game.PhaseResult[game.SkillResultMap])
	}{
		{
			name: "多人投票",
			setupActions: func() []*game.Action {
				villagerRole1, _ := role.New(game.RoleTypeVillager)
				villagerRole2, _ := role.New(game.RoleTypeVillager)
				werewolfRole, _ := role.New(game.RoleTypeWerewolf)

				return []*game.Action{
					{
						Caster: player.New(villagerRole1),
						Target: player.New(werewolfRole),
						Skill:  skill.NewVoteSkill(),
					},
					{
						Caster: player.New(villagerRole2),
						Target: player.New(werewolfRole),
						Skill:  skill.NewVoteSkill(),
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
			phase := NewVotePhase()
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

func TestVotePhase_VoteOrder(t *testing.T) {
	phase := NewVotePhase()

	// 创建多个角色和玩家
	roles := make([]game.Role, 4)
	players := make([]game.Player, 4)

	for i := 0; i < 4; i++ {
		if i < 2 {
			roles[i], _ = role.New(game.RoleTypeVillager)
		} else {
			roles[i], _ = role.New(game.RoleTypeWerewolf)
		}
		players[i] = player.New(roles[i])
	}

	// 按不同顺序投票
	voteSkill := skill.NewVoteSkill()
	phase.Handle(&game.Action{Caster: players[0], Target: players[2], Skill: voteSkill})
	phase.Handle(&game.Action{Caster: players[1], Target: players[2], Skill: voteSkill})
	phase.Handle(&game.Action{Caster: players[2], Target: players[1], Skill: voteSkill})
	phase.Handle(&game.Action{Caster: players[3], Target: players[1], Skill: voteSkill})

	result := phase.GetPhaseResult()

	// 验证投票结果
	voteResult := result.ExtraData[game.SkillTypeVote]
	if voteResult == nil {
		t.Fatal("Vote result should not be nil")
	}

	voteData, ok := voteResult.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Failed to convert vote result data")
	}

	// 检查投票计数
	voteCount, ok := voteData["voteCount"].(map[game.Player]int)
	if !ok {
		t.Fatal("Failed to convert vote count data")
	}

	// 验证票数
	if count := voteCount[players[2]]; count != 2 {
		t.Errorf("Player[2] should have 2 votes, got %d", count)
	}
	if count := voteCount[players[1]]; count != 2 {
		t.Errorf("Player[1] should have 2 votes, got %d", count)
	}

	// 由于是平票，应该没有玩家被投出
	if len(result.Deaths) != 0 {
		t.Errorf("Expected no deaths in tie vote, got %d deaths", len(result.Deaths))
	}
}
