// messaging.go 消息系统：玩家发言的路由。
//
// 发言不走技能通道——它不产生状态变更，也就没有 Effect。
// 谁能听到由当前阶段决定：狼人阶段只有狼人互通，白天全场可闻。

package werewolf

import (
	"time"

	pb "github.com/Zereker/werewolf/proto"
)

// Message 游戏内消息
type Message struct {
	SenderID  string       // 发送者ID
	Content   string       // 消息内容
	Phase     pb.PhaseType // 发送时的阶段
	Round     int          // 发送时的回合
	Timestamp time.Time    // 发送时间
}

// MessageHandler 消息处理器
// msg: 消息内容
// receiverIDs: 接收者列表
type MessageHandler func(msg *Message, receiverIDs []string)

// PhaseInfo 阶段信息（纯状态，不含消息内容）

// ==================== 消息系统 ====================

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
	e.mu.RLock()

	// 验证发送者
	sender, ok := e.state.getPlayer(senderID)
	if !ok {
		e.mu.RUnlock()
		return ErrPlayerNotFound
	}
	if !sender.Alive {
		e.mu.RUnlock()
		return ErrPlayerDead
	}

	// 获取接收者
	receiverIDs := e.getMessageReceivers(senderID)
	if len(receiverIDs) == 0 {
		e.mu.RUnlock()
		return ErrMessageNotAllowed
	}

	// 构建消息
	msg := &Message{
		SenderID:  senderID,
		Content:   content,
		Phase:     e.state.Phase,
		Round:     e.state.Round,
		Timestamp: time.Now(),
	}

	// 复制 handlers 与 logger 以避免在回调中死锁、并避免锁外读取竞争
	handlers := make([]MessageHandler, len(e.messageHandlers))
	copy(handlers, e.messageHandlers)
	logger := e.logger

	e.mu.RUnlock()

	// 发布消息（锁外执行，避免死锁）
	publishMessage(handlers, logger, msg, receiverIDs)

	logger.Debug("message sent",
		PlayerField(senderID),
		PhaseField(msg.Phase),
		F("receiver_count", len(receiverIDs)))

	return nil
}

// MessageReceivers 获取消息接收者列表（公开方法）
// 返回当前阶段下，指定发送者的消息可以发送给哪些玩家
func (e *Engine) MessageReceivers(senderID string) []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.getMessageReceivers(senderID)
}

// getMessageReceivers 获取消息接收者（内部方法，调用前需持有 e.mu）
func (e *Engine) getMessageReceivers(senderID string) []string {
	sender, ok := e.state.getPlayer(senderID)
	if !ok || !sender.Alive {
		return nil
	}

	switch e.state.Phase {
	case pb.PhaseType_PHASE_TYPE_NIGHT_WOLF:
		// 狼人阶段：只有狼人能互相交流
		if sender.Role != pb.RoleType_ROLE_TYPE_WEREWOLF {
			return nil
		}
		// 返回所有存活的狼人（包括自己，方便处理）
		return e.state.getAlivePlayerIDsByRole(pb.RoleType_ROLE_TYPE_WEREWOLF)

	case pb.PhaseType_PHASE_TYPE_DAY:
		// 白天阶段：所有存活玩家都能听到
		return e.state.getAlivePlayerIDs()

	default:
		// 其他阶段不允许发言
		return nil
	}
}

// publishMessage 在锁外发布消息。
func publishMessage(handlers []MessageHandler, logger Logger, msg *Message, receiverIDs []string) {
	for _, handler := range handlers {
		func() {
			defer recoverHandlerPanic(logger, "message handler",
				PlayerField(msg.SenderID), PhaseField(msg.Phase))
			handler(msg, receiverIDs)
		}()
	}
}
