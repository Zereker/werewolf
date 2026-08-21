package werewolf

import (
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	pb "github.com/Zereker/werewolf/proto"
)

// EventHandler 事件处理器
type EventHandler func(event *pb.Event)

// Message 游戏内消息
type Message struct {
	SenderID  string       // 发送者ID
	Content   string       // 消息内容
	Phase     pb.PhaseType // 发送时的阶段
	Round     int          // 发送时的回合
	Timestamp time.Time    // 发送时间
}

// MessageHandler 消息处理器
// msg: 消息内容
// receiverIDs: 接收者列表
type MessageHandler func(msg *Message, receiverIDs []string)

// PhaseInfo 阶段信息（纯状态，不含消息内容）
// 调用方根据此信息构建上帝公告
type PhaseInfo struct {
	Phase       pb.PhaseType                   // 当前阶段
	Round       int                            // 当前回合
	Steps       []PhaseStep                    // 当前阶段的步骤配置（包含上帝公告和玩家行动）
	ActiveRoles []pb.RoleType                  // 需要行动的玩家角色（不含上帝）
	RoleInfos   map[pb.RoleType]*RolePhaseInfo // 各角色的阶段信息
}

// NeedsGodAnnouncement 判断当前阶段是否需要上帝公告
func (p *PhaseInfo) NeedsGodAnnouncement() bool {
	if len(p.Steps) == 0 {
		return false
	}
	return p.Steps[0].Role == pb.RoleType_ROLE_TYPE_GOD &&
		p.Steps[0].Skill == pb.SkillType_SKILL_TYPE_ANNOUNCE
}

// GetGodAnnouncementStep 获取上帝公告步骤（如果存在）
func (p *PhaseInfo) GetGodAnnouncementStep() *PhaseStep {
	if p.NeedsGodAnnouncement() {
		return &p.Steps[0]
	}
	return nil
}

// GetPlayerActionSteps 获取玩家行动步骤（不含上帝公告）
func (p *PhaseInfo) GetPlayerActionSteps() []PhaseStep {
	if len(p.Steps) == 0 {
		return nil
	}
	if p.NeedsGodAnnouncement() {
		return p.Steps[1:]
	}
	return p.Steps
}

// RolePhaseInfo 角色阶段信息
type RolePhaseInfo struct {
	PlayerIDs     []string            // 该角色的玩家ID列表
	AllowedSkills []pb.SkillType      // 可用技能
	Teammates     map[string][]string // 队友信息（狼人：玩家ID -> 队友IDs）
	KillTarget    string              // 被杀目标（女巫可见）
}

// Engine 游戏引擎（轻量状态机）
type Engine struct {
	mu sync.RWMutex

	config  *GameConfig
	state   *gameState
	phase   *Phase
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
		phase:           NewPhase(config),
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
	if err := e.phase.ValidateSkillUse(use, e.state); err != nil {
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
	if resolver := e.phase.GetResolver(currentPhase); resolver != nil {
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

// GetPlayerInfo 获取玩家信息的只读副本（推荐使用）
// 返回 PlayerInfo 结构体副本，避免外部修改内部状态
func (e *Engine) GetPlayerInfo(playerID string) (PlayerInfo, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.GetPlayerInfo(playerID)
}

// GetCurrentPhase 获取当前阶段
func (e *Engine) GetCurrentPhase() pb.PhaseType {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.Phase
}

// GetCurrentRound 获取当前回合
func (e *Engine) GetCurrentRound() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.Round
}

// GetAllowedSkills 获取玩家当前可用的技能
func (e *Engine) GetAllowedSkills(playerID string) []pb.SkillType {
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

	return e.phase.GetAllowedSkills(e.state.Phase, player.Role)
}

// isPendingActor 该玩家是否是当前阶段待结算死亡技能的持有者。
// 调用前需持有 e.mu。
func (e *Engine) isPendingActor(playerID string) bool {
	t, ok := e.state.peekTrigger()
	return ok && t.PlayerID == playerID && t.Phase == e.state.Phase
}

// IsGameOver 游戏是否结束
func (e *Engine) IsGameOver() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.Phase == pb.PhaseType_PHASE_TYPE_END
}

// GetNightKillTarget 获取当晚被狼人击杀的目标（女巫可查询）
func (e *Engine) GetNightKillTarget() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.RoundCtx.KillTarget
}

// GetRoundContext 获取回合上下文的只读副本
func (e *Engine) GetRoundContext() *RoundContext {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.GetRoundContext()
}

