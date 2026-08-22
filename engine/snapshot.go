package engine

import (
	"sort"
)

// SnapshotVersion 当前快照格式的版本号。
//
// 每次对快照结构做出不向后兼容的改动时递增，RestoreEngine 会拒绝无法识别
// 的版本，以免把旧数据按新结构解读出一个看似正常、实则错乱的局面。
//
// 这套机制原本有个缺口：改了结构却**忘了**递增，没有任何东西会报警——
// 而那恰恰是这个版本号想防的事。现在规则包里有一个 golden 测试
// （TestSnapshot_ShapeIsPinnedToVersion）把序列化形状钉住，字段增删改名
// 都会让它变红，红了之后再判断该不该递增。
const SnapshotVersion = 11

// Snapshot 引擎的完整可序列化快照。
//
// 快照结构与引擎内部结构是刻意分开的两套类型：内部结构随重构演进，
// 而快照是写进存储的格式，字段名必须稳定。两者之间的转换集中在本文件，
// 增减字段时这里会显式报错，不会悄悄丢数据。
//
// 快照**不包含** Config、Logger、Metrics 与回调：
// 这些由调用方在恢复时提供，规则配置本身也应由调用方掌握版本。
//
// 枚举以**名字**序列化（"NIGHT_GUARD" 而不是 21）。存档是要给人看、
// 也可能被别的语言读的东西，编号对不上号。
//
// v10 起这一点由类型本身保证：枚举的底层就是字符串，不再有一层
// 「编号到名字」的翻译。第三方的自定义取值此前没有名字、按编号写
// （`"role":1000`），现在与内置的一样是名字（`"role":"WOLF_KING"`）——
// 这就是 v9 到 v10 的全部区别，也是它要递增版本号的原因。
type Snapshot struct {
	Version int `json:"version"`

	Phase PhaseType `json:"phase"`
	Round int       `json:"round"`

	// Seed 随机流的种子。它决定对局结果，因此随快照走——恢复出来的对局
	// 摇出同一串数，不必指望调用方记得传对配置。
	Seed int64 `json:"seed,omitempty"`

	// Vars 整局有效、不属于任何玩家的状态。
	Vars map[string]string `json:"vars,omitempty"`

	// Actors 规则为各阶段指定的行动者。名单往往在更早的阶段算出来
	// （阿瓦隆的任务队伍是提名阶段选的），因此必须随快照走，
	// 否则从提名与任务之间恢复出来的对局会丢掉队伍。
	Actors map[PhaseType][]string `json:"actors,omitempty"`

	Players      []PlayerSnapshot   `json:"players"`
	RoundContext RoundCtxSnapshot   `json:"round_context"`
	PendingUses  []SkillUseSnapshot `json:"pending_uses"`
}

// PlayerSnapshot 单个玩家的快照
type PlayerSnapshot struct {
	ID    string   `json:"id"`
	Role  RoleType `json:"role"`
	Alive bool     `json:"alive"`

	// RoundVars 这名玩家在本回合的标记，每回合清零。今晚谁被守了、
	// 被救了、被毒了都在这里——它们此前是 RoundCtxSnapshot 上三个
	// []string，v8 起并入玩家自身，与规则包自己定的标记同一条路。
	RoundVars map[string]string `json:"round_vars,omitempty"`

	// Vars 角色私有的状态（狼人杀的女巫药剂就在其中）——
	// 它们此前是这里两个具名 bool 字段，v7 起并入 Vars，与第三方角色
	// 同一条路。存这一项是整个机制成立的前提：带不上它，角色的状态
	// 就只能藏在 Resolver 里，那正是要解决的问题。
	Vars map[string]string `json:"vars,omitempty"`
}

// RoundCtxSnapshot 回合上下文的快照
type RoundCtxSnapshot struct {
	PendingTriggers []PendingTriggerSnapshot `json:"pending_triggers,omitempty"`

	// Vars 第三方角色的回合级自定义状态
	Vars map[string]string `json:"vars,omitempty"`
}

// PendingTriggerSnapshot 一个待结算的死亡技能
type PendingTriggerSnapshot struct {
	PlayerID string    `json:"player_id"`
	Phase    PhaseType `json:"phase"`
}

