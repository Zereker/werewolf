package werewolf

import (
	"sort"

	pb "github.com/Zereker/werewolf/proto"
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
}

// PendingTrigger 一个待结算的死亡技能
type PendingTrigger struct {
	PlayerID string       // 触发者
	Phase    pb.PhaseType // 该去哪个阶段结算
}

// NewRoundContext 创建新的回合上下文
func NewRoundContext() *RoundContext {
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
// 屠边判定需要区分「神职」与「平民」，而 pb.Camp 只有好人/狼人两值，
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
// 只覆盖内置的六个角色。pb.RoleType 的底层是 int32，调用方可以用
// 超出内置枚举的取值来定义自己的角色（建议从 1000 起，避免与后续
// 内置角色撞号）；这类角色会落到 Unknown，需通过 AddCustomPlayer
// 显式给出阵营与类别，否则不参与屠边判定。
func CategoryOf(role pb.RoleType) RoleCategory {
	switch role {
	case pb.RoleType_ROLE_TYPE_WEREWOLF:
		return RoleCategoryWolf
	case pb.RoleType_ROLE_TYPE_SEER,
		pb.RoleType_ROLE_TYPE_WITCH,
		pb.RoleType_ROLE_TYPE_HUNTER,
		pb.RoleType_ROLE_TYPE_GUARD:
		return RoleCategoryGod
	case pb.RoleType_ROLE_TYPE_VILLAGER:
		return RoleCategoryVillager
	default:
		return RoleCategoryUnknown
	}
}

// PlayerState 玩家状态
type PlayerState struct {
	ID       string
	Role     pb.RoleType
	Camp     pb.Camp
	Category RoleCategory // 角色类别（神职/平民/狼人），用于屠边判定
	Alive    bool

	// 女巫药剂状态
	HasAntidote bool // 是否有解药
	HasPoison   bool // 是否有毒药

	// 守卫连续保护限制
	LastProtectedTarget string // 上一回合保护的目标
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
	Phase   pb.PhaseType            // 当前阶段
	Round   int                     // 当前回合
	players map[string]*PlayerState // 玩家状态（私有，通过方法访问）

	// 回合临时上下文（每个回合重新创建）
	RoundCtx *RoundContext
}

// newState 创建游戏状态
func newState() *gameState {
	return &gameState{
		Phase:    pb.PhaseType_PHASE_TYPE_START,
		Round:    0,
		players:  make(map[string]*PlayerState),
		RoundCtx: NewRoundContext(),
	}
}

// CampOf 由角色推导阵营。
//
// 内置的六个角色中只有狼人属于狼人阵营。扩展角色（隐狼、狼美人等）
// 阵营与角色的对应关系不同，需用 AddCustomPlayer 显式指定。
func CampOf(role pb.RoleType) pb.Camp {
	if role == pb.RoleType_ROLE_TYPE_WEREWOLF {
		return pb.Camp_CAMP_EVIL
	}
	return pb.Camp_CAMP_GOOD
}

// addPlayer 添加玩家。阵营与角色类别由角色推导。
//
// 返回错误：ID 为空、ID 已存在、角色不能作为玩家身份（如上帝）。
func (s *gameState) addPlayer(id string, role pb.RoleType) error {
	return s.addCustomPlayer(id, role, CampOf(role), CategoryOf(role))
}

// addCustomPlayer 添加玩家并显式指定阵营与角色类别。
//
// 供扩展角色使用：隐狼是好人牌面的狼、白痴是不参与屠边的好人，
// 这类角色无法从内置映射推导，需要调用方直接给出。
func (s *gameState) addCustomPlayer(id string, role pb.RoleType, camp pb.Camp, category RoleCategory) error {
	if id == "" {
		return ErrInvalidPlayerID
	}
	// 上帝是系统角色，不是玩家身份
	if role == pb.RoleType_ROLE_TYPE_UNSPECIFIED || role == pb.RoleType_ROLE_TYPE_GOD {
		return WrapError(pb.ErrorCode_ERROR_CODE_INVALID_ROLE,
			"role %v cannot be assigned to a player", role)
	}

	if _, exists := s.players[id]; exists {
		return WrapError(pb.ErrorCode_ERROR_CODE_PLAYER_EXISTS, "player %q already exists", id)
	}

	player := &PlayerState{
		ID:       id,
		Role:     role,
		Camp:     camp,
		Category: category,
		Alive:    true,
	}

	// 女巫初始有解药和毒药各一瓶
	if role == pb.RoleType_ROLE_TYPE_WITCH {
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
		case pb.Camp_CAMP_GOOD:
			good++
		case pb.Camp_CAMP_EVIL:
			evil++
		}
	}
	return good, evil
}

