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
	sync.RWMutex

	id       string
	runtime  *werewolf.Runtime
	capacity int
	players  map[int64]*Player
}

// NewRoom 创建新房间
func NewRoom(id string, capacity int) *Room {
	return &Room{
		id:       id,
		runtime:  werewolf.NewRuntime(),
		capacity: capacity,
		players:  make(map[int64]*Player),
	}
}

// Join 玩家加入房间
func (r *Room) Join(id int64, conn Connection) error {
	r.Lock()
	defer r.Unlock()

	if len(r.players) >= r.capacity {
		return ErrRoomFull
	}

	// 分配角色（这里简化处理，实际应该有更复杂的角色分配逻辑）
	roleType := r.assignRole()
	playerRole, err := role.New(roleType)
	if err != nil {
		return err
	}

	// 创建玩家
	player := NewPlayer(id, playerRole, conn, r)
	r.players[id] = player

	// 添加到游戏运行时
	if err := r.runtime.AddPlayer(id, playerRole); err != nil {
		delete(r.players, id)
		return err
	}

	// 启动玩家消息处理
	player.Start()
	return nil
}

// Leave 玩家离开房间
func (r *Room) Leave(id int64) {
	r.Lock()
	defer r.Unlock()

	if player, exists := r.players[id]; exists {
		player.Stop()
		delete(r.players, id)
	}
}

// Start 开始游戏
func (r *Room) Start() error {
	r.Lock()
	defer r.Unlock()

	// 订阅游戏事件
	r.runtime.Subscribe(r.broadcastEvent)

	return r.runtime.Start()
}

// Stop 停止游戏
func (r *Room) Stop() {
	r.Lock()
	defer r.Unlock()

	r.runtime.Stop()
	for _, player := range r.players {
		player.Stop()
	}
}

// broadcastEvent 广播游戏事件给所有玩家
func (r *Room) broadcastEvent(evt *werewolf.Event) {
	r.RLock()
	defer r.RUnlock()

	for _, player := range r.players {
		player.OnEvent(evt)
	}
}

// assignRole 分配角色（简化版本）
func (r *Room) assignRole() game.RoleType {
	// 这里应该实现更复杂的角色分配逻辑
	return game.RoleTypeVillager
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