// SkillUseSnapshot 已提交但尚未结算的技能
type SkillUseSnapshot struct {
	PlayerID string    `json:"player_id"`
	Skill    SkillType `json:"skill"`
	Targets  []string  `json:"targets,omitempty"`
	Phase    PhaseType `json:"phase"`
	Round    int       `json:"round"`
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
		Seed:         e.state.Seed,
		Vars:         copyVars(e.state.Vars),
		Actors:       copyActors(e.state.Actors),
		Players:      e.state.snapshotPlayers(),
		RoundContext: e.state.snapshotRoundCtx(),
		PendingUses:  make([]SkillUseSnapshot, 0, len(e.pendingUses)),
	}

	for _, use := range e.pendingUses {
		snap.PendingUses = append(snap.PendingUses, SkillUseSnapshot{
			PlayerID: use.PlayerID,
			Skill:    use.Skill,
			Targets:  append([]string(nil), use.Targets...),
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
// 自定义角色的解析器必须经 opts 传入（WithResolver）。漏掉会让该阶段的
// 技能被静默丢弃，所以这里会跑一遍解析器校验，缺了就直接报错。
//
// 返回错误：快照为 nil、版本不受支持、玩家 ID 为空或重复、阶段不在配置中、
// 有阶段缺少解析器。
func RestoreEngine(config *Config, snap *Snapshot, opts ...EngineOption) (*Engine, error) {
	if snap == nil {
		return nil, ErrNilSnapshot
	}
	if snap.Version != SnapshotVersion {
		return nil, WrapError(CodeInvalidSnapshot,
			"unsupported snapshot version %d (expected %d)", snap.Version, SnapshotVersion)
	}

	engine, err := NewEngine(config, opts...)
	if err != nil {
		return nil, err
	}

	// 与 Start 同一条校验：缺解析器的阶段会把收到的技能悄悄丢掉，
	// 这种失败在对局中几乎无法定位，必须在把引擎交出去之前拦下
	if err := engine.phase.validateResolvers(); err != nil {
		return nil, err
	}

	// 引擎尚未交给调用方，但仍走一遍锁：状态的所有访问都在引擎锁内，
	// 是这套并发模型唯一的前提，不留例外
	engine.mu.Lock()
	defer engine.mu.Unlock()

	if err := engine.restorePhase(snap.Phase); err != nil {
		return nil, err
	}
	if err := engine.restorePlayers(snap.Players); err != nil {
		return nil, err
	}
	if err := engine.restorePendingUses(snap.PendingUses); err != nil {
		return nil, err
	}

	engine.state.Seed = snap.Seed
	engine.state.Vars = copyVars(snap.Vars)
	engine.state.Actors = copyActors(snap.Actors)
	engine.state.restoreProgress(snap.Phase, snap.Round, snap.RoundContext)

	return engine, nil
}

// restorePhase 校验快照里的阶段能在配置中找到。
//
// 找不到的话恢复出来的引擎推进不下去。START 与 END 是流程的两端，
// 不出现在阶段配置中，单独放行。
func (e *Engine) restorePhase(phase PhaseType) error {
	if phase == PhaseStart || phase == PhaseEnd {
		return nil
	}
	if e.phase.phaseConfig(phase) == nil {
		return WrapError(CodeInvalidSnapshot,
			"phase %v is not present in the supplied config", phase)
	}
	return nil
}

// restorePlayers 按快照写入玩家。
//
// 校验与 AddPlayer 一致：restorePlayer 刻意不走 AddPlayer（要原样还原
// 存活状态与药剂），但不该顺带把 AddPlayer 会拒绝的身份也放行。
func (e *Engine) restorePlayers(players []PlayerSnapshot) error {
	for _, p := range players {
		if p.ID == "" {
			return ErrInvalidPlayerID
		}
		if p.Role == RoleUnspecified || p.Role == RoleGod {
			return WrapError(CodeInvalidRole,
				"role %v cannot be assigned to a player", p.Role)
		}
		if _, exists := e.state.getPlayer(p.ID); exists {
			return WrapError(CodeInvalidSnapshot,
				"duplicate player %q in snapshot", p.ID)
		}
		e.state.restorePlayer(p)
	}
	return nil
}

// restorePendingUses 还原已提交但尚未结算的技能。
// 引用的玩家与目标都必须存在，否则结算时会被静默丢弃。
func (e *Engine) restorePendingUses(uses []SkillUseSnapshot) error {
	for _, u := range uses {
		if _, ok := e.state.getPlayer(u.PlayerID); !ok {
			return WrapError(CodeInvalidSnapshot,
				"pending skill references unknown player %q", u.PlayerID)
		}
		for _, id := range u.Targets {
			if id == "" {
				continue
			}
			if _, ok := e.state.getPlayer(id); !ok {
				return WrapError(CodeInvalidSnapshot,
					"pending skill references unknown target %q", id)
			}
		}
		e.pendingUses = append(e.pendingUses, &SkillUse{
			PlayerID: u.PlayerID,
			Skill:    u.Skill,
			Targets:  append([]string(nil), u.Targets...),
			Phase:    u.Phase,
			Round:    u.Round,
		})
	}
	return nil
}

// ==================== State 侧的转换 ====================

// snapshotPlayers 导出玩家列表（按 ID 排序，保证快照可比较）
func (s *gameState) snapshotPlayers() []PlayerSnapshot {
	out := make([]PlayerSnapshot, 0, len(s.players))
	for _, p := range s.players {
		out = append(out, PlayerSnapshot{
			ID:        p.ID,
			Role:      p.Role,
			Alive:     p.Alive,
			RoundVars: copyVars(p.RoundVars),
			Vars:      copyVars(p.Vars),
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
		PendingTriggers: snapshotTriggers(s.RoundCtx.PendingTriggers),
		Vars:            copyVars(s.RoundCtx.Vars),
	}
}

// restorePlayer 按快照写入一名玩家。
//
// 不走 AddPlayer：恢复时要原样还原快照里的存活状态与 Vars，
// 而 AddPlayer 会经 RoleSetup 重新发一遍初始状态——用掉的药会回来。
func (s *gameState) restorePlayer(p PlayerSnapshot) {
	s.players[p.ID] = &playerState{
		ID:        p.ID,
		Role:      p.Role,
		Alive:     p.Alive,
		RoundVars: copyVars(p.RoundVars),
		Vars:      copyVars(p.Vars),
	}
}

// copyVars 复制自定义状态。快照是深拷贝，这一项也不能例外——
// 否则恢复出来的引擎与原引擎共用同一张 map，改一边动两边。
func copyVars(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// restoreProgress 还原阶段、回合与回合上下文
func (s *gameState) restoreProgress(phase PhaseType, round int, rc RoundCtxSnapshot) {
	s.Phase = phase
	s.Round = round
	s.RoundCtx = &RoundContext{
		PendingTriggers: restoreTriggers(rc.PendingTriggers),
		Vars:            copyVars(rc.Vars),
	}
}

// ==================== 小工具 ====================

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