// getPlayerSnapshot 返回玩家内部状态的值副本（包内使用）
func (s *gameState) getPlayerSnapshot(id string) (PlayerState, bool) {
	p, ok := s.players[id]
	if !ok {
		return PlayerState{}, false
	}
	return *p, true
}

// currentPhase 当前阶段（包内使用，自带锁）
func (s *gameState) currentPhase() pb.PhaseType {
	return s.Phase
}

// currentRound 当前回合（包内使用，自带锁）
func (s *gameState) currentRound() int {
	return s.Round
}

// getPlayer 获取玩家（包内使用）
// 返回内部指针，仅限包内代码使用
// 外部请使用 GetPlayerInfo(id) 获取只读副本
func (s *gameState) getPlayer(id string) (*PlayerState, bool) {
	p, ok := s.players[id]
	return p, ok
}

// PlayerInfo 玩家信息只读视图
type PlayerInfo struct {
	ID          string
	Role        pb.RoleType
	Camp        pb.Camp
	Category    RoleCategory
	Alive       bool
	Protected   bool // 今晚是否被保护（取自本回合上下文）
	HasAntidote bool
	HasPoison   bool
}

// GetPlayerInfo 获取玩家信息的只读副本
func (s *gameState) GetPlayerInfo(id string) (PlayerInfo, bool) {
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
	}, true
}

