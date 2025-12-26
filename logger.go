package werewolf

import (
	pb "github.com/Zereker/werewolf/proto"
)

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

// Field 日志字段
type Field struct {
	Key   string
	Value interface{}
}

// F 创建日志字段的快捷方法
func F(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// PhaseField 创建阶段字段
func PhaseField(phase pb.PhaseType) Field {
	return Field{Key: "phase", Value: phase.String()}
}

// RoundField 创建回合字段
func RoundField(round int) Field {
	return Field{Key: "round", Value: round}
}

// PlayerField 创建玩家字段
func PlayerField(playerID string) Field {
	return Field{Key: "player_id", Value: playerID}
}

// TargetField 创建目标字段
func TargetField(targetID string) Field {
	return Field{Key: "target_id", Value: targetID}
}

// SkillField 创建技能字段
func SkillField(skill pb.SkillType) Field {
	return Field{Key: "skill", Value: skill.String()}
}

// EventField 创建事件字段
func EventField(event pb.EventType) Field {
	return Field{Key: "event", Value: event.String()}
}

// NopLogger 空日志实现（默认）
type NopLogger struct{}

func (l *NopLogger) Debug(msg string, fields ...Field) {}
func (l *NopLogger) Info(msg string, fields ...Field)  {}
func (l *NopLogger) Warn(msg string, fields ...Field)  {}
func (l *NopLogger) Error(msg string, fields ...Field) {}

// NewNopLogger 创建空日志
func NewNopLogger() *NopLogger {
	return &NopLogger{}
}

// Metrics 指标收集接口
// 允许外部注入指标实现，用于生产环境监控
type Metrics interface {
	// IncSkillSubmitted 技能提交计数
	IncSkillSubmitted(skill pb.SkillType)
	// IncPhaseEnded 阶段结束计数
	IncPhaseEnded(phase pb.PhaseType)
	// IncGameEnded 游戏结束计数
	IncGameEnded(winner pb.Camp)
	// IncEffectApplied 效果应用计数
	IncEffectApplied(eventType pb.EventType)
}

// NopMetrics 空指标实现（默认）
type NopMetrics struct{}

func (m *NopMetrics) IncSkillSubmitted(skill pb.SkillType)   {}
func (m *NopMetrics) IncPhaseEnded(phase pb.PhaseType)       {}
func (m *NopMetrics) IncGameEnded(winner pb.Camp)            {}
func (m *NopMetrics) IncEffectApplied(eventType pb.EventType) {}

// NewNopMetrics 创建空指标收集器
func NewNopMetrics() *NopMetrics {
	return &NopMetrics{}
}
