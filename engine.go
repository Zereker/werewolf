package werewolf

import (
	"sync"

	pb "github.com/Zereker/werewolf/proto"
)

type Engine struct {
	mu sync.RWMutex

	config  *GameConfig
	state   *gameState
	phase   *phaseManager
	logger  Logger
	metrics Metrics

	// 当前阶段收集的技能使用
	pendingUses []*SkillUse

	// 自建局以来的完整效果流，只追加
	effectLog []*Effect

	// 事件通知（可选）
	eventHandlers []EventHandler

	// 消息通知（可选）
	messageHandlers []MessageHandler
}

// NewEngine 创建游戏引擎。
//
// config 为 nil 时使用默认配置。配置会先经 GameConfig.Validate 校验——
// 阶段流转图是使用者可替换的数据，悬空的 NextPhase 会让游戏推进到一半
// 静默结束，这类问题必须在构造时暴露。
func NewEngine(config *GameConfig) (*Engine, error) {
	if config == nil {
		config = DefaultGameConfig()
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &Engine{
		config:          config,
		state:           newState(),
		phase:           newPhaseManager(config),
		logger:          NewNopLogger(),
		metrics:         NewNopMetrics(),
		pendingUses:     make([]*SkillUse, 0),
		effectLog:       make([]*Effect, 0),
		eventHandlers:   make([]EventHandler, 0),
		messageHandlers: make([]MessageHandler, 0),
	}, nil
}

// MustNewEngine 同 NewEngine，配置不合法时 panic。
//
// 适用于配置是编译期常量的场合（示例、测试、写死默认配置的服务启动路径）。
func MustNewEngine(config *GameConfig) *Engine {
	engine, err := NewEngine(config)
	if err != nil {
		panic("werewolf: invalid game config: " + err.Error())
	}
	return engine
}

// SetLogger 设置日志接口
func (e *Engine) SetLogger(logger Logger) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if logger != nil {
		e.logger = logger
	}
}

// SetMetrics 设置指标收集器
func (e *Engine) SetMetrics(metrics Metrics) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if metrics != nil {
		e.metrics = metrics
	}
}

// addPlayer 添加玩家。阵营与角色类别由角色推导。
//
// 只能在 Start 之前调用。返回错误：游戏已开始、ID 为空、ID 已存在、
// 角色不能作为玩家身份。
func (e *Engine) AddPlayer(id string, role pb.RoleType) error {
	return e.AddCustomPlayer(id, role, CampOf(role), CategoryOf(role))
}

// addCustomPlayer 添加玩家并显式指定阵营与角色类别，供扩展角色使用。
func (e *Engine) AddCustomPlayer(id string, role pb.RoleType, camp pb.Camp, category RoleCategory) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 开局后再改动玩家会让已发出的身份信息与引擎状态不一致
	if e.state.Phase != pb.PhaseType_PHASE_TYPE_START {
		return ErrGameAlreadyStarted
	}

	if err := e.state.addCustomPlayer(id, role, camp, category); err != nil {
		return err
	}
	e.effectLog = append(e.effectLog, newPlayerAddedEffect(id, role, camp, category))
	return nil
}

// RegisterResolver 注册或替换某个阶段的解析器。
//
// 这是扩展新角色的入口。引擎内置的六个角色只是一套默认板子，
// 加入狼王、白痴、骑士等角色不应该要求 fork 这个库：
//
//	cfg := werewolf.DefaultGameConfig()
//	cfg.Phases[myPhase] = &werewolf.PhaseConfig{ ... }   // 声明阶段
//	engine, _ := werewolf.NewEngine(cfg)
//	engine.RegisterResolver(myPhase, myResolver)          // 注册解析器
//	engine.AddCustomPlayer("p1", myRole, camp, category)  // 指定阵营与类别
//
// 死亡时触发的能力由 Resolver 产出 NewAbilityTriggerEffect 即可，
// 引擎不需要认识具体角色。
//
// 只能在 Start 之前调用。resolver 为 nil 时报错——若想让某阶段
// 不产生任何效果，注册一个返回空切片的解析器。
func (e *Engine) RegisterResolver(phase pb.PhaseType, resolver Resolver) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state.Phase != pb.PhaseType_PHASE_TYPE_START {
		return ErrGameAlreadyStarted
	}
	if resolver == nil {
		return WrapError(pb.ErrorCode_ERROR_CODE_INVALID_PHASE,
			"resolver for phase %v must not be nil", phase)
	}

	e.phase.registerResolver(phase, resolver)
	return nil
}