// getAlivePlayerIDsByRole 获取指定角色的存活玩家ID列表（包内使用）
func (s *gameState) getAlivePlayerIDsByRole(role pb.RoleType) []string {
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
	// 被取消的效果不改变状态，但仍会出现在 EndPhase 的返回值里，
	// 好让调用方知道「某人试了但没成」以及原因
	if effect.Canceled {
		return
	}

	// 确保 RoundCtx 已初始化
	if s.RoundCtx == nil {
		s.RoundCtx = NewRoundContext()
	}

	switch effect.Type {
	// 各种死亡：狼刀、毒杀、放逐、开枪
	case pb.EventType_EVENT_TYPE_KILL,
		pb.EventType_EVENT_TYPE_POISON,
		pb.EventType_EVENT_TYPE_ELIMINATE,
		pb.EventType_EVENT_TYPE_SHOOT:
		if target, ok := s.players[effect.TargetID]; ok {
			target.Alive = false
		}

	case pb.EventType_EVENT_TYPE_PROTECT:
		if _, ok := s.players[effect.TargetID]; ok {
			s.RoundCtx.ProtectedPlayers[effect.TargetID] = true
		}

	case pb.EventType_EVENT_TYPE_SAVE:
		// 只记录「被救过」，不改存活状态。
		// 死亡统一在夜晚结算阶段发生，此刻目标还活着；
		// 若在这里置 Alive=true，就成了一个能让任意玩家复活的原语。
		if _, ok := s.players[effect.TargetID]; ok {
			s.RoundCtx.SavedPlayers[effect.TargetID] = true
		}

	// 内部状态变更
	case pb.EventType_EVENT_TYPE_SET_NIGHT_KILL:
		s.RoundCtx.KillTarget = effect.TargetID
	case pb.EventType_EVENT_TYPE_CLEAR_NIGHT_KILL:
		s.RoundCtx.KillTarget = ""
	case pb.EventType_EVENT_TYPE_SET_LAST_PROTECTED:
		if guard, ok := s.players[effect.SourceID]; ok && guard.Role == pb.RoleType_ROLE_TYPE_GUARD {
			guard.LastProtectedTarget = effect.TargetID
		}
	case pb.EventType_EVENT_TYPE_USE_ANTIDOTE:
		if witch, ok := s.players[effect.SourceID]; ok && witch.Role == pb.RoleType_ROLE_TYPE_WITCH {
			witch.HasAntidote = false
		}
	case pb.EventType_EVENT_TYPE_USE_POISON:
		if witch, ok := s.players[effect.SourceID]; ok && witch.Role == pb.RoleType_ROLE_TYPE_WITCH {
			witch.HasPoison = false
			s.RoundCtx.PoisonedPlayers[effect.TargetID] = true
		}
	case pb.EventType_EVENT_TYPE_ABILITY_TRIGGERED:
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
	s.RoundCtx = NewRoundContext()
}

// startAt 把状态置到开局：指定阶段、第一回合、干净的回合上下文
func (s *gameState) startAt(phase pb.PhaseType) {
	s.Phase = phase
	s.Round = 1
	s.resetRoundStateUnlocked()
}

// nextPhase 切换到下一阶段
func (s *gameState) nextPhase(phase pb.PhaseType) {
	s.Phase = phase
	// 进入新的夜晚（守卫阶段）时增加回合数并重置状态
	if phase == pb.PhaseType_PHASE_TYPE_NIGHT_GUARD {
		s.Round++
		s.resetRoundStateUnlocked()
	}
}

// getWolfTeammates 获取狼人队友（不包括自己）
// 只有狼人才能查询队友，非狼人返回空列表
func (s *gameState) getWolfTeammates(playerID string) []string {
	// 检查请求者是否是狼人
	player, ok := s.players[playerID]
	if !ok || player.Role != pb.RoleType_ROLE_TYPE_WEREWOLF {
		return []string{}
	}

	result := make([]string, 0)
	for _, p := range s.players {
		if p.Role == pb.RoleType_ROLE_TYPE_WEREWOLF && p.ID != playerID {
			result = append(result, p.ID)
		}
	}
	return result
}

// checkVictory 按指定方式检查胜利条件。
//
// 好人阵营的胜利条件与判定方式无关：「將狼人淘汰以獲取勝利」。
// 狼人阵营的胜利条件取决于 mode：
//
//	VictoryModeSideWipe（屠边）「需要淘汰所有平民或神職人員」
//	VictoryModeTownWipe（屠城）好人存活数 <= 狼人存活数
//
// 屠边判定只对开局就存在的类别生效：没有神职的板子不会因
// 「神职全灭」在开局瞬间判负，平民同理。
func (s *gameState) checkVictory(mode VictoryMode) (bool, pb.Camp) {
	var goodAlive, evilAlive int
	var godsTotal, godsAlive int
	var villagersTotal, villagersAlive int

	for _, p := range s.players {
		switch p.Category {
		case RoleCategoryGod:
			godsTotal++
			if p.Alive {
				godsAlive++
			}
		case RoleCategoryVillager:
			villagersTotal++
			if p.Alive {
				villagersAlive++
			}
		}

		if !p.Alive {
			continue
		}
		switch p.Camp {
		case pb.Camp_CAMP_GOOD:
			goodAlive++
		case pb.Camp_CAMP_EVIL:
			evilAlive++
		}
	}

	// 狼人全死，好人胜利（两种判定方式一致）
	if evilAlive == 0 {
		return true, pb.Camp_CAMP_GOOD
	}

	// 好人全灭，狼人胜利（兜底，避免无神职无平民的板子永不结束）
	if goodAlive == 0 {
		return true, pb.Camp_CAMP_EVIL
	}

	switch mode {
	case VictoryModeTownWipe:
		if goodAlive <= evilAlive {
			return true, pb.Camp_CAMP_EVIL
		}

	default: // VictoryModeSideWipe
		// 屠神：开局有神职且已全部出局
		if godsTotal > 0 && godsAlive == 0 {
			return true, pb.Camp_CAMP_EVIL
		}
		// 屠民：开局有平民且已全部出局
		if villagersTotal > 0 && villagersAlive == 0 {
			return true, pb.Camp_CAMP_EVIL
		}
	}

	return false, pb.Camp_CAMP_UNSPECIFIED
}

// anyAliveWitchHasAntidote 是否还有存活女巫持有解药。
//
// 用于规则「解藥未使用時可以得知狼人的殺害對象」：解药用完后，
// 女巫不再获知刀口。标准板子只有一名女巫，此时即「该女巫是否仍持有解药」；
// 多女巫板子下只要有一人持有解药，刀口就仍需下发。
func (s *gameState) anyAliveWitchHasAntidote() bool {
	for _, p := range s.players {
		if p.Alive && p.Role == pb.RoleType_ROLE_TYPE_WITCH && p.HasAntidote {
			return true
		}
	}
	return false
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

// hasPendingTrigger 是否还有未结算的死亡技能
func (s *gameState) hasPendingTrigger() bool {
	_, ok := s.peekTrigger()
	return ok
}

// GetRoundContext 获取回合上下文的只读副本
func (s *gameState) GetRoundContext() *RoundContext {
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
