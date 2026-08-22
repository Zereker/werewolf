// Package main 演示怎么在不 fork 这个库的前提下加一个新角色。
//
// 前两个示例（example/cli 与 example/netserver）走的都是内置板子。
// 这一个专门走扩展那条路：自定义角色、包装内置解析器、自定义事件类型，
// 全程只用导出 API。
//
// 加的角色是白痴：被投票放逐时翻牌，不出局，此后失去投票权。
//
//	go run ./example/extension
package main

import (
	"fmt"
	"log"

	"github.com/Zereker/werewolf"
	"github.com/Zereker/werewolf/engine"
)

func main() {
	fmt.Println("=== 扩展新角色：白痴 ===")
	fmt.Println()

	eng := build(newIdiotRule(werewolf.NewVoteResolver()))

	// 第三方的事件也走 OnEvent。编号 1000 以上是扩展的地盘，
	// 引擎不认识它们，但也不替它们决定「不该外发」。
	eng.OnEvent(func(ev *engine.Event) {
		if ev.Type != eventRevealed {
			return
		}
		audience, known := eng.AudienceOf(ev)
		fmt.Printf("  [事件] %s 翻牌 -> 引擎认得这个类型吗: %v，受众: %v\n",
			ev.SourceID, known, audience)
		fmt.Println("         （引擎不认识第三方的类型，路由得由扩展自己决定）")
	})

	fmt.Println("【第一次投票：把白痴投出去】")
	toVote(eng)
	voteAll(eng, "idiot")
	show(eng, endPhase(eng))

	alive := aliveOf(eng, "idiot")
	fmt.Printf("  白痴还活着吗: %v（翻过牌: %v）\n\n", alive, revealedIn(eng, "idiot"))

	fmt.Println("【翻牌之后，白痴的票不再算数】")
	toVote(eng)
	// 全场都投 v1，唯独白痴投 w1。若他的票算数，w1 会有 1 票、v1 有 3 票，
	// 结果不变；所以改成让白痴的票成为决定性的一票：他和另外两人投 w1，
	// 其余投 v1，票数持平——如果他的票被丢掉，w1 就少一票。
	vote(eng, "w1", "idiot", "v2", "v3")
	vote(eng, "v1", "w2", "s", "g")
	effects := endPhase(eng)
	show(eng, effects)
	fmt.Printf("  w1 还活着吗: %v（白痴那一票没算，w1 是 2 票、v1 是 3 票）\n\n",
		aliveOf(eng, "w1"))

	fmt.Println("【存档与恢复：扩展的状态跟着一起回来】")
	demoRestore(eng)
}

// build 组一局带白痴的游戏。三步，全是导出 API。
func build(rule *idiotRule) *werewolf.Engine {
	// 1. 用配置声明这个角色参与的阶段。
	//    白痴不需要自己的阶段——它改的是投票的结果，所以只换投票解析器。
	cfg := werewolf.DefaultGameConfig()

	// 2. 构造时把解析器和白痴的初始状态一起给上。
	//    只能在构造时给：恢复出来的引擎已经在局中，那时再注册就晚了。
	//
	//    阵营与类别写在角色自己身上，不是入座时的参数——引擎不认识
	//    「白痴」，也就没有办法替它推导。类别决定屠边怎么算：白痴算神职。
	eng, err := werewolf.NewWith(cfg, werewolf.DefaultRules(),
		engine.WithResolver(werewolf.PhaseVote, rule),
		engine.WithRoleSetup(roleIdiot, engine.RoleSetupFunc(
			func(string, werewolf.RoleType) map[string]string {
				return werewolf.CampVars(werewolf.CampGood, werewolf.RoleCategoryGod)
			})))
	if err != nil {
		log.Fatalf("配置不合法: %v", err)
	}

	// 3. 入座。白痴与内置角色走同一个 AddPlayer，没有区别。
	if err := eng.AddPlayer("idiot", roleIdiot); err != nil {
		log.Fatal(err)
	}
	for id, role := range map[string]werewolf.RoleType{
		"w1": werewolf.RoleWerewolf,
		"w2": werewolf.RoleWerewolf,
		"s":  werewolf.RoleSeer,
		"g":  werewolf.RoleGuard,
		"v1": werewolf.RoleVillager,
		"v2": werewolf.RoleVillager,
		"v3": werewolf.RoleVillager,
	} {
		if err := eng.AddPlayer(id, role); err != nil {
			log.Fatal(err)
		}
	}
	if err := eng.Start(); err != nil {
		log.Fatal(err)
	}
	return eng
}

// demoRestore 存档、恢复。扩展的状态跟着一起回来，不需要额外做什么。
func demoRestore(eng *werewolf.Engine) {
	snap := eng.Snapshot()

	// 恢复时必须再把解析器给一遍——快照只记局面，不记规则。
	restored, err := engine.RestoreEngine(nil, snap,
		engine.WithResolver(werewolf.PhaseVote,
			newIdiotRule(werewolf.NewVoteResolver())))
	if err != nil {
		fmt.Printf("  恢复失败: %v\n", err)
		return
	}

	st := restored.Status()
	fmt.Printf("  局面: 第%d回合 %v\n", st.Round, st.Phase)
	fmt.Printf("  白痴翻过牌了吗: %v\n", revealedIn(restored, "idiot"))
	fmt.Println()
	fmt.Println("  这一项之所以能跟着回来，是因为它住在引擎里而不是解析器里：")
	fmt.Println("  写走 NewSetVarEffect(ScopeGame.Of(id), ...)，读走 GameView.Var。")
	fmt.Println("  解析器因此是无状态的——那正是 Resolver 接口要求的。")
}

// revealedIn 从引擎读这个扩展自己的状态。
func revealedIn(eng *werewolf.Engine, id string) bool {
	p, ok := eng.PlayerInfo(id)
	return ok && p.Vars[varRevealed] != ""
}

// ==================== 小工具 ====================

func toVote(eng *werewolf.Engine) {
	for st := eng.Status(); st.Phase != werewolf.PhaseVote && !st.Over; st = eng.Status() {
		if _, err := eng.EndPhase(); err != nil {
			log.Fatal(err)
		}
	}
}

func endPhase(eng *werewolf.Engine) []*werewolf.Effect {
	effects, err := eng.EndPhase()
	if err != nil {
		log.Fatal(err)
	}
	return effects
}

func vote(eng *werewolf.Engine, target string, voters ...string) {
	for _, v := range voters {
		if err := eng.SubmitSkillUse(&werewolf.SkillUse{
			PlayerID: v, Skill: werewolf.SkillVote, Targets: []string{target},
		}); err != nil {
			log.Fatalf("%s 投票失败: %v", v, err)
		}
	}
}

func voteAll(eng *werewolf.Engine, target string) {
	for _, id := range eng.AlivePlayerIDs() {
		if id == target {
			continue
		}
		if err := eng.SubmitSkillUse(&werewolf.SkillUse{
			PlayerID: id, Skill: werewolf.SkillVote, Targets: []string{target},
		}); err != nil {
			log.Fatalf("%s 投票失败: %v", id, err)
		}
	}
}

func show(eng *werewolf.Engine, effects []*werewolf.Effect) {
	for _, ef := range effects {
		fmt.Printf("  %s\n", describe(ef))
	}
}

func aliveOf(eng *werewolf.Engine, id string) bool {
	p, ok := eng.PlayerInfo(id)
	return ok && p.Alive
}
