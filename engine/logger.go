package engine

import ()

// Logger 日志接口
// 允许外部注入日志实现，用于记录游戏事件和调试信息
type Logger interface {
	// Debug 调试级别日志
	Debug(msg string, fields ...Field)
	// Info 信息级别日志
	Info(msg string, fields ...Field)
	// Warn 警告级别日志
	Warn(msg string, fields ...Field)
	// Error 错误级别日志
	Error(msg string, fields ...Field)
}

// Field 日志字段。
//
// 只有内核会写日志——Resolver 与 VictoryChecker 拿到的只有 GameView，
// 拿不到 Logger。因此本包不导出任何 Field 的构造函数：实现 Logger 的人
// 只会读它，读不需要构造器。真要自己拼一个（比如包一层 Logger 再补个
// 字段），字段是导出的，Field{Key: ..., Value: ...} 就够。
type Field struct {
	Key   string
	Value interface{}
}

// f 创建日志字段的快捷方法
func logField(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// phaseField 创建阶段字段
func phaseField(phase PhaseType) Field {
	return Field{Key: "phase", Value: phase.String()}
}

// roundField 创建回合字段
func roundField(round int) Field {
	return Field{Key: "round", Value: round}
}

// playerField 创建玩家字段
func playerField(playerID string) Field {
	return Field{Key: "player_id", Value: playerID}
}

// targetField 创建目标字段
func targetField(targetID string) Field {
	return Field{Key: "target_id", Value: targetID}
}

// skillField 创建技能字段
func skillField(skill SkillType) Field {
	return Field{Key: "skill", Value: skill.String()}
}

// eventField 创建事件字段
func eventField(event EventType) Field {
	return Field{Key: "event", Value: event.String()}
}

// nopLogger 空日志实现（默认）
type nopLogger struct{}

func (l *nopLogger) Debug(msg string, fields ...Field) {}
func (l *nopLogger) Info(msg string, fields ...Field)  {}
func (l *nopLogger) Warn(msg string, fields ...Field)  {}
func (l *nopLogger) Error(msg string, fields ...Field) {}

// newNopLogger 创建空日志
func newNopLogger() *nopLogger {
	return &nopLogger{}
}
