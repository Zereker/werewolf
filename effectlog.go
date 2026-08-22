package werewolf

import ()

// 效果流中用于重建局面的几个键
const (
	roleKey     = "role"
	campKey     = "camp"
	categoryKey = "category"
	phaseKey    = "phase"
)

// newPlayerAddedEffect 记录一名玩家入座
func newPlayerAddedEffect(id string, role RoleType, camp Camp, category RoleCategory) *Effect {
	return NewEffect(EventPlayerAdded, "", id).
		WithData(roleKey, role).
		WithData(campKey, camp).
		WithData(categoryKey, category)
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
	copy(out, e.effectLog)
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
func ReplayEngine(config *GameConfig, log []*Effect, opts ...EngineOption) (*Engine, error) {
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

	engine.effectLog = append(engine.effectLog, log...)

	return engine, nil
}

// replayEffect 重放单个效果
func (e *Engine) replayEffect(effect *Effect) error {
	switch effect.Type {
	case EventPlayerAdded:
		role, _ := effect.Data[roleKey].(RoleType)
		camp, _ := effect.Data[campKey].(Camp)
		category, _ := effect.Data[categoryKey].(RoleCategory)
		if err := e.state.addCustomPlayer(effect.TargetID, role, camp, category); err != nil {
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
		e.state.consumeTriggerFor(e.state.Phase)
		e.state.nextPhase(phase, e.config.startPhase())

	case EventGameEnded:
		e.state.nextPhase(PhaseEnd, e.config.startPhase())

	default:
		e.state.applyEffect(effect)
	}
	return nil
}
