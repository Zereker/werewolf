package werewolf

import (
	pb "github.com/Zereker/werewolf/proto"
)

// 效果流中用于重建局面的几个键
const (
	roleKey     = "role"
	campKey     = "camp"
	categoryKey = "category"
	phaseKey    = "phase"
)

// newPlayerAddedEffect 记录一名玩家入座
func newPlayerAddedEffect(id string, role pb.RoleType, camp pb.Camp, category RoleCategory) *Effect {
	return NewEffect(pb.EventType_EVENT_TYPE_PLAYER_ADDED, "", id).
		WithData(roleKey, role).
		WithData(campKey, camp).
		WithData(categoryKey, category)
}

// newPhaseChangedEffect 记录一次阶段流转
func newPhaseChangedEffect(phase pb.PhaseType) *Effect {
	return NewEffect(pb.EventType_EVENT_TYPE_PHASE_CHANGED, "", "").
		WithData(phaseKey, phase)
}

// newGameStartedEffect 记录开局
func newGameStartedEffect(phase pb.PhaseType) *Effect {
	return NewEffect(pb.EventType_EVENT_TYPE_GAME_STARTED, "", "").
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
func ReplayEngine(config *GameConfig, log []*Effect) (*Engine, error) {
	engine, err := NewEngine(config)
	if err != nil {
		return nil, err
	}

	for i, effect := range log {
		if effect == nil {
			return nil, WrapError(pb.ErrorCode_ERROR_CODE_INVALID_SNAPSHOT,
				"effect log contains a nil entry at index %d", i)
		}
		if err := engine.replayEffect(effect); err != nil {
			return nil, err
		}
	}

	engine.mu.Lock()
	engine.effectLog = append(engine.effectLog, log...)
	engine.mu.Unlock()

	return engine, nil
}

// replayEffect 重放单个效果
func (e *Engine) replayEffect(effect *Effect) error {
	switch effect.Type {
	case pb.EventType_EVENT_TYPE_PLAYER_ADDED:
		role, _ := effect.Data[roleKey].(pb.RoleType)
		camp, _ := effect.Data[campKey].(pb.Camp)
		category, _ := effect.Data[categoryKey].(RoleCategory)
		if err := e.state.addCustomPlayer(effect.TargetID, role, camp, category); err != nil {
			return err
		}

	case pb.EventType_EVENT_TYPE_GAME_STARTED:
		phase, ok := effect.Data[phaseKey].(pb.PhaseType)
		if !ok {
			return WrapError(pb.ErrorCode_ERROR_CODE_INVALID_SNAPSHOT,
				"game started effect carries no phase")
		}
		e.state.startAt(phase)

	case pb.EventType_EVENT_TYPE_PHASE_CHANGED:
		phase, ok := effect.Data[phaseKey].(pb.PhaseType)
		if !ok {
			return WrapError(pb.ErrorCode_ERROR_CODE_INVALID_SNAPSHOT,
				"phase changed effect carries no phase")
		}
		e.state.nextPhase(phase)

	case pb.EventType_EVENT_TYPE_GAME_ENDED:
		e.state.nextPhase(pb.PhaseType_PHASE_TYPE_END)

	default:
		e.state.applyEffect(effect)
	}
	return nil
}
