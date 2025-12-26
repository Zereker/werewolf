package werewolf

import (
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
	state   *State
	phase   *Phase
	logger  Logger
	metrics Metrics

	// 当前阶段收集的技能使用
	pendingUses []*SkillUse

	// 事件通知（可选）
	eventHandlers []EventHandler

	// 消息通知（可选）
	messageHandlers []MessageHandler
}

// NewEngine 创建游戏引擎
func NewEngine(config *GameConfig) *Engine {
	if config == nil {
		config = DefaultGameConfig()
	}

	return &Engine{
		config:          config,
		state:           NewState(),
		phase:           NewPhase(config),
		logger:          NewNopLogger(),
		metrics:         NewNopMetrics(),
		pendingUses:     make([]*SkillUse, 0),
		eventHandlers:   make([]EventHandler, 0),
		messageHandlers: make([]MessageHandler, 0),
	}
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

// AddPlayer 添加玩家
func (e *Engine) AddPlayer(id string, role pb.RoleType, camp pb.Camp) {
	e.state.AddPlayer(id, role, camp)
}

// Start 开始游戏
func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state.Phase != pb.PhaseType_PHASE_TYPE_START {
		return ErrGameNotStarted
	}

	// 进入第一个夜晚（从守卫阶段开始）
	e.state.Phase = pb.PhaseType_PHASE_TYPE_NIGHT_GUARD
	e.state.Round = 1
	e.state.ResetRoundState()

	e.logger.Info("game started", RoundField(1), PhaseField(pb.PhaseType_PHASE_TYPE_NIGHT_GUARD))

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

// nextPhaseFunc 定义计算下一阶段的函数类型
type nextPhaseFunc func(current pb.PhaseType) pb.PhaseType

// endPhaseInternal 结束阶段的公共逻辑
// calcNextPhase: 计算下一阶段的函数
func (e *Engine) endPhaseInternal(calcNextPhase nextPhaseFunc) ([]*Effect, error) {
	// 收集需要发布的事件（在锁外发布，避免死锁）
	var eventsToPublish []*pb.Event
	var gameEndEvent *pb.Event

	// 加锁处理状态变更
	e.mu.Lock()

	currentPhase := e.state.Phase
	currentRound := e.state.Round

	if currentPhase == pb.PhaseType_PHASE_TYPE_END {
		e.mu.Unlock()
		return nil, ErrGameEnded
	}

	e.logger.Debug("ending phase", PhaseField(currentPhase), RoundField(currentRound))

	// 1. 获取当前阶段的解析器
	resolver := e.phase.GetResolver(currentPhase)

	// 2. 解析技能，产生效果
	var effects []*Effect
	if resolver != nil {
		effects = resolver.Resolve(e.pendingUses, e.state, e.config)
		e.logger.Debug("resolved effects", PhaseField(currentPhase), F("effect_count", len(effects)))
	}

	// 3. 应用效果，收集外部事件
	for _, effect := range effects {
		e.state.ApplyEffect(effect)
		// 只发布外部可见事件（内部事件类型 >= 100）
		if effect.Type < 100 {
			eventsToPublish = append(eventsToPublish, effect.ToEvent())
			e.logger.Debug("effect applied",
				EventField(effect.Type),
				PlayerField(effect.SourceID),
				TargetField(effect.TargetID))
			e.metrics.IncEffectApplied(effect.Type)
		}
	}

	// 4. 清空待处理列表
	e.pendingUses = make([]*SkillUse, 0)
	e.metrics.IncPhaseEnded(currentPhase)

	// 5. 检查胜利条件
	if gameOver, winner := e.state.CheckVictory(); gameOver {
		e.state.Phase = pb.PhaseType_PHASE_TYPE_END
		e.logger.Info("game ended", F("winner", winner.String()))
		e.metrics.IncGameEnded(winner)
		gameEndEvent = &pb.Event{
			Type: pb.EventType_EVENT_TYPE_GAME_ENDED,
			Data: map[string]string{"winner": winner.String()},
		}
	} else {
		// 6. 流转到下一阶段
		nextPhase := calcNextPhase(currentPhase)
		e.state.NextPhase(nextPhase)
		e.logger.Debug("phase transition",
			F("from", currentPhase.String()),
			F("to", nextPhase.String()))
	}

	// 释放锁后再发布事件，避免用户回调中调用 Engine 方法导致死锁
	e.mu.Unlock()

	// 发布收集的事件
	for _, event := range eventsToPublish {
		e.publishEvent(event)
	}
	if gameEndEvent != nil {
		e.publishEvent(gameEndEvent)
	}

	return effects, nil
}

// EndPhase 结束当前阶段，解析技能，流转到下一阶段
func (e *Engine) EndPhase() ([]*Effect, error) {
	return e.endPhaseInternal(e.phase.NextSubPhase)
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
	if !ok || !player.Alive {
		return nil
	}

	return e.phase.GetAllowedSkills(e.state.Phase, player.Role)
}

// IsGameOver 游戏是否结束
func (e *Engine) IsGameOver() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.Phase == pb.PhaseType_PHASE_TYPE_END
}

