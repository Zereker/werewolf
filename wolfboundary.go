// wolfboundary.go 狼人杀的信息边界。
//
// 三个问题的答案：一件事该告诉谁、谁和谁是一边的、发言谁能听到。
// 它们此前写在内核里，现在只是这套规则的默认实现——没有特权，
// 经 WithAudience / WithTeammates / WithSpeech 可以整个换掉。
//
// 这一层整体属于狼人杀规则包。

package werewolf

// builtinAudience 狼人杀的默认可见性划分。
var builtinAudience = AudienceFunc(wolfAudience)

// wolfAudience 一件事该告诉哪些玩家。
//
// 被否决的行动只有行动者本人需要知道，且必须**先于**类型划分判断：
// 「女巫想毒人但今晚已用过解药」产出的是一条 source=女巫 的 POISON，
// 与结算阶段那条 source="" 的「某人毒发身亡」是同一个类型。只按类型分桶，
// 前者会被当成公开死讯广播给全场，女巫当场暴露。
func wolfAudience(event *Event, view GameView) ([]string, bool) {
	if event.Canceled {
		return actorAudience(event.SourceID, view), true
	}

	switch event.Type {
	// 公开事件：死亡、出局、投票结果全场可见
	case EventKill,
		EventPoison,
		EventEliminate,
		EventShoot,
		EventVoteTied,
		EventGameStarted,
		EventGameEnded:
		return allPlayerIDs(view), true

	// 私密事件：只有行动者本人知道
	case EventCheck,
		EventProtect,
		EventSave,
		EventSkip:
		return actorAudience(event.SourceID, view), true

	default:
		// 不认得：可能是第三方角色自定的事件，路由交给调用方
		return nil, false
	}
}

// actorAudience 只给行动者本人。行动者不存在时谁都不给。
func actorAudience(sourceID string, view GameView) []string {
	if sourceID == "" {
		return nil
	}
	if _, ok := view.Player(sourceID); !ok {
		return nil
	}
	return []string{sourceID}
}

// allPlayerIDs 全场，含已出局的——死讯要让所有人知道。
func allPlayerIDs(view GameView) []string {
	all := view.AllPlayers()
	out := make([]string, 0, len(all))
	for _, p := range all {
		out = append(out, p.ID)
	}
	return out
}

// builtinTeammates 狼队互相可见。
var builtinTeammates = TeammateFunc(wolfTeammates)

// wolfTeammates 同为狼人阵营的其余玩家，按 ID 排序。
//
// 按**阵营**判而不是按角色：写成 `case RoleWerewolf` 的话，
// AddCustomPlayer 加进来的狼王、隐狼在队友名单里就是空的。
// 含已出局的狼队友——夜里睁眼时死掉的队友也还认得。
func wolfTeammates(playerID string, view GameView) []string {
	self, ok := view.Player(playerID)
	if !ok || self.Camp != CampEvil {
		return nil
	}

	out := make([]string, 0, 4)
	for _, p := range view.AllPlayers() {
		if p.Camp == CampEvil && p.ID != playerID {
			out = append(out, p.ID)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// builtinSpeech 狼人杀的发言范围。
var builtinSpeech = SpeechFunc(wolfSpeech)

// wolfSpeech 此刻这名玩家说话谁能听到。
//
// 夜里只有狼队内部能交流，白天全体能听，其余阶段说不了话。
// 出局的玩家不能发言——是否允许遗言属于桌面规则，由调用方决定。
func wolfSpeech(senderID string, view GameView) []string {
	sender, ok := view.Player(senderID)
	if !ok || !sender.Alive {
		return nil
	}

	switch view.Phase() {
	case PhaseNightWolf:
		if sender.Camp != CampEvil {
			return nil
		}
		// 含自己，方便调用方直接拿去广播
		out := make([]string, 0, 4)
		for _, p := range view.AlivePlayers() {
			if p.Camp == CampEvil {
				out = append(out, p.ID)
			}
		}
		return out

	case PhaseDay:
		out := make([]string, 0, len(view.AlivePlayers()))
		for _, p := range view.AlivePlayers() {
			out = append(out, p.ID)
		}
		return out

	default:
		return nil
	}
}
