package engine

import "testing"

// gotoAndTrigger 同时产出一条死亡触发和一条改写出口——两者抢同一个决定。
type gotoAndTrigger struct {
	trigger PhaseType
	goto_   PhaseType
}

func (r gotoAndTrigger) Resolve([]*SkillUse, GameView) []*Effect {
	return []*Effect{
		NewAbilityTriggerEffect("w1", r.trigger),
		NewGotoPhaseEffect(r.goto_),
	}
}

// TestGotoPhase_TriggerQueueWins 待结算的触发排在改写出口前面。
//
// 这条优先级不是随便定的：触发队列必须排空，胜负判定与回合边界都等着它
// （见 advancePhase 与 nextPhase）。中途被 GOTO 跳走的话，还没结算的死亡
// 技能会凭空消失——被投出去的猎人那一枪没了，而规则那边完全看不出来。
//
// 写这个测试是因为变异验证发现了缺口：把 GOTO 挪到触发前面，整套测试
// 一条都不红。文档里写着的规矩必须有东西守着，否则它只是一句话。
func TestGotoPhase_TriggerQueueWins(t *testing.T) {
	opts := append(withNoopResolvers(),
		WithResolver(phaseNightGuard, gotoAndTrigger{
			trigger: phaseNightHunter,
			goto_:   phaseDay,
		}))
	e := newTestEngine(t, opts...)
	mustAdd(t, e, "w1", roleWerewolf)
	mustAdd(t, e, "g", roleGuard)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if _, err := e.EndPhase(); err != nil {
		t.Fatalf("EndPhase: %v", err)
	}
	if got := e.Phase(); got != phaseNightHunter {
		t.Fatalf("阶段 = %v，期望 %v——触发队列必须排在 GOTO 前面",
			got, phaseNightHunter)
	}

	// 触发结算完之后，那条 GOTO 不该被记着：出口回到该阶段自己的 NextPhase
	if _, err := e.EndPhase(); err != nil {
		t.Fatalf("EndPhase: %v", err)
	}
	if got, want := e.Phase(), testConfig().Phases[phaseNightHunter].NextPhase; got != want {
		t.Errorf("阶段 = %v，期望 %v——上一阶段的 GOTO 不该跨阶段生效", got, want)
	}
}

// gotoOnly 只产出一条改写出口
type gotoOnly struct{ to PhaseType }

func (r gotoOnly) Resolve([]*SkillUse, GameView) []*Effect {
	return []*Effect{NewGotoPhaseEffect(r.to)}
}

// TestGotoPhase_UnknownTargetFallsBack 目标阶段不在配置里时退回 NextPhase。
//
// 一条效果写错了不该让整局崩掉，但也不能安静地跳去一个没人预期的地方——
// 内核记一条错误日志再退回默认出口。
func TestGotoPhase_UnknownTargetFallsBack(t *testing.T) {
	rec := &recordingLogger{}
	opts := append(withNoopResolvers(),
		WithResolver(phaseNightGuard, gotoOnly{to: PhaseType("NO_SUCH_PHASE")}),
		WithLogger(rec))
	e := newTestEngine(t, opts...)
	mustAdd(t, e, "w1", roleWerewolf)
	mustAdd(t, e, "g", roleGuard)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := e.EndPhase(); err != nil {
		t.Fatalf("EndPhase: %v", err)
	}

	if got, want := e.Phase(), testConfig().Phases[phaseNightGuard].NextPhase; got != want {
		t.Errorf("阶段 = %v，期望退回默认出口 %v", got, want)
	}
	if !rec.sawError {
		t.Error("目标阶段不存在时该记一条错误日志——安静地退回等于把 bug 藏起来")
	}
}

// TestGotoPhase_CanceledIsIgnored 被否决的 GOTO 不算数。
//
// 规则自己把它 Cancel 掉了，说明那条指令不该生效。
func TestGotoPhase_CanceledIsIgnored(t *testing.T) {
	opts := append(withNoopResolvers(),
		WithResolver(phaseNightGuard, canceledGoto{to: phaseVote}))
	e := newTestEngine(t, opts...)
	mustAdd(t, e, "w1", roleWerewolf)
	mustAdd(t, e, "g", roleGuard)
	if err := e.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := e.EndPhase(); err != nil {
		t.Fatalf("EndPhase: %v", err)
	}
	if got, want := e.Phase(), testConfig().Phases[phaseNightGuard].NextPhase; got != want {
		t.Errorf("阶段 = %v，期望 %v——被否决的 GOTO 不该生效", got, want)
	}
}

type canceledGoto struct{ to PhaseType }

func (r canceledGoto) Resolve([]*SkillUse, GameView) []*Effect {
	ef := NewGotoPhaseEffect(r.to)
	ef.Cancel("规则自己撤回了")
	return []*Effect{ef}
}

// recordingLogger 只关心有没有记过错误
type recordingLogger struct{ sawError bool }

func (l *recordingLogger) Debug(string, ...Field) {}
func (l *recordingLogger) Info(string, ...Field)  {}
func (l *recordingLogger) Warn(string, ...Field)  {}
func (l *recordingLogger) Error(string, ...Field) { l.sawError = true }
