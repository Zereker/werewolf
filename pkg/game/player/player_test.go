package player

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Zereker/werewolf/pkg/game/role"
)

func TestPlayer_GetRole(t *testing.T) {
	villagerRole := role.NewVillager()
	player := New(villagerRole)
	assert.Equal(t, villagerRole, player.GetRole())
}

func TestPlayer_IsAlive(t *testing.T) {
	player := New(role.NewVillager())
	// 默认应该是活着的
	assert.True(t, player.IsAlive())

	// 设置为死亡
	player.SetAlive(false)
	assert.False(t, player.IsAlive())

	// 重新设置为活着
	player.SetAlive(true)
	assert.True(t, player.IsAlive())
}

func TestPlayer_IsProtected(t *testing.T) {
	player := New(role.NewVillager())
	// 默认应该是未被保护的
	assert.False(t, player.IsProtected())

	// 设置为被保护
	player.SetProtected(true)
	assert.True(t, player.IsProtected())

	// 取消保护
	player.SetProtected(false)
	assert.False(t, player.IsProtected())
}
