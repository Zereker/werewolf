package avalon

import (
	"strconv"

	"github.com/Zereker/werewolf/engine"
)

// resolver.go 四个阶段各自的结算。

// proposeResolver 队长提名任务队伍。
//
// 一支队伍要几个人就提交几次 PROPOSE——SkillUse.TargetID 是单个字符串，
// 表达不了「一次提名一支队伍」。只取队长本人的提交，按提交顺序去重，
// 多于所需人数的部分丢掉。
type proposeResolver struct{}

func (proposeResolver) Resolve(uses []*engine.SkillUse, view engine.GameView) []*engine.Effect {
	leader := leaderID(view)
	need := MissionSize(len(view.AllPlayers()), mission(view))

	// 一次提交带整支队伍。
	//
	// SkillUse 此前只能带一个目标，队长得提交 N 次——就绪判定因此说不清
	// 「还差几个人没提」，提名了 1 人（需要 2 人）之后就报 Ready=true。
	// 现在一次提交就是一支完整的队伍，就绪判定跟着说对话。
	//
	// 不检查提交者是不是队长：内核已经在 SubmitSkillUse 就拦下了非队长的
	// 提交（这个阶段的行动者由 SetActors 指定）。
	seen := map[string]bool{}
	var team []string
	for _, u := range uses {
		if u.Skill != SkillPropose {
			continue
		}
		for _, id := range u.Targets {
			if id == "" || seen[id] || len(team) >= need {
				continue
			}
			if _, ok := view.Player(id); !ok {
				continue
			}
			seen[id] = true
			team = append(team, id)
		}
	}

	// 先清掉上一次提名留下的标记。
	//
	// 被否决的提名现在直接跳回本阶段（不再绕经任务阶段），因此不会跨过
	// 回合边界，上一次的标记还在身上。这是「回合边界交给规则」之后规则
	// 要自己承担的那部分：内核不再替它清，什么时候清由它说了算。
	var effects []*engine.Effect
	chosen := map[string]bool{}
	for _, id := range team {
		chosen[id] = true
	}
	for _, id := range teamIDs(view) {
		if !chosen[id] {
			effects = append(effects, engine.NewSetPlayerRoundVarEffect(id, varOnTeam, ""))
		}
	}
	for _, id := range team {
		effects = append(effects,
			engine.NewEffect(EventProposed, leader, id),
			engine.NewSetPlayerRoundVarEffect(id, varOnTeam, engine.VarPresent))
	}
	// 点名任务阶段的行动者：只有这几个人能投成败。
	//
	// 名单在这里算出来、到下一个阶段才用——这正是 SetActors 要指定阶段
	// 而不是只作用于当前阶段的原因。
	return append(effects, engine.NewSetActorsEffect(PhaseMission, team...))
}

// teamVoteResolver 全员表决这支队伍。
//
// 票是公开的：条目里表决牌同时揭晓，谁投了什么全场都看得到。因此每一票
// 都产出一条公开事件——这与任务阶段正好相反，那里连谁投了失败都不能露。
type teamVoteResolver struct{}

func (teamVoteResolver) Resolve(uses []*engine.SkillUse, view engine.GameView) []*engine.Effect {
	need := MissionSize(len(view.AllPlayers()), mission(view))
	team := teamIDs(view)

	voted := map[string]bool{}
	approve, reject := 0, 0
	var effects []*engine.Effect
	for _, u := range uses {
		if u.Skill != SkillApprove && u.Skill != SkillReject {
			continue
		}
		if voted[u.PlayerID] {
			continue // 一人一票，以第一次为准
		}
		voted[u.PlayerID] = true
		if u.Skill == SkillApprove {
			approve++
		} else {
			reject++
		}
		effects = append(effects,
			engine.NewEffect(EventVote, u.PlayerID, "").WithData("approve", u.Skill == SkillApprove))
	}

	// 队伍人数不对（队长没提够、或者根本没提）一律按否决处理
	ok := len(team) == need && need > 0 && approve > reject

	// 队长每一轮都往下传一位，无论通过与否
	next := gameNum(view, varLeader) + 1
	effects = append(effects,
		setGameNum(view, varLeader, next),
		engine.NewEffect(EventLeaderChanged, "", ""),
		engine.NewSetActorsEffect(PhasePropose, leaderAt(view, next)))

	if ok {
		return append(effects,
			engine.NewEffect(EventTeamApproved, "", "").WithData("team", len(team)),
			engine.NewSetRoundVarEffect(varApproved, engine.VarPresent),
			setGameNum(view, varRejects, 0),
			engine.NewGotoPhaseEffect(PhaseMission))
	}

	n := rejects(view) + 1
	effects = append(effects,
		engine.NewEffect(EventTeamRejected, "", "").WithData("consecutive", n),
		setGameNum(view, varRejects, n))
	if n >= HammerRejections {
		// 连续五次否决，坏人直接获胜。胜负由 VictoryChecker 读这个数判定。
		effects = append(effects, engine.NewEffect(EventHammerReached, "", ""))
	}
	// 被否决就直接回提名，不再空转一次任务阶段。
	//
	// 这是内核把「下一步去哪」交给规则之后立刻兑现的：条件分支的结果由
	// 本阶段的结算算出来，静态的 NextPhase 表达不了。顺带把回合数也修对了
	// ——任务阶段声明了 EndsRound，空转一次就多推一个回合。
	return append(effects, engine.NewGotoPhaseEffect(PhasePropose))
}

