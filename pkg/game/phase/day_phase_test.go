package phase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Zereker/werewolf/pkg/game"
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
	//
	phase.discussionTime = 100 * time.Millisecond // 缩短讨论时间用于测试

	// 启动阶段
	ctx := context.Background()
	err := phase.Start(ctx)
	assert.NoError(t, err)
}