// getWolfTeammates 获取狼人队友
func (e *Engine) GetWolfTeammates(playerID string) []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	player, ok := e.state.getPlayer(playerID)
	if !ok || player.Role != pb.RoleType_ROLE_TYPE_WEREWOLF {
		return nil
	}

	return e.state.getWolfTeammates(playerID)
}

// GetPhaseInfo 获取当前阶段信息（上帝视角）。
//
// 返回的内容包含狼队名单、女巫可见的刀口等敏感信息，供调用方作为主持人
// 组织本阶段的流程与公告使用，**不可以整体转发给玩家**。
// 要拿到可以直接发给某个玩家的内容，用 GetPlayerView。
//
// 各角色的信息由阶段配置（PhaseConfig.Steps）派生，因此第三方通过
// RegisterResolver 加入的自定义角色同样能拿到——此前这里是一个写死
// 内置阶段的 switch，自定义阶段拿不到任何信息。
func (e *Engine) GetPhaseInfo() *PhaseInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()

	info := &PhaseInfo{
		Phase:       e.state.Phase,
		Round:       e.state.Round,
		Steps:       make([]PhaseStep, 0),
		ActiveRoles: make([]pb.RoleType, 0),
		RoleInfos:   make(map[pb.RoleType]*RolePhaseInfo),
	}

	phaseConfig := e.phase.GetPhaseConfig(e.state.Phase)
	if phaseConfig == nil {
		return info
	}

	// 返回副本：Steps 直接暴露会让调用方改到引擎内部的阶段配置
	info.Steps = make([]PhaseStep, len(phaseConfig.Steps))
	copy(info.Steps, phaseConfig.Steps)

	// 本阶段若在结算某个死亡技能，则只有触发者能行动
	trigger, hasTrigger := e.state.peekTrigger()
	triggerActive := hasTrigger && trigger.Phase == e.state.Phase

	seen := make(map[pb.RoleType]bool)
	for _, step := range phaseConfig.Steps {
		// 上帝是系统角色，不是需要行动的玩家
		if step.Role == pb.RoleType_ROLE_TYPE_GOD || seen[step.Role] {
			continue
		}
		seen[step.Role] = true

		info.ActiveRoles = append(info.ActiveRoles, step.Role)
		info.RoleInfos[step.Role] = e.buildRolePhaseInfo(step.Role, triggerActive, trigger)
	}

	return info
}

// allowedSkillsFor 返回指定角色在当前阶段可用的技能。
//
// 唯一真相来源是阶段配置（PhaseConfig.Steps），与 ValidateSkillUse 走同一条路径。
func (e *Engine) allowedSkillsFor(role pb.RoleType) []pb.SkillType {
	return e.phase.GetAllowedSkills(e.state.Phase, role)
}

// buildRolePhaseInfo 组装某个角色在当前阶段的信息。
// 调用前需持有 e.mu。
func (e *Engine) buildRolePhaseInfo(role pb.RoleType, triggerActive bool, trigger PendingTrigger) *RolePhaseInfo {
	ri := &RolePhaseInfo{
		AllowedSkills: e.allowedSkillsFor(role),
	}

	switch {
	case triggerActive:
		// 死亡技能阶段：行动者只有触发者本人
		ri.PlayerIDs = []string{trigger.PlayerID}
	case role == pb.RoleType_ROLE_TYPE_UNSPECIFIED:
		// UNSPECIFIED 表示所有存活玩家（如投票）
		ri.PlayerIDs = e.state.getAlivePlayerIDs()
	default:
		ri.PlayerIDs = e.state.getAlivePlayerIDsByRole(role)
	}

	switch role {
	case pb.RoleType_ROLE_TYPE_WEREWOLF:
		// 狼人需要知道队友才能协商
		ri.Teammates = make(map[string][]string, len(ri.PlayerIDs))
		for _, id := range ri.PlayerIDs {
			ri.Teammates[id] = e.state.getWolfTeammates(id)
		}
	case pb.RoleType_ROLE_TYPE_WITCH:
		// 规则：解药未使用时才可得知刀口
		if e.state.anyAliveWitchHasAntidote() {
			ri.KillTarget = e.state.RoundCtx.KillTarget
		}
	}

	return ri
}

// calculateNextPhase 计算下一阶段（考虑动态触发）
//
// 猎人触发标记是「一次性」的：由死亡结算置位，进入猎人阶段后必须消费掉。
// 若不消费，标记会在整个回合内持续为真——夜里开过枪的猎人，会在当天
// 投票结束后被再次拉进 DAY_HUNTER 并开出第二枪。
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
	return e.phase.NextSubPhase(currentPhase)
}

// OnEvent 注册事件处理器
func (e *Engine) OnEvent(handler EventHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.eventHandlers = append(e.eventHandlers, handler)
}

