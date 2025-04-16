package room

import (
	"errors"
	"sync"

	"github.com/Zereker/werewolf/pkg/arean/werewolf"
	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/role"
)

var (
	ErrRoomFull        = errors.New("room is full")
	ErrRoomNotFound    = errors.New("room not found")
	ErrPlayerNotInRoom = errors.New("player not in room")
)

// Room 游戏房间
type Room struct {
	id       string
	runtime  *werewolf.Runtime
	capacity int

	sync.RWMutex
	players map[int64]struct{}
}

// NewRoom 创建新房间
func NewRoom(id string, capacity int) *Room {
	return &Room{
		id:       id,
		runtime:  werewolf.NewRuntime(),
		players:  make(map[int64]struct{}),
		capacity: capacity,
	}
}

// Join 玩家加入房间
func (r *Room) Join(playerID int64) error {
	r.Lock()
	defer r.Unlock()

	if len(r.players) >= r.capacity {
		return ErrRoomFull
	}

	r.players[playerID] = struct{}{}
	return nil
}

// Leave 玩家离开房间
func (r *Room) Leave(playerID int64) {
	r.Lock()
	defer r.Unlock()

	delete(r.players, playerID)
}

// Start 开始游戏
func (r *Room) Start() error {
	r.Lock()
	defer r.Unlock()

	// 分配角色
	roles := r.assignRoles(len(r.players))
	i := 0
	for playerID := range r.players {
		if err := r.runtime.AddPlayer(playerID, roles[i]); err != nil {
			return err
		}
		i++
	}

	return r.runtime.Start()
}

// assignRoles 分配角色
func (r *Room) assignRoles(playerCount int) []game.Role {
	// 这里简化了角色分配逻辑，实际应该根据玩家数量有不同的配置
	roles := make([]game.Role, 0, playerCount)

	// 添加狼人
	roles = append(roles, role.NewWerewolf())

	// 添加预言家
	roles = append(roles, role.NewSeer())

	// 添加女巫
	roles = append(roles, role.NewWitch())

	// 其余都是村民
	for i := len(roles); i < playerCount; i++ {
		roles = append(roles, role.NewVillager())
	}

	// TODO: 随机打乱角色顺序
	return roles
}

// GetRuntime 获取游戏运行时
func (r *Room) GetRuntime() *werewolf.Runtime {
	return r.runtime
}

// GetPlayerCount 获取玩家数量
func (r *Room) GetPlayerCount() int {
	r.RLock()
	defer r.RUnlock()

	return len(r.players)
}

// HasPlayer 检查玩家是否在房间中
func (r *Room) HasPlayer(playerID int64) bool {
	r.RLock()
	defer r.RUnlock()

	_, exists := r.players[playerID]
	return exists
}
