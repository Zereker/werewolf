package werewolf

import (
	pb "github.com/Zereker/werewolf/proto"
)

// PlayerView 站在某一名玩家的角度，他此刻有权知道的全部信息。
//
// # 为什么需要它
//
// 狼人杀这个游戏唯一真正难的东西就是「谁能知道什么」。引擎此前只提供
// 上帝视角（PlayerInfo 能查到任何人的身份、PhaseInfo 一次性交出
// 狼队名单与刀口），把最安全攸关的信息过滤逻辑推给了调用方——
// 只要某个 handler 手滑把整个 PhaseInfo 广播出去，一局游戏当场作废。
//
// 调用方作为上帝，确实需要上帝视角；但它不应该被迫自己实现「投影」。
// PlayerView 把这件事收回库内：给一个玩家 ID，返回的东西可以直接发给他。
//
// # 不包含什么
//
// 视图是「此刻的状态」，不是「历史」。预言家历次查验的结果、公开的死亡
// 记录属于历史，由 Effect 日志（Engine.EffectLog）承载。
type PlayerView struct {
	PlayerID string       // 视角所属玩家
	Round    int          // 当前回合
	Phase    pb.PhaseType // 当前阶段

	// Self 自己的完整信息：身份、阵营、存活、（女巫的）药剂
	Self PlayerInfo

	// Players 全场玩家的公开信息，按 ID 排序。
	// 身份只在对本视角公开时才填充（自己、狼队友）。
	Players []PublicPlayerInfo

	// AllowedSkills 本阶段自己可以提交的技能。
	// 不该自己行动时为空——这也是判断「轮到我了吗」的依据。
	AllowedSkills []pb.SkillType

	// Teammates 狼人可见：其余狼队友的 ID。非狼人恒为空。
	Teammates []string

	// KillTarget 女巫可见：今晚狼人的击杀目标。
	// 依规则「解藥未使用時可以得知狼人的殺害對象」，解药用完后恒为空。
	KillTarget string
}

// PublicPlayerInfo 一名玩家对外公开的信息
type PublicPlayerInfo struct {
	ID    string `json:"id"`
	Alive bool   `json:"alive"`

	// Role 仅在该玩家的身份对本视角公开时填充，否则为 UNSPECIFIED。
	// 引擎默认只公开「自己」和「狼队友」——出局者是否翻牌属于桌面
	// 规则，由调用方决定，引擎不替它做主。
	Role pb.RoleType `json:"role,omitempty"`
}

// PlayerView 返回指定玩家的视角。
//
// 返回的内容可以直接发给该玩家，不需要调用方再做过滤。
// 玩家不存在时返回 nil。
//
// 与之相对，PhaseInfo / PlayerInfo / WolfTeammates /
// NightKillTarget 是上帝视角接口：调用方作为主持人需要它们，
// 但它们的内容不可以整体转发给玩家。
func (e *Engine) PlayerView(playerID string) *PlayerView {
	e.mu.RLock()
	defer e.mu.RUnlock()

	self, ok := e.state.PlayerInfo(playerID)
	if !ok {
		return nil
	}

	view := &PlayerView{
		PlayerID:      playerID,
		Round:         e.state.Round,
		Phase:         e.state.Phase,
		Self:          self,
		AllowedSkills: e.allowedSkillsForPlayer(playerID, self),
	}

	// 狼人互相可见
	revealed := map[string]bool{playerID: true}
	if self.Role == pb.RoleType_ROLE_TYPE_WEREWOLF {
		view.Teammates = e.state.getWolfTeammates(playerID)
		for _, id := range view.Teammates {
			revealed[id] = true
		}
	}

	view.Players = e.publicPlayers(revealed)

	// 女巫在解药尚在手时可知刀口
	if self.Role == pb.RoleType_ROLE_TYPE_WITCH && self.HasAntidote {
		view.KillTarget = e.state.RoundCtx.KillTarget
	}

	return view
}

// publicPlayers 组装全场公开信息。revealed 中的玩家会带上身份。
// 调用前需持有 e.mu。
func (e *Engine) publicPlayers(revealed map[string]bool) []PublicPlayerInfo {
	ids := e.state.allPlayerIDs()
	out := make([]PublicPlayerInfo, 0, len(ids))
	for _, id := range ids {
		p, ok := e.state.PlayerInfo(id)
		if !ok {
			continue
		}
		info := PublicPlayerInfo{ID: p.ID, Alive: p.Alive}
		if revealed[id] {
			info.Role = p.Role
		}
		out = append(out, info)
	}
	return out
}

// allowedSkillsForPlayer 该玩家此刻能提交的技能。
// 调用前需持有 e.mu。
func (e *Engine) allowedSkillsForPlayer(playerID string, info PlayerInfo) []pb.SkillType {
	// 死亡技能阶段只有触发者能行动
	if t, ok := e.state.peekTrigger(); ok && t.Phase == e.state.Phase {
		if t.PlayerID != playerID {
			return nil
		}
		return e.allowedSkillsFor(info.Role)
	}
	if !info.Alive {
		return nil
	}
	return e.allowedSkillsFor(info.Role)
}

// ==================== 效果的接收者 ====================

// AudienceOf 返回一个效果应该发给哪些玩家。
//
// 这是配套 PlayerView 的另一半：视图解决「玩家该看到什么状态」，
// 这里解决「发生的事该告诉谁」。引擎给出默认的可见性划分，
// 调用方可以据此路由，而不必自己去记「查验结果只能给预言家」。
//
// 引擎内部事件（SET_NIGHT_KILL 等）返回空——它们不该出现在任何玩家面前。
func (e *Engine) AudienceOf(effect *Effect) []string {
	if effect == nil || isInternalEvent(effect.Type) {
		return nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	switch effect.Type {
	// 公开事件：死亡与出局全场可见
	case pb.EventType_EVENT_TYPE_KILL,
		pb.EventType_EVENT_TYPE_POISON,
		pb.EventType_EVENT_TYPE_ELIMINATE,
		pb.EventType_EVENT_TYPE_SHOOT,
		pb.EventType_EVENT_TYPE_GAME_STARTED,
		pb.EventType_EVENT_TYPE_GAME_ENDED:
		return e.state.allPlayerIDs()

	// 私密事件：只有行动者本人知道
	case pb.EventType_EVENT_TYPE_CHECK,
		pb.EventType_EVENT_TYPE_PROTECT,
		pb.EventType_EVENT_TYPE_SAVE,
		pb.EventType_EVENT_TYPE_SKIP:
		if effect.SourceID == "" {
			return nil
		}
		return []string{effect.SourceID}

	default:
		return nil
	}
}
