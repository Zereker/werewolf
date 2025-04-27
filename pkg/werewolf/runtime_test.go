package werewolf

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/role"
)

func TestGameplay(t *testing.T) {
	// 创建游戏运行时
	runtime := NewRuntime()

	// 添加玩家
	// 3个狼人，3个村民，1个预言家，1个女巫，1个守卫
	players := map[string]game.Role{
		"wolf1":     role.NewWerewolf(),
		"wolf2":     role.NewWerewolf(),
		"wolf3":     role.NewWerewolf(),
		"villager1": role.NewVillager(),
		"villager2": role.NewVillager(),
		"villager3": role.NewVillager(),
		"seer":      role.NewSeer(),
		"witch":     role.NewWitch(),
		"guard":     role.NewGuard(),
	}

	for id, r := range players {
		err := runtime.AddPlayer(id, r)
		assert.NoError(t, err)
	}

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动游戏
	err := runtime.Start(ctx)
	assert.NoError(t, err)

	// 等待游戏开始事件
	time.Sleep(100 * time.Millisecond)

	// 第一回合
	t.Run("Round 1", func(t *testing.T) {
		// 夜晚阶段
		t.Run("Night Phase", func(t *testing.T) {
			// 守卫保护村民1
			err := runtime.SendUserEvent("guard", EventUserSkill, &UserSkillData{
				SkillType: game.SkillTypeProtect,
				TargetID:  "villager1",
			})
			assert.NoError(t, err)

			// 预言家查验狼人1
			err = runtime.SendUserEvent("seer", EventUserSkill, &UserSkillData{
				SkillType: game.SkillTypeCheck,
				TargetID:  "wolf1",
			})
			assert.NoError(t, err)

			// 狼人击杀村民2
			err = runtime.SendUserEvent("wolf1", EventUserSkill, &UserSkillData{
				SkillType: game.SkillTypeKill,
				TargetID:  "villager2",
			})
			assert.NoError(t, err)

			// 女巫救人
			err = runtime.SendUserEvent("witch", EventUserSkill, &UserSkillData{
				SkillType: game.SkillTypeAntidote,
				TargetID:  "villager2",
			})
			assert.NoError(t, err)

			// 等待夜晚阶段结束
			time.Sleep(100 * time.Millisecond)

		})

		// 白天阶段
		t.Run("Day Phase", func(t *testing.T) {
			// 所有玩家发言
			for id := range players {
				err := runtime.SendUserEvent(id, EventUserSpeak, &UserSpeakData{
					Message: "我是好人",
				})
				assert.NoError(t, err)
			}

			// 等待白天阶段结束
			time.Sleep(100 * time.Millisecond)
		})

		// 投票阶段
		t.Run("Vote Phase", func(t *testing.T) {
			// 所有玩家投票给wolf1
			for id := range players {
				if id != "wolf1" {
					err := runtime.SendUserEvent(id, EventUserVote, &UserVoteData{
						TargetID: "wolf1",
					})
					assert.NoError(t, err)
				}
			}

			// 等待投票阶段结束
			time.Sleep(100 * time.Millisecond)
		})
	})

	// 第二回合
	t.Run("Round 2", func(t *testing.T) {
		// 夜晚阶段
		t.Run("Night Phase", func(t *testing.T) {
			// 守卫保护村民2
			err := runtime.SendUserEvent("guard", EventUserSkill, &UserSkillData{
				SkillType: game.SkillTypeProtect,
				TargetID:  "villager2",
			})
			assert.NoError(t, err)

			// 预言家查验狼人2
			err = runtime.SendUserEvent("seer", EventUserSkill, &UserSkillData{
				SkillType: game.SkillTypeCheck,
				TargetID:  "wolf2",
			})
			assert.NoError(t, err)

			// 狼人击杀预言家
			err = runtime.SendUserEvent("wolf2", EventUserSkill, &UserSkillData{
				SkillType: game.SkillTypeKill,
				TargetID:  "seer",
			})
			assert.NoError(t, err)

			// 女巫使用毒药
			err = runtime.SendUserEvent("witch", EventUserSkill, &UserSkillData{
				SkillType: game.SkillTypePoison,
				TargetID:  "wolf2",
			})
			assert.NoError(t, err)

			// 等待夜晚阶段结束
			time.Sleep(100 * time.Millisecond)
		})

		// 继续游戏直到结束
		// 这里可以继续添加更多回合的测试...
	})

	// 验证游戏状态
	t.Run("Game State", func(t *testing.T) {
		// 检查玩家存活状态
		for id, player := range runtime.players {
			t.Logf("Player %s: alive=%v, r=%v", id, player.IsAlive(), player.GetRole())
		}

		// 检查游戏是否结束
		if runtime.ended {
			t.Logf("Game ended. Winner: %v", runtime.winner)
		}
	})
}
