// Package engine 是社会推理游戏的规则内核。
//
// 它不知道狼人杀是什么。它知道的是：有一批玩家，有一个阶段环，
// 每个阶段结束时问一下那个阶段的解析器「发生了什么」，然后把结果
// 折进状态。以及最难的那件事——谁有权知道什么。
//
// 一套具体的规则（角色、技能、死法、胜负、信息边界）由规则包提供，
// 全部经公开的构造选项装上来。github.com/Zereker/werewolf 是第一个
// 这样的规则包，它没有走任何后门，这件事可以查：本包的非测试源码里，
// RoleType 的取值一共两个（RoleUnspecified、RoleSystem），PhaseType 三个、
// SkillType 三个，全在 types.go 里。「女巫」「狼人」一个都没有。
//
// # 状态机认得的全部东西
//
//	SubmitSkillUse  ->  Resolver.Resolve  ->  []*Effect  ->  applyEffect
//	   收集技能           裁决（纯函数）        状态变更的描述     唯一的写入点
//
// Resolver 拿到的是只读的 GameView，只能通过返回 Effect 表达状态变更。
// 这条约束由签名保证而非靠约定——状态的每一次改变都经由同一个写入点，
// 快照、回放、审计这些能力才成立。
//
// 状态机认得两条原语：
//
//	NewSetAliveEffect              改存活
//	NewSetVarEffect(scope, k, v)   写一项自定义状态
//
// 作用域是一张 2×2 的表（时间尺度 × 有没有主人），四格由 ScopeGame /
// ScopeRound 叉乘 .Of(playerID) 得出，见 VarScope。
//
// 外加一条 NewDetourEffect，排一笔欠账：为了某个人，绕一趟某个阶段。
//
// 「狼刀」「放逐」「开枪」这些是规则给「发生了什么」起的名字，
// 状态机不认得——一个 KILL 效果单独发出去，谁都不会死。规则要让人出局，
// 就在它旁边产出一条 SET_ALIVE。两个效果，两件事：前者给受众与效果流看，
// 后者给状态机看。
//
// # 谁能知道什么
//
//	Engine.PlayerView(id)     某个玩家有权知道的一切，可以原样发给他
//	Engine.AudienceOf(event)  一件事该发给哪些玩家
//
// 具体的划分由规则给：AudienceProvider（一件事该告诉谁）、
// TeammateProvider（谁和谁是一边的，允许不对称）、
// SpeechProvider（发言谁能听到）。
//
// 内核在这一层只守一条底线，且不可配置：**自己的状态原语永远不外发**。
// 它们是状态机的记账，推给玩家等于把上帝视角直接发出去。
//
// # 写一个规则包
//
//	cfg := &engine.Config{StartPhase: myFirstPhase, Phases: ...}
//	e, err := engine.NewEngine(cfg,
//		engine.WithResolver(myPhase, myResolver),   // 这个阶段怎么结算
//		engine.WithRoleSetup(myRole, mySetup),      // 这个角色带着什么入座
//		engine.WithVictoryChecker(myChecker),       // 怎么算赢
//		engine.WithAudience(myAudience),            // 一件事该告诉谁
//		engine.WithTeammates(myTeammates),          // 谁和谁是一边的
//		engine.WithSpeech(mySpeech))                // 发言谁能听到
//
// 不给这些的话，造出来的引擎能推进阶段，但永远不会分出胜负、
// 也不认得任何角色——那正是「内核什么都不知道」的意思。
//
// 单元测试自己的解析器用 Board：手工摆一副局面，转成 GameView 喂给
// 解析器，再用 Board.Apply 把产出的效果折回去看局面变成了什么样。
//
// # 内核不替规则做的两个决定
//
// 一局游戏怎么推进，有两个决定**只有规则知道答案**，内核不猜：
//
//	下一步去哪个阶段    PhaseConfig.NextPhase 是默认出口，
//	                   规则可以用 NewGotoPhaseEffect 在结算时改写
//	这一步之后是不是新回合  由 PhaseConfig.EndsRound 声明
//
// 这两件事此前都是内核自己定的：出口查一张静态图，回合边界猜「绕回起始
// 阶段就算」。狼人杀里两个猜测都恰好成立（夜→昼→夜），换一套规则就不成立
// ——阿瓦隆每提名一次绕一圈，于是「回合」成了提名计数器，而「表决通过去
// 任务、否则回提名」这种分支静态图根本表达不了。
//
// 判据是一句话：**内核能不能在不知道这是什么游戏的情况下，独立判断这件事
// 对不对？**「状态改了没有」能判断，归内核；「现在是不是新回合」判断不了，
// 归规则。
//
// 出口的优先级：待结算的绕道队列 > GOTO_PHASE > NextPhase。触发排最前是
// 因为队列必须排空——胜负判定与回合边界都等着它。
//
// 交出决定权换回了可检查性：内核自己猜回合边界的时候没法检查猜得对不对，
// 规则声明出来之后 Config.Validate 反而查得动了——一个阶段都没声明
// EndsRound 的配置会在建局时被拒，而它的后果（回合状态永不重置）过去
// 只能等跑到半局才发现。
//
// # 扩展点不能回头找引擎
//
// 七个扩展点——Resolver、VictoryChecker、AudienceProvider、
// TeammateProvider、SpeechProvider、RoleInfoProvider、RoleSetup——
// 全部在引擎**持锁期间**被同步调用。实现里回调 Engine 的任何方法，
// 后果是**挂住**，不是报错：Go 的读写锁不可重入，那一局从此没有响应。
//
// 它们不需要回调：想知道的一切都在参数里。Resolver 与几个 provider 拿到
// 的 GameView 就是那一刻的完整局面；RoleSetup 连 GameView 都不需要，
// 因为入座发生在开局之前。签名是刻意收窄的——扩展点拿不到 *Engine，
// 要绕过这条约束得自己把引擎存进结构体，那是一个有意的动作。
//
// 要在回调里问引擎，用 OnEvent / OnMessage 的处理器：事件与消息都在
// **锁外**发布，处理器里调 AudienceOf、PlayerView、Snapshot 都是安全的
// 用法。把引擎接进一个服务端正是这么做的——收到事件，问一句「这该发给
// 谁」，再往那几条连接上写，见 example/netserver。这条性质由
// TestCallbacks_MayCallBackIntoTheEngine 盯着，它带超时兜底：真被挪进
// 锁内，那个测试会红，而不是让整套测试挂住。
//
// # 边界：内核不做什么
//
//   - 不计时。PhaseConfig.Timeout 只是建议值。
//   - 不联网、不做房间、不做匹配。
//   - 不做存储。Snapshot 导出局面、RestoreEngine 重建，存到哪是使用者的事。
//   - 不知道任何游戏的规则。那是规则包的事。
package engine
