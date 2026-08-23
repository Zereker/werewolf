package hiddenrole

import (
	"sync"
)

type Engine struct {
	mu sync.RWMutex

	config *Config
	state  *gameState
	phase  *phaseManager

	// logger 与 metrics 在构造时定好，此后不再改变，因此可以在锁外读。
	// 它们此前有各自的 setter，于是每一处锁外使用都得先在锁内复制一份；
	// 收进构造选项之后这层防御就没有必要了。
	logger Logger

	// victory 胜负判定。内核的缺省是「永不结束」——它不知道什么叫赢；
	// 规则包用 WithVictoryChecker 装上自己的那一套。
	victory VictoryChecker

	// roleInfo 各角色的专属信息提供者。内置的与第三方注册的同在一张表里，
	// 读取路径也是同一条——内置角色在这件事上没有特权。
	roleInfo map[RoleType]RoleInfoProvider

	// roleSetup 各角色的初始状态。同上：女巫开局两瓶药与第三方角色
	// 开局带什么，走的是同一张表、同一条写入路径。
	roleSetup map[RoleType]RoleSetup

	// gameSetup 开局那一刻的初始化。与 roleSetup 是一对：那个管一名玩家
	// 入座时带着什么，这个管整局开始时的局面——初始化整局计数器，
	// 以及指定第一个阶段的行动者（后者是它存在的直接原因：行动者集合通常
	// 由上一个阶段的解析器算出来，而第一个阶段前面没有阶段）。
	gameSetup GameSetup

	// 信息边界的三个问题，全部由规则回答（见 boundary.go）：
	// 一件事该告诉谁、谁和谁是一边的、发言谁能听到。内核只保证
	// 自己的状态原语永远不外发。
	audience  AudienceProvider
	teammates TeammateProvider
	speech    SpeechProvider

	// winner 分出胜负时的赢家，未分出时为 CampUnspecified。
	//
	// 记下来而不是每次现算：判定器是可替换的，「这局谁赢了」应该是
	// 结束那一刻定下的事实，不该因为之后有人换了判定器就变了。
	winner Camp

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
// 造出来的是一台**什么都不认识**的状态机：没有解析器、没有胜负判定、
// 没有受众划分。规则全部经 opts 传入——狼人杀的那一整套见 werewolf.New，
// 它自己也只是这么组装的，没有走任何后门。
//
// config 是必需的：内核没有默认板子可给。配置会先经 Config.Validate
// 校验——阶段流转图是使用者可替换的数据，悬空的 NextPhase 会让游戏推进到
// 一半静默结束，这类问题必须在构造时暴露。
func NewEngine(config *Config, opts ...EngineOption) (*Engine, error) {
	if config == nil {
		return nil, WrapError(CodeInvalidConfig, "config must not be nil")
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}

	e := &Engine{
		config:          config,
		state:           newState(),
		phase:           newPhaseManager(config),
		logger:          newNopLogger(),
		victory:         neverEnds{},
		roleInfo:        make(map[RoleType]RoleInfoProvider, 4),
		roleSetup:       make(map[RoleType]RoleSetup, 8),
		pendingUses:     make([]*SkillUse, 0),
		effectLog:       make([]*Effect, 0),
		eventHandlers:   make([]EventHandler, 0),
		messageHandlers: make([]MessageHandler, 0),
	}
	if err := e.applyOptions(opts); err != nil {
		return nil, err
	}
	return e, nil
}

// MustNewEngine 同 NewEngine，配置不合法时 panic。
//
// 适用于配置是编译期常量的场合（示例、测试、写死默认配置的服务启动路径）。
func MustNewEngine(config *Config, opts ...EngineOption) *Engine {
	engine, err := NewEngine(config, opts...)
	if err != nil {
		panic("werewolf: invalid game config: " + err.Error())
	}
	return engine
}

// AddPlayer 让一名玩家入座。
//
// 只能在 Start 之前调用。返回错误：游戏已开始、ID 为空、ID 已存在、
// 角色不能作为玩家身份。
//
// 阵营、角色类别这些**不是参数**：它们是规则的分法，由该角色的
// RoleSetup 在入座时作为初始状态发放（见 WithRoleSetup）。这里此前
// 还有一个多两个参数的重载，专供扩展角色显式给出阵营与类别——
// 于是「这个角色属于哪一边」这件事的答案，取决于调用方在每一处入座
// 时记得填对，而不是写在角色自己身上。
func (e *Engine) AddPlayer(id string, role RoleType) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 开局后再改动玩家会让已发出的身份信息与引擎状态不一致
	if e.state.Phase != PhaseStart {
		return ErrGameAlreadyStarted
	}

	vars := e.setupFor(id, role)
	if err := e.seatPlayer(id, role, vars); err != nil {
		return err
	}
	e.recordEffects(newPlayerAddedEffect(id, role, vars))
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
func (e *Engine) seatPlayer(id string, role RoleType, vars map[string]string) error {
	if err := e.state.addPlayer(id, role); err != nil {
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

	// 校验板子：开局就已分出胜负的局面，与其让它「开局即结束」，
	// 不如在这里直接拒绝。
	//
	// 这一条此前写成「必须有狼人、必须有好人」——那是狼人杀的说法，
	// 内核不认识阵营。改成问胜负判定器：既然它是「这一刻分出胜负了吗」
	// 的唯一权威，开局前问它一次就够了，而且顺带覆盖了原来漏掉的情况
	// （屠城模式下 2 狼对 2 好人，第一次结算即狼人胜）。
	if over, winner := e.victory.CheckVictory(newStateView(e.state)); over {
		return nil, nil, WrapError(CodeInvalidBoard,
			"board is already decided before the game starts: winner=%v", winner)
	}

	// 每个阶段都必须有解析器，否则推进到那里时技能会被静默丢弃。
	// 解析器可以在构造之后注册，故此项校验放在这里而非 NewEngine。
	if err := e.phase.validateResolvers(); err != nil {
		return nil, nil, err
	}

	start := e.config.startPhase()
	e.state.startAt(start)

	effect := newGameStartedEffect(start)
	e.recordEffects(effect)

	// 规则的开局初始化。走与其余效果完全相同的写入点，因此进效果流、
	// 能回放。放在 GAME_STARTED 之后，好让效果流读起来就是事情发生的顺序。
	if e.gameSetup != nil {
		setupEffects, _ := e.applyEffects(e.gameSetup.Setup(newStateView(e.state)))
		e.recordEffects(setupEffects...)
	}
	e.logger.Info("game started", roundField(1), phaseField(start))

	return effect, e.snapshotEventHandlersLocked(), nil
}

// SubmitSkillUse 提交技能使用
func (e *Engine) SubmitSkillUse(use *SkillUse) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 验证技能使用
	if err := e.phase.validateSkillUse(use, e.state); err != nil {
		e.logger.Debug("skill validation failed",
			playerField(use.PlayerID),
			skillField(use.Skill),
			logField("error", err.Error()))
		return err
	}

	// 添加到待处理列表
	use.Phase = e.state.Phase
	use.Round = e.state.Round
	e.pendingUses = append(e.pendingUses, use)

	e.logger.Debug("skill submitted",
		playerField(use.PlayerID),
		skillField(use.Skill),
		targetField(use.Target()))

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

	e.logger.Debug("ending phase", phaseField(currentPhase), roundField(e.state.Round))

	out := phaseOutcome{}

	// 1. 解析技能，产生效果
	if resolver := e.phase.resolver(currentPhase); resolver != nil {
		out.effects = resolver.Resolve(e.pendingUses, newStateView(e.state))
		e.logger.Debug("resolved effects", phaseField(currentPhase), logField("effect_count", len(out.effects)))
	}

	// 2. 应用效果，收集对外可见的事件
	out.effects, out.events = e.applyEffects(out.effects)
	e.recordEffects(out.effects...)

	// 3. 清空待处理列表
	e.pendingUses = nil

	// 4. 计算下一阶段。
	//    绕道可能改变胜负——被刀的猎人开枪带走最后一只狼，好人反而获胜——
	//    因此只要绕道队列还没排空，就推迟胜负判定，先让它走完。
	// 离开这个阶段：行动者名单与队首那笔绕道欠账都用掉了（见 leavePhase）。
	e.state.leavePhase()

	nextPhase := e.calculateNextPhase(currentPhase, out.effects)

	gameOver, winner := e.victory.CheckVictory(newStateView(e.state))
	endNow := gameOver && !e.state.hasPendingDetour()
	if endNow {
		nextPhase = PhaseEnd
	}

	// 5. 流转。END 也走 nextPhase，不直接赋值 Phase——
	//    状态的每一次改动都经同一条路径，别处才不会漏掉伴随的逻辑
	//
	//    回合边界与胜负判定守同一条：**还有待结算的绕道时不能落下**。
	//    理由不同但都硬：胜负是因为绕道可能翻盘；回合边界是因为
	//    待结算队列本身就住在回合上下文里，清掉回合状态等于把队列抹掉,
	//    被投出去的猎人那一枪会凭空消失。
	//    整局结束不算新回合：END 之后没有下一回合，多推一次会让回放对不上
	//    （回放那条路上 GAME_ENDED 走的是 nextPhase(PhaseEnd, false)）。
	//    「回合数 +1」与「回合级变量清空」是两件事，分开算：绝大多数板子
	//    把它们标在同一个阶段（EndsRound 蕴含清空），需要更细的变量寿命时
	//    单独标 ClearsRoundVars——missions 包的队伍标记活到下一次提名，
	//    而回合数要跟着第几轮任务走，两者不重合。
	//    计数看**刚结束的**阶段，清空看**要进入的**阶段——前者说「我结束了
	//    就是一回合」，后者说「我开始时是干净的」。
	settled := !endNow && !e.state.hasPendingDetour()
	e.state.nextPhase(nextPhase,
		settled && e.config.endsRound(currentPhase),
		settled && e.config.clearsRoundVars(nextPhase))

	if endNow {
		// 结束事件与其他事件走同一条构造路径：Effect -> ToEvent，
		// 避免同一个事件有两份分别构造、日后各自漂移的实现。
		//
		// 三条出口都要给到：EndPhase 的返回值、OnEvent 的事件流、效果流。
		// 少了返回值那一条的话，照着 EndPhase -> AudienceOf 路由的调用方
		// 会漏掉整局最重要的一件事——谁赢了。
		endEffect := NewEffect(EventGameEnded, "", "").
			WithData(winnerKey, winner)
		out.effects = append(out.effects, endEffect)
		e.recordEffects(endEffect)
		out.events = append(out.events, endEffect.ToEvent())

		e.winner = winner
		e.logger.Info("game ended", logField("winner", winner.String()))
	} else {
		e.recordEffects(newPhaseChangedEffect(nextPhase))
		e.logger.Debug("phase transition",
			logField("from", currentPhase.String()),
			logField("to", nextPhase.String()))
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

// Status 一眼能看完的局面：走到哪了、结束没有、谁赢了。
//
// 此前这是 Phase / Round / IsGameOver / Winner 四个方法。它们各自取一次
// 读锁，于是**四个答案彼此可以对不上**：宿主要渲染「第 3 回合的白天」
// 得连问两次，中间另一个 goroutine 结算掉一个阶段的话，读到的会是一组
// 从来不曾同时成立的值。合成一次读取之后这件事没有了。
//
// 四项都是标量，不分配内存——「便宜」这条理由（不必像 View 那样克隆整个
// 局面）因此仍然成立，只是不再摊成四个名字。要玩家名单用 AlivePlayerIDs，
// 要一份能反复查询的完整局面用 View。
//
// Winner 是结束那一刻由 VictoryChecker 定下的，此后不再变——之后换掉
// 判定器也不会改写已经结束的这一局。
type Status struct {
	// Phase 当前阶段。
	Phase PhaseType

	// Round 当前回合，从 1 起。
	Round int

	// Over 这一局是否已经结束。
	Over bool

	// Winner 赢家。还没分出胜负时为 CampUnspecified。
	Winner Camp
}

// Status 读一次局面摘要。四项在同一个读锁里取出，因此彼此一致。
func (e *Engine) Status() Status {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return Status{
		Phase:  e.state.Phase,
		Round:  e.state.Round,
		Over:   e.state.Phase == PhaseEnd,
		Winner: e.winner,
	}
}

// View 返回当前局面的只读视图。
//
// 与 Resolver 拿到的是同一种东西。宿主想自己算一次什么（「按我这套判定
// 现在谁赢了」「还有几个神职活着」）时用它，不必把局面一项项读出来再拼。
//
// 视图是取那一刻的值：之后引擎推进不会改动已经拿到的这一份。
func (e *Engine) View() GameView {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return newStateView(e.state.clone())
}

// Apply 直接施加一批效果，绕开阶段结算。
//
// 这是一把有刃的工具，但它是必需的：宿主真的会遇到「玩家掉线判死」
// 「管理员踢人」「后台修正一次误判」这类不属于任何阶段的状态变更，
// 而规则包要单元测试自己的解析器时也需要它。
//
// 它走的仍然是**同一个写入点**：效果进效果流、被否决的不生效、内核的
// 状态原语不外发、其余推给 OnEvent。因此存档、回放、审计都不会因为
// 用了它而失真——这正是它比「伸手改 playerState」强的地方。
//
// 它不做的事：不判胜负、不流转阶段。想让引擎重新算一次胜负，
// 调用 EndPhase。
//
// 返回真正生效的那些效果（nil 会被剔除）。
func (e *Engine) Apply(effects ...*Effect) []*Effect {
	e.mu.Lock()
	kept, events := e.applyEffects(effects)
	e.recordEffects(kept...)
	handlers := e.snapshotEventHandlersLocked()
	e.mu.Unlock()

	for _, event := range events {
		dispatchEvent(handlers, e.logger, event)
	}
	return kept
}

// PlayerInfo 读一名玩家的**上帝视角**信息，不存在时第二个返回值为 false。
//
// 返回的是副本，含 Vars 与 RoundVars——那是给宿主与规则看的，
// **不是**给玩家看的。要发给玩家的那一份用 PlayerView。
func (e *Engine) PlayerInfo(playerID string) (PlayerInfo, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.PlayerInfo(playerID)
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

// Var 读某个作用域下的一项自定义状态，没有则为空串（见 VarScope）。
//
// 规则用它提供自己的便利读法：狼人杀的「今晚刀口」是
// Var(ScopeRound, ...)，missions 包的「第几轮任务」是 Var(ScopeGame, ...)，
// 内核只知道有这么一个键。
func (e *Engine) Var(scope VarScope, key string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.varOf(scope, key)
}

// RoundContext 获取回合上下文的只读副本
func (e *Engine) RoundContext() *RoundContext {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state.RoundContext()
}

// Teammates 这名玩家被告知与他同一边的人，不含自己。
//
// 与 PlayerView.Teammates、PhaseInfo 里的那一份共用同一个
// TeammateProvider——换掉 provider，三处一起变。
func (e *Engine) Teammates(playerID string) []string {
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
		// vetDetour 与日志字段。
		if effect == nil {
			continue
		}
		kept = append(kept, effect)

		e.vetDetour(effect)
		e.state.applyEffect(effect)

		e.logger.Debug("effect applied",
			eventField(effect.Type),
			playerField(effect.SourceID),
			targetField(effect.TargetID),
			logField("canceled", effect.Canceled))

		if !isInternalEvent(effect.Type) {
			events = append(events, effect.ToEvent())
		}
	}

	return kept, events
}

// vetDetour 否决指向未配置阶段的绕道。
//
// 绕道的流转是运行期才成形的一条边：Resolver 产出
// NewDetourEffect 指定去哪个阶段，calculateNextPhase 无条件照办。
// 配置里若没有那个阶段（比如板子有猎人却删掉了猎人阶段），
// 引擎会流转到一个没有配置、没有解析器的阶段，玩家提交什么都不允许，
// 下一次推进直接进 END——游戏在第一夜无声收场，连 GAME_ENDED 都没有。
// Config.Validate 看不见这条边，只能在这里拦。
//
// 调用前需持有 e.mu。
func (e *Engine) vetDetour(effect *Effect) {
	if effect.Canceled || effect.Type != EventDetour {
		return
	}
	phase, ok := effect.detourPhase()
	if !ok {
		effect.Cancel("ability trigger carries no target phase")
		e.logger.Error("ability trigger carries no target phase",
			playerField(effect.SourceID))
		return
	}
	if e.phase.phaseConfig(phase) == nil {
		effect.Cancel("target phase is not present in the game config")
		e.logger.Error("ability trigger points to an unconfigured phase",
			playerField(effect.SourceID), phaseField(phase))
	}
}

// calculateNextPhase 计算下一阶段，处理绕道带来的动态流转。
// 调用前需持有 e.mu。
func (e *Engine) calculateNextPhase(currentPhase PhaseType, effects []*Effect) PhaseType {
	// 离开这个阶段的记账已经由 leavePhase 做过了：队首那笔指向刚结束的
	// 阶段的绕道已经出队，这个阶段的行动者名单也已作废。

	// 还有待结算的绕道，先去处理（可能有多个，逐个来）。
	//
	// 它排在 GOTO_PHASE 前面：队列必须排空——胜负判定与回合边界都等着它，
	// 中途跳走会把还没结算的那笔欠账丢掉。
	if t, ok := e.state.peekDetour(); ok {
		return t.Phase
	}

	// 规则可以改写出口：本阶段产出了 GOTO_PHASE 就听它的。
	// 多条以最后一条为准——与「同一个角色重复注册以最后一次为准」同一个口径。
	if p, ok := e.gotoFrom(effects); ok {
		return p
	}

	// 都没有，走声明式配置里的默认出口
	return e.phase.nextSubPhase(currentPhase)
}

// gotoFrom 从本阶段产出的效果里找出规则指定的下一阶段。
//
// 被否决的效果不算数：规则自己把它 Cancel 掉了，说明那条指令不该生效。
func (e *Engine) gotoFrom(effects []*Effect) (PhaseType, bool) {
	var out PhaseType
	var found bool
	for _, ef := range effects {
		if ef == nil || ef.Canceled || ef.Type != EventGotoPhase {
			continue
		}
		p, ok := ef.gotoPhase()
		if !ok {
			continue
		}
		if e.config.Phases[p] == nil {
			// 写错的目标不该让整局崩掉，但也不能安静地跳去没人预期的地方
			e.logger.Error("goto phase not in config, falling back to NextPhase",
				phaseField(p))
			continue
		}
		out, found = p, true
	}
	return out, found
}
