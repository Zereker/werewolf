package engine

import (
	"sort"
)

// RoundContext 回合上下文（每个回合重新创建）
// 用于管理回合内各阶段之间共享的临时状态
// 本回合内有效的状态，跨回合自动清零。
type RoundContext struct {
	// PendingTriggers 待结算的死亡技能，先进先出。
	//
	// 此前这里是两个「某个具体角色专属」的字段，
	// 每加一个死亡触发角色就要再加两个字段、
	// 并在引擎的阶段流转里多一个分支。改成队列后引擎不认识任何具体角色。
	PendingTriggers []PendingTrigger

	// Vars 本回合的自定义状态，每回合自动清空，不属于任何玩家。
	//
	// 狼人杀的「今晚刀口」就存在这里。它此前是上面一个
	// 叫 KillTarget 的字段，与另外三张 map 一起，把「某一套规则有哪些
	// 回合状态」写进了内核——换一套规则，那四样一个都用不上。
	//
	// 三种作用域：playerState.Vars 跟着玩家走一整局，这里每回合清零，
	// playerState.RoundVars 是「某个玩家在本回合的标记」。
	// 写走 NewSetRoundVarEffect，读走 GameView.RoundVar。
	Vars map[string]string
}

// PendingTrigger 一个待结算的死亡技能
type PendingTrigger struct {
	PlayerID string    // 触发者
	Phase    PhaseType // 该去哪个阶段结算
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
	// EventSetPlayerVar 改、GameView.PlayerVar 读，随快照走、回放能重建。
	//
	// 需要跨回合的记录也在这里（狼人杀的「守卫上回合守了谁」就是）：
	// 判定由规则自己做，内核只管存。
	Vars map[string]string

	// RoundVars 这名玩家在本回合的标记，每回合自动清空。
	//
	// 今晚谁被守了、谁被救了、谁被毒了都是这一类，此前是 RoundContext
	// 上三张 map[string]bool——第三方角色既改不了也读不到，而「本回合
	// 标记了某人」是任何一套社会推理规则都会用到的形状。
	// 写走 NewSetPlayerRoundVarEffect，读走 GameView.PlayerRoundVar。
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
	if role == RoleUnspecified || role == RoleGod {
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

// Var 返回该玩家的一项自定义状态，没有则为空串。
//
// 只是省掉 nil map 的判断——PlayerInfo 是副本，直接读 Vars 也一样。
// 「这名玩家还有没有某件东西」就是 p.Var(key) != ""。
func (p PlayerInfo) Var(key string) string {
	return p.Vars[key]
}

// RoundVar 返回该玩家在本回合的一项标记，没有则为空串。
func (p PlayerInfo) RoundVar(key string) string {
	return p.RoundVars[key]
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
	out.RoundCtx = &RoundContext{
		PendingTriggers: append([]PendingTrigger(nil), s.RoundCtx.PendingTriggers...),
		Vars:            copyVars(s.RoundCtx.Vars),
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

	case EventSetPlayerRoundVar:
		// 某个玩家在本回合的标记，每回合清零。值为空即删除。
		if p, ok := s.players[effect.TargetID]; ok {
			key, value := playerRoundVarOf(effect)
			if key != "" {
				if value == "" {
					delete(p.RoundVars, key)
				} else {
					if p.RoundVars == nil {
						p.RoundVars = make(map[string]string, 1)
					}
					p.RoundVars[key] = value
				}
			}
		}

	case EventSetPlayerVar:
		// 跟着玩家走一整局的状态。值为空即删除，免得快照里堆一堆空串。
		if p, ok := s.players[effect.TargetID]; ok {
			key, value := playerVarOf(effect)
			if key != "" {
				if value == "" {
					delete(p.Vars, key)
				} else {
					if p.Vars == nil {
						p.Vars = make(map[string]string, 1)
					}
					p.Vars[key] = value
				}
			}
		}

	case EventSetRoundVar:
		// 本回合的状态，不属于任何玩家。值为空即删除。
		key, value := roundVarOf(effect)
		if key != "" {
			if value == "" {
				delete(s.RoundCtx.Vars, key)
			} else {
				if s.RoundCtx.Vars == nil {
					s.RoundCtx.Vars = make(map[string]string, 1)
				}
				s.RoundCtx.Vars[key] = value
			}
		}

	case EventAbilityTriggered:
		// 死亡技能入队，等待流转到对应阶段结算
		if phase, ok := effect.triggerPhase(); ok && effect.SourceID != "" {
			s.RoundCtx.PendingTriggers = append(s.RoundCtx.PendingTriggers,
				PendingTrigger{PlayerID: effect.SourceID, Phase: phase})
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

// nextPhase 切换到下一阶段。
//
// roundStart 是本局的起始阶段：绕回它即是新的一回合，此时回合数加一、
// 回合上下文重置。回合边界此前写死成 NIGHT_GUARD，而起始阶段和阶段环
// 都是可配置的——环里不含守卫阶段时，回合数永远停在 1，回合上下文也
// 永远不重置，上一夜的「被救」「被守」「被毒」记录会一直累积下去。
func (s *gameState) nextPhase(phase, roundStart PhaseType) {
	s.Phase = phase
	if phase == roundStart {
		s.Round++
		s.resetRoundStateUnlocked()
	}
}

// lastProtectedTarget 该守卫在**上一回合**守护的目标，无则为空。
//
// 连守判定问的是「上一晚是不是守的同一个人」，而不是「上一次守的是谁」：
// 守卫空守一晚就打断了连续性，被判连守而取消的那一次也从来没生效过。
// 两者都不会写进 LastProtectedRound，因此按回合号一比就都对了。

// roundVar 读本回合的自定义状态，没有则为空串。
func (s *gameState) roundVar(key string) string {
	if s.RoundCtx == nil {
		return ""
	}
	return s.RoundCtx.Vars[key]
}

// playerRoundVar 读某个玩家在本回合的一项标记，没有则为空串。
func (s *gameState) playerRoundVar(playerID, key string) string {
	p, ok := s.players[playerID]
	if !ok {
		return ""
	}
	return p.RoundVars[key]
}

// playerVar 读某个玩家的自定义状态，没有则为空串。
func (s *gameState) playerVar(playerID, key string) string {
	p, ok := s.players[playerID]
	if !ok {
		return ""
	}
	return p.Vars[key]
}

// peekTrigger 查看队首的待结算死亡技能
func (s *gameState) peekTrigger() (PendingTrigger, bool) {
	if s.RoundCtx == nil || len(s.RoundCtx.PendingTriggers) == 0 {
		return PendingTrigger{}, false
	}
	return s.RoundCtx.PendingTriggers[0], true
}

// popTrigger 弹出队首的待结算死亡技能
func (s *gameState) popTrigger() {
	if s.RoundCtx == nil || len(s.RoundCtx.PendingTriggers) == 0 {
		return
	}
	s.RoundCtx.PendingTriggers = s.RoundCtx.PendingTriggers[1:]
}

// consumeTriggerFor 若队首的待结算技能正是 phase，则出队。
//
// 待结算队列是「一次性」的：由死亡结算入队，进入对应阶段后必须出队。
// 不出队的话它会在整个回合内持续非空，同一个玩家会被反复拉回来再用一次技能。
//
// 正常推进（calculateNextPhase）与效果流回放（replayEffect 处理
// PHASE_CHANGED）都要做这一步，且必须做得一模一样，否则回放出来的引擎
// 会带着一条本该消费掉的触发，从下一步起与原引擎分叉。
func (s *gameState) consumeTriggerFor(phase PhaseType) {
	if t, ok := s.peekTrigger(); ok && t.Phase == phase {
		s.popTrigger()
	}
}

// hasPendingTrigger 是否还有未结算的死亡技能
func (s *gameState) hasPendingTrigger() bool {
	_, ok := s.peekTrigger()
	return ok
}

// RoundContext 获取回合上下文的只读副本
func (s *gameState) RoundContext() *RoundContext {
	if s.RoundCtx == nil {
		return nil
	}

	// 返回副本以避免外部修改
	return &RoundContext{
		PendingTriggers: append([]PendingTrigger(nil), s.RoundCtx.PendingTriggers...),
		Vars:            copyVars(s.RoundCtx.Vars),
	}
}
