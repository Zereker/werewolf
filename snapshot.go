package werewolf

import (
	"sort"

	pb "github.com/Zereker/werewolf/proto"
)

// SnapshotVersion 当前快照格式的版本号。
//
// 每次对快照结构做出不向后兼容的改动时递增，Restore 会拒绝无法识别的版本，
// 以免把旧数据按新结构解读出一个看似正常、实则错乱的局面。
const SnapshotVersion = 2

// Snapshot 引擎的完整可序列化快照。
//
// 快照结构与引擎内部结构是刻意分开的两套类型：内部结构随重构演进，
// 而快照是写进存储的格式，字段名必须稳定。两者之间的转换集中在本文件，
// 增减字段时这里会显式报错，不会悄悄丢数据。
//
// 快照**不包含** GameConfig、Logger、Metrics 与回调：
// 这些由调用方在恢复时提供，规则配置本身也应由调用方掌握版本。
//
// 枚举以数值形式序列化。protobuf 的枚举编号是稳定契约，
// 而名称可能被重命名，故不用名称。
type Snapshot struct {
	Version int `json:"version"`

	Phase pb.PhaseType `json:"phase"`
	Round int          `json:"round"`

	Players      []PlayerSnapshot   `json:"players"`
	RoundContext RoundCtxSnapshot   `json:"round_context"`
	PendingUses  []SkillUseSnapshot `json:"pending_uses"`
}

// PlayerSnapshot 单个玩家的快照
type PlayerSnapshot struct {
	ID       string       `json:"id"`
	Role     pb.RoleType  `json:"role"`
	Camp     pb.Camp      `json:"camp"`
	Category RoleCategory `json:"category"`
	Alive    bool         `json:"alive"`

	HasAntidote bool `json:"has_antidote"`
	HasPoison   bool `json:"has_poison"`

	LastProtectedTarget string `json:"last_protected_target,omitempty"`
}

// RoundCtxSnapshot 回合上下文的快照
type RoundCtxSnapshot struct {
	KillTarget       string                   `json:"kill_target,omitempty"`
	ProtectedPlayers []string                 `json:"protected_players,omitempty"`
	SavedPlayers     []string                 `json:"saved_players,omitempty"`
	PoisonedPlayers  []string                 `json:"poisoned_players,omitempty"`
	PendingTriggers  []PendingTriggerSnapshot `json:"pending_triggers,omitempty"`
}

// PendingTriggerSnapshot 一个待结算的死亡技能
type PendingTriggerSnapshot struct {
	PlayerID string       `json:"player_id"`
	Phase    pb.PhaseType `json:"phase"`
}

// SkillUseSnapshot 已提交但尚未结算的技能
type SkillUseSnapshot struct {
	PlayerID string       `json:"player_id"`
	Skill    pb.SkillType `json:"skill"`
	TargetID string       `json:"target_id,omitempty"`
	Phase    pb.PhaseType `json:"phase"`
	Round    int          `json:"round"`
}

// Snapshot 导出引擎的当前状态。
//
// 返回的快照是深拷贝，可以安全地序列化、跨 goroutine 传递或长期持有，
// 后续的游戏推进不会影响它。
//
// 快照包含当前阶段已提交但尚未结算的技能，因此可以在一个阶段的中途保存，
// 恢复后继续收技能、再调用 EndPhase 结算。
func (e *Engine) Snapshot() *Snapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()

	snap := &Snapshot{
		Version:      SnapshotVersion,
		Phase:        e.state.Phase,
		Round:        e.state.Round,
		Players:      e.state.snapshotPlayers(),
		RoundContext: e.state.snapshotRoundCtx(),
		PendingUses:  make([]SkillUseSnapshot, 0, len(e.pendingUses)),
	}

	for _, use := range e.pendingUses {
		snap.PendingUses = append(snap.PendingUses, SkillUseSnapshot{
			PlayerID: use.PlayerID,
			Skill:    use.Skill,
			TargetID: use.TargetID,
			Phase:    use.Phase,
			Round:    use.Round,
		})
	}

	return snap
}

// RestoreEngine 从快照重建引擎。
//
// config 为 nil 时使用默认配置。**恢复时必须提供与保存时一致的规则配置**——
// 快照只记录局面，不记录规则；用不同的配置恢复会得到一局规则被中途换掉的游戏。
//
// 返回错误：快照为 nil、版本不受支持、玩家 ID 为空或重复、阶段不在配置中。
func RestoreEngine(config *GameConfig, snap *Snapshot) (*Engine, error) {
	if snap == nil {
		return nil, ErrNilSnapshot
	}
	if snap.Version != SnapshotVersion {
		return nil, WrapError(pb.ErrorCode_ERROR_CODE_INVALID_SNAPSHOT,
			"unsupported snapshot version %d (expected %d)", snap.Version, SnapshotVersion)
	}

	engine, err := NewEngine(config)
	if err != nil {
		return nil, err
	}

	// 引擎尚未交给调用方，但仍走一遍锁：状态的所有访问都在引擎锁内，
	// 是这套并发模型唯一的前提，不留例外
	engine.mu.Lock()
	defer engine.mu.Unlock()

	// 阶段必须能在配置里找到，否则恢复出来的引擎推进不下去。
	// START 与 END 是流程的两端，不出现在阶段配置中，单独放行。
	if snap.Phase != pb.PhaseType_PHASE_TYPE_START &&
		snap.Phase != pb.PhaseType_PHASE_TYPE_END &&
		engine.phase.phaseConfig(snap.Phase) == nil {
		return nil, WrapError(pb.ErrorCode_ERROR_CODE_INVALID_SNAPSHOT,
			"phase %v is not present in the supplied config", snap.Phase)
	}

	for _, p := range snap.Players {
		if p.ID == "" {
			return nil, ErrInvalidPlayerID
		}
		if _, exists := engine.state.getPlayer(p.ID); exists {
			return nil, WrapError(pb.ErrorCode_ERROR_CODE_INVALID_SNAPSHOT,
				"duplicate player %q in snapshot", p.ID)
		}
		engine.state.restorePlayer(p)
	}

	// 技能引用的玩家必须存在，否则结算时会静默丢弃
	for _, u := range snap.PendingUses {
		if _, ok := engine.state.getPlayer(u.PlayerID); !ok {
			return nil, WrapError(pb.ErrorCode_ERROR_CODE_INVALID_SNAPSHOT,
				"pending skill references unknown player %q", u.PlayerID)
		}
		engine.pendingUses = append(engine.pendingUses, &SkillUse{
			PlayerID: u.PlayerID,
			Skill:    u.Skill,
			TargetID: u.TargetID,
			Phase:    u.Phase,
			Round:    u.Round,
		})
	}

	engine.state.restoreProgress(snap.Phase, snap.Round, snap.RoundContext)

	return engine, nil
}