// GetCurrentSubStep 获取当前子步骤
func (e *Engine) GetCurrentSubStep() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.SubStep
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

// GetWolfTeammates 获取狼人队友
func (e *Engine) GetWolfTeammates(playerID string) []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	player, ok := e.state.getPlayer(playerID)
	if !ok || player.Role != pb.RoleType_ROLE_TYPE_WEREWOLF {
		return nil
	}

	return e.state.GetWolfTeammates(playerID)
}

// GetPhaseInfo 获取当前阶段信息（纯状态，不含消息内容）
// 调用方根据此信息决定上帝发送什么消息
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

	// 获取当前阶段的配置
	phaseConfig := e.phase.GetPhaseConfig(e.state.Phase)
	if phaseConfig != nil {
		info.Steps = phaseConfig.Steps
	}

	switch e.state.Phase {
	case pb.PhaseType_PHASE_TYPE_NIGHT_GUARD:
		info.ActiveRoles = []pb.RoleType{pb.RoleType_ROLE_TYPE_GUARD}
		info.RoleInfos[pb.RoleType_ROLE_TYPE_GUARD] = e.buildGuardPhaseInfo()

	case pb.PhaseType_PHASE_TYPE_NIGHT_WOLF:
		info.ActiveRoles = []pb.RoleType{pb.RoleType_ROLE_TYPE_WEREWOLF}
		info.RoleInfos[pb.RoleType_ROLE_TYPE_WEREWOLF] = e.buildWolfPhaseInfo()

	case pb.PhaseType_PHASE_TYPE_NIGHT_WITCH:
		info.ActiveRoles = []pb.RoleType{pb.RoleType_ROLE_TYPE_WITCH}
		info.RoleInfos[pb.RoleType_ROLE_TYPE_WITCH] = e.buildWitchPhaseInfo()

	case pb.PhaseType_PHASE_TYPE_NIGHT_SEER:
		info.ActiveRoles = []pb.RoleType{pb.RoleType_ROLE_TYPE_SEER}
		info.RoleInfos[pb.RoleType_ROLE_TYPE_SEER] = e.buildSeerPhaseInfo()

	case pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE:
		// 结算阶段只有上帝公告，没有玩家行动
		info.ActiveRoles = []pb.RoleType{}

	case pb.PhaseType_PHASE_TYPE_NIGHT_HUNTER, pb.PhaseType_PHASE_TYPE_DAY_HUNTER:
		info.ActiveRoles = []pb.RoleType{pb.RoleType_ROLE_TYPE_HUNTER}
		info.RoleInfos[pb.RoleType_ROLE_TYPE_HUNTER] = e.buildHunterPhaseInfo()

	case pb.PhaseType_PHASE_TYPE_DAY:
		info.ActiveRoles = []pb.RoleType{pb.RoleType_ROLE_TYPE_UNSPECIFIED}
		info.RoleInfos[pb.RoleType_ROLE_TYPE_UNSPECIFIED] = e.buildDayPhaseInfo()

	case pb.PhaseType_PHASE_TYPE_VOTE:
		info.ActiveRoles = []pb.RoleType{pb.RoleType_ROLE_TYPE_UNSPECIFIED}
		info.RoleInfos[pb.RoleType_ROLE_TYPE_UNSPECIFIED] = e.buildVotePhaseInfo()
	}

	return info
}

// buildGuardPhaseInfo 构建守卫阶段信息
func (e *Engine) buildGuardPhaseInfo() *RolePhaseInfo {
	return &RolePhaseInfo{
		PlayerIDs:     e.state.getAlivePlayerIDsByRole(pb.RoleType_ROLE_TYPE_GUARD),
		AllowedSkills: []pb.SkillType{pb.SkillType_SKILL_TYPE_PROTECT},
	}
}

// buildWolfPhaseInfo 构建狼人阶段信息
func (e *Engine) buildWolfPhaseInfo() *RolePhaseInfo {
	playerIDs := e.state.getAlivePlayerIDsByRole(pb.RoleType_ROLE_TYPE_WEREWOLF)
	teammates := make(map[string][]string)
	for _, id := range playerIDs {
		teammates[id] = e.state.GetWolfTeammates(id)
	}
	return &RolePhaseInfo{
		PlayerIDs:     playerIDs,
		AllowedSkills: []pb.SkillType{pb.SkillType_SKILL_TYPE_KILL},
		Teammates:     teammates,
	}
}

