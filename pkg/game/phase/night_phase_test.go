package phase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Zereker/werewolf/pkg/game"
)

func TestNewNightPhase(t *testing.T) {
	players := []game.Player{
		NewMockPlayer("player1", game.RoleTypeWerewolf),
		NewMockPlayer("player2", game.RoleTypeVillager),
	}

	phase := NewNightPhase(players)
	assert.NotNil(t, phase)
	assert.Equal(t, game.PhaseNight, phase.GetName())
}

func TestNightPhase_Start(t *testing.T) {
	// 创建测试玩家
	mockWerewolf := NewMockPlayer("werewolf", game.RoleTypeWerewolf)
	mockSeer := NewMockPlayer("seer", game.RoleTypeSeer)
	mockGuard := NewMockPlayer("guard", game.RoleTypeGuard)
	mockWitch := NewMockPlayer("witch", game.RoleTypeWitch)
	mockVillager := NewMockPlayer("villager", game.RoleTypeVillager)

	players := []game.Player{
		mockWerewolf,
		mockSeer,
		mockGuard,
		mockWitch,
		mockVillager,
	}

	// 设置玩家的技能事件
	mockWerewolf.SetNextSkillEvent(game.SkillTypeKill, mockVillager.GetID(), "")
	mockSeer.SetNextSkillEvent(game.SkillTypeCheck, mockVillager.GetID(), "")
	mockGuard.SetNextSkillEvent(game.SkillTypeProtect, mockVillager.GetID(), "")
	mockWitch.SetNextSkillEvent("witch_choice", mockVillager.GetID(), "")

	// 创建夜晚阶段
	phase := NewNightPhase(players)

	// 启动阶段
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := phase.Start(ctx)
	assert.NoError(t, err)
}

func TestNightPhase_HandleWerewolfActions(t *testing.T) {
	// 创建测试玩家
	mockWerewolf := NewMockPlayer("werewolf", game.RoleTypeWerewolf)
	mockVillager := NewMockPlayer("villager", game.RoleTypeVillager)

	players := []game.Player{
		mockWerewolf,
		mockVillager,
	}

	// 设置狼人的技能事件
	mockWerewolf.SetNextSkillEvent(game.SkillTypeKill, mockVillager.GetID(), "")

	phase := NewNightPhase(players)
	err := phase.handleWerewolfActions()
	assert.NoError(t, err)

	// 验证狼人技能使用结果
	results := phase.calculatePhaseResult()
	assert.Equal(t, len(results.Deaths), 1)
	assert.False(t, mockVillager.IsAlive(), "村民应该被狼人杀死")
}

func TestNightPhase_HandleSeerActions(t *testing.T) {
	// 创建测试玩家
	mockSeer := NewMockPlayer("seer", game.RoleTypeSeer)
	mockVillager := NewMockPlayer("villager", game.RoleTypeVillager)

	players := []game.Player{
		mockSeer,
		mockVillager,
	}

	// 设置预言家的技能事件
	mockSeer.SetNextSkillEvent(game.SkillTypeCheck, mockVillager.GetID(), "")

	phase := NewNightPhase(players)
	err := phase.handleSeerActions()
	assert.NoError(t, err)

	results := phase.calculatePhaseResult()
	assert.Equal(t, len(results.Deaths), 0)
}

func TestNightPhase_HandleGuardActions(t *testing.T) {
	// 创建测试玩家
	mockGuard := NewMockPlayer("guard", game.RoleTypeGuard)
	mockVillager := NewMockPlayer("villager", game.RoleTypeVillager)
	mockWerewolf := NewMockPlayer("werewolf", game.RoleTypeWerewolf)

	players := []game.Player{
		mockGuard,
		mockVillager,
		mockWerewolf,
	}

	// 设置守卫的技能事件
	mockGuard.SetNextSkillEvent(game.SkillTypeProtect, mockVillager.GetID(), "")
	mockWerewolf.SetNextSkillEvent(game.SkillTypeKill, mockVillager.GetID(), "")

	phase := NewNightPhase(players)
	err := phase.handleGuardActions()
	assert.NoError(t, err)

	result := phase.calculatePhaseResult()
	assert.Equal(t, len(result.Deaths), 0)
}

func TestNightPhase_HandleWitchActions(t *testing.T) {
	// 创建测试玩家
	mockWitch := NewMockPlayer("witch", game.RoleTypeWitch)
	mockVillager := NewMockPlayer("villager", game.RoleTypeVillager)
	mockWerewolf := NewMockPlayer("werewolf", game.RoleTypeWerewolf)

	players := []game.Player{
		mockWitch,
		mockVillager,
		mockWerewolf,
	}

	// 设置女巫的技能事件（使用解药）
	mockWerewolf.SetNextSkillEvent(game.SkillTypeKill, mockVillager.GetID(), "")
	mockWitch.SetNextSkillEvent(game.SkillTypeAntidote, mockVillager.GetID(), "")

	phase := NewNightPhase(players)
	phase.handleWerewolfActions()
	err := phase.handleWitchActions()
	assert.NoError(t, err)

	// 验证女巫技能使用结果
	assert.True(t, mockVillager.IsAlive(), "村民应该被女巫救活")
}

func TestNightPhase_CalculatePhaseResult(t *testing.T) {
	// 创建测试玩家
	mockWerewolf := NewMockPlayer("werewolf", game.RoleTypeWerewolf)
	mockVillager := NewMockPlayer("villager", game.RoleTypeVillager)

	players := []game.Player{
		mockWerewolf,
		mockVillager,
	}

	phase := NewNightPhase(players)
	result := phase.calculatePhaseResult()
	assert.NotNil(t, result)
	assert.Empty(t, result.Deaths)
}

func TestNightPhase_CompleteNight(t *testing.T) {
	// 创建测试玩家
	mockWerewolf := NewMockPlayer("werewolf", game.RoleTypeWerewolf)
	mockSeer := NewMockPlayer("seer", game.RoleTypeSeer)
	mockGuard := NewMockPlayer("guard", game.RoleTypeGuard)
	mockWitch := NewMockPlayer("witch", game.RoleTypeWitch)
	mockVillager := NewMockPlayer("villager", game.RoleTypeVillager)

	players := []game.Player{
		mockWerewolf,
		mockSeer,
		mockGuard,
		mockWitch,
		mockVillager,
	}

	// 设置各个角色的技能事件
	mockWerewolf.SetNextSkillEvent(game.SkillTypeKill, mockVillager.GetID(), "")
	mockSeer.SetNextSkillEvent(game.SkillTypeCheck, mockWerewolf.GetID(), "")
	mockGuard.SetNextSkillEvent(game.SkillTypeProtect, mockVillager.GetID(), "")
	mockWitch.SetNextSkillEvent("witch_choice", mockVillager.GetID(), "save")

	// 创建夜晚阶段
	phase := NewNightPhase(players)

	// 启动阶段
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := phase.Start(ctx)
	assert.NoError(t, err)

	// 验证最终结果
	assert.True(t, mockVillager.IsAlive(), "村民应该存活（被守卫保护或被女巫救活）")
	assert.True(t, mockVillager.IsProtected(), "村民应该被守卫保护")
}
