package role

import (
	"fmt"

	"github.com/Zereker/werewolf/pkg/game"
)

// Type 角色类型
type Type string

const (
	TypeWerewolf Type = "werewolf" // 狼人
	TypeSeer     Type = "seer"     // 预言家
	TypeWitch    Type = "witch"    // 女巫
	TypeHunter   Type = "hunter"   // 猎人
	TypeVillager Type = "villager" // 村民
	TypeGuard    Type = "guard"    // 守卫
)

// baseRole 基础角色结构体
type baseRole struct {
	camp game.Camp
}

// newBaseRole 创建基础角色
func newBaseRole(camp game.Camp) *baseRole {
	return &baseRole{
		camp: camp,
	}
}

// GetCamp 获取角色所属阵营
func (r *baseRole) GetCamp() game.Camp {
	return r.camp
}

// New 通过角色类型创建角色对象
func New(role string) (game.Role, error) {
	switch Type(role) {
	case TypeWerewolf:
		return NewWerewolf(), nil
	case TypeSeer:
		return NewSeer(), nil
	case TypeWitch:
		return NewWitch(), nil
	case TypeHunter:
		return NewHunter(), nil
	case TypeVillager:
		return NewVillager(), nil
	case TypeGuard:
		return NewGuard(), nil
	default:
		return nil, fmt.Errorf("unknown role type: %s", role)
	}
}
