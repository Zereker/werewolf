// messaging.go 消息系统：玩家发言的路由。
//
// 发言不走技能通道——它不产生状态变更，也就没有 Effect。
// 谁能听到由当前阶段决定：狼人阶段只有狼人互通，白天全场可闻。

package werewolf

import (
	"time"
)

// Message 游戏内消息
type Message struct {
	SenderID  string    // 发送者ID
	Content   string    // 消息内容
	Phase     PhaseType // 发送时的阶段
	Round     int       // 发送时的回合
	Timestamp time.Time // 发送时间
}

// MessageHandler 消息处理器
// msg: 消息内容
// receiverIDs: 接收者列表
type MessageHandler func(msg *Message, receiverIDs []string)

// OnMessage 注册消息处理器
// 当玩家发送消息时，处理器会收到消息和接收者列表
func (e *Engine) OnMessage(handler MessageHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.messageHandlers = append(e.messageHandlers, handler)
}

// SendMessage 发送消息
// 根据当前阶段自动路由到正确的接收者
// 返回错误：玩家不存在、玩家已死亡、当前阶段不允许发言
func (e *Engine) SendMessage(senderID, content string) error {
	msg, receiverIDs, handlers, err := e.prepareMessage(senderID, content)
	if err != nil {
		return err
	}

	// 发布在锁外：回调里可能回调 Engine
	publishMessage(handlers, e.logger, msg, receiverIDs)

	e.logger.Debug("message sent",
		PlayerField(senderID),
		PhaseField(msg.Phase),
		F("receiver_count", len(receiverIDs)))

	return nil
}

// prepareMessage 在锁内完成校验与取材，返回需要在锁外发布的内容。
//
// 拆成一个函数而不是在 SendMessage 里手动 RUnlock：手动解锁有四条
// 提前返回的路径，日后任何人再加一条都可能漏掉解锁。EndPhase 那边
// 是同样的写法，一份代码不该有两套标准。
func (e *Engine) prepareMessage(senderID, content string) (
	*Message, []string, []MessageHandler, error,
) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	sender, ok := e.state.getPlayer(senderID)
	if !ok {
		return nil, nil, nil, ErrPlayerNotFound
	}
	if !sender.Alive {
		return nil, nil, nil, ErrPlayerDead
	}

	receiverIDs := e.getMessageReceivers(senderID)
	if len(receiverIDs) == 0 {
		return nil, nil, nil, ErrMessageNotAllowed
	}

	msg := &Message{
		SenderID:  senderID,
		Content:   content,
		Phase:     e.state.Phase,
		Round:     e.state.Round,
		Timestamp: time.Now(),
	}

	// 复制 handlers 以避免锁外读取与 OnMessage 竞争
	handlers := make([]MessageHandler, len(e.messageHandlers))
	copy(handlers, e.messageHandlers)

	return msg, receiverIDs, handlers, nil
}

// MessageReceivers 获取消息接收者列表（公开方法）
// 返回当前阶段下，指定发送者的消息可以发送给哪些玩家
func (e *Engine) MessageReceivers(senderID string) []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.getMessageReceivers(senderID)
}

// getMessageReceivers 获取消息接收者（内部方法，调用前需持有 e.mu）。
//
// 「此刻谁能说话、谁能听到」是规则的事：夜里只有狼队交流是狼人杀的规矩，
// 换一套规则完全不同。判定交给 SpeechProvider，狼人杀的那份见 wolfSpeech。
func (e *Engine) getMessageReceivers(senderID string) []string {
	if e.speech == nil {
		return nil
	}
	return e.speech.Receivers(senderID, newStateView(e.state))
}

// publishMessage 在锁外发布消息。
//
// 每个 handler 拿到自己的一份接收者列表：共用一个切片的话，
// 某个 handler 就地排序或过滤会影响到后面的 handler。
func publishMessage(handlers []MessageHandler, logger Logger, msg *Message, receiverIDs []string) {
	for _, handler := range handlers {
		func() {
			defer recoverHandlerPanic(logger, "message handler",
				PlayerField(msg.SenderID), PhaseField(msg.Phase))
			handler(msg, append([]string(nil), receiverIDs...))
		}()
	}
}
