// events.go 事件通知：把引擎内部产生的 Effect 转成对外事件推给调用方。
//
// 回调一律在释放引擎锁之后执行，handler 列表在锁内快照——既不会死锁
// （回调里可以安全调用 Engine 方法），也不会与 OnEvent 的并发注册竞争。

package werewolf

import (
	"fmt"
	"runtime/debug"

	pb "github.com/Zereker/werewolf/proto"
)

// EventHandler 事件处理器
type EventHandler func(event *pb.Event)

// OnEvent 注册事件处理器
func (e *Engine) OnEvent(handler EventHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.eventHandlers = append(e.eventHandlers, handler)
}

// snapshotEventHandlersLocked 复制事件处理器列表。
// 调用前必须持有 e.mu（读锁或写锁）。
func (e *Engine) snapshotEventHandlersLocked() []EventHandler {
	handlers := make([]EventHandler, len(e.eventHandlers))
	copy(handlers, e.eventHandlers)
	return handlers
}

// dispatchEvent 在锁外分发事件。
// 每个 handler 独立执行，单个 handler panic 不影响其他 handler。
func dispatchEvent(handlers []EventHandler, logger Logger, event *pb.Event) {
	for _, handler := range handlers {
		func() {
			defer recoverHandlerPanic(logger, "event handler", EventField(event.Type))
			handler(event)
		}()
	}
}

// recoverHandlerPanic 捕获用户回调中的 panic 并记录。
//
// 吞掉 panic 是为了让单个 handler 的故障不波及其他 handler，
// 但必须留下日志——静默吞掉会让线上问题完全没有痕迹。
func recoverHandlerPanic(logger Logger, kind string, fields ...Field) {
	r := recover()
	if r == nil {
		return
	}
	if logger == nil {
		return
	}
	logger.Error(kind+" panicked",
		append(fields,
			F("panic", fmt.Sprintf("%v", r)),
			F("stack", string(debug.Stack())),
		)...)
}
