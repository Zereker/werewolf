package hiddenrole

import (
	"sort"
)

// RoundContext 回合上下文（每个回合重新创建）
// 用于管理回合内各阶段之间共享的临时状态
// 本回合内有效的状态，跨回合自动清零。
type RoundContext struct {
	// Detours 待结算的绕道，先进先出。
	//
	// 此前这里是两个「某个具体角色专属」的字段，每加一个会在死亡时开枪的
	// 角色就要再加两个字段、并在引擎的阶段流转里多一个分支。改成队列后
	// 引擎不认识任何具体角色。
	Detours []Detour

	// Vars 本回合的自定义状态，每回合自动清空，不属于任何玩家。
	//
	// 狼人杀的「今晚刀口」就存在这里。它此前是上面一个
	// 叫 KillTarget 的字段，与另外三张 map 一起，把「某一套规则有哪些
	// 回合状态」写进了内核——换一套规则，那四样一个都用不上。
	//
	// 四格作用域（见 VarScope）：playerState.Vars 跟着玩家走一整局，这里每回合清零，
	// playerState.RoundVars 是「某个玩家在本回合的标记」。
	// 写走 NewSetVarEffect(ScopeRound, ...)，读走 GameView.Var(ScopeRound, ...)。
	Vars map[string]string
}

// Detour 一次待结算的绕道：**为了某个人，去一趟某个阶段**。
//
// 它此前叫 PendingTrigger，文档说的是「一个待结算的死亡技能」。那是狼人杀
// 的说法——猎人被刀之后开枪。而内核认得的从来不是「死亡」也不是「技能」，
// 只是「谁、去哪个阶段」：什么触发了它、他到了那儿要干什么，全是规则的事。
//
// 它管三件事，后两件没有别的机制能替代：
//
//  1. 把阶段引到欠账的地方        —— GOTO_PHASE 也能做
//  2. 排空之前拦住胜负判定与回合边界 —— 绕道可能翻盘（那一枪带走最后一只狼）
//  3. 按队首一条一条来             —— 两个人同一夜欠账，各走各的
//
// 它**不**回答「谁能行动」：进入欠账的阶段时它写一份行动者名单
// （见 gameState.nameDetourActor），之后走与 NewSetActorsEffect 完全相同的
// 那条路。
type Detour struct {
	PlayerID string    // 为谁绕这一趟
	Phase    PhaseType // 绕到哪个阶段
}

// newRoundContext 创建新的回合上下文
func newRoundContext() *RoundContext {
	return &RoundContext{}
}

// playerState 玩家状态
type playerState struct {
	ID    string
	Role  RoleType
	Alive bool

	// Vars 角色私有的、会影响规则判定的状态。
	//
	// 规则把角色私有的状态放在这里：狼人杀的女巫两瓶药、骑士的一次决斗，
	// 都是同一件事。此前内核为内置角色写了专门的 bool 字段，于是第三方
	// 角色改不动自己的状态，也没有任何办法给自己发初始状态——
	// 那正是「加一个角色不该改引擎」要消灭的东西。
	//
	// 初始值由 RoleSetup 发放（见 WithRoleSetup），此后走
	// NewSetVarEffect(ScopeGame.Of(id), ...) 改、GameView.Var 读，随快照走、回放能重建。
	//
	// 需要跨回合的记录也在这里（狼人杀的「守卫上回合守了谁」就是）：
	// 判定由规则自己做，内核只管存。
	Vars map[string]string

	// RoundVars 这名玩家在本回合的标记，每回合自动清空。
	//
	// 今晚谁被守了、谁被救了、谁被毒了都是这一类，此前是 RoundContext
	// 上三张 map[string]bool——第三方角色既改不了也读不到，而「本回合
	// 标记了某人」是任何一套社会推理规则都会用到的形状。
	// 写走 NewSetVarEffect(ScopeRound.Of(id), ...)，读走 GameView.Var。
	RoundVars map[string]string
}

