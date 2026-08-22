package werewolf

import (
	"sort"
)

// RoundContext 回合上下文（每个回合重新创建）
// 用于管理回合内各阶段之间共享的临时状态
// 包含夜晚和白天的相关状态（如猎人触发可能发生在投票阶段）
type RoundContext struct {
	KillTarget       string          // 狼人击杀目标（女巫可查询）
	ProtectedPlayers map[string]bool // 被守卫保护的玩家
	SavedPlayers     map[string]bool // 被女巫救的玩家
	PoisonedPlayers  map[string]bool // 被女巫毒的玩家

	// PendingTriggers 待结算的死亡技能，先进先出。
	//
	// 此前这里是 HunterTriggered / TriggeredHunterID 两个猎人专属字段，
	// 每加一个死亡触发角色（狼王、白痴）就要再加两个字段、
	// 并在引擎的阶段流转里多一个分支。改成队列后引擎不认识任何具体角色。
	PendingTriggers []PendingTrigger

	// Vars 第三方角色的回合级自定义状态，每回合自动清空。
	//
	// 上面那四个字段——刀口、被守、被救、被毒——与这里是同一件事：
	// 「本回合有效、会影响规则判定的状态」。引擎为内置角色把它们写成了
	// 字段，第三方改不了，于是回合级的扩展状态无处可放。
	//
	// PendingTriggers 的注释里已经记着同一个教训（猎人专属字段改成队列），
	// 但当时只泛化了死亡触发那一项。这里补上其余。
	//
	// 与 PlayerState.Vars 的分工：那个跟着玩家走一整局，这个每回合清零。
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
	return &RoundContext{
		ProtectedPlayers: make(map[string]bool),
		SavedPlayers:     make(map[string]bool),
		PoisonedPlayers:  make(map[string]bool),
	}
}

// IsProtected 检查玩家是否被保护
func (rc *RoundContext) IsProtected(playerID string) bool {
	if rc == nil {
		return false
	}
	return rc.ProtectedPlayers[playerID]
}

// IsSaved 检查玩家是否被救
func (rc *RoundContext) IsSaved(playerID string) bool {
	if rc == nil {
		return false
	}
	return rc.SavedPlayers[playerID]
}

// IsPoisoned 检查玩家是否被毒
func (rc *RoundContext) IsPoisoned(playerID string) bool {
	if rc == nil {
		return false
	}
	return rc.PoisonedPlayers[playerID]
}

// RoleCategory 角色类别
//
// 屠边判定需要区分「神职」与「平民」，而 Camp 只有好人/狼人两值，
// 表达不了这个维度，故单列一个类别。
type RoleCategory int

const (
	RoleCategoryUnknown  RoleCategory = iota // 未知（上帝等系统角色）
	RoleCategoryWolf                         // 狼人阵营
	RoleCategoryGod                          // 神职：预言家、女巫、猎人、守卫
	RoleCategoryVillager                     // 平民
)

// String 实现 fmt.Stringer
func (c RoleCategory) String() string {
	switch c {
	case RoleCategoryWolf:
		return "WOLF"
	case RoleCategoryGod:
		return "GOD"
	case RoleCategoryVillager:
		return "VILLAGER"
	default:
		return "UNKNOWN"
	}
}

// CategoryOf 由角色推导默认类别。
//
// 只覆盖内置的六个角色。RoleType 的底层是 int32，调用方可以用
// 超出内置枚举的取值来定义自己的角色（建议从 1000 起，避免与后续
// 内置角色撞号）；这类角色会落到 Unknown，需通过 AddCustomPlayer
// 显式给出阵营与类别，否则不参与屠边判定。
func CategoryOf(role RoleType) RoleCategory {
	switch role {
	case RoleWerewolf:
		return RoleCategoryWolf
	case RoleSeer,
		RoleWitch,
		RoleHunter,
		RoleGuard:
		return RoleCategoryGod
	case RoleVillager:
		return RoleCategoryVillager
	default:
		return RoleCategoryUnknown
	}
}

// PlayerState 玩家状态
type PlayerState struct {
	ID       string
	Role     RoleType
	Camp     Camp
	Category RoleCategory // 角色类别（神职/平民/狼人），用于屠边判定
	Alive    bool

	// 女巫药剂状态
	HasAntidote bool // 是否有解药
	HasPoison   bool // 是否有毒药

	// 守卫连续保护限制。
	//
	// 只记「哪一回合守了谁」，不记「最后一次成功守护的目标」——
	// 后者不会因为守卫弃权而失效，一旦命中就把那个目标永久锁死。
	// 是否构成连守由 gameState.lastProtectedTarget 按回合号判定。
	LastProtectedTarget string // 最近一次生效守护的目标
	LastProtectedRound  int    // 那次守护发生在第几回合，0 表示从未守护

	// Vars 第三方角色的自定义状态。
	//
	// 上面那几个字段——女巫的药、守卫的守护记录——本质上是同一件事：
	// 「某个角色私有的、会影响规则判定的状态」。引擎为内置角色把它们
	// 写成了字段，为它们各写了一条 applyEffect 分支；而这两处第三方
	// 都改不了，于是自定义角色只能把状态藏在自己的 Resolver 里，
	// 也就是只能违反「状态变更一律经由 Effect」这条不变量。
	//
	// Vars 把那个口子开出来：走 EventSetPlayerVar 写，随快照走，
	// 回放能重建。内置的那几个字段保留——它们是默认板子，
	// p.HasAntidote 比 p.Vars["antidote"] 好读得多，而且要出现在
	// 面向玩家的 SelfInfo 上。
	Vars map[string]string
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
	players map[string]*PlayerState // 玩家状态（私有，通过方法访问）

	// 回合临时上下文（每个回合重新创建）
	RoundCtx *RoundContext
}

