package role

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Zereker/werewolf/pkg/game"
)

func TestVillager(t *testing.T) {
	role := NewVillager()
	assert.Equal(t, game.RoleTypeVillager, role.GetName())
	assert.Equal(t, game.CampGood, role.GetCamp())
}

func TestWerewolf(t *testing.T) {
	role := NewWerewolf()
	assert.Equal(t, game.RoleTypeWerewolf, role.GetName())
	assert.Equal(t, game.CampEvil, role.GetCamp())
}

func TestWitch(t *testing.T) {
	role := NewWitch()
	assert.Equal(t, game.RoleTypeWitch, role.GetName())
	assert.Equal(t, game.CampGood, role.GetCamp())
}

func TestSeer(t *testing.T) {
	role := NewSeer()
	assert.Equal(t, game.RoleTypeSeer, role.GetName())
	assert.Equal(t, game.CampGood, role.GetCamp())
}

func TestGuard(t *testing.T) {
	role := NewGuard()
	assert.Equal(t, game.RoleTypeGuard, role.GetName())
	assert.Equal(t, game.CampGood, role.GetCamp())
}

func TestCamp_String(t *testing.T) {
	tests := []struct {
		name     string
		camp     game.Camp
		expected string
	}{
		{
			name:     "好人阵营字符串表示",
			camp:     game.CampGood,
			expected: "good",
		},
		{
			name:     "坏人阵营字符串表示",
			camp:     game.CampEvil,
			expected: "evil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.camp.String())
		})
	}
}

func TestRole_IsSameCamp(t *testing.T) {
	tests := []struct {
		name     string
		role1    game.Role
		role2    game.Role
		expected bool
	}{
		{
			name:     "同阵营角色",
			role1:    NewVillager(),
			role2:    NewWitch(),
			expected: true,
		},
		{
			name:     "不同阵营角色",
			role1:    NewVillager(),
			role2:    NewWerewolf(),
			expected: false,
		},
		{
			name:     "相同角色",
			role1:    NewWerewolf(),
			role2:    NewWerewolf(),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.role1.GetCamp() == tt.role2.GetCamp())
		})
	}
}
