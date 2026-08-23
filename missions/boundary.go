package missions

import "github.com/Zereker/hiddenrole"

// boundary.go 一件事该告诉谁、谁能听到谁说话。
//
// 本包的信息边界比狼人杀简单得多：**桌面上发生的事几乎全是公开的**。
// 谁被提名、谁投了赞成还是反对、任务成了没成、有几张失败票——全场都看得到。
// 私密的只有两样：开局发的身份信息（走 RoleInfo 与 Teammates，不走事件），
// 以及「谁投了失败票」——而后者的实现方式是**根本不产出那条事件**。
func audience(event *hiddenrole.Event, view hiddenrole.GameView) ([]string, bool) {
	// 被否决的行动只有行动者本人需要知道。
	//
	// 这一条必须**先于**类型划分：好人误投失败被驳回，那条事件的类型
	// 是规则自己的 FAIL_REJECTED，只按类型分桶会把它广播出去，
	// 等于当场点名。
	if event.Canceled || event.Type == EventFailRejected {
		return actorOnly(event.SourceID, view), true
	}

	switch event.Type {
	case EventProposed, EventVote, EventTeamApproved, EventTeamRejected,
		EventLeaderChanged, EventMissionSucceeded, EventMissionFailed,
		EventHammerReached, EventAssassinated:
		return allIDs(view), true
	}
	return nil, false
}

func actorOnly(id string, view hiddenrole.GameView) []string {
	if id == "" {
		return nil
	}
	if _, ok := view.Player(id); !ok {
		return nil
	}
	return []string{id}
}

func allIDs(view hiddenrole.GameView) []string {
	out := make([]string, 0, len(view.AllPlayers()))
	for _, p := range view.AllPlayers() {
		out = append(out, p.ID)
	}
	return out
}

// speech 谁能听到谁说话。
//
// 这套规则整局都是公开讨论——没有狼人夜间私聊那种分频道的场面。
// 这也是内核的一处正面证据：SpeechProvider 换成「全场都听得到」
// 只是一个函数，不需要内核知道有没有「夜晚」这回事。
func speech(_ string, view hiddenrole.GameView) []string { return allIDs(view) }