// newState 创建游戏状态
func newState() *gameState {
	return &gameState{
		Phase:    PhaseStart,
		Round:    0,
		players:  make(map[string]*PlayerState),
		RoundCtx: newRoundContext(),
	}
}

// CampOf 由角色推导阵营。
//
// 内置的六个角色中只有狼人属于狼人阵营。扩展角色（隐狼、狼美人等）
// 阵营与角色的对应关系不同，需用 AddCustomPlayer 显式指定。
func CampOf(role RoleType) Camp {
	if role == RoleWerewolf {
		return CampEvil
	}
	return CampGood
}

// addPlayer 添加玩家。阵营与角色类别由角色推导。
//
// 返回错误：ID 为空、ID 已存在、角色不能作为玩家身份（如上帝）。
func (s *gameState) addPlayer(id string, role RoleType) error {
	return s.addCustomPlayer(id, role, CampOf(role), CategoryOf(role))
}

// addCustomPlayer 添加玩家并显式指定阵营与角色类别。
//
// 供扩展角色使用：隐狼是好人牌面的狼、白痴是不参与屠边的好人，
// 这类角色无法从内置映射推导，需要调用方直接给出。
func (s *gameState) addCustomPlayer(id string, role RoleType, camp Camp, category RoleCategory) error {
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

	player := &PlayerState{
		ID:       id,
		Role:     role,
		Camp:     camp,
		Category: category,
		Alive:    true,
	}

	// 女巫初始有解药和毒药各一瓶
	if role == RoleWitch {
		player.HasAntidote = true
		player.HasPoison = true
	}

	s.players[id] = player
	return nil
}