// buildWitchPhaseInfo 构建女巫阶段信息
func (e *Engine) buildWitchPhaseInfo() *RolePhaseInfo {
	return &RolePhaseInfo{
		PlayerIDs: e.state.getAlivePlayerIDsByRole(pb.RoleType_ROLE_TYPE_WITCH),
		AllowedSkills: []pb.SkillType{
			pb.SkillType_SKILL_TYPE_ANTIDOTE,
			pb.SkillType_SKILL_TYPE_POISON,
		},
		KillTarget: e.state.RoundCtx.KillTarget,
	}
}

// buildSeerPhaseInfo 构建预言家阶段信息
func (e *Engine) buildSeerPhaseInfo() *RolePhaseInfo {
	return &RolePhaseInfo{
		PlayerIDs:     e.state.getAlivePlayerIDsByRole(pb.RoleType_ROLE_TYPE_SEER),
		AllowedSkills: []pb.SkillType{pb.SkillType_SKILL_TYPE_CHECK},
	}
}

// buildDayPhaseInfo 构建白天阶段信息
func (e *Engine) buildDayPhaseInfo() *RolePhaseInfo {
	return &RolePhaseInfo{
		PlayerIDs:     e.state.getAlivePlayerIDs(),
		AllowedSkills: []pb.SkillType{pb.SkillType_SKILL_TYPE_SPEAK},
	}
}

// buildVotePhaseInfo 构建投票阶段信息
func (e *Engine) buildVotePhaseInfo() *RolePhaseInfo {
	return &RolePhaseInfo{
		PlayerIDs:     e.state.getAlivePlayerIDs(),
		AllowedSkills: []pb.SkillType{pb.SkillType_SKILL_TYPE_VOTE},
	}
}

// buildHunterPhaseInfo 构建猎人阶段信息
func (e *Engine) buildHunterPhaseInfo() *RolePhaseInfo {
	// 获取被触发的猎人ID
	hunterID := e.state.RoundCtx.TriggeredHunterID
	playerIDs := []string{}
	if hunterID != "" {
		playerIDs = []string{hunterID}
	}
	return &RolePhaseInfo{
		PlayerIDs: playerIDs,
		AllowedSkills: []pb.SkillType{
			pb.SkillType_SKILL_TYPE_SHOOT,
			pb.SkillType_SKILL_TYPE_SKIP,
		},
	}
}

// EndSubStep 结束当前子阶段（子步骤模式）
// 与 EndPhase 类似，但使用 calculateNextPhase 支持动态阶段转换（如猎人触发）
func (e *Engine) EndSubStep() ([]*Effect, error) {
	return e.endPhaseInternal(e.calculateNextPhase)
}

// isValidPhase 检查是否是有效的游戏阶段
func (e *Engine) isValidPhase(phase pb.PhaseType) bool {
	return e.phase.GetPhaseConfig(phase) != nil
}

// calculateNextPhase 计算下一阶段（考虑动态触发）
func (e *Engine) calculateNextPhase(currentPhase pb.PhaseType) pb.PhaseType {
	// 夜晚结算阶段后，检查是否有猎人被触发
	if currentPhase == pb.PhaseType_PHASE_TYPE_NIGHT_RESOLVE {
		if e.state.RoundCtx.HunterTriggered {
			return pb.PhaseType_PHASE_TYPE_NIGHT_HUNTER
		}
	}

	// 投票阶段后，检查被投票出局的是否是猎人
	if currentPhase == pb.PhaseType_PHASE_TYPE_VOTE {
		// 检查是否有猎人刚刚死亡（通过 RoundContext 判断）
		if e.state.RoundCtx.HunterTriggered {
			return pb.PhaseType_PHASE_TYPE_DAY_HUNTER
		}
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

// publishEvent 发布事件
// 每个 handler 独立执行，单个 handler panic 不影响其他 handler
func (e *Engine) publishEvent(event *pb.Event) {
	for _, handler := range e.eventHandlers {
		func() {
			defer func() {
				// 捕获 panic，防止单个 handler 影响其他 handler
				_ = recover()
			}()
			handler(event)
		}()
	}
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

	// 复制 handlers 以避免在回调中死锁
	handlers := make([]MessageHandler, len(e.messageHandlers))
	copy(handlers, e.messageHandlers)

	e.mu.RUnlock()

	// 发布消息（锁外执行，避免死锁）
	e.publishMessage(msg, receiverIDs, handlers)

	e.logger.Debug("message sent",
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

// publishMessage 发布消息
func (e *Engine) publishMessage(msg *Message, receiverIDs []string, handlers []MessageHandler) {
	for _, handler := range handlers {
		func() {
			defer func() {
				_ = recover()
			}()
			handler(msg, receiverIDs)
		}()
	}
}
