package werewolf

import (
	"sync"
)

type Engine struct {
	mu sync.RWMutex

	config *GameConfig
	state  *gameState
	phase  *phaseManager

	// logger 与 metrics 在构造时定好，此后不再改变，因此可以在锁外读。
	// 它们此前有各自的 setter，于是每一处锁外使用都得先在锁内复制一份；
	// 收进构造选项之后这层防御就没有必要了。
	logger  Logger
	metrics Metrics

	// victory 胜负判定。默认按 GameConfig.VictoryMode 走内置规则，
	// 可用 WithVictoryChecker 换掉——第三方阵营有自己的胜利条件，
	// 判定写死在引擎里的话那类板子根本没有地方表达。
	victory VictoryChecker

	// roleInfo 各角色的专属信息提供者。内置的与第三方注册的同在一张表里，
	// 读取路径也是同一条——内置角色在这件事上没有特权。
	roleInfo map[RoleType]RoleInfoProvider

	// roleSetup 各角色的初始状态。同上：女巫开局两瓶药与第三方角色
	// 开局带什么，走的是同一张表、同一条写入路径。
	roleSetup map[RoleType]RoleSetup

	// 信息边界的三个问题，全部由规则回答（见 boundary.go）：
	// 一件事该告诉谁、谁和谁是一边的、发言谁能听到。内核只保证
	// 自己的状态原语永远不外发。
	audience  AudienceProvider
	teammates TeammateProvider
	speech    SpeechProvider

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
func NewEngine(config *GameConfig, opts ...EngineOption) (*Engine, error) {
	if config == nil {
		config = DefaultGameConfig()
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}

	e := &Engine{
		config:          config,
		state:           newState(),
		phase:           newPhaseManager(config),
		logger:          NewNopLogger(),
		metrics:         NewNopMetrics(),
		victory:         DefaultVictoryChecker{Mode: config.VictoryMode},
		roleInfo:        make(map[RoleType]RoleInfoProvider, len(builtinRoleInfo)),
		roleSetup:       make(map[RoleType]RoleSetup, len(builtinRoleSetup)),
		audience:        builtinAudience,
		teammates:       builtinTeammates,
		speech:          builtinSpeech,
		pendingUses:     make([]*SkillUse, 0),
		effectLog:       make([]*Effect, 0),
		eventHandlers:   make([]EventHandler, 0),
		messageHandlers: make([]MessageHandler, 0),
	}
	for role, p := range builtinRoleInfo {
		e.roleInfo[role] = p
	}
	for role, su := range builtinRoleSetup {
		e.roleSetup[role] = su
	}
	if err := e.applyOptions(opts); err != nil {
		return nil, err
	}
	return e, nil
}

// MustNewEngine 同 NewEngine，配置不合法时 panic。
//
// 适用于配置是编译期常量的场合（示例、测试、写死默认配置的服务启动路径）。
func MustNewEngine(config *GameConfig, opts ...EngineOption) *Engine {
	engine, err := NewEngine(config, opts...)
	if err != nil {
		panic("werewolf: invalid game config: " + err.Error())
	}
	return engine
}

// addPlayer 添加玩家。阵营与角色类别由角色推导。
//
// 只能在 Start 之前调用。返回错误：游戏已开始、ID 为空、ID 已存在、
// 角色不能作为玩家身份。
func (e *Engine) AddPlayer(id string, role RoleType) error {
	return e.AddCustomPlayer(id, role, CampOf(role), CategoryOf(role))
}

// addCustomPlayer 添加玩家并显式指定阵营与角色类别，供扩展角色使用。
func (e *Engine) AddCustomPlayer(id string, role RoleType, camp Camp, category RoleCategory) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 开局后再改动玩家会让已发出的身份信息与引擎状态不一致
	if e.state.Phase != PhaseStart {
		return ErrGameAlreadyStarted
	}

	vars := e.setupFor(id, role)
	if err := e.seatPlayer(id, role, camp, category, vars); err != nil {
		return err
	}
	e.effectLog = append(e.effectLog, newPlayerAddedEffect(id, role, camp, category, vars))
	return nil
}

// seatPlayer 让一名玩家带着给定的初始状态入座。调用前需持有 e.mu。
//
// 正常入座与回放入座共用这一条路径，区别只在 vars 从哪儿来：
// 正常入座问 RoleSetup，回放读效果流里记着的那一份。
//
// 初始状态记进效果流、而不是在回放时重新问一遍 RoleSetup，是刻意的：
// 「女巫带着两瓶药入座」本来就是发生过的事，效果流记的就是这个。
// 重新问的话，回放方少传一个 WithRoleSetup，重建出来的角色就悄悄
// 空着手——解析器漏传有 validateResolvers 拦，这里拦不住，因为
// 「这个角色没有初始状态」与「你忘了传」在签名上无法区分。
func (e *Engine) seatPlayer(id string, role RoleType, camp Camp, category RoleCategory, vars map[string]string) error {
	if err := e.state.addCustomPlayer(id, role, camp, category); err != nil {
		return err
	}
	e.state.setPlayerVars(id, vars)
	return nil
}