// gameState 游戏状态。
//
// # 并发
//
// 本类型自身不加锁。它是 Engine 的内部状态，不导出、也不出现在任何
// 导出签名里，全部访问都发生在 Engine 持锁期间；Resolver 拿到的是
// 只读的 GameView，同样在锁内构造与使用。
//
// 此前这里有一层自己的 RWMutex，与 Engine 的锁构成嵌套双锁，理由是
// 「State 可以独立使用」——但收进包内之后这个前提不再成立，多出来的
// 一层锁只剩开销与心智负担。
type gameState struct {
	Phase   PhaseType               // 当前阶段
	Round   int                     // 当前回合
	players map[string]*playerState // 玩家状态（私有，通过方法访问）

	// Vars 整局有效、不属于任何玩家的状态。
	//
	// 变量作用域是一张 2x2 的表——时间尺度（整局 / 本回合）乘以有没有主人
	// （无主 / 属于某个玩家）。此前只有三格：
	//
	//	              无主          属于某个玩家
	//	  整局有效     （缺）        playerState.Vars
	//	  本回合有效   RoundCtx.Vars playerState.RoundVars
	//
	// 缺的那一格不是刻意留白，是漏了：狼人杀整局有效的状态恰好都挂在人身上
	// （女巫的药、守卫上回合守了谁），所以一直没人撞到。任务制那一套撞到了——
	// 「第几轮任务」「成功几次」「连续否决几次」「队长轮到谁」四样全是整局
	// 有效且不属于任何玩家，只能挂到某个玩家的私有状态上当账本，
	// 那个玩家的 PlayerView 里于是凭空多出四个与他无关的字段。
	//
	// 写走 NewSetVarEffect(ScopeGame, ...)，读走 GameView.Var / Engine.Var。
	Vars map[string]string

	// Actors 「哪些玩家可以在某个阶段行动」，由规则在运行时指定。
	//
	// 内核判定行动者此前只有一条路：拿 PhaseStep.Role 去比对玩家的角色。
	// 而角色是入座时定死的——任何运行时才选出来的行动者集合都表达不了。
	// 这个抽象已经被逃逸三次：狼人杀的猎人开枪（内核为它开了绕道队列这个
	// 单人特例）、missions 包的队长提名、missions 包的任务队伍。后两处只能让所有人
	// 都提交、再由解析器丢掉不该算的，代价是 AllowedSkills 对没资格的玩家
	// 说谎、PhaseReadiness 等一群不可能行动的人。
	//
	// 现在规则可以直接说：「这几个人，在那个阶段行动」。写走
	// NewSetActorsEffect，内核在 SubmitSkillUse 就拦，不是让规则事后过滤。
	//
	// 按阶段存而不是只存「当前阶段」，是因为集合往往在**更早的阶段**算出来
	// ——missions 包的任务队伍是提名阶段选的，到任务阶段才用。
	// 某个阶段结算完，它的那一份就被消费掉。
	Actors map[PhaseType][]string

	// 回合临时上下文（每个回合重新创建）
	RoundCtx *RoundContext
}

// newState 创建游戏状态
func newState() *gameState {
	return &gameState{
		Phase:    PhaseStart,
		Round:    0,
		players:  make(map[string]*playerState),
		RoundCtx: newRoundContext(),
	}
}

// addPlayer 添加玩家。
//
// 内核只记 ID、角色与存活；阵营、类别这些是规则的分法，由 RoleSetup
// 在入座时作为初始状态发放（见 seatPlayer）。
//
// 返回错误：ID 为空、ID 已存在、角色不能作为玩家身份（如上帝）。
func (s *gameState) addPlayer(id string, role RoleType) error {
	if id == "" {
		return ErrInvalidPlayerID
	}
	// 上帝是系统角色，不是玩家身份
	if role == RoleUnspecified || role == RoleSystem {
		return WrapError(CodeInvalidRole,
			"role %v cannot be assigned to a player", role)
	}

	if _, exists := s.players[id]; exists {
		return WrapError(CodePlayerExists, "player %q already exists", id)
	}

	player := &playerState{
		ID:    id,
		Role:  role,
		Alive: true,
	}

	s.players[id] = player
	return nil
}