// missionResolver 任务结算。
//
// 这里有两处阿瓦隆特有的信息约束：
//
//   - **只有队员能投**。内核判定行动者只看角色，队伍却是运行时定的，
//     因此这一步只能由解析器自己把非队员的提交丢掉。代价见 SCARS.md 第 1 条。
//   - **失败票不能记名**。全场只能知道「有几张失败票」，不能知道是谁投的。
//     实现上就是**不为每一票产出事件**，只产出一条带票数的聚合事件。
//     好人误投失败按成功计，且那条否决只有他自己看得到。
type missionResolver struct{}

func (missionResolver) Resolve(uses []*engine.SkillUse, view engine.GameView) []*engine.Effect {
	if !approved(view) {
		return nil // 上一轮表决没通过，这个阶段空转
	}

	// 不再检查「他在不在队伍里」：内核已经拦下了非队员的提交。
	acted := map[string]bool{}
	fails := 0
	var effects []*engine.Effect
	for _, u := range uses {
		if u.Skill != SkillMissionSuccess && u.Skill != SkillMissionFail {
			continue
		}
		if acted[u.PlayerID] {
			continue // 一人一票，以第一次为准
		}
		acted[u.PlayerID] = true
		if u.Skill != SkillMissionFail {
			continue
		}
		p, _ := view.Player(u.PlayerID)
		if !isEvil(p.Role) {
			// 好人不能投失败。这条否决只发给他本人——旁人连「有人试过」
			// 都不该知道，否则等于点名。
			effects = append(effects, cancel(
				engine.NewEffect(EventFailRejected, u.PlayerID, ""), "好人只能投成功"))
			continue
		}
		fails++
	}

	players := len(view.AllPlayers())
	m := mission(view)
	failed := fails >= FailsNeeded(players, m)

	if failed {
		effects = append(effects,
			engine.NewEffect(EventMissionFailed, "", "").
				WithData("mission", m).WithData("fails", strconv.Itoa(fails)),
			setGameNum(view, varFail, failures(view)+1))
	} else {
		effects = append(effects,
			engine.NewEffect(EventMissionSucceeded, "", "").WithData("mission", m),
			setGameNum(view, varSuccess, successes(view)+1))
	}
	effects = append(effects, setGameNum(view, varMission, m+1))

	// 好人凑满三次成功：把刺杀排进队列。
	//
	// 用的是内核那套「谁、去哪个阶段」的触发队列——它原本是为出局技能
	// 做的，语义却正好是这里要的，还顺带把胜负判定推迟到刺杀结算之后。
	if !failed && successes(view)+1 >= 3 {
		if ids := idsWithRole(view, RoleAssassin); len(ids) > 0 {
			effects = append(effects, engine.NewAbilityTriggerEffect(ids[0], PhaseAssassin))
		}
	}
	return effects
}

// assassinResolver 刺客指认梅林。
type assassinResolver struct{}

func (assassinResolver) Resolve(uses []*engine.SkillUse, view engine.GameView) []*engine.Effect {
	for _, u := range uses {
		if u.Skill != SkillAssassinate || u.Target() == "" {
			continue
		}
		p, ok := view.Player(u.Target())
		if !ok {
			continue
		}
		hit := p.Role == RoleMerlin
		return []*engine.Effect{
			engine.NewEffect(EventAssassinated, u.PlayerID, u.Target()).WithData("hit", hit),
			engine.NewSetGameVarEffect(varAssassinated, boolVar(hit)),
		}
	}
	// 没有指认视为刺杀落空
	return []*engine.Effect{
		engine.NewEffect(EventAssassinated, "", "").WithData("hit", false),
		engine.NewSetGameVarEffect(varAssassinated, "miss"),
	}
}

func boolVar(b bool) string {
	if b {
		return "hit"
	}
	return "miss"
}

func cancel(e *engine.Effect, reason string) *engine.Effect {
	e.Cancel(reason)
	return e
}
