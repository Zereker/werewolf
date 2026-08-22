package engine

import ()

// 效果流中用于重建局面的几个键
//
// 这里此前还有 camp 与 category：它们曾是内核状态的一部分，于是入座那一条
// 效果要单独记。现在它们只是玩家身上的两项状态，跟着 vars 一起走。
const (
	roleKey  = "role"
	phaseKey = "phase"
	varsKey  = "vars"
)

// newPlayerAddedEffect 记录一名玩家入座，连同该角色的初始状态。
//
// 记下 vars 而不是在回放时重新问一遍 RoleSetup，理由见 Engine.seatPlayer。
func newPlayerAddedEffect(id string, role RoleType, vars map[string]string) *Effect {
	effect := NewEffect(EventPlayerAdded, "", id).
		WithData(roleKey, role)
	if len(vars) > 0 {
		effect = effect.WithData(varsKey, copyVars(vars))
	}
	return effect
}

// newPhaseChangedEffect 记录一次阶段流转
func newPhaseChangedEffect(phase PhaseType) *Effect {
	return NewEffect(EventPhaseChanged, "", "").
		WithData(phaseKey, phase)
}

// newGameStartedEffect 记录开局
func newGameStartedEffect(phase PhaseType) *Effect {
	return NewEffect(EventGameStarted, "", "").
		WithData(phaseKey, phase)
}

// EffectLog 返回自建局以来的完整效果流。
//
// 这套架构本来就在产出一条干净的事件流——Resolver 是纯函数，
// 状态变更只经 ApplyEffect 一个写入点——把它累积下来几乎是白捡的：
// 战报回放、复盘、「第三夜到底发生了什么」的排查全都有了依据。
//
// 返回的是切片副本，其中的 *Effect 仍是引擎持有的对象，请勿修改。
//
// # 与 Snapshot 的分工
//
// 效果流是历史，快照是状态。要做持久化请用 Snapshot：
// Effect.Data 是 map[string]interface{}，经 JSON 往返后类型会退化，
// 效果流的设计目标是进程内的回放与审计，不是存储格式。
func (e *Engine) EffectLog() []*Effect {
	e.mu.RLock()
	defer e.mu.RUnlock()

	out := make([]*Effect, len(e.effectLog))
	for i, ef := range e.effectLog {
		out[i] = ef.clone()
	}
	return out
}

// ReplayEngine 按效果流重建引擎。
//
// config 需与录制时一致——效果流记录的是「发生了什么」，不含规则。
//
// 重建结果与录制结束时的引擎在玩家状态、阶段、回合上一致；
// 但当前阶段尚未提交的技能不在效果流里（它们还没变成效果），
// 需要那部分请用 Snapshot。
//
// 自定义角色的解析器必须经 opts 传入，理由同 RestoreEngine。
// 初始状态不用：它记在效果流里的入座那一条上（见 Engine.seatPlayer）。
func ReplayEngine(config *Config, log []*Effect, opts ...EngineOption) (*Engine, error) {
	engine, err := NewEngine(config, opts...)
	if err != nil {
		return nil, err
	}

	if err := engine.phase.validateResolvers(); err != nil {
		return nil, err
	}

	engine.mu.Lock()
	defer engine.mu.Unlock()

	for i, effect := range log {
		if effect == nil {
			return nil, WrapError(CodeInvalidEffectLog,
				"effect log contains a nil entry at index %d", i)
		}
		if err := engine.replayEffect(effect); err != nil {
			return nil, err
		}
	}

	engine.recordEffects(log...)

	return engine, nil
}

// replayEffect 重放单个效果
func (e *Engine) replayEffect(effect *Effect) error {
	switch effect.Type {
	case EventPlayerAdded:
		role, _ := effect.Data[roleKey].(RoleType)
		// 入座要连初始状态一起发。少了这一步，回放出来的女巫手里
		// 没有药、狼人不属于任何阵营，而分叉要到用药或判胜负时才暴露。
		vars, _ := effect.Data[varsKey].(map[string]string)
		if err := e.seatPlayer(effect.TargetID, role, vars); err != nil {
			return err
		}

	case EventGameStarted:
		phase, ok := effect.Data[phaseKey].(PhaseType)
		if !ok {
			return WrapError(CodeInvalidEffectLog,
				"game started effect carries no phase")
		}
		e.state.startAt(phase)

	case EventPhaseChanged:
		phase, ok := effect.Data[phaseKey].(PhaseType)
		if !ok {
			return WrapError(CodeInvalidEffectLog,
				"phase changed effect carries no phase")
		}
		// 离开一个阶段时消费掉它对应的待结算技能，与正常推进
		// （calculateNextPhase）做同样的事。少了这一步，回放出来的引擎
		// 会带着一条本该消费掉的触发，从下一步起与原引擎分叉
		// 回合边界由**刚离开的**那个阶段声明，先取下来再流转。
		// 与正常推进同一条规矩：还有待结算的触发时不能落下，
		// 否则会把住在回合上下文里的待结算队列一起抹掉。
		endsRound := e.config.endsRound(e.state.Phase)
		e.state.consumeTriggerFor(e.state.Phase)
		settled := !e.state.hasPendingTrigger()
		e.state.nextPhase(phase, endsRound && settled,
			settled && e.config.clearsRoundVars(phase))

	case EventGameEnded:
		e.state.nextPhase(PhaseEnd, false, false) // 整局结束，不是新回合

	default:
		e.state.applyEffect(effect)
	}
	return nil
}

// recordEffects 把一批效果记进历史。
//
// 存的是副本：out.effects 会原样返回给 EndPhase 的调用方，
// 共用同一批指针的话，对方改一个字段就改了引擎的历史。
// 这是效果流唯一的写入口，与 applyEffect 是状态唯一的写入口对称。
func (e *Engine) recordEffects(effects ...*Effect) {
	for _, ef := range effects {
		e.effectLog = append(e.effectLog, ef.clone())
	}
}
