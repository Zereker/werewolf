package player

import (
	"errors"
	"time"

	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/event"
)

type player struct {
	id        string
	alive     bool
	protected bool
	role      game.Role

	// 事件相关字段
	eventChan chan event.Event[any]
}

func New(id string, role game.Role) game.Player {
	return &player{
		id:        id,
		alive:     true,
		protected: false,
		role:      role,
		
		eventChan: make(chan event.Event[any], 100), // 设置一个合理的缓冲区大小
	}
}

func (p *player) GetID() string {
	return p.id
}

func (p *player) GetRole() game.Role {
	return p.role
}

func (p *player) IsAlive() bool {
	return p.alive
}

func (p *player) SetAlive(alive bool) {
	p.alive = alive
}

func (p *player) IsProtected() bool {
	return p.protected
}

func (p *player) SetProtected(protected bool) {
	p.protected = protected
}

// Write 写入事件
func (p *player) Write(evt event.Event[any]) error {
	select {
	case p.eventChan <- evt:
		return nil
	default:
		return errors.New("event channel is full")
	}
}

// Read 读取事件，带超时
func (p *player) Read(timeout time.Duration) (event.Event[any], error) {
	select {
	case evt, ok := <-p.eventChan:
		if !ok {
			return event.Event[any]{}, errors.New("event channel is closed")
		}
		return evt, nil
	case <-time.After(timeout):
		return event.Event[any]{}, errors.New("read event timeout")
	}
}
