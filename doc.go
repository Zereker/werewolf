// Package werewolf 是狼人杀（Mafia）的规则引擎。
//
// 它只负责一件事：给定一副板子和一套规则，裁决每个阶段发生了什么，
// 以及每个人有权知道什么。计时、联网、房间、持久化都不在里面——
// 那些是使用者的事，见下文「边界」。
//
// # 起手
//
//	engine, err := werewolf.NewEngine(nil)   // nil 表示默认配置
//	engine.AddPlayer("w1", werewolf.RoleWerewolf)
//	engine.AddPlayer("s", werewolf.RoleSeer)
//	// ... 其余玩家
//	engine.Start()
//
//	engine.SubmitSkillUse(&werewolf.SkillUse{
//		PlayerID: "w1", Skill: werewolf.SkillKill, TargetID: "s",
//	})
//	effects, err := engine.EndPhase()   // 结算并流转到下一阶段
//
// 完整可运行的例子见 example/，其中 example/cli 是一个能真的从头玩完
// 一局的命令行主持台。
//
// # 一局游戏怎么推进
//
// 引擎是一台显式的阶段状态机。每个阶段收集技能，EndPhase 触发结算：
//
//	SubmitSkillUse  ->  Resolver.Resolve  ->  []*Effect  ->  applyEffect
//	   收集技能           裁决（纯函数）        状态变更的描述     唯一的写入点
//
// Resolver 拿到的是只读的 GameView，只能通过返回 Effect 表达状态变更。
// 这条约束由签名保证而非靠约定——状态的每一次改变都经由同一个写入点，
// 快照、回放、审计这些能力才成立。
//
// 阶段之间怎么流转由 GameConfig.Phases 声明；猎人开枪这类「死亡时触发」
// 的能力由 Resolver 产出 NewAbilityTriggerEffect，引擎会排队并自动流转
// 过去，它不需要认识任何具体角色。
//
// # 谁能知道什么
//
// 狼人杀真正难的部分是信息边界，所以引擎把它收在库内，分成两半：
//
//	Engine.PlayerView(id)     某个玩家有权知道的一切，可以原样发给他
//	Engine.AudienceOf(event)  一件事该发给哪些玩家
//
// 与之相对，PhaseInfo、PlayerInfo、WolfTeammates、NightKillTarget 是
// 上帝视角：调用方作为主持人需要它们来组织流程，但它们的内容
// 不可以整体转发给玩家。
//
// # 扩展新角色
//
// 内置的六个角色只是一套默认板子，不是能力上限。加入狼王、白痴、骑士
// 不需要 fork 这个库：
//
//	cfg.Phases[myPhase] = &werewolf.PhaseConfig{ ... }        // 声明阶段
//	engine, _ := werewolf.NewEngine(cfg,
//		werewolf.WithResolver(myPhase, myResolver))           // 注册解析器
//	engine.AddCustomPlayer("p1", myRole, camp, category)      // 阵营与类别
//
// 角色自身的状态走 Var：跟着玩家走一整局的用 PlayerVar（白痴翻没翻牌），
// 每回合清零的用 RoundVar（今晚谁被标记了）。读用 GameView.PlayerVar /
// RoundVar，写用 NewSetPlayerVarEffect / NewSetRoundVarEffect。
// 它们随快照走、回放能重建，因此 Resolver 可以保持无状态——
// 而无状态正是这个接口的要求。内置角色的药剂、守护记录、刀口、被守被救
// 都是同一件事，只是它们在 PlayerState / RoundContext 上有专门的字段。
//
// 角色额外让玩家看到什么（女巫的刀口、盗贼的底牌）由 RoleInfoProvider
// 回答，用 WithRoleInfo 注册，结果出现在 PlayerView.RoleInfo 与
// RolePhaseInfo.RoleInfo。内置女巫走的就是这条路，没有特权。
//
// 胜负条件由 VictoryChecker 决定，可用 WithVictoryChecker 换掉。
// 第三方阵营（丘比特的情侣）有自己的胜利条件，判定写死在引擎里的话
// 那类板子根本没有地方表达；包一层 DefaultVictoryChecker 即可在内置
// 规则之上再加一条。
//
// 自定义取值一律从 1000 起。这不只是「避免撞号」的建议——事件类型的
// 编号是分段的，1000 以上才是扩展的地盘：
//
//	  1 ..  99   引擎的外部可见事件，通过 OnEvent 推给调用方
//	100 .. 999   引擎的内部状态变更，不外发
//	1000 起      第三方扩展自己的事件类型，照常推给 OnEvent；
//	             AudienceOf 对它们回答「不知道」，路由由扩展自己决定
//
// example/extension 用白痴把这条路径走通，全程只用导出 API；
// extension_test.go 用狼王再走一遍死亡触发那条分支。
//
// # 边界：引擎不做什么
//
//   - 不计时。PhaseConfig.Timeout 只是建议值，什么时候调 EndPhase
//     完全由调用方决定；未就绪也不会被拒绝。
//   - 不联网、不做房间、不做匹配。
//   - 不做存储。Snapshot 导出局面、RestoreEngine 重建，存到哪是使用者的事。
//   - 不起 goroutine、不做回调调度。事件处理器在调用方的 goroutine 上同步执行。
//   - 不决定桌面规则。出局者是否翻牌、遗言怎么给，引擎不替调用方做主。
//
// # 并发
//
// Engine 的所有导出方法都可以并发调用。回调（OnEvent、OnMessage）
// 一律在释放引擎锁之后执行，因此在回调里回调 Engine 不会死锁。
// 日志与指标在构造时定下，此后不再改变。
//
// # 规则依据
//
// 规则以维基百科「狼人殺」条目为基准，逐条固化在 rules_test.go 里，
// 是一份可执行的规格说明。有争议的地方（同守同救是否致死、女巫能否
// 同夜双开药等）做成了 GameConfig 上的开关，默认取维基的表述。
package werewolf
