package phase

import (
	"context"
	"fmt"
	"time"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/event"
	"github.com/Zereker/werewolf/pkg/game/role"
)

// MockPlayer 是一个用于测试的玩家实现
type MockPlayer struct {
	id        string
	alive     bool
	protected bool
	role      game.Role
	nextEvent event.Event[any] // 下一个要返回的事件
}

// NewMockPlayer 创建一个新的 MockPlayer
func NewMockPlayer(id string, roleType game.RoleType) *MockPlayer {
	r, _ := role.New(roleType) // 在测试中，我们忽略错误
	return &MockPlayer{
		id:        id,
		alive:     true,
		protected: false,
		role:      r,
	}
}

// GetID 获取玩家ID
func (m *MockPlayer) GetID() string {
	return m.id
}

// GetRole 获取玩家角色
func (m *MockPlayer) GetRole() game.Role {
	return m.role
}

// IsAlive 检查玩家是否存活
func (m *MockPlayer) IsAlive() bool {
	return m.alive
}

// SetAlive 设置玩家存活状态
func (m *MockPlayer) SetAlive(alive bool) {
	m.alive = alive
}

// IsProtected 检查玩家是否被保护
func (m *MockPlayer) IsProtected() bool {
	return m.protected
}

// SetProtected 设置玩家保护状态
func (m *MockPlayer) SetProtected(protected bool) {
	m.protected = protected
}

// Write 写入事件
func (m *MockPlayer) Write(evt event.Event[any]) error {
	fmt.Printf("[Player %s] 收到事件: Type=%v, Data=%+v\n", m.id, evt.Type, evt.Data)
	return nil
}

// Read 读取事件，带超时
func (m *MockPlayer) Read(ctx context.Context) (event.Event[any], error) {
	// 如果设置了下一个事件，返回它
	if m.nextEvent.Type != "" {
		evt := m.nextEvent
		m.nextEvent = event.Event[any]{} // 清空事件
		return evt, nil
	}
	// 否则返回空事件
	time.Sleep(100 * time.Millisecond) // 模拟一些延迟
	return event.Event[any]{}, nil
}

// SetNextSkillEvent 设置下一个技能事件
func (m *MockPlayer) SetNextSkillEvent(skillType game.SkillType, targetID string, content string) {
	m.nextEvent = event.Event[any]{
		Type:      event.UserSkill,
		PlayerID:  m.id,
		Timestamp: time.Now(),
		Data: &event.UserSkillData{
			TargetID:  targetID,
			SkillType: string(skillType),
			Content:   content,
		},
	}
}