// Start 开始游戏。
//
// 开局事件会推给 OnEvent 的订阅者，与其他事件走同一条通道。
func (e *Engine) Start() error {
	effect, handlers, err := e.startLocked()
	if err != nil {
		return err
	}

	// 分发在锁外：回调里可能回调 Engine
	dispatchEvent(handlers, e.logger, effect.ToEvent())
	return nil
}

// startLocked 在锁内完成开局，返回需要在锁外发布的内容。
func (e *Engine) startLocked() (*Effect, []EventHandler, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state.Phase != PhaseStart {
		return nil, nil, ErrGameAlreadyStarted
	}

	// 校验板子：缺任一阵营的局面从第一次结算起就已分出胜负，
	// 与其让它「开局即结束」，不如在这里直接拒绝
	good, evil := e.state.countCamps()
	if evil == 0 {
		return nil, nil, ErrNoWerewolf
	}
	if good == 0 {
		return nil, nil, ErrNoGoodPlayer
	}

	// 每个阶段都必须有解析器，否则推进到那里时技能会被静默丢弃。
	// 解析器可以在构造之后注册，故此项校验放在这里而非 NewEngine。
	if err := e.phase.validateResolvers(); err != nil {
		return nil, nil, err
	}

	start := e.config.startPhase()
	e.state.startAt(start)

	effect := newGameStartedEffect(start)
	e.effectLog = append(e.effectLog, effect)
	e.logger.Info("game started", RoundField(1), PhaseField(start))

	return effect, e.snapshotEventHandlersLocked(), nil
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
	events   []*Event       // 需要对外发布的事件
	handlers []EventHandler // 锁内快照的处理器
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
		dispatchEvent(out.handlers, e.logger, event)
	}

	return out.effects, nil
}

