package hiddenrole

// Logger is the logging interface.
// It lets a caller inject their own logging implementation for game events
// and debugging information.
type Logger interface {
	// Debug logs at debug level.
	Debug(msg string, fields ...Field)
	// Info logs at info level.
	Info(msg string, fields ...Field)
	// Warn logs at warning level.
	Warn(msg string, fields ...Field)
	// Error logs at error level.
	Error(msg string, fields ...Field)
}

// Field is one structured log field.
//
// Only the kernel writes logs -- a Resolver or VictoryChecker is handed a
// GameView and nothing else, never a Logger. So this package exports no
// constructor for Field: whoever implements Logger only ever reads one, and
// reading needs no constructor. If you really do want to build one (say to
// wrap a Logger and add a field), the fields are exported and
// Field{Key: ..., Value: ...} is enough.
type Field struct {
	Key   string
	Value interface{}
}

// logField builds a log field.
func logField(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// phaseField builds a phase field.
func phaseField(phase PhaseType) Field {
	return Field{Key: "phase", Value: phase.String()}
}

// roundField builds a round field.
func roundField(round int) Field {
	return Field{Key: "round", Value: round}
}

// playerField builds a player field.
func playerField(playerID string) Field {
	return Field{Key: "player_id", Value: playerID}
}

// targetField builds a target field.
func targetField(targetID string) Field {
	return Field{Key: "target_id", Value: targetID}
}

// skillField builds a skill field.
func skillField(skill SkillType) Field {
	return Field{Key: "skill", Value: skill.String()}
}

// eventField builds an event field.
func eventField(event EventType) Field {
	return Field{Key: "event", Value: event.String()}
}

// nopLogger is the no-op logger used by default.
type nopLogger struct{}

func (l *nopLogger) Debug(msg string, fields ...Field) {}
func (l *nopLogger) Info(msg string, fields ...Field)  {}
func (l *nopLogger) Warn(msg string, fields ...Field)  {}
func (l *nopLogger) Error(msg string, fields ...Field) {}

// newNopLogger builds a no-op logger.
func newNopLogger() *nopLogger {
	return &nopLogger{}
}
