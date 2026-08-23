// messaging.go is the messaging system: routing what players say.
//
// Speech does not go through the skill channel -- it produces no state
// change, and therefore no Effect. Who hears it is decided by the current
// phase: during the wolf phase only wolves hear each other, during the day
// the whole table does.

package hiddenrole

import (
	"time"
)

// Message is an in-game message.
type Message struct {
	SenderID  string    // who sent it
	Content   string    // what was said
	Phase     PhaseType // the phase it was sent in
	Round     int       // the round it was sent in
	Timestamp time.Time // when it was sent
}

// MessageHandler handles one message.
// msg: the message itself.
// receiverIDs: who it should reach.
type MessageHandler func(msg *Message, receiverIDs []string)

// OnMessage registers a message handler.
// When a player sends a message the handler receives it along with the list
// of receivers.
func (e *Engine) OnMessage(handler MessageHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.messageHandlers = append(e.messageHandlers, handler)
}

// SendMessage sends one player's speech, routed by the current phase to
// whoever should hear it.
//
// The audible range is answered by a SpeechProvider (see WithSpeech).
// **With no provider installed** the kernel falls back to a default:
// eliminated players may not speak, and a phase where nobody can hear is a
// rejection. With one installed it decides -- whether the dead may speak is
// the rules' judgement, not the kernel's law (the dead in Blood on the
// Clocktower hold a ghost vote, and werewolf has a last-words phase).
//
// Errors: no such player (ErrPlayerNotFound); an eliminated player speaking
// under the default rule (ErrPlayerDead); no receivers at all in the current
// phase (ErrMessageNotAllowed).
func (e *Engine) SendMessage(senderID, content string) error {
	msg, receiverIDs, handlers, err := e.prepareMessage(senderID, content)
	if err != nil {
		return err
	}

	// Publish outside the lock: a callback may call back into the Engine.
	publishMessage(handlers, e.logger, msg, receiverIDs)

	e.logger.Debug("message sent",
		playerField(senderID),
		phaseField(msg.Phase),
		logField("receiver_count", len(receiverIDs)))

	return nil
}

// prepareMessage does the validation and gathering under the lock, and
// returns what has to be published outside it.
//
// It is a separate function rather than a manual RUnlock inside SendMessage:
// a manual unlock has four early-return paths, and whoever adds a fifth may
// well miss one. EndPhase is written the same way, and one codebase should
// not have two standards.
func (e *Engine) prepareMessage(senderID, content string) (
	*Message, []string, []MessageHandler, error,
) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	sender, ok := e.state.getPlayer(senderID)
	if !ok {
		return nil, nil, nil, ErrPlayerNotFound
	}
	// Being alive is the **default** qualification to speak, not the law.
	//
	// This used to reject an eliminated player outright, leaving a
	// SpeechProvider no way to overrule it -- yet whether the dead may speak
	// is the rules' judgement: the dead in Blood on the Clocktower take part
	// in discussion as usual, and werewolf has a last-words phase.
	//
	// The division now: if the rules installed a SpeechProvider it decides
	// (an empty list means "they cannot speak right now"); if they did not,
	// the kernel falls back to its default -- the dead stay silent.
	if e.speech == nil && !sender.Alive {
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

	// Copy the handlers so that reading them outside the lock does not race
	// with OnMessage.
	handlers := make([]MessageHandler, len(e.messageHandlers))
	copy(handlers, e.messageHandlers)

	return msg, receiverIDs, handlers, nil
}

// MessageReceivers returns the receivers of a message.
// It reports which players a message from the given sender may reach in the
// current phase.
func (e *Engine) MessageReceivers(senderID string) []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.getMessageReceivers(senderID)
}

// getMessageReceivers returns the receivers. The caller must hold e.mu.
//
// "Who may speak right now and who hears it" belongs to the rules: night
// chat being wolves-only is werewolf's convention, and another ruleset does
// it entirely differently. The decision goes to a SpeechProvider; werewolf's
// is wolfSpeech.
func (e *Engine) getMessageReceivers(senderID string) []string {
	if e.speech == nil {
		return nil
	}
	return e.speech.Receivers(senderID, newStateView(e.state))
}

// publishMessage publishes a message outside the lock.
//
// Each handler gets its own copy of the receiver list: were they to share one
// slice, a handler that sorts or filters in place would affect the handlers
// after it.
func publishMessage(handlers []MessageHandler, logger Logger, msg *Message, receiverIDs []string) {
	for _, handler := range handlers {
		func() {
			defer recoverHandlerPanic(logger, "message handler",
				playerField(msg.SenderID), phaseField(msg.Phase))
			handler(msg, append([]string(nil), receiverIDs...))
		}()
	}
}