// snapshotEventHandlersLocked 复制事件处理器列表。
// 调用前必须持有 e.mu（读锁或写锁）。
func (e *Engine) snapshotEventHandlersLocked() []EventHandler {
	handlers := make([]EventHandler, len(e.eventHandlers))
	copy(handlers, e.eventHandlers)
	return handlers
}

// dispatchEvent 在锁外分发事件。
// 每个 handler 独立执行，单个 handler panic 不影响其他 handler。
func dispatchEvent(handlers []EventHandler, logger Logger, event *pb.Event) {
	for _, handler := range handlers {
		func() {
			defer recoverHandlerPanic(logger, "event handler", EventField(event.Type))
			handler(event)
		}()
	}
}

// recoverHandlerPanic 捕获用户回调中的 panic 并记录。
//
// 吞掉 panic 是为了让单个 handler 的故障不波及其他 handler，
// 但必须留下日志——静默吞掉会让线上问题完全没有痕迹。
func recoverHandlerPanic(logger Logger, kind string, fields ...Field) {
	r := recover()
	if r == nil {
		return
	}
	if logger == nil {
		return
	}
	logger.Error(kind+" panicked",
		append(fields,
			F("panic", fmt.Sprintf("%v", r)),
			F("stack", string(debug.Stack())),
		)...)
}

// ==================== 消息系统 ====================

// OnMessage 注册消息处理器
// 当玩家发送消息时，处理器会收到消息和接收者列表
func (e *Engine) OnMessage(handler MessageHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.messageHandlers = append(e.messageHandlers, handler)
}

// SendMessage 发送消息
// 根据当前阶段自动路由到正确的接收者
// 返回错误：玩家不存在、玩家已死亡、当前阶段不允许发言
func (e *Engine) SendMessage(senderID, content string) error {
	e.mu.RLock()

	// 验证发送者
	sender, ok := e.state.getPlayer(senderID)
	if !ok {
		e.mu.RUnlock()
		return ErrPlayerNotFound
	}
	if !sender.Alive {
		e.mu.RUnlock()
		return ErrPlayerDead
	}

	// 获取接收者
	receiverIDs := e.getMessageReceivers(senderID)
	if len(receiverIDs) == 0 {
		e.mu.RUnlock()
		return ErrMessageNotAllowed
	}

	// 构建消息
	msg := &Message{
		SenderID:  senderID,
		Content:   content,
		Phase:     e.state.Phase,
		Round:     e.state.Round,
		Timestamp: time.Now(),
	}

	// 复制 handlers 与 logger 以避免在回调中死锁、并避免锁外读取竞争
	handlers := make([]MessageHandler, len(e.messageHandlers))
	copy(handlers, e.messageHandlers)
	logger := e.logger

	e.mu.RUnlock()

	// 发布消息（锁外执行，避免死锁）
	publishMessage(handlers, logger, msg, receiverIDs)

	logger.Debug("message sent",
		PlayerField(senderID),
		PhaseField(msg.Phase),
		F("receiver_count", len(receiverIDs)))

	return nil
}

// GetMessageReceivers 获取消息接收者列表（公开方法）
// 返回当前阶段下，指定发送者的消息可以发送给哪些玩家
func (e *Engine) GetMessageReceivers(senderID string) []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.getMessageReceivers(senderID)
}

// getMessageReceivers 获取消息接收者（内部方法，调用前需持有锁）
func (e *Engine) getMessageReceivers(senderID string) []string {
	sender, ok := e.state.getPlayer(senderID)
	if !ok || !sender.Alive {
		return nil
	}

	switch e.state.Phase {
	case pb.PhaseType_PHASE_TYPE_NIGHT_WOLF:
		// 狼人阶段：只有狼人能互相交流
		if sender.Role != pb.RoleType_ROLE_TYPE_WEREWOLF {
			return nil
		}
		// 返回所有存活的狼人（包括自己，方便处理）
		return e.state.getAlivePlayerIDsByRole(pb.RoleType_ROLE_TYPE_WEREWOLF)

	case pb.PhaseType_PHASE_TYPE_DAY:
		// 白天阶段：所有存活玩家都能听到
		return e.state.getAlivePlayerIDs()

	default:
		// 其他阶段不允许发言
		return nil
	}
}

// publishMessage 在锁外发布消息。
func publishMessage(handlers []MessageHandler, logger Logger, msg *Message, receiverIDs []string) {
	for _, handler := range handlers {
		func() {
			defer recoverHandlerPanic(logger, "message handler",
				PlayerField(msg.SenderID), PhaseField(msg.Phase))
			handler(msg, receiverIDs)
		}()
	}
}
