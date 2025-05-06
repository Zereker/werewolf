package phase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/skill"
)

func TestNewDayPhase(t *testing.T) {
	players := []game.Player{
		NewMockPlayer("player1", game.RoleTypeVillager),
		NewMockPlayer("player2", game.RoleTypeVillager),
	}

	phase := NewDayPhase(players)
	assert.NotNil(t, phase)
	assert.Equal(t, game.PhaseDay, phase.GetName())
	assert.Equal(t, 5*time.Minute, phase.discussionTime)
}

func TestDayPhase_Start(t *testing.T) {
	// 创建测试玩家
	mockPlayer1 := NewMockPlayer("player1", game.RoleTypeVillager)
	mockPlayer2 := NewMockPlayer("player2", game.RoleTypeVillager)
	players := []game.Player{mockPlayer1, mockPlayer2}

	// 设置玩家的发言事件
	mockPlayer1.SetNextSkillEvent(game.SkillTypeSpeak, "", "我是好人")
	mockPlayer2.SetNextSkillEvent(game.SkillTypeSpeak, "", "我也是好人")

	// 创建白天阶段
	phase := NewDayPhase(players)
	phase.discussionTime = 100 * time.Millisecond // 缩短讨论时间用于测试

	// 启动阶段
	ctx := context.Background()
	err := phase.Start(ctx)
	assert.NoError(t, err)
}

func TestDayPhase_GetPhaseResult(t *testing.T) {
	// 创建测试玩家
	mockPlayer1 := NewMockPlayer("player1", game.RoleTypeVillager)
	mockPlayer2 := NewMockPlayer("player2", game.RoleTypeVillager)
	players := []game.Player{mockPlayer1, mockPlayer2}

	// 创建白天阶段
	phase := NewDayPhase(players)

	// 添加一些测试行动
	speakSkill := skill.NewSpeakSkill()
	action1 := &game.Action{
		Caster: mockPlayer1,
		Target: nil,
		Skill:  speakSkill,
	}
	action2 := &game.Action{
		Caster: mockPlayer2,
		Target: nil,
		Skill:  speakSkill,
	}

	phase.AddAction(action1)
	phase.AddAction(action2)

	// 获取阶段结果
	result := phase.GetPhaseResult()
	assert.NotNil(t, result)

	// 验证结果中包含了发言记录
	speakResult, exists := result.ExtraData[game.SkillTypeSpeak]
	assert.True(t, exists)
	assert.True(t, speakResult.Success)

	// 验证发言内容
	data, ok := speakResult.Data.(map[string]interface{})
	assert.True(t, ok)
	spoken, ok := data["spoken"].(map[game.Player]string)
	assert.True(t, ok)
	assert.Equal(t, "我是好人", spoken[mockPlayer1])
	assert.Equal(t, "我也是好人", spoken[mockPlayer2])
}