// countCamps 统计各阵营存活人数（包内使用）
func (s *gameState) countCamps() (good, evil int) {
	for _, p := range s.players {
		if !p.Alive {
			continue
		}
		switch p.Camp {
		case CampGood:
			good++
		case CampEvil:
			evil++
		}
	}
	return good, evil
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
func (s *gameState) getPlayer(id string) (*PlayerState, bool) {
	p, ok := s.players[id]
	return p, ok
}

// PlayerInfo 玩家信息只读视图（上帝视角）。
//
// 含 Protected 这类只有上帝该知道的信息，不可整体转发给玩家——
// 要发给玩家的内容用 Engine.PlayerView。
type PlayerInfo struct {
	ID          string       `json:"id"`
	Role        RoleType     `json:"role"`
	Camp        Camp         `json:"camp"`
	Category    RoleCategory `json:"category"`
	Alive       bool         `json:"alive"`
	Protected   bool         `json:"protected"` // 今晚是否被保护（取自本回合上下文）
	HasAntidote bool         `json:"has_antidote"`
	HasPoison   bool         `json:"has_poison"`

	// Vars 第三方角色的自定义状态。
	//
	// 刻意只出现在这里（上帝视角），不出现在面向玩家的 SelfInfo 上：
	// 扩展往里放什么由扩展决定，默认把它交给玩家等于让每个扩展
	// 自己去想「这一项能不能给他看」——那正是这个库要替调用方
	// 收掉的那类判断。要给玩家看的，由扩展自己推。
	Vars map[string]string `json:"vars,omitempty"`
}

// PlayerInfo 获取玩家信息的只读副本
func (s *gameState) PlayerInfo(id string) (PlayerInfo, bool) {
	p, ok := s.players[id]
	if !ok {
		return PlayerInfo{}, false
	}

	return PlayerInfo{
		ID:          p.ID,
		Role:        p.Role,
		Camp:        p.Camp,
		Category:    p.Category,
		Alive:       p.Alive,
		Protected:   s.RoundCtx.IsProtected(id), // 从 RoundContext 获取
		HasAntidote: p.HasAntidote,
		HasPoison:   p.HasPoison,
		Vars:        copyVars(p.Vars),
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

// getAlivePlayerIDs 获取所有存活玩家ID列表（包内使用）
func (s *gameState) getAlivePlayerIDs() []string {
	result := make([]string, 0)
	for id, p := range s.players {
		if p.Alive {
			result = append(result, id)
		}
	}
	return result
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
	// 各种死亡：狼刀、毒杀、放逐、开枪
	case EventKill,
		EventPoison,
		EventEliminate,
		EventShoot:
		if target, ok := s.players[effect.TargetID]; ok {
			target.Alive = false
		}

	case EventProtect:
		if _, ok := s.players[effect.TargetID]; ok {
			s.RoundCtx.ProtectedPlayers[effect.TargetID] = true
		}

	case EventSave:
		// 只记录「被救过」，不改存活状态。
		// 死亡统一在夜晚结算阶段发生，此刻目标还活着；
		// 若在这里置 Alive=true，就成了一个能让任意玩家复活的原语。
		if _, ok := s.players[effect.TargetID]; ok {
			s.RoundCtx.SavedPlayers[effect.TargetID] = true
		}

	// 内部状态变更
	case EventSetNightKill:
		s.RoundCtx.KillTarget = effect.TargetID
	case EventClearNightKill:
		s.RoundCtx.KillTarget = ""
	case EventSetLastProtected:
		if guard, ok := s.players[effect.SourceID]; ok {
			guard.LastProtectedTarget = effect.TargetID
			guard.LastProtectedRound = s.Round
		}
	// 药剂与守护记录不按角色设限：这里是状态的写入点，谁有资格用药
	// 是规则问题，由 Resolver 判定（内置的 WitchResolver 会查 Role）。
	// 在这里再写死一遍角色，等于第三方的「女巫类」角色改不动自己的状态。
	case EventUseAntidote:
		if witch, ok := s.players[effect.SourceID]; ok {
			witch.HasAntidote = false
		}
	case EventUsePoison:
		if witch, ok := s.players[effect.SourceID]; ok {
			witch.HasPoison = false
			s.RoundCtx.PoisonedPlayers[effect.TargetID] = true
		}
	case EventSetPlayerVar:
		// 第三方角色的自定义状态。值为空即删除，免得快照里堆一堆空串。
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
		// 回合级的自定义状态。值为空即删除。
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
	// 创建新的回合上下文
	s.RoundCtx = newRoundContext()
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

// getWolfTeammates 获取狼队队友（不包括自己），按 ID 排序。
//
// 按阵营而不是按角色判定：狼王、白狼王、狼美人这些角色经
// AddCustomPlayer 加进来时 Camp 是 EVIL、Role 却不是 WEREWOLF，
// 按角色判会让他们既看不到队友、也不被真狼看到，等于自定义狼队角色
// 实际不可用。狼队内部视野不对称的变体（如某些板子的隐狼）需要调用方
// 自行过滤，引擎给的是「同阵营即队友」这个默认。
//
// 非狼队成员返回空列表。
func (s *gameState) getWolfTeammates(playerID string) []string {
	player, ok := s.players[playerID]
	if !ok || player.Camp != CampEvil {
		return []string{}
	}

	result := make([]string, 0)
	for _, p := range s.players {
		if p.Camp == CampEvil && p.ID != playerID {
			result = append(result, p.ID)
		}
	}
	return sortedStrings(result)
}

// alivePlayerIDsByCamp 指定阵营的存活玩家 ID，按 ID 排序（包内使用）
func (s *gameState) alivePlayerIDsByCamp(camp Camp) []string {
	result := make([]string, 0)
	for id, p := range s.players {
		if p.Alive && p.Camp == camp {
			result = append(result, id)
		}
	}
	return sortedStrings(result)
}

// lastProtectedTarget 该守卫在**上一回合**守护的目标，无则为空。
//
// 连守判定问的是「上一晚是不是守的同一个人」，而不是「上一次守的是谁」：
// 守卫空守一晚就打断了连续性，被判连守而取消的那一次也从来没生效过。
// 两者都不会写进 LastProtectedRound，因此按回合号一比就都对了。
func (s *gameState) lastProtectedTarget(guardID string) string {
	p, ok := s.players[guardID]
	if !ok || p.LastProtectedRound != s.Round-1 {
		return ""
	}
	return p.LastProtectedTarget
}

// roundVar 读本回合的自定义状态，没有则为空串。
func (s *gameState) roundVar(key string) string {
	if s.RoundCtx == nil {
		return ""
	}
	return s.RoundCtx.Vars[key]
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
		KillTarget:       s.RoundCtx.KillTarget,
		ProtectedPlayers: copyStringBoolMap(s.RoundCtx.ProtectedPlayers),
		SavedPlayers:     copyStringBoolMap(s.RoundCtx.SavedPlayers),
		PoisonedPlayers:  copyStringBoolMap(s.RoundCtx.PoisonedPlayers),
		PendingTriggers:  append([]PendingTrigger(nil), s.RoundCtx.PendingTriggers...),
		Vars:             copyVars(s.RoundCtx.Vars),
	}
}

// copyStringBoolMap 复制 map[string]bool
func copyStringBoolMap(m map[string]bool) map[string]bool {
	if m == nil {
		return nil
	}
	result := make(map[string]bool, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}