// setPlayerVars 批量写入一名玩家的自定义状态，供入座时发放初始状态。
//
// 空值按删除处理，与 EventSetPlayerVar 的写入点保持一致——否则
// 规则写出来的空串会留在快照里。
func (s *gameState) setPlayerVars(id string, vars map[string]string) {
	if len(vars) == 0 {
		return
	}
	p, ok := s.players[id]
	if !ok {
		return
	}
	for k, v := range vars {
		if k == "" {
			continue
		}
		if v == "" {
			delete(p.Vars, k)
			continue
		}
		if p.Vars == nil {
			p.Vars = make(map[string]string, len(vars))
		}
		p.Vars[k] = v
	}
}

// currentPhase 当前阶段（包内使用，自带锁）
func (s *gameState) currentPhase() PhaseType {
	return s.Phase
}

// currentRound 当前回合（包内使用，自带锁）
func (s *gameState) currentRound() int {
	return s.Round
}

// getPlayer 获取玩家（包内使用）
// 返回内部指针，仅限包内代码使用
// 外部请使用 PlayerInfo(id) 获取只读副本
func (s *gameState) getPlayer(id string) (*playerState, bool) {
	p, ok := s.players[id]
	return p, ok
}

// PlayerInfo 玩家信息只读视图（上帝视角）。
//
// 含 Protected 这类只有上帝该知道的信息，不可整体转发给玩家——
// 要发给玩家的内容用 Engine.PlayerView。
type PlayerInfo struct {
	ID    string   `json:"id"`
	Role  RoleType `json:"role"`
	Alive bool     `json:"alive"`

	// RoundVars 这名玩家在本回合的标记，每回合清零。
	//
	// 此前这里是一个叫 Protected 的 bool——「今晚是否被守卫守护」是
	// 狼人杀的概念，内核不该认得。现在它只是规则自己定的一个键，
	// 与其余标记同列。
	RoundVars map[string]string `json:"round_vars,omitempty"`

	// Vars 角色私有的状态，规则自己定键名。
	//
	// 刻意只出现在这里（上帝视角），不出现在面向玩家的 SelfInfo 上：
	// 往里放什么由角色决定，默认把它交给玩家等于让每个角色自己去想
	// 「这一项能不能给他看」——那正是这个库要替调用方收掉的那类判断。
	// 要给玩家看的，由角色经 RoleInfoProvider 显式投射。
	Vars map[string]string `json:"vars,omitempty"`
}

// PlayerInfo 获取玩家信息的只读副本
func (s *gameState) PlayerInfo(id string) (PlayerInfo, bool) {
	p, ok := s.players[id]
	if !ok {
		return PlayerInfo{}, false
	}

	return PlayerInfo{
		ID:        p.ID,
		Role:      p.Role,
		Alive:     p.Alive,
		RoundVars: copyVars(p.RoundVars),
		Vars:      copyVars(p.Vars),
	}, true
}

// getAlivePlayerIDsByRole 获取指定角色的存活玩家ID列表（包内使用）
func (s *gameState) getAlivePlayerIDsByRole(role RoleType) []string {
	result := make([]string, 0)
	for id, p := range s.players {
		if p.Alive && p.Role == role {
			result = append(result, id)
		}
	}
	return result
}

