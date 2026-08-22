// option.go 构造选项：把「开局前必须配好」的东西收到构造函数里。
//
// 引擎有一批只能在开局前设置的东西：自定义 Resolver、日志、指标。
// 它们此前各有一个 setter，而 setter 只能在拿到引擎之后调用——
// 对 RestoreEngine / ReplayEngine 这两个「一步就把引擎推到局中」的
// 入口来说太晚了：恢复出来的引擎已经不在 START 阶段，注册会直接被拒，
// 自定义角色的技能从此被静默丢弃。
//
// 收进构造选项之后还多了一层好处：这些东西在引擎交给调用方之前就已定死、
// 此后不再改变，锁外读取它们也就不再需要防御性的复制。

package engine

import ()

// EngineOption 构造引擎时的可选设置。
//
// 三个构造入口（NewEngine / RestoreEngine / ReplayEngine）都接受它，
// 因此扩展角色在「新开一局」和「从存档续上」两种场合下的写法是同一套。
type EngineOption func(*Engine) error

// WithResolver 注册或替换某个阶段的解析器。
//
// 这是扩展新角色的唯一入口，对 RestoreEngine / ReplayEngine 同样有效：
//
//	cfg := werewolf.DefaultGameConfig()
//	cfg.Phases[myPhase] = &werewolf.PhaseConfig{ ... }
//	engine, err := werewolf.RestoreEngine(cfg, snap,
//		werewolf.WithResolver(myPhase, myResolver))
func WithResolver(phase PhaseType, resolver Resolver) EngineOption {
	return func(e *Engine) error {
		if resolver == nil {
			return WrapError(CodeInvalidConfig,
				"resolver for phase %v must not be nil", phase)
		}
		e.phase.registerResolver(phase, resolver)
		return nil
	}
}

// WithLogger 设置日志接口。logger 为 nil 时保持默认的空实现。
func WithLogger(logger Logger) EngineOption {
	return func(e *Engine) error {
		if logger != nil {
			e.logger = logger
		}
		return nil
	}
}

// WithMetrics 设置指标收集器。metrics 为 nil 时保持默认的空实现。
func WithMetrics(metrics Metrics) EngineOption {
	return func(e *Engine) error {
		if metrics != nil {
			e.metrics = metrics
		}
		return nil
	}
}

// applyOptions 依次应用构造选项。
// 调用时引擎尚未交给调用方，不需要加锁。
func (e *Engine) applyOptions(opts []EngineOption) error {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(e); err != nil {
			return err
		}
	}
	return nil
}

// WithGameSetup 注册开局时的初始化。
//
// 在 Start() 里调用一次，产出的效果经与其余效果相同的写入点落地。
// 重复注册以最后一次为准。
func WithGameSetup(setup GameSetup) EngineOption {
	return func(e *Engine) error {
		if setup == nil {
			return WrapError(CodeInvalidConfig, "game setup must not be nil")
		}
		e.gameSetup = setup
		return nil
	}
}
