// Package werewolf 是狼人杀（Mafia）的规则引擎，零依赖。
//
// 它只负责一件事：给定一副板子和一套规则，裁决每个阶段发生了什么，
// 以及每个人有权知道什么。计时、联网、房间、持久化都不在里面——
// 那些是使用者的事，见下文「边界」。
//
// 社会推理游戏容易做出来，难在做对。难的不是状态机，是信息边界：
// 女巫只在解药还在手上时看得到今晚的刀口；一次被否决的用毒只能给女巫
// 本人，否则她当场暴露；同守同救到底死不死取决于一条桌面规则。
// 这类判断错了不会崩，只会安静地产出一局不公平的游戏。
//
// # 内核与规则
//
// 这个包里其实是两层东西：一层通用内核，一层狼人杀规则。
//
// 内核不认识任何角色、阵营或死法。它知道的只有：玩家、阶段、效果、
// 四条状态原语，以及「有些事只该给某些人看」。狼人杀的六个角色、
// 九个阶段的默认板子、屠边屠城，全部由公开的扩展点组装出来——与第三方角色
// 走的是同一批入口，没有特权。
//
// 这是 dogfooding 的最强形式：内置角色如果能用公开 API 完整表达，
// 扩展性就被证明了；如果不能，缺什么当场暴露。这几版补上的
// WithRoleSetup、WithAudience、WithTeammates 都是这么找出来的。
//
// 两层是两个包：
//
//	github.com/Zereker/werewolf         这个包，狼人杀规则
//	github.com/Zereker/werewolf/engine  内核
//
// 接线的方向是「规则组装内核」，不是「内核认得规则」：engine.NewEngine
// 造出来的是一台什么都不认识的状态机——没有解析器、不会判出胜负、
// 不认得任何角色、不划分信息边界。狼人杀的那一整套由 Options 经公开选项
// 装上去，与第三方注册自定义角色走的是同一批入口。
//
// 这件事由**编译器**保证，不靠自觉：本包在内核之外，它能用的东西
// 使用者也能用。想验证的话，看 engine/types.go——内核的词汇表只有五个
// 非空取值：START、END、GOD、SKIP、ANNOUNCE，外加各类型的空零值。
// 「女巫」「狼人」一个都没有。
//
// 开一局只 import 本包就够：内核的一小部分名字在本包再导出了一遍
// （见 alias.go，二十来个），收录规则是「本包自己的导出 API 用得到的
// 才留」。它们是纯别名，werewolf.Effect 与 engine.Effect 是同一个类型。
//
// 一旦要**改**规则——自己写解析器、换胜负判定、接日志与指标、按错误码
// 分支、拆快照——那些名字在内核包，调用点上就会出现 engine. 这个前缀。
// 这不是遗漏，是想让边界在代码里看得见；本包自己的 resolver.go、
// rolesetup.go 就是这么写的。
//
// # 起手
//
//	g, err := werewolf.New(werewolf.DefaultRules())
//	g.AddPlayer("w1", werewolf.RoleWerewolf)
//	g.AddPlayer("s", werewolf.RoleSeer)
//	// ... 其余玩家
//	g.Start()
//
//	g.SubmitSkillUse(&werewolf.SkillUse{
//		PlayerID: "w1", Skill: werewolf.SkillKill, Targets: []string{"s"},
//	})
//	effects, err := g.EndPhase()   // 结算并流转到下一阶段
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
// 状态机认得的只有四条原语：改存活（NewSetAliveEffect）、写三种作用域的
// 变量、排队一个死亡触发。「狼刀」「放逐」「开枪」「守护」「解药」这些是
// 规则给「发生了什么」起的名字，状态机不认得——一个 KILL 效果单独发出去，
// 谁都不会死。规则要让人出局，就在它旁边产出一条 SET_ALIVE。
//
// 这样分是为了让规则可换：一套新规则不必来改状态机，就能表达自己的
// 死法、标记与道具。狼人杀在这件事上没有特权，它自己也是这么写的。
//
// 阶段之间怎么流转由 GameConfig.Phases 声明；猎人开枪这类「死亡时触发」
// 的能力由 Resolver 产出 NewAbilityTriggerEffect，引擎会排队并自动流转
// 过去，它不需要认识任何具体角色。
//
// # 谁能知道什么
//
// 这类游戏真正难的部分是信息边界，所以库把它收进来，分成两半：
//
//	Engine.PlayerView(id)     某个玩家有权知道的一切，可以原样发给他
//	Engine.AudienceOf(event)  一件事该发给哪些玩家
//
// 与之相对，PhaseInfo、PlayerInfo、WolfTeammates、NightKillTarget 是
// 上帝视角：调用方作为主持人需要它们来组织流程，但它们的内容
// 不可以整体转发给玩家。
//
// 具体的划分由规则给，不由内核给：一件事该告诉谁（AudienceProvider）、
// 谁和谁是一边的（TeammateProvider）、发言谁能听到（SpeechProvider），
// 三个都能整个换掉。「同伴」允许不对称——血染钟楼的恶魔认得爪牙，
// 反过来不成立。
//
// 内核在这一层只守一条底线，且不可配置：**自己的状态原语永远不外发**。
// 它们是状态机的记账，推给玩家等于把上帝视角直接发出去。
//
// # 扩展新角色
//
// 内置的六个角色只是一套默认板子，不是能力上限。加入狼王、白痴、骑士
// 不需要 fork 这个库：
//
//	cfg.Phases[myPhase] = &werewolf.PhaseConfig{ ... }        // 声明阶段
//	g, _ := werewolf.NewWith(cfg, werewolf.DefaultRules(),
//		engine.WithResolver(myPhase, myResolver),             // 注册行为
//		engine.WithRoleSetup(myRole, mySetup))                // 注册初始状态
//	g.AddPlayer("p1", myRole)                            // 入座，与内置角色同一个入口
//
// 阵营与角色类别写在角色自己的 setup 里（werewolf.CampVars），不是入座时
// 的参数：引擎不认识你的角色，也就没有办法替它推导；写在角色身上，
// 每一处入座都不会填错。
//
// 状态一律走 Var，一共三种作用域：跟着玩家走一整局的用 PlayerVar
// （白痴翻没翻牌、女巫的药），本回合有效且不属于任何人的用 RoundVar
// （今晚的刀口），「本回合标记了某个玩家」的用 PlayerRoundVar
// （今晚谁被守了、被救了、被毒了）。读用 GameView 上的同名方法，
// 写用 NewSetPlayerVarEffect / NewSetRoundVarEffect /
// NewSetPlayerRoundVarEffect。
// 它们随快照走、回放能重建，因此 Resolver 可以保持无状态——
// 而无状态正是这个接口的要求。内置女巫的两瓶药就存在 PlayerVar 里
// （VarWitchAntidote / VarWitchPoison），与第三方角色同一条路。
//
// 状态的初始值由 RoleSetup 在入座时发放，用 WithRoleSetup 注册。
// 女巫开局的两瓶药走的就是这条路，注册一个空的 setup 她就空手上桌——
// 引擎里再没有第二条给内置角色发状态的暗道。
//
// 角色额外让玩家看到什么（女巫的刀口与药剂存量、盗贼的底牌）由
// RoleInfoProvider 回答，用 WithRoleInfo 注册，结果出现在 PlayerView.RoleInfo
// 与 RolePhaseInfo.RoleInfo。存储（Vars）与投射（RoleInfo）分开是刻意的：
// 存储只有一种，谁都能写；给玩家看成什么样由角色自己决定。
//
// 胜负条件由 VictoryChecker 决定，可用 WithVictoryChecker 换掉。
// 第三方阵营（丘比特的情侣）有自己的胜利条件，判定写死在引擎里的话
// 那类板子根本没有地方表达；包一层 DefaultVictoryChecker 即可在内置
// 规则之上再加一条。
//
// 自定义取值直接用自己的字符串即可，比如 RoleType("KNIGHT")、
// EventType("IDIOT_REVEALED")。枚举的底层是字符串，不会与内置的撞号——
// 「自定义取值从 1000 起」那条旧约定不再需要，它自己也咬到过自己：
// 事件类型的内部段曾写成「>= 100」，于是第三方定义的每一个事件类型都被
// 判成内核的内部事件，扩展的事件根本发不出去。
//
// 内核只把自己那七个状态原语当内部事件，别的一律推给 OnEvent；
// 对不认得的类型，AudienceOf 回答「不知道」，路由由扩展自己决定。
//
// example/extension 用白痴把这条路径走通，全程只用导出 API；
// extension_test.go 用狼王再走一遍死亡触发那条分支。
//
// # 边界：引擎不做什么
//
//   - 不计时。PhaseConfig.Timeout 只是建议值，什么时候调 EndPhase
//     完全由调用方决定；未就绪也不会被拒绝。
//   - 不联网、不做房间、不做匹配。
//   - 不做存储。Snapshot 导出局面、Restore 重建，存到哪是使用者的事。
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