// allPlayerIDs 返回全部玩家ID，按字典序排序（包内使用）。
// 排序是为了让面向玩家的视图输出稳定，不受 map 遍历顺序影响。
func (s *gameState) allPlayerIDs() []string {
	result := make([]string, 0, len(s.players))
	for id := range s.players {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

// getAlivePlayerIDs 获取所有存活玩家ID列表，按字典序排序（包内使用）。
//
// 排序不是可有可无的：这份名单会流进规则产出的效果里（发言受众、
// 结算顺序），而 map 的遍历顺序每次都不一样——不排的话同一个局面
// 两次结算产出的效果流不同，回放与逐字节比对就没了确定性。
func (s *gameState) getAlivePlayerIDs() []string {
	result := make([]string, 0, len(s.players))
	for id, p := range s.players {
		if p.Alive {
			result = append(result, id)
		}
	}
	sort.Strings(result)
	return result
}

// clone 复制一份状态，供 Engine.View 使用。
//
// 视图必须与引擎脱钩：拿到视图之后引擎继续推进，那一份不该跟着变——
// 否则「这一刻的局面」这个说法就不成立了。
func (s *gameState) clone() *gameState {
	out := newState()
	out.Phase = s.Phase
	out.Round = s.Round
	out.Vars = copyVars(s.Vars)
	out.Actors = copyActors(s.Actors)
	out.RoundCtx = &RoundContext{
		Detours: append([]Detour(nil), s.RoundCtx.Detours...),
		Vars:    copyVars(s.RoundCtx.Vars),
	}
	for id, p := range s.players {
		out.players[id] = &playerState{
			ID: p.ID, Role: p.Role, Alive: p.Alive,
			Vars: copyVars(p.Vars), RoundVars: copyVars(p.RoundVars),
		}
	}
	return out
}

// applyEffect 应用效果
// applyEffect 应用一个效果。这是状态的唯一写入点。
//
// 未知的效果类型会被静默忽略——第三方 Resolver 若发出引擎不认识的类型，
// 不会报错也不会改变状态。扩展时请复用已有类型，或让 Resolver 自己把
// 语义拆解成引擎认识的效果。
func (s *gameState) applyEffect(effect *Effect) {
	// 第三方 Resolver 返回的切片里可能混进 nil，不值得为此让整局崩掉
	if effect == nil {
		return
	}

	// 被取消的效果不改变状态，但仍会出现在 EndPhase 的返回值里，
	// 好让调用方知道「某人试了但没成」以及原因
	if effect.Canceled {
		return
	}

	// 确保 RoundCtx 已初始化
	if s.RoundCtx == nil {
		s.RoundCtx = newRoundContext()
	}

	switch effect.Type {
	// —— 以下是内核的全部状态原语 ——
	//
	// 引擎此前还认得 KILL / POISON / ELIMINATE / SHOOT（各种死法）、
	// PROTECT / SAVE（今晚的标记）、SET_NIGHT_KILL / CLEAR_NIGHT_KILL、
	// SET_LAST_PROTECTED、USE_ANTIDOTE / USE_POISON——十来条分支，
	// 每一条都是狼人杀的规则。换一套规则，它们一条都用不上，而新规则
	// 要表达自己的状态变更又只能来改这个 switch。
	//
	// 现在规则自己命名发生了什么（KILL、SHOOT、殉情、决斗），再产出
	// 下面这几个原语之一来真正改状态。两个效果，两件事：前者给受众与
	// 效果流看，后者给状态机看。
	case EventSetAlive:
		if alive, ok := aliveOf(effect); ok {
			if target, found := s.players[effect.TargetID]; found {
				target.Alive = alive
			}
		}

	case EventSetVar:
		// 一项自定义状态，作用域在效果里。值为空即删除，免得快照里堆一堆空串。
		if scope, key, value := varOf(effect); key != "" {
			s.setVar(scope, key, value)
		}

	case EventSetActors:
		// 规则指定某个阶段的行动者。不存在的玩家忽略掉。
		if phase, ids, ok := actorsOf(effect); ok {
			kept := make([]string, 0, len(ids))
			for _, id := range ids {
				if _, exists := s.players[id]; exists {
					kept = append(kept, id)
				}
			}
			s.setActors(phase, kept)
		}

	case EventDetour:
		// 绕道入队，等待流转到对应阶段结算
		if phase, ok := effect.detourPhase(); ok && effect.SourceID != "" {
			s.RoundCtx.Detours = append(s.RoundCtx.Detours,
				Detour{PlayerID: effect.SourceID, Phase: phase})
		}
	}
}

// resetRoundState 重置回合状态（每回合开始时调用）
func (s *gameState) resetRoundState() {
	s.resetRoundStateUnlocked()
}

// resetRoundStateUnlocked 内部方法，不获取锁
func (s *gameState) resetRoundStateUnlocked() {
	s.RoundCtx = newRoundContext()
	// 玩家身上的回合级标记同属本回合，一起清掉——漏掉这一步，
	// 上一夜的「被守」「被毒」会一直累积下去，与回合边界写死成
	// NIGHT_GUARD 那次是同一类错误。
	for _, p := range s.players {
		p.RoundVars = nil
	}
}

// startAt 把状态置到开局：指定阶段、第一回合、干净的回合上下文
func (s *gameState) startAt(phase PhaseType) {
	s.Phase = phase
	s.Round = 1
	s.resetRoundStateUnlocked()
}

// leavePhase 离开当前阶段时，把一次性的东西消费掉。
//
// 两样：这个阶段的行动者名单、以及队首那笔指向这个阶段的绕道欠账。
// 两样都是「用过就作废」——不清的话，下一次进同一个阶段会沿用上一轮的
// 名单，或者同一个人被反复拉回来再用一次技能。
//
// **收成一个函数，是因为它此前散在两条路上，而两条路漂移了三次。**
// 正常推进（endPhaseInternal）与效果流回放（replayEffect）必须做得一模
// 一样，否则回放出来的引擎从下一步起就与原局分叉。三次分叉全是随机对局
// 的不变量抓出来的，每一次都是回放这条路少做了一样：
//
//	少消费行动者名单        —— 回放带着上一个阶段的名单
//	结束那一步少消费名单     —— GAME_ENDED 分支没走 consumeActors
//	结束那一步少消费绕道     —— 同上，没走 consumeDetourFor
//
// 打第三块补丁不如把它收成一处。
func (s *gameState) leavePhase() {
	s.consumeActors(s.Phase)
	s.consumeDetourFor(s.Phase)
}

// nextPhase 切换到下一阶段。
//
// endsRound 由**刚结算完的那个阶段**声明（PhaseConfig.EndsRound）：
// 它为真即这一回合到此为止，回合数加一、回合级状态全部清空。
//
// 这件事此前是内核猜的——「绕回起始阶段就算新回合」。那个猜测对狼人杀
// 成立（夜→昼→夜），对别的规则不成立：任务制那一套每提名一次就绕一圈，
// 于是「回合」成了提名计数器。一局游戏的「一回合」是什么只有规则知道，
// 内核不再替它决定。
func (s *gameState) nextPhase(phase PhaseType, endsRound, clearVars bool) {
	s.Phase = phase
	if endsRound {
		s.Round++
	}
	if clearVars {
		s.resetRoundStateUnlocked()
	}
	s.nameDetourActor()
}

// nameDetourActor 若刚进入的正是队首那笔欠账要去的阶段，把欠账的人写成
// 这个阶段的行动者名单。
//
// 这是「绕道队列」与「规则点名」的接缝。此前它们是两套并行的机制：
// actorsForStep 与 validateSkillUse 各有一个三层判断，第一层问绕道队列、
// 第二层问点名、第三层按角色算。而两条路回答的是同一个问题，实现也几乎
// 逐字相同（triggerActorFor 与 namedActorsFor 都是「点到的人里，谁承担
// 这个角色的步骤」）——一个概念，两份实现，两处都要记得对齐。
//
// 现在队列不再回答「谁能行动」，它只**产出**一份名单，之后一切照点名走。
// 层数从三降到二，triggerActorFor 与 isTriggerActor 一起删掉。
//
// 写在这里而不是 ABILITY_TRIGGERED 的写入点：队列里可以有多条指向同一个
// 阶段的触发（两名猎人同一夜出局），在写入点各写一次会互相覆盖，只剩最后
// 一个人开得了枪。进入阶段时按**队首**取，才是队列本来的语义。
//
// 正常推进与效果流回放共用这一条路径（两者都经 nextPhase），因此不会分叉。
func (s *gameState) nameDetourActor() {
	t, ok := s.peekDetour()
	if !ok || t.Phase != s.Phase {
		return
	}
	s.setActors(s.Phase, []string{t.PlayerID})
}

// lastProtectedTarget 该守卫在**上一回合**守护的目标，无则为空。
//
// 连守判定问的是「上一晚是不是守的同一个人」，而不是「上一次守的是谁」：
// 守卫空守一晚就打断了连续性，被判连守而取消的那一次也从来没生效过。
// 两者都不会写进 LastProtectedRound，因此按回合号一比就都对了。

// varsFor 定位某个作用域对应的那张表，以及它是否存在。
//
// 四种作用域在这里收口：无主的两格挂在 gameState 与 RoundContext 上，
// 有主的两格挂在 playerState 上。取不到（玩家不存在、回合上下文为空）
// 时返回 nil 与一个不可用的写入器。
func (s *gameState) varsFor(scope VarScope) (read map[string]string, write func(map[string]string)) {
	if scope.owner == "" {
		if scope.perRound {
			if s.RoundCtx == nil {
				return nil, nil
			}
			return s.RoundCtx.Vars, func(m map[string]string) { s.RoundCtx.Vars = m }
		}
		return s.Vars, func(m map[string]string) { s.Vars = m }
	}

	p, ok := s.players[scope.owner]
	if !ok {
		return nil, nil
	}
	if scope.perRound {
		return p.RoundVars, func(m map[string]string) { p.RoundVars = m }
	}
	return p.Vars, func(m map[string]string) { p.Vars = m }
}

// varOf 读某个作用域下的一项自定义状态，没有则为空串。
func (s *gameState) varOf(scope VarScope, key string) string {
	vars, _ := s.varsFor(scope)
	return vars[key]
}

// setVar 写某个作用域下的一项自定义状态。空串等同删除——四种作用域同一个口径。
func (s *gameState) setVar(scope VarScope, key, value string) {
	vars, write := s.varsFor(scope)
	if write == nil {
		return
	}
	if value == "" {
		delete(vars, key)
		return
	}
	if vars == nil {
		vars = make(map[string]string, 1)
		write(vars)
	}
	vars[key] = value
}

// peekDetour 查看队首那笔待结算的绕道
func (s *gameState) peekDetour() (Detour, bool) {
	if s.RoundCtx == nil || len(s.RoundCtx.Detours) == 0 {
		return Detour{}, false
	}
	return s.RoundCtx.Detours[0], true
}

// popDetour 弹出队首那笔待结算的绕道
func (s *gameState) popDetour() {
	if s.RoundCtx == nil || len(s.RoundCtx.Detours) == 0 {
		return
	}
	s.RoundCtx.Detours = s.RoundCtx.Detours[1:]
}

// consumeDetourFor 若队首的待结算技能正是 phase，则出队。
//
// 待结算队列是「一次性」的：由死亡结算入队，进入对应阶段后必须出队。
// 不出队的话它会在整个回合内持续非空，同一个玩家会被反复拉回来再用一次技能。
//
// 正常推进（calculateNextPhase）与效果流回放（replayEffect 处理
// PHASE_CHANGED）都要做这一步，且必须做得一模一样，否则回放出来的引擎
// 会带着一条本该消费掉的触发，从下一步起与原引擎分叉。
func (s *gameState) consumeDetourFor(phase PhaseType) {
	if t, ok := s.peekDetour(); ok && t.Phase == phase {
		s.popDetour()
	}
}

// hasPendingDetour 是否还有没结算完的绕道
func (s *gameState) hasPendingDetour() bool {
	_, ok := s.peekDetour()
	return ok
}

// RoundContext 获取回合上下文的只读副本
func (s *gameState) RoundContext() *RoundContext {
	if s.RoundCtx == nil {
		return nil
	}

	// 返回副本以避免外部修改
	return &RoundContext{
		Detours: append([]Detour(nil), s.RoundCtx.Detours...),
		Vars:    copyVars(s.RoundCtx.Vars),
	}
}

// copyActors 深拷一份行动者表。
func copyActors(in map[PhaseType][]string) map[PhaseType][]string {
	if in == nil {
		return nil
	}
	out := make(map[PhaseType][]string, len(in))
	for k, v := range in {
		out[k] = append([]string(nil), v...)
	}
	return out
}

// actorsFor 规则为某个阶段指定的行动者，没指定则为 nil。
//
// nil 与空切片是两件事：nil 是「规则没说，按角色算」，空切片是
// 「规则说了，这个阶段没有人能行动」。
func (s *gameState) actorsFor(phase PhaseType) ([]string, bool) {
	if s.Actors == nil {
		return nil, false
	}
	v, ok := s.Actors[phase]
	return v, ok
}

// setActors 指定某个阶段的行动者。
func (s *gameState) setActors(phase PhaseType, ids []string) {
	if s.Actors == nil {
		s.Actors = map[PhaseType][]string{}
	}
	s.Actors[phase] = sortedStrings(ids)
}

// consumeActors 某个阶段结算完，它的行动者指定就用掉了。
//
// 不清的话，下一次进同一个阶段会沿用上一次的名单——那几乎总是错的：
// 名单是上一轮算出来的。
func (s *gameState) consumeActors(phase PhaseType) {
	delete(s.Actors, phase)
}