// Start 开始游戏
func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state.Phase != pb.PhaseType_PHASE_TYPE_START {
		return ErrGameAlreadyStarted
	}

	// 校验板子：缺任一阵营的局面从第一次结算起就已分出胜负，
	// 与其让它「开局即结束」，不如在这里直接拒绝
	good, evil := e.state.countCamps()
	if evil == 0 {
		return ErrNoWerewolf
	}
	if good == 0 {
		return ErrNoGoodPlayer
	}

	// 每个阶段都必须有解析器，否则推进到那里时技能会被静默丢弃。
	// 解析器可以在构造之后注册，故此项校验放在这里而非 NewEngine。
	if err := e.phase.validateResolvers(); err != nil {
		return err
	}

	start := e.config.startPhase()
	e.state.startAt(start)

	e.effectLog = append(e.effectLog, newGameStartedEffect(start))
	e.logger.Info("game started", RoundField(1), PhaseField(start))

	return nil
}

// SubmitSkillUse 提交技能使用
func (e *Engine) SubmitSkillUse(use *SkillUse) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 验证技能使用
	if err := e.phase.validateSkillUse(use, e.state); err != nil {
		e.logger.Debug("skill validation failed",
			PlayerField(use.PlayerID),
			SkillField(use.Skill),
			F("error", err.Error()))
		return err
	}

	// 添加到待处理列表
	use.Phase = e.state.Phase
	use.Round = e.state.Round
	e.pendingUses = append(e.pendingUses, use)

	e.logger.Debug("skill submitted",
		PlayerField(use.PlayerID),
		SkillField(use.Skill),
		TargetField(use.TargetID))
	e.metrics.IncSkillSubmitted(use.Skill)

	return nil
}

// phaseOutcome 一次阶段推进的结果，供锁外使用
type phaseOutcome struct {
	effects  []*Effect      // 本阶段产生的全部效果（含内部效果）
	events   []*pb.Event    // 需要对外发布的事件
	handlers []EventHandler // 锁内快照的处理器
	logger   Logger         // 锁内快照的日志器
}

// endPhaseInternal 结束阶段：先在锁内推进状态，再在锁外分发事件。
//
// 拆成两段是有意的：分发必须在锁外（用户回调里可能回调 Engine），
// 而推进必须全程持锁。写在一个函数里就得手动 Unlock，
// 任何人日后加一条提前返回都会漏掉解锁。
func (e *Engine) endPhaseInternal() ([]*Effect, error) {
	out, err := e.advancePhase()
	if err != nil {
		return nil, err
	}

	for _, event := range out.events {
		dispatchEvent(out.handlers, out.logger, event)
	}

	return out.effects, nil
}

// advancePhase 在锁内完成一次阶段推进，返回需要在锁外发布的内容。
func (e *Engine) advancePhase() (phaseOutcome, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	currentPhase := e.state.Phase
	if currentPhase == pb.PhaseType_PHASE_TYPE_END {
		return phaseOutcome{}, ErrGameEnded
	}

	e.logger.Debug("ending phase", PhaseField(currentPhase), RoundField(e.state.Round))

	out := phaseOutcome{}

	// 1. 解析技能，产生效果
	if resolver := e.phase.resolver(currentPhase); resolver != nil {
		out.effects = resolver.Resolve(e.pendingUses, newStateView(e.state), e.config)
		e.logger.Debug("resolved effects", PhaseField(currentPhase), F("effect_count", len(out.effects)))
	}

	// 2. 应用效果，收集对外可见的事件
	for _, effect := range out.effects {
		e.state.applyEffect(effect)
		if isInternalEvent(effect.Type) {
			continue
		}
		out.events = append(out.events, effect.ToEvent())
		e.logger.Debug("effect applied",
			EventField(effect.Type),
			PlayerField(effect.SourceID),
			TargetField(effect.TargetID))
		e.metrics.IncEffectApplied(effect.Type)
	}
	e.effectLog = append(e.effectLog, out.effects...)

	// 3. 清空待处理列表
	e.pendingUses = nil
	e.metrics.IncPhaseEnded(currentPhase)

	// 4. 计算下一阶段。
	//    死亡技能可能改变胜负——被刀的猎人开枪带走最后一只狼，好人反而获胜——
	//    因此只要还有待结算的死亡技能，就推迟胜负判定，先让它结算完。
	nextPhase := e.calculateNextPhase(currentPhase)

	gameOver, winner := e.state.checkVictory(e.config.VictoryMode)
	endNow := gameOver && !e.state.hasPendingTrigger()
	if endNow {
		nextPhase = pb.PhaseType_PHASE_TYPE_END
	}

	// 5. 流转。END 也走 nextPhase，不直接赋值 Phase——
	//    状态的每一次改动都经同一条路径，别处才不会漏掉伴随的逻辑
	e.state.nextPhase(nextPhase)

	if endNow {
		// 结束事件与其他事件走同一条构造路径：Effect -> ToEvent，
		// 避免同一个事件有两份分别构造、日后各自漂移的实现
		endEffect := NewEffect(pb.EventType_EVENT_TYPE_GAME_ENDED, "", "").
			WithData("winner", winner)
		e.effectLog = append(e.effectLog, endEffect)
		out.events = append(out.events, endEffect.ToEvent())

		e.logger.Info("game ended", F("winner", winner.String()))
		e.metrics.IncGameEnded(winner)
	} else {
		e.effectLog = append(e.effectLog, newPhaseChangedEffect(nextPhase))
		e.logger.Debug("phase transition",
			F("from", currentPhase.String()),
			F("to", nextPhase.String()))
	}

	// 在锁内快照 handler 与 logger：回调要在锁外执行，
	// 而在锁外读取 e.eventHandlers 会与 OnEvent 竞争
	out.handlers = e.snapshotEventHandlersLocked()
	out.logger = e.logger

	return out, nil
}

