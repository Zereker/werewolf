package werewolf

import (
	"sync"

	pb "github.com/Zereker/werewolf/proto"
)

// RoundContext 回合上下文（每个回合重新创建）
// 用于管理回合内各阶段之间共享的临时状态
// 包含夜晚和白天的相关状态（如猎人触发可能发生在投票阶段）
type RoundContext struct {
	KillTarget        string          // 狼人击杀目标（女巫可查询）
	ProtectedPlayers  map[string]bool // 被守卫保护的玩家
	SavedPlayers      map[string]bool // 被女巫救的玩家
	PoisonedPlayers   map[string]bool // 被女巫毒的玩家
	HunterTriggered   bool            // 猎人是否被触发（死亡时）
	TriggeredHunterID string          // 被触发的猎人ID
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

// PlayerState 玩家状态
type PlayerState struct {
	ID    string
	Role  pb.RoleType
	Camp  pb.Camp
	Alive bool

	// 女巫药剂状态
	HasAntidote bool // 是否有解药
	HasPoison   bool // 是否有毒药

	// 守卫连续保护限制
	LastProtectedTarget string // 上一回合保护的目标
}

// State 游戏状态
//
// # 并发安全说明
//
// State 使用 RWMutex 保护所有字段。当通过 Engine 访问时，
// Engine 也有自己的 RWMutex，形成嵌套锁（双重锁）。
//
// 设计选择说明：
//   - 这种设计是有意为之，确保 State 可以独立使用时也是线程安全的
//   - 嵌套锁不会死锁，因为总是按相同顺序获取（Engine.mu -> State.mu）
//   - 性能影响：有一定开销，但对于回合制游戏场景可以接受
//
// 替代方案（未采用）：
//   - 只在 Engine 层加锁：需要确保 State 永远不会被直接访问
//   - 使用 sync.Map：对于复杂状态结构不太适合
//
// 使用建议：
//   - 优先通过 Engine 的方法访问状态
//   - 避免持有锁时进行耗时操作
//   - 如需高性能场景，可重构为单层锁设计
type State struct {
	mu sync.RWMutex

	Phase   pb.PhaseType            // 当前阶段
	SubStep int                     // 当前子步骤（夜晚阶段使用）
	Round   int                     // 当前回合
	players map[string]*PlayerState // 玩家状态（私有，通过方法访问）

	// 回合临时上下文（每个回合重新创建）
	RoundCtx *RoundContext
}

// NewState 创建游戏状态
func NewState() *State {
	return &State{
		Phase:   pb.PhaseType_PHASE_TYPE_START,
		Round:   0,
		players: make(map[string]*PlayerState),
		RoundCtx:  NewRoundContext(),
	}
}

// AddPlayer 添加玩家
// 如果玩家ID已存在，会被覆盖
func (s *State) AddPlayer(id string, role pb.RoleType, camp pb.Camp) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否已存在（可选：记录警告或返回错误）
	// 目前采用覆盖策略，允许重新设置玩家属性
	player := &PlayerState{
		ID:    id,
		Role:  role,
		Camp:  camp,
		Alive: true,
	}

	// 女巫初始有解药和毒药各一瓶
	if role == pb.RoleType_ROLE_TYPE_WITCH {
		player.HasAntidote = true
		player.HasPoison = true
	}

	s.players[id] = player
}

// AddPlayerIfNotExists 添加玩家（如果不存在）
// 返回 true 表示添加成功，false 表示玩家已存在
func (s *State) AddPlayerIfNotExists(id string, role pb.RoleType, camp pb.Camp) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.players[id]; exists {
		return false
	}

	player := &PlayerState{
		ID:    id,
		Role:  role,
		Camp:  camp,
		Alive: true,
	}

	if role == pb.RoleType_ROLE_TYPE_WITCH {
		player.HasAntidote = true
		player.HasPoison = true
	}

	s.players[id] = player
	return true
}

// getPlayer 获取玩家（包内使用）
// 返回内部指针，仅限包内代码使用
// 外部请使用 GetPlayerInfo(id) 获取只读副本
func (s *State) getPlayer(id string) (*PlayerState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.players[id]
	return p, ok
}

