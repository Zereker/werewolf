package werewolf

import ()

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
	PlayerID string    `json:"player_id"` // 视角所属玩家
	Round    int       `json:"round"`     // 当前回合
	Phase    PhaseType `json:"phase"`     // 当前阶段

	// Self 自己的信息：身份、阵营、存活、（女巫的）药剂
	Self SelfInfo `json:"self"`

	// Players 全场玩家的公开信息，按 ID 排序。
	// 身份只在对本视角公开时才填充（自己、狼队友）。
	Players []PublicPlayerInfo `json:"players"`

	// AllowedSkills 本阶段自己可以提交的技能，永不为 nil。
	// 不该自己行动时为空切片——这也是判断「轮到我了吗」的依据。
	AllowedSkills []SkillType `json:"allowed_skills"`

	// Teammates 狼队可见：其余狼队友的 ID。好人阵营恒为空。
	Teammates []string `json:"teammates,omitempty"`

	// RoleInfo 角色专属信息：这个角色额外让他看到的东西。
	//
	// 由角色自己的 RoleInfoProvider 回答（见 WithRoleInfo），引擎不认识
	// 任何具体角色。内置女巫的刀口与药剂存量都在这里（键见
	// RoleInfoKillTarget / RoleInfoAntidote / RoleInfoPoison）——它们此前
	// 是 PlayerView 与 SelfInfo 上的具名字段，等于内置角色比第三方角色
	// 多一等公民的待遇，而加一个角色不该要求改引擎。
	RoleInfo map[string]string `json:"role_info,omitempty"`
}

// SelfInfo 一名玩家对自己有权知道的全部信息。
//
// 刻意不复用上帝视角的 PlayerInfo：那个结构体带着 Protected
// （今晚是否被守卫守护），而「守卫守了谁」是守卫独占的信息——
// 被守的人一旦知道，就等于知道自己刀不死，也大幅缩小了守卫的范围。
// 一个字段的可见性差别不该靠调用方记得清空。
type SelfInfo struct {
	ID       string       `json:"id"`
	Role     RoleType     `json:"role"`
	Camp     Camp         `json:"camp"`
	Category RoleCategory `json:"category"`
	Alive    bool         `json:"alive"`
}

// PublicPlayerInfo 一名玩家对外公开的信息
type PublicPlayerInfo struct {
	ID    string `json:"id"`
	Alive bool   `json:"alive"`

	// Role 仅在该玩家的身份对本视角公开时填充，否则为 UNSPECIFIED。
	// 引擎默认只公开「自己」和「狼队友」——出局者是否翻牌属于桌面
	// 规则，由调用方决定，引擎不替它做主。
	Role RoleType `json:"role,omitempty"`
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
		PlayerID: playerID,
		Round:    e.state.Round,
		Phase:    e.state.Phase,
		Self: SelfInfo{
			ID:       self.ID,
			Role:     self.Role,
			Camp:     self.Camp,
			Category: self.Category,
			Alive:    self.Alive,
		},
		AllowedSkills: e.allowedSkillsForPlayer(playerID, self),
	}

	// 狼队互相可见（按阵营，狼王这类自定义狼队角色同样适用）
	revealed := map[string]bool{playerID: true}
	if self.Camp == CampEvil {
		view.Teammates = e.state.getWolfTeammates(playerID)
		for _, id := range view.Teammates {
			revealed[id] = true
		}
	}

	view.Players = e.publicPlayers(revealed)

	// 角色专属信息由角色自己回答
	view.RoleInfo = e.roleInfoFor(playerID, self.Role)

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

// allowedSkillsForPlayer 该玩家此刻能提交的技能，永不返回 nil。
//
// 「为空表示还没轮到我」在语义上与 nil 等价，但序列化出去一个是 []
// 一个是 null，同一个字段两种形状，调用方要分别处理。
// 调用前需持有 e.mu。
func (e *Engine) allowedSkillsForPlayer(playerID string, info PlayerInfo) []SkillType {
	// 死亡技能阶段只有触发者能行动
	if t, ok := e.state.peekTrigger(); ok && t.Phase == e.state.Phase {
		if t.PlayerID != playerID {
			return []SkillType{}
		}
		return e.allowedSkillsFor(info.Role)
	}
	if !info.Alive {
		return []SkillType{}
	}
	return e.allowedSkillsFor(info.Role)
}

// ==================== 效果的接收者 ====================

// AudienceOf 返回一件事应该发给哪些玩家。
//
// 这是配套 PlayerView 的另一半：视图解决「玩家该看到什么状态」，
// 这里解决「发生的事该告诉谁」。引擎给出默认的可见性划分，
// 调用方可以据此路由，而不必自己去记「查验结果只能给预言家」。
//
// 参数是对外的 Event 而不是内部的 Effect：这个问题问的是「外面的人
// 该看到什么」，而 OnEvent 推给调用方的正是 Event。手里拿着 Effect
// （EndPhase 的返回值）时用 Effect.ToEvent() 转一下。
//
// 引擎内部事件（SET_NIGHT_KILL 等）返回空——它们不该出现在任何玩家面前。
//
// 第二个返回值表示引擎是否认得这个事件类型。第三方 Resolver 可以产出
// 自定义类型的事件，引擎对它们的可见性无从判断，此时返回 (nil, false)：
// 调用方需要自己路由，而不该把「引擎不知道」当成「不给任何人看」。
func (e *Engine) AudienceOf(event *Event) ([]string, bool) {
	if event == nil {
		return nil, false
	}
	effect := event
	if isInternalEvent(effect.Type) {
		// 内部事件是引擎的状态变更，不给任何玩家看——这是明确的判断
		return nil, true
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	// 被否决的行动只有行动者本人需要知道，且必须先于类型划分判断。
	//
	// 「女巫想毒人但今晚已用过解药」产出的是一条 source=女巫 的 POISON，
	// 与结算阶段那条 source="" 的「某人毒发身亡」是同一个类型。
	// 只按类型分桶，前者会被当成公开死讯广播给全场，女巫当场暴露。
	if effect.Canceled {
		return e.actorAudience(effect.SourceID), true
	}

	switch effect.Type {
	// 公开事件：死亡、出局、投票结果全场可见
	case EventKill,
		EventPoison,
		EventEliminate,
		EventShoot,
		EventVoteTied,
		EventGameStarted,
		EventGameEnded:
		return e.state.allPlayerIDs(), true

	// 私密事件：只有行动者本人知道
	case EventCheck,
		EventProtect,
		EventSave,
		EventSkip:
		return e.actorAudience(effect.SourceID), true

	default:
		// 未知的外部类型：可能是第三方角色自定的事件
		return nil, false
	}
}

// actorAudience 只发给行动者本人。行动者为空或不在场上时无人可发——
// 让调用方拿到一个不在这局游戏里的 ID，只会变成一次投递失败。
// 调用前需持有 e.mu。
func (e *Engine) actorAudience(sourceID string) []string {
	if sourceID == "" {
		return nil
	}
	if _, ok := e.state.getPlayer(sourceID); !ok {
		return nil
	}
	return []string{sourceID}
}
