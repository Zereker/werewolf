// events.go is event notification: turning Effects produced inside the engine
// into outward events pushed to the caller.
//
// Callbacks always run after the engine lock is released, and the handler
// list is snapshotted while holding it -- so there is no deadlock (a callback
// may safely call Engine methods) and no race with a concurrent OnEvent
// registration.

package hiddenrole

import (
	"fmt"
	"runtime/debug"
)

// EventHandler handles one event.
type EventHandler func(event *Event)

// OnEvent registers an event handler.
func (e *Engine) OnEvent(handler EventHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.eventHandlers = append(e.eventHandlers, handler)
}

// snapshotEventHandlersLocked copies the handler list.
// The caller must hold e.mu (read or write).
func (e *Engine) snapshotEventHandlersLocked() []EventHandler {
	handlers := make([]EventHandler, len(e.eventHandlers))
	copy(handlers, e.eventHandlers)
	return handlers
}

// dispatchEvent dispatches an event outside the lock.
// Each handler runs independently; one handler panicking does not affect the
// others.
func dispatchEvent(handlers []EventHandler, logger Logger, event *Event) {
	for _, handler := range handlers {
		func() {
			defer recoverHandlerPanic(logger, "event handler", eventField(event.Type))
			handler(event)
		}()
	}
}

// recoverHandlerPanic recovers a panic from a user callback and logs it.
//
// Swallowing the panic keeps one handler's failure from spreading to the
// others, but it must leave a log behind -- swallowing it silently would make
// a production problem completely traceless.
func recoverHandlerPanic(logger Logger, kind string, fields ...Field) {
	r := recover()
	if r == nil {
		return
	}
	logger.Error(kind+" panicked",
		append(fields,
			logField("panic", fmt.Sprintf("%v", r)),
			logField("stack", string(debug.Stack())),
		)...)
}
