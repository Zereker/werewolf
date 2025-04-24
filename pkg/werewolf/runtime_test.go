package werewolf

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/role"
)

func TestRuntime_Init(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() *Runtime
		wantErr   bool
	}{
		{
			name: "正常初始化",
			setupFunc: func() *Runtime {
				r := NewRuntime()
				_ = r.AddPlayer("player1", role.NewWerewolf())
				_ = r.AddPlayer("player2", role.NewVillager())
				_ = r.AddPlayer("player3", role.NewVillager())
				return r
			},
			wantErr: false,
		},
		{
			name: "重复初始化",
			setupFunc: func() *Runtime {
				r := NewRuntime()
				_ = r.AddPlayer("player1", role.NewWerewolf())
				_ = r.Init()
				return r
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.setupFunc()
			err := r.Init()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.False(t, r.started)
				assert.NotEmpty(t, r.skills)
			}
		})
	}
}

func TestRuntime_AddPlayer(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() *Runtime
		playerID  string
		role      game.Role
		wantErr   bool
	}{
		{
			name: "添加新玩家",
			setupFunc: func() *Runtime {
				return NewRuntime()
			},
			playerID: "player1",
			role:     role.NewWerewolf(),
			wantErr:  false,
		},
		{
			name: "游戏已开始后添加玩家",
			setupFunc: func() *Runtime {
				r := NewRuntime()
				_ = r.AddPlayer("player1", role.NewWerewolf())
				_ = r.Init()
				// 启动游戏
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				err := r.Start(ctx)
				assert.NoError(t, err)
				return r
			},
			playerID: "player2",
			role:     role.NewVillager(),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.setupFunc()
			err := r.AddPlayer(tt.playerID, tt.role)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				player, exists := r.players[tt.playerID]
				assert.True(t, exists)
				assert.Equal(t, tt.playerID, player.GetID())
			}
		})
	}
}

func TestRuntime_UseSkill(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func() *Runtime
		casterID  string
		targetID  string
		skillType game.SkillType
		wantErr   bool
	}{
		{
			name: "狼人使用技能",
			setupFunc: func() *Runtime {
				r := NewRuntime()
				_ = r.AddPlayer("werewolf", role.NewWerewolf())
				_ = r.AddPlayer("villager", role.NewVillager())
				_ = r.Init()
				// 启动游戏
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				err := r.Start(ctx)
				assert.NoError(t, err)
				return r
			},
			casterID:  "werewolf",
			targetID:  "villager",
			skillType: game.SkillTypeKill,
			wantErr:   false,
		},
		{
			name: "使用不存在的技能",
			setupFunc: func() *Runtime {
				r := NewRuntime()
				_ = r.AddPlayer("villager1", role.NewVillager())
				_ = r.AddPlayer("villager2", role.NewVillager())
				_ = r.Init()

				// 启动游戏
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				err := r.Start(ctx)
				assert.NoError(t, err)

				return r
			},
			casterID:  "villager1",
			targetID:  "villager2",
			skillType: game.SkillTypeKill,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.setupFunc()
			err := r.useSkill(tt.casterID, tt.targetID, tt.skillType)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRuntime_GameFlow(t *testing.T) {
	r := NewRuntime()

	// 添加玩家
	_ = r.AddPlayer("werewolf1", role.NewWerewolf())
	_ = r.AddPlayer("werewolf2", role.NewWerewolf())
	_ = r.AddPlayer("villager1", role.NewVillager())
	_ = r.AddPlayer("villager2", role.NewVillager())
	_ = r.AddPlayer("villager3", role.NewVillager())

	// 初始化游戏
	err := r.Init()
	assert.NoError(t, err)

	// 启动游戏
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = r.Start(ctx)
	assert.NoError(t, err)

	// 测试狼人击杀
	err = r.SendUserEvent("werewolf1", EventUserSkill, &UserSkillData{
		SkillType: game.SkillTypeKill,
		TargetID:  "villager1",
	})
	assert.NoError(t, err)

	// 等待一段时间让事件处理完成
	time.Sleep(1 * time.Second)

	// 验证游戏状态
	assert.True(t, r.started)
	assert.False(t, r.ended)
}

func TestRuntime_EventHandling(t *testing.T) {
	r := NewRuntime()

	// 添加玩家
	_ = r.AddPlayer("player1", role.NewVillager())
	_ = r.AddPlayer("player2", role.NewVillager())
	_ = r.Init()

	// 启动游戏
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := r.Start(ctx)
	assert.NoError(t, err)

	// 测试发送无效事件
	err1 := r.SendUserEvent("nonexistent", EventUserSpeak, &UserSpeakData{Message: "test message"})
	assert.Error(t, err1)

	// 测试发送有效事件
	err2 := r.SendUserEvent("player1", EventUserSpeak, &UserSpeakData{Message: "test message"})
	assert.NoError(t, err2)
}
