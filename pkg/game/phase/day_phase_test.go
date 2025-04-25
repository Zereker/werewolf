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
						Caster:  player.New(villagerRole1),
						Skill:   skill.NewSpeakSkill(),
						Content: "我是村民1",
					},
					{
						Caster:  player.New(villagerRole2),
						Skill:   skill.NewSpeakSkill(),
						Content: "我是村民2",
					},
					{
						Caster:  player.New(werewolfRole),
						Skill:   skill.NewSpeakSkill(),
						Content: "我是好人",
					},
				}
			},
			checkResult: func(t *testing.T, result *game.PhaseResult[game.SkillResultMap]) {
				// 检查发言结果
				speakResult := result.ExtraData[game.SkillTypeSpeak]
				if speakResult == nil {
					t.Errorf("发言结果为空，期望包含 SkillTypeSpeak 的结果")
					return
				}

				// 检查发言数据
				speakData, ok := speakResult.Data.(map[string]interface{})
				if !ok {
					t.Errorf("发言结果数据类型错误，期望 map[string]interface{}，实际类型 %T", speakResult.Data)
					return
				}

				// 检查发言记录
				spoken, ok := speakData["spoken"].(map[game.Player]string)
				if !ok {
					t.Errorf("发言记录数据类型错误，期望 map[game.Player]string，实际类型 %T", speakData["spoken"])
					return
				}

				// 检查发言人数
				if len(spoken) != 3 {
					t.Errorf("发言人数错误，期望 3 人，实际 %d 人", len(spoken))
				}

				// 检查发言内容
				for _, content := range spoken {
					if content == "" {
						t.Error("发言内容不应为空")
					}
				}
			},
		},
		{
			name: "遗言",
			setupActions: func() []*game.Action {
				villagerRole, _ := role.New(game.RoleTypeVillager)
				deadPlayer := player.New(villagerRole)
				deadPlayer.SetAlive(false) // 设置玩家为死亡状态
				return []*game.Action{
					{
						Caster:  deadPlayer,
						Skill:   skill.NewLastWordsSkill(),
						Content: "我是村民，我被冤枉了",
					},
				}
			},
			checkResult: func(t *testing.T, result *game.PhaseResult[game.SkillResultMap]) {
				// 检查遗言结果
				lastWordsResult := result.ExtraData[game.SkillTypeLastWords]
				if lastWordsResult == nil {
					t.Errorf("遗言结果为空，期望包含 SkillTypeLastWords 的结果")
					return
				}

				// 检查遗言数据
				lastWordsData, ok := lastWordsResult.Data.(map[string]interface{})
				if !ok {
					t.Errorf("遗言结果数据类型错误，期望 map[string]interface{}，实际类型 %T", lastWordsResult.Data)
					return
				}

				// 检查遗言记录
				lastWords, ok := lastWordsData["lastWords"].(map[game.Player]string)
				if !ok {
					t.Errorf("遗言记录数据类型错误，期望 map[game.Player]string，实际类型 %T", lastWordsData["lastWords"])
					return
				}

				// 检查遗言人数
				if len(lastWords) != 1 {
					t.Errorf("遗言人数错误，期望 1 人，实际 %d 人", len(lastWords))
				}

				// 检查遗言内容
				for _, content := range lastWords {
					if content == "" {
						t.Error("遗言内容不应为空")
					}
				}
			},
		},
		{
			name: "混合场景",
			setupActions: func() []*game.Action {
				villagerRole1, _ := role.New(game.RoleTypeVillager)
				villagerRole2, _ := role.New(game.RoleTypeVillager)
				werewolfRole, _ := role.New(game.RoleTypeWerewolf)

				deadPlayer := player.New(villagerRole1)
				deadPlayer.SetAlive(false)

				return []*game.Action{
					{
						Caster:  player.New(villagerRole2),
						Skill:   skill.NewSpeakSkill(),
						Content: "我是村民2",
					},
					{
						Caster:  player.New(werewolfRole),
						Skill:   skill.NewSpeakSkill(),
						Content: "我是好人",
					},
					{
						Caster:  deadPlayer,
						Skill:   skill.NewLastWordsSkill(),
						Content: "我是村民，我被冤枉了",
					},
				}
			},
			checkResult: func(t *testing.T, result *game.PhaseResult[game.SkillResultMap]) {
				// 检查发言结果
				speakResult := result.ExtraData[game.SkillTypeSpeak]
				if speakResult == nil {
					t.Errorf("发言结果为空，期望包含 SkillTypeSpeak 的结果")
					return
				}

				speakData, ok := speakResult.Data.(map[string]interface{})
				if !ok {
					t.Errorf("发言结果数据类型错误，期望 map[string]interface{}，实际类型 %T", speakResult.Data)
					return
				}

				spoken, ok := speakData["spoken"].(map[game.Player]string)
				if !ok {
					t.Errorf("发言记录数据类型错误，期望 map[game.Player]string，实际类型 %T", speakData["spoken"])
					return
				}

				if len(spoken) != 2 {
					t.Errorf("发言人数错误，期望 2 人，实际 %d 人", len(spoken))
				}

				// 检查发言内容
				for _, content := range spoken {
					if content == "" {
						t.Error("发言内容不应为空")
					}
				}

				// 检查遗言结果
				lastWordsResult := result.ExtraData[game.SkillTypeLastWords]
				if lastWordsResult == nil {
					t.Errorf("遗言结果为空，期望包含 SkillTypeLastWords 的结果")
					return
				}

				lastWordsData, ok := lastWordsResult.Data.(map[string]interface{})
				if !ok {
					t.Errorf("遗言结果数据类型错误，期望 map[string]interface{}，实际类型 %T", lastWordsResult.Data)
					return
				}

				lastWords, ok := lastWordsData["lastWords"].(map[game.Player]string)
				if !ok {
					t.Errorf("遗言记录数据类型错误，期望 map[game.Player]string，实际类型 %T", lastWordsData["lastWords"])
					return
				}

				if len(lastWords) != 1 {
					t.Errorf("遗言人数错误，期望 1 人，实际 %d 人", len(lastWords))
				}

				// 检查遗言内容
				for _, content := range lastWords {
					if content == "" {
						t.Error("遗言内容不应为空")
					}
				}
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
