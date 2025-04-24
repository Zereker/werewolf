package role

import (
	"fmt"

	"github.com/Zereker/werewolf/pkg/game"
)

// New 通过角色类型创建角色对象
func New(role game.RoleType) (game.Role, error) {
	switch role {
	case game.RoleTypeWerewolf:
		return NewWerewolf(), nil
	case game.RoleTypeSeer:
		return NewSeer(), nil
	case game.RoleTypeWitch:
		return NewWitch(), nil
	case game.RoleTypeHunter:
		return NewHunter(), nil
	case game.RoleTypeVillager:
		return NewVillager(), nil
	case game.RoleTypeGuard:
		return NewGuard(), nil
	default:
		return nil, fmt.Errorf("unknown role type: %s", role)
	}
}