// advancePhase 在锁内完成一次阶段推进，返回需要在锁外发布的内容。
func (e *Engine) advancePhase() (phaseOutcome, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	currentPhase := e.state.Phase
	if currentPhase == PhaseEnd {
		return phaseOutcome{}, ErrGameEnded
	}
	// 未开局就推进会绕过 Start 的全部前置校验——板子里有没有狼、
	// 每个阶段有没有解析器——而 Start 此后永远返回「已开始」，
	// 那些校验再也跑不到
	if currentPhase == PhaseStart {
		return phaseOutcome{}, ErrGameNotStarted
	}

	e.logger.Debug("ending phase", PhaseField(currentPhase), RoundField(e.state.Round))

	out := phaseOutcome{}

	// 1. 解析技能，产生效果
	if resolver := e.phase.resolver(currentPhase); resolver != nil {
		out.effects = resolver.Resolve(e.pendingUses, newStateView(e.state), e.config)
		e.logger.Debug("resolved effects", PhaseField(currentPhase), F("effect_count", len(out.effects)))
	}

	// 2. 应用效果，收集对外可见的事件
	out.effects, out.events = e.applyEffects(out.effects)
	e.effectLog = append(e.effectLog, out.effects...)

	// 3. 清空待处理列表
	e.pendingUses = nil
	e.metrics.IncPhaseEnded(currentPhase)

	// 4. 计算下一阶段。
	//    死亡技能可能改变胜负——被刀的猎人开枪带走最后一只狼，好人反而获胜——
	//    因此只要还有待结算的死亡技能，就推迟胜负判定，先让它结算完。
	nextPhase := e.calculateNextPhase(currentPhase)

	gameOver, winner := e.victory.CheckVictory(newStateView(e.state))
	endNow := gameOver && !e.state.hasPendingTrigger()
	if endNow {
		nextPhase = PhaseEnd
	}

	// 5. 流转。END 也走 nextPhase，不直接赋值 Phase——
	//    状态的每一次改动都经同一条路径，别处才不会漏掉伴随的逻辑
	e.state.nextPhase(nextPhase, e.config.startPhase())

	if endNow {
		// 结束事件与其他事件走同一条构造路径：Effect -> ToEvent，
		// 避免同一个事件有两份分别构造、日后各自漂移的实现。
		//
		// 三条出口都要给到：EndPhase 的返回值、OnEvent 的事件流、效果流。
		// 少了返回值那一条的话，照着 EndPhase -> AudienceOf 路由的调用方
		// 会漏掉整局最重要的一件事——谁赢了。
		endEffect := NewEffect(EventGameEnded, "", "").
			WithData("winner", winner)
		out.effects = append(out.effects, endEffect)
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

	// 在锁内快照 handler：回调要在锁外执行，
	// 而在锁外读取 e.eventHandlers 会与 OnEvent 竞争
	out.handlers = e.snapshotEventHandlersLocked()

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
func (e *Engine) Phase() PhaseType {
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

// AllowedSkills 该玩家此刻能提交的技能，为空即「还没轮到他」。
//
// 与 PlayerView(id).AllowedSkills 走同一条路径，也与 SubmitSkillUse
// 的校验一致：三者答案不同的话，调用方按其中一个组织流程，
// 玩家的提交就会被另一个拒掉。
func (e *Engine) AllowedSkills(playerID string) []SkillType {
	e.mu.RLock()
	defer e.mu.RUnlock()

	info, ok := e.state.PlayerInfo(playerID)
	if !ok {
		return nil
	}
	return e.allowedSkillsForPlayer(playerID, info)
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
	return e.state.Phase == PhaseEnd
}

// NightKillTarget 获取当晚被狼人击杀的目标（女巫可查询）。
//
// 这是狼人杀规则包提供的便利读法，内核只知道有一个叫
// RoundVarKillTarget 的回合变量，不知道它是什么意思。
func (e *Engine) NightKillTarget() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.roundVar(RoundVarKillTarget)
}

// RoundContext 获取回合上下文的只读副本
func (e *Engine) RoundContext() *RoundContext {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.RoundContext()
}

// WolfTeammates 获取狼队队友（不含自己），非狼队成员返回 nil。
//
// 这是狼人杀规则包的便利读法，与 PlayerView.Teammates、PhaseInfo 里的
// 那一份共用同一个 TeammateProvider——换掉 provider，三处一起变。
func (e *Engine) WolfTeammates(playerID string) []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.teammatesOf(playerID)
}

// applyEffects 逐个应用效果，返回清理后的效果与需要对外发布的事件。
// 调用前需持有 e.mu。
func (e *Engine) applyEffects(effects []*Effect) ([]*Effect, []*Event) {
	kept := make([]*Effect, 0, len(effects))
	events := make([]*Event, 0, len(effects))

	for _, effect := range effects {
		// 第三方 Resolver 返回的切片里可能混进 nil，就地剔除而不是让整局崩掉。
		// 这道判断得在最前面：applyEffect 内部那道 nil 保护够不着下面的
		// vetTrigger 与日志字段。
		if effect == nil {
			continue
		}
		kept = append(kept, effect)

		e.vetTrigger(effect)
		e.state.applyEffect(effect)

		e.logger.Debug("effect applied",
			EventField(effect.Type),
			PlayerField(effect.SourceID),
			TargetField(effect.TargetID),
			F("canceled", effect.Canceled))
		e.metrics.IncEffectApplied(effect.Type)

		if !isInternalEvent(effect.Type) {
			events = append(events, effect.ToEvent())
		}
	}

	return kept, events
}

// vetTrigger 否决指向未配置阶段的死亡技能触发。
//
// 死亡技能的流转是运行期才成形的一条边：Resolver 产出
// NewAbilityTriggerEffect 指定去哪个阶段，calculateNextPhase 无条件照办。
// 配置里若没有那个阶段（比如板子有猎人却删掉了猎人阶段），
// 引擎会流转到一个没有配置、没有解析器的阶段，玩家提交什么都不允许，
// 下一次推进直接进 END——游戏在第一夜无声收场，连 GAME_ENDED 都没有。
// GameConfig.Validate 看不见这条边，只能在这里拦。
//
// 调用前需持有 e.mu。
func (e *Engine) vetTrigger(effect *Effect) {
	if effect.Canceled || effect.Type != EventAbilityTriggered {
		return
	}
	phase, ok := effect.triggerPhase()
	if !ok {
		effect.Cancel("ability trigger carries no target phase")
		e.logger.Error("ability trigger carries no target phase",
			PlayerField(effect.SourceID))
		return
	}
	if e.phase.phaseConfig(phase) == nil {
		effect.Cancel("target phase is not present in the game config")
		e.logger.Error("ability trigger points to an unconfigured phase",
			PlayerField(effect.SourceID), PhaseField(phase))
	}
}

// calculateNextPhase 计算下一阶段，处理死亡技能带来的动态流转。
// 调用前需持有 e.mu。
func (e *Engine) calculateNextPhase(currentPhase PhaseType) PhaseType {
	// 刚结束的正是队首触发要求的阶段，说明该技能已结算，出队
	e.state.consumeTriggerFor(currentPhase)

	// 还有待结算的死亡技能，先去处理（可能有多个，逐个来）
	if t, ok := e.state.peekTrigger(); ok {
		return t.Phase
	}

	// 使用声明式配置获取下一阶段
	return e.phase.nextSubPhase(currentPhase)
}