// PlayerInfo 玩家信息只读视图
type PlayerInfo struct {
	ID          string
	Role        pb.RoleType
	Camp        pb.Camp
	Alive       bool
	Protected   bool // 今晚是否被保护（从 NightContext 计算）
	HasAntidote bool
	HasPoison   bool
}

// GetPlayerInfo 获取玩家信息的只读副本
func (s *State) GetPlayerInfo(id string) (PlayerInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.players[id]
	if !ok {
		return PlayerInfo{}, false
	}

	return PlayerInfo{
		ID:          p.ID,
		Role:        p.Role,
		Camp:        p.Camp,
		Alive:       p.Alive,
		Protected:   s.RoundCtx.IsProtected(id), // 从 RoundContext 获取
		HasAntidote: p.HasAntidote,
		HasPoison:   p.HasPoison,
	}, true
}

// getAlivePlayers 获取存活玩家（包内使用）
func (s *State) getAlivePlayers() []*PlayerState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*PlayerState, 0)
	for _, p := range s.players {
		if p.Alive {
			result = append(result, p)
		}
	}
	return result
}

// getAlivePlayersByRole 获取指定角色的存活玩家（包内使用）
func (s *State) getAlivePlayersByRole(role pb.RoleType) []*PlayerState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*PlayerState, 0)
	for _, p := range s.players {
		if p.Alive && p.Role == role {
			result = append(result, p)
		}
	}
	return result
}

// getAlivePlayersByCamp 获取指定阵营的存活玩家（包内使用）
func (s *State) getAlivePlayersByCamp(camp pb.Camp) []*PlayerState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*PlayerState, 0)
	for _, p := range s.players {
		if p.Alive && p.Camp == camp {
			result = append(result, p)
		}
	}
	return result
}

// getAlivePlayerIDsByRole 获取指定角色的存活玩家ID列表（包内使用）
func (s *State) getAlivePlayerIDsByRole(role pb.RoleType) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]string, 0)
	for id, p := range s.players {
		if p.Alive && p.Role == role {
			result = append(result, id)
		}
	}
	return result
}

// getAlivePlayerIDs 获取所有存活玩家ID列表（包内使用）
func (s *State) getAlivePlayerIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]string, 0)
	for id, p := range s.players {
		if p.Alive {
			result = append(result, id)
		}
	}
	return result
}

// ApplyEffect 应用效果
func (s *State) ApplyEffect(effect *Effect) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查效果是否被取消（必须在锁内检查以避免竞态）
	if effect.Canceled {
		return
	}

	// 确保 RoundCtx 已初始化
	if s.RoundCtx == nil {
		s.RoundCtx = NewRoundContext()
	}

	switch effect.Type {
	// 外部可见效果 - 需要目标玩家
	case pb.EventType_EVENT_TYPE_KILL, pb.EventType_EVENT_TYPE_POISON, pb.EventType_EVENT_TYPE_ELIMINATE:
		if target, ok := s.players[effect.TargetID]; ok {
			target.Alive = false
		}
	case pb.EventType_EVENT_TYPE_PROTECT:
		if _, ok := s.players[effect.TargetID]; ok {
			s.RoundCtx.ProtectedPlayers[effect.TargetID] = true
		}
	case pb.EventType_EVENT_TYPE_SAVE:
		if target, ok := s.players[effect.TargetID]; ok {
			target.Alive = true
			s.RoundCtx.SavedPlayers[effect.TargetID] = true
		}
	case pb.EventType_EVENT_TYPE_SHOOT:
		// 猎人开枪，目标死亡
		if target, ok := s.players[effect.TargetID]; ok {
			target.Alive = false
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
	case pb.EventType_EVENT_TYPE_HUNTER_TRIGGERED:
		// 标记猎人被触发
		s.RoundCtx.HunterTriggered = true
		s.RoundCtx.TriggeredHunterID = effect.SourceID
	}
}

// ResetRoundState 重置回合状态（每回合开始时调用）
func (s *State) ResetRoundState() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.resetRoundStateUnlocked()
}

// resetRoundStateUnlocked 内部方法，不获取锁
func (s *State) resetRoundStateUnlocked() {
	s.SubStep = 0
	// 创建新的回合上下文
	s.RoundCtx = NewRoundContext()
}