// EndPhase 结束当前阶段：解析技能、应用效果、判定胜负、流转到下一阶段。
//
// 这是驱动游戏推进的唯一入口。流转规则以阶段配置（PhaseConfig.NextPhase）
// 为准，并处理动态触发的阶段（猎人死亡后的开枪阶段）。
func (e *Engine) EndPhase() ([]*Effect, error) {
	return e.endPhaseInternal()
}

// PlayerInfo 获取玩家信息的只读副本（推荐使用）
// 返回 PlayerInfo 结构体副本，避免外部修改内部状态
func (e *Engine) PlayerInfo(playerID string) (PlayerInfo, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.PlayerInfo(playerID)
}

// Phase 获取当前阶段
func (e *Engine) Phase() pb.PhaseType {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.Phase
}

// Round 获取当前回合
func (e *Engine) Round() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.Round
}

// allowedSkills 获取玩家当前可用的技能
func (e *Engine) AllowedSkills(playerID string) []pb.SkillType {
	e.mu.RLock()
	defer e.mu.RUnlock()

	player, ok := e.state.getPlayer(playerID)
	if !ok {
		return nil
	}

	// 待结算死亡技能的玩家即便已出局，本阶段仍持有其技能
	if !player.Alive && !e.isPendingActor(playerID) {
		return nil
	}

	return e.phase.allowedSkills(e.state.Phase, player.Role)
}

// isPendingActor 该玩家是否是当前阶段待结算死亡技能的持有者。
// 调用前需持有 e.mu。
func (e *Engine) isPendingActor(playerID string) bool {
	t, ok := e.state.peekTrigger()
	return ok && t.PlayerID == playerID && t.Phase == e.state.Phase
}

// AlivePlayerIDs 返回所有存活玩家的 ID，按字典序排序。
//
// 谁还活着是公开信息。此前调用方要拿这份名单只能绕道
// PhaseInfo().RoleInfos[UNSPECIFIED]——而那个入口依赖当前阶段
// 恰好声明了面向全体玩家的步骤，白天不再有玩家技能步骤之后就取不到了。
func (e *Engine) AlivePlayerIDs() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return sortedStrings(e.state.getAlivePlayerIDs())
}

// IsGameOver 游戏是否结束
func (e *Engine) IsGameOver() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.Phase == pb.PhaseType_PHASE_TYPE_END
}

// NightKillTarget 获取当晚被狼人击杀的目标（女巫可查询）
func (e *Engine) NightKillTarget() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.RoundCtx.KillTarget
}

// RoundContext 获取回合上下文的只读副本
func (e *Engine) RoundContext() *RoundContext {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.RoundContext()
}

// getWolfTeammates 获取狼人队友
func (e *Engine) WolfTeammates(playerID string) []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	player, ok := e.state.getPlayer(playerID)
	if !ok || player.Role != pb.RoleType_ROLE_TYPE_WEREWOLF {
		return nil
	}

	return e.state.getWolfTeammates(playerID)
}

// PhaseInfo 获取当前阶段信息（上帝视角）。
//
// 返回的内容包含狼队名单、女巫可见的刀口等敏感信息，供调用方作为主持人
// 组织本阶段的流程与公告使用，**不可以整体转发给玩家**。
// 要拿到可以直接发给某个玩家的内容，用 PlayerView。
//
// 各角色的信息由阶段配置（PhaseConfig.Steps）派生，因此第三方通过
// RegisterResolver 加入的自定义角色同样能拿到——此前这里是一个写死
func (e *Engine) calculateNextPhase(currentPhase pb.PhaseType) pb.PhaseType {
	// 刚结束的正是队首触发要求的阶段，说明该技能已结算，出队。
	// 不出队的话标记会在整个回合内持续为真，同一个玩家会被反复拉回来。
	if t, ok := e.state.peekTrigger(); ok && t.Phase == currentPhase {
		e.state.popTrigger()
	}

	// 还有待结算的死亡技能，先去处理（可能有多个，逐个来）
	if t, ok := e.state.peekTrigger(); ok {
		return t.Phase
	}

	// 使用声明式配置获取下一阶段
	return e.phase.nextSubPhase(currentPhase)
}