// ==================== State 侧的转换 ====================

// snapshotPlayers 导出玩家列表（按 ID 排序，保证快照可比较）
func (s *gameState) snapshotPlayers() []PlayerSnapshot {
	out := make([]PlayerSnapshot, 0, len(s.players))
	for _, p := range s.players {
		out = append(out, PlayerSnapshot{
			ID:                  p.ID,
			Role:                p.Role,
			Camp:                p.Camp,
			Category:            p.Category,
			Alive:               p.Alive,
			HasAntidote:         p.HasAntidote,
			HasPoison:           p.HasPoison,
			LastProtectedTarget: p.LastProtectedTarget,
		})
	}
	sortPlayerSnapshots(out)
	return out
}

// snapshotRoundCtx 导出回合上下文
func (s *gameState) snapshotRoundCtx() RoundCtxSnapshot {
	if s.RoundCtx == nil {
		return RoundCtxSnapshot{}
	}

	return RoundCtxSnapshot{
		KillTarget:       s.RoundCtx.KillTarget,
		ProtectedPlayers: sortedKeys(s.RoundCtx.ProtectedPlayers),
		SavedPlayers:     sortedKeys(s.RoundCtx.SavedPlayers),
		PoisonedPlayers:  sortedKeys(s.RoundCtx.PoisonedPlayers),
		PendingTriggers:  snapshotTriggers(s.RoundCtx.PendingTriggers),
	}
}

// restorePlayer 按快照写入一名玩家。
//
// 不走 AddPlayer：恢复时要原样还原快照里的存活状态与药剂，
// 而 AddPlayer 会按「新玩家」的规则重新初始化。
func (s *gameState) restorePlayer(p PlayerSnapshot) {
	s.players[p.ID] = &PlayerState{
		ID:                  p.ID,
		Role:                p.Role,
		Camp:                p.Camp,
		Category:            p.Category,
		Alive:               p.Alive,
		HasAntidote:         p.HasAntidote,
		HasPoison:           p.HasPoison,
		LastProtectedTarget: p.LastProtectedTarget,
	}
}

// restoreProgress 还原阶段、回合与回合上下文
func (s *gameState) restoreProgress(phase pb.PhaseType, round int, rc RoundCtxSnapshot) {
	s.Phase = phase
	s.Round = round
	s.RoundCtx = &RoundContext{
		KillTarget:       rc.KillTarget,
		ProtectedPlayers: keySet(rc.ProtectedPlayers),
		SavedPlayers:     keySet(rc.SavedPlayers),
		PoisonedPlayers:  keySet(rc.PoisonedPlayers),
		PendingTriggers:  restoreTriggers(rc.PendingTriggers),
	}
}

// ==================== 小工具 ====================

// sortedKeys 把集合导出为有序切片。
// 排序是为了让同一局面导出的快照字节一致，便于比对与幂等写入。
func sortedKeys(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k, v := range m {
		if v {
			out = append(out, k)
		}
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
}

// keySet 把切片还原为集合
func keySet(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

// sortedStrings 原地排序并返回，用于让面向调用方的列表输出稳定
func sortedStrings(in []string) []string {
	sort.Strings(in)
	return in
}

// sortPlayerSnapshots 按 ID 排序
func sortPlayerSnapshots(ps []PlayerSnapshot) {
	sort.Slice(ps, func(i, j int) bool { return ps[i].ID < ps[j].ID })
}

// snapshotTriggers 导出待结算队列
func snapshotTriggers(ts []PendingTrigger) []PendingTriggerSnapshot {
	if len(ts) == 0 {
		return nil
	}
	out := make([]PendingTriggerSnapshot, 0, len(ts))
	for _, t := range ts {
		// 刻意逐字段写而不是做类型转换：两个类型当前恰好同形，
		// 但快照是存储格式、PendingTrigger 是内部结构，不应绑定在一起。
		//nolint:staticcheck // S1016: 见上
		out = append(out, PendingTriggerSnapshot{PlayerID: t.PlayerID, Phase: t.Phase})
	}
	return out
}

// restoreTriggers 还原待结算队列（顺序即结算顺序，不排序）
func restoreTriggers(ts []PendingTriggerSnapshot) []PendingTrigger {
	if len(ts) == 0 {
		return nil
	}
	out := make([]PendingTrigger, 0, len(ts))
	for _, t := range ts {
		//nolint:staticcheck // S1016: 同 snapshotTriggers，刻意不做类型转换
		out = append(out, PendingTrigger{PlayerID: t.PlayerID, Phase: t.Phase})
	}
	return out
}