// NextPhase 切换到下一阶段
func (s *State) NextPhase(phase pb.PhaseType) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Phase = phase
	// 进入新的夜晚（守卫阶段）时增加回合数并重置状态
	if phase == pb.PhaseType_PHASE_TYPE_NIGHT_GUARD {
		s.Round++
		s.resetRoundStateUnlocked()
	}
}

// GetWolfTeammates 获取狼人队友（不包括自己）
// 只有狼人才能查询队友，非狼人返回空列表
func (s *State) GetWolfTeammates(playerID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

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

// CheckVictory 检查胜利条件
func (s *State) CheckVictory() (bool, pb.Camp) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	goodAlive := 0
	evilAlive := 0

	for _, p := range s.players {
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

	// 狼人全死，好人胜利
	if evilAlive == 0 {
		return true, pb.Camp_CAMP_GOOD
	}

	// 好人数量 <= 狼人数量，狼人胜利
	if goodAlive <= evilAlive {
		return true, pb.Camp_CAMP_EVIL
	}

	return false, pb.Camp_CAMP_UNSPECIFIED
}

// UseAntidote 女巫使用解药
// 返回 true 表示成功使用，false 表示没有解药
func (s *State) UseAntidote(witchID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	witch, ok := s.players[witchID]
	if !ok || witch.Role != pb.RoleType_ROLE_TYPE_WITCH {
		return false
	}

	if !witch.HasAntidote {
		return false
	}

	witch.HasAntidote = false
	return true
}

// UsePoison 女巫使用毒药
// 返回 true 表示成功使用，false 表示没有毒药
func (s *State) UsePoison(witchID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	witch, ok := s.players[witchID]
	if !ok || witch.Role != pb.RoleType_ROLE_TYPE_WITCH {
		return false
	}

	if !witch.HasPoison {
		return false
	}

	witch.HasPoison = false
	return true
}

// CanUseAntidote 检查女巫是否有解药
func (s *State) CanUseAntidote(witchID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	witch, ok := s.players[witchID]
	if !ok || witch.Role != pb.RoleType_ROLE_TYPE_WITCH {
		return false
	}
	return witch.HasAntidote
}

// CanUsePoison 检查女巫是否有毒药
func (s *State) CanUsePoison(witchID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	witch, ok := s.players[witchID]
	if !ok || witch.Role != pb.RoleType_ROLE_TYPE_WITCH {
		return false
	}
	return witch.HasPoison
}

// SetLastProtectedTarget 设置守卫上一回合保护的目标
func (s *State) SetLastProtectedTarget(guardID, targetID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	guard, ok := s.players[guardID]
	if !ok || guard.Role != pb.RoleType_ROLE_TYPE_GUARD {
		return
	}
	guard.LastProtectedTarget = targetID
}

// CanProtect 检查守卫是否可以保护目标（考虑连续保护限制）
func (s *State) CanProtect(guardID, targetID string, canRepeat bool) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	guard, ok := s.players[guardID]
	if !ok || guard.Role != pb.RoleType_ROLE_TYPE_GUARD {
		return false
	}

	// 如果允许连续保护，直接返回 true
	if canRepeat {
		return true
	}

	// 否则检查是否与上一回合保护相同目标
	return guard.LastProtectedTarget != targetID
}

// GetRoundContext 获取回合上下文的只读副本
func (s *State) GetRoundContext() *RoundContext {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.RoundCtx == nil {
		return nil
	}

	// 返回副本以避免外部修改
	return &RoundContext{
		KillTarget:        s.RoundCtx.KillTarget,
		ProtectedPlayers:  copyStringBoolMap(s.RoundCtx.ProtectedPlayers),
		SavedPlayers:      copyStringBoolMap(s.RoundCtx.SavedPlayers),
		PoisonedPlayers:   copyStringBoolMap(s.RoundCtx.PoisonedPlayers),
		HunterTriggered:   s.RoundCtx.HunterTriggered,
		TriggeredHunterID: s.RoundCtx.TriggeredHunterID,
	}
}

// IsPlayerProtectedThisRound 检查玩家本回合是否被保护
func (s *State) IsPlayerProtectedThisRound(playerID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.RoundCtx.IsProtected(playerID)
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
