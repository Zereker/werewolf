# 更新日志

本项目遵循 [语义化版本](https://semver.org/lang/zh-CN/)，破坏性变更在每个版本的开头单独列出。

> 发版由 GitHub Actions 完成：**Actions → Release → Run workflow**，填版本号即可。
> tag 与 Release 都由 workflow 创建，发布说明取自本文件对应的小节——
> 没有小节就发不出去。

> 公开的 tag 只有 `v1.0.0` 与 `v1.2.0`。`v1.0.0` 到 `v1.2.0` 之间的全部改动
> 都归入 `v1.2.0` 一节——对使用者而言，中间没有可取用的版本。

## 未发布（v2 开发中）

v2 的方向是把内核与狼人杀规则拆开（见下面「路线」一节）。这一节记录
朝那个方向走出的每一步；`go.mod` 的 `/v2` 后缀会在真正发版前一次性改掉。

### 信息边界也交给规则

**破坏性变更**：`GameView.AlivePlayers()` 现在保证按 ID 排序（此前是 map
遍历顺序，无序）。

内核此前认得三件狼人杀的事，全在「谁能知道什么」这一层：

| 内核里写着 | 它其实是 |
|---|---|
| `AudienceOf` 的类型表 | 查验只给预言家、狼刀全场可见 |
| `PlayerView.Teammates` | 同阵营互相可见，而「阵营」只有好人与狼人两值 |
| `MessageReceivers` | 夜里只有狼队能说话，白天全体能听 |

换一套规则这三样全不成立：阿瓦隆的梅林看得到坏人但坏人不知道他是谁，
血染钟楼的恶魔与爪牙互见是**单向**的，谁能在什么时候说话更是每套规则
自己的事。

现在三个问题都由规则回答：`AudienceProvider` / `TeammateProvider` /
`SpeechProvider`，对应 `WithAudience` / `WithTeammates` / `WithSpeech`。
狼人杀的那三份实现收进 `wolfboundary.go`，没有特权，可以整个换掉。
「同伴」允许**不对称**，内核不检查两边是否一致——它根本不知道阵营这个概念。

**内核在这一层只保留一条判断，且不可配置：自己的状态原语永远不外发。**
`SET_ALIVE` 这类效果是状态机的记账，推给玩家等于把上帝视角直接发出去。
一个「什么都给全场」的 provider 也打不开这个口子，
`TestAudienceOf_KernelPrimitivesAreNeverPublic` 守的就是这一条。

顺带修掉一个确定性隐患：`GameView.AlivePlayers()` 无序而 `AllPlayers()`
有序。这份名单会流进规则产出的效果里（发言受众、结算顺序），不排序的话
同一个局面两次结算的效果流不同，回放与逐字节比对就失去确定性。

三处队友判定（`PlayerView`、`PhaseInfo`、`WolfTeammates`）现在共用同一个
provider。上一版刚修过「狼王在 `PhaseInfo` 里拿不到队友」——那正是同一个
判定写了三遍、漏改一处的结果，现在结构上不会再有。

变异验证：拿掉内核对状态原语的拦截 → 3 条用例红（含随机对局）；
`PhaseInfo` 绕开 provider → `TestWithTeammates_Replaceable` 红；
发言范围写死回内核 → `TestWithSpeech_Replaceable` 红。

### 内核只剩四条状态原语

**破坏性变更**：`RoundContext` 上的 `KillTarget` / `ProtectedPlayers` /
`SavedPlayers` / `PoisonedPlayers` 四个字段与 `IsProtected` / `IsSaved` /
`IsPoisoned` 三个方法删除；`PlayerState` 的 `LastProtectedTarget` /
`LastProtectedRound`、`PlayerInfo.Protected`、`GameView.LastProtectedTarget`
删除；事件类型 `SET_NIGHT_KILL` / `CLEAR_NIGHT_KILL` / `SET_LAST_PROTECTED` /
`USE_ANTIDOTE` / `USE_POISON` 删除（编号 100..104 留空不再复用）；
**快照版本 7 → 8**。

`applyEffect` 是全局唯一的状态写入点，此前它认得十来种效果类型：

| 分支 | 它其实是 |
|---|---|
| `KILL` / `POISON` / `ELIMINATE` / `SHOOT` | 狼人杀有这四种死法 |
| `PROTECT` / `SAVE` | 今晚可以标记「被守」「被救」 |
| `SET_NIGHT_KILL` / `CLEAR_NIGHT_KILL` | 有一个叫「刀口」的东西 |
| `SET_LAST_PROTECTED` | 守卫不能连守 |
| `USE_ANTIDOTE` / `USE_POISON` | 女巫有两瓶药 |

每一条都是狼人杀的规则。换一套规则（阿瓦隆没有夜里的刀口，血染钟楼的
标记有十几种），它们一条都用不上；而新规则要表达自己的状态变更，
又只能回头来改这个 switch——这正是「加一个角色不该改引擎」在**规则**
这一层的同一个问题。

现在状态机只认四条原语：

- `NewSetAliveEffect(id, alive)` —— 唯一的生死原语
- `NewSetPlayerVarEffect(id, k, v)` —— 跟着玩家走一整局
- `NewSetRoundVarEffect(k, v)` —— 本回合有效，不属于任何人
- `NewSetPlayerRoundVarEffect(id, k, v)` —— **新增**，本回合标记了某个玩家
- （外加 `NewAbilityTriggerEffect`，排队一个死亡触发）

规则自己命名发生了什么，再产出原语真正改状态。**两个效果，两件事**：
前者给受众与效果流看，后者给状态机看。一个 `KILL` 效果单独发出去，
现在谁都不会死——`TestApplyEffect_RuleEventsDoNotTouchState` 就是这句话的
可执行说法。

狼人杀那一层随之收进 `nightstate.go`：刀口、被守、被救、被毒、守卫的
守护记录全都变成键名（`RoundVarKillTarget`、`PlayerRoundVarProtected` 等），
连守判定（`lastProtected`）也从内核搬进规则——「守卫不能连守」是狼人杀的
规则，不是状态机的事。

第三方角色在这件事上没有额外负担，也没有特权：`extension_test.go` 的狼王
开枪现在也要自己产出 `SET_ALIVE`，与内置的狼刀、投票放逐走的是同一条路。
改的时候它当场没打死人，测试直接红了。

**`example/extension` 又撞出一个缺口**：白痴否决的是 `ELIMINATE`，而人是被
旁边那条 `SET_ALIVE` 打死的——跑起来白痴当场出局。补了 `Effect.SetsAlive()`，
让扩展能认出致死的原语。改完之后白痴的拦截**与死因无关**了：同一段代码
挡得住狼刀、毒杀、枪口和任何第三方规则的死法，因为它们最终都走这一条。
这比原来只认识 `ELIMINATE` 更强，不是妥协。

变异验证：回合边界不清玩家标记 → 随机对局 `seed=0 step=6` 报错；
`SET_ALIVE` 不改状态 → 35 条用例红；把 `KILL` 重新接回状态机 →
`TestApplyEffect_RuleEventsDoNotTouchState` 红。

覆盖率 92.9% → 94.1%。

### 第五个扩展点：角色的初始状态

**破坏性变更**：`PlayerState` / `PlayerInfo` / `PlayerSnapshot` / `SelfInfo`
上的 `HasAntidote` 与 `HasPoison` 四处具名字段全部删除，女巫的药并入 `Vars`；
**快照版本 6 → 7**，不兼容旧存档。

v1.4.0 的说明里写着「引擎里已经没有第三方做不到、内置角色能做的事了」，
这句话当时就不准确。引擎里还剩最后一处因具体角色而分叉的逻辑：

```go
// state.go，addCustomPlayer 内
if role == RoleWitch {
    player.HasAntidote = true
    player.HasPoison = true
}
```

它的代价不是这三行本身，而是第三方角色**没有任何办法**给自己发初始状态——
骑士开局带一次决斗、摄梦人开局带两条命，都得改引擎才能表达。

- **新增 `WithRoleSetup(role, setup)`**，与 `WithResolver`、`WithVictoryChecker`、
  `WithRoleInfo` 同构。入座时问一次，返回的键值写进该玩家的 `Vars`。
  签名里刻意没有 `GameView`：入座发生在开局之前，初始状态只能由角色本身决定，
  不能取决于谁先入座、场上还有谁。需要看局面的初始化（丘比特连情侣、盗贼选底牌）
  是一个阶段，用 `Resolver` 做。
- **女巫的两瓶药走同一张表**（`builtinRoleSetup`），并从 `PlayerState` 的两个
  bool 字段搬进 `Vars`（键为 `VarWitchAntidote` / `VarWitchPoison`）。
  注册一个空的 setup，她就真的空手上桌、解药也用不出来——引擎里再没有
  第二条给内置角色发状态的暗道，测试里也确实是这么验的。
- **药剂存量改由 `RoleInfo` 投射**（`RoleInfoAntidote` / `RoleInfoPoison`），
  `SelfInfo` 上那两个具名字段删除。存储与投射从此是两件事：存储只有 `Vars`
  一种、谁都能写；给玩家看成什么样由角色的 `RoleInfoProvider` 决定。
  这与上一版把 `KillTarget` 改成 `RoleInfo` 是同一个动作，只是当时只做了投射，
  没做存储。
- **初始状态记进效果流**的入座那一条上，而不是回放时重新问一遍 `RoleSetup`。
  「女巫带着两瓶药入座」本来就是发生过的事，效果流记的就是这个。反过来做的话，
  回放方少传一个 `WithRoleSetup`，重建出来的角色就悄悄空着手——解析器漏传有
  `validateResolvers` 拦得住，这里拦不住，因为「这个角色本来就没有初始状态」
  与「你忘了传」在签名上无法区分。
- **新增 `PlayerInfo.Var(key)`**，省掉 nil map 判断。
- **随机对局覆盖到了这条路**：`extension_test.go` 的狼王现在开局带一发子弹
  （`varWolfKingGun`，经 `WithRoleSetup` 发放，开枪时用 `NewSetPlayerVarEffect`
  清掉），5000 局里 1484 局带着它跑。加这一发子弹时，`newWolfKingGame` 忘了
  注册 setup，狼王当场开不出枪——初始状态是真的参与规则判定，不是视图上
  多显示一行。
- `example/cli` 里主持台不再认识女巫：`if v.Self.Role == RoleWitch` 换成
  遍历 `RoleInfo`，认识的键给个中文说法，扩展角色自己定的键原样打出来。

变异验证：去掉入座时的状态发放，6 条新测试里 6 条报错；去掉效果流里的
初始状态记录，`TestRoleSetup_SurvivesReplayWithoutTheOption` 单独报错。

## v1.4.0 — 2026-08-22

这一版把扩展性补齐：加一个角色，不需要改引擎里任何一行。四个扩展点
（行为、状态、胜利条件、专属信息）内置角色与第三方走同一条路，内置的
没有特权，也都可以被换掉。

也是 v1.x 的最后一个功能版本。下一步是把内核与狼人杀规则拆开
（见「路线」一节），那是 v2 的事。

### 破坏性变更

- **枚举的 JSON 从编号改成名字。** 此前 `json.Marshal` 出来是 `{"role":2,"phase":21}`，
  每个客户端都得自己维护一张对照表——`example/netserver` 推给客户端的就是这个。
  现在是 `{"role":"WEREWOLF","phase":"NIGHT_GUARD"}`。第三方的自定义取值（1000 起）
  没有名字，仍按编号写；读的时候名字与编号都接受，不认识的名字直接报错而不是
  静默变成零值。**快照版本 3 → 4**，不兼容旧存档。
- **`AudienceOf` 的参数从 `*Effect` 改为 `*Event`。** 这个问题问的是「外面的人该看到
  什么」，而 `OnEvent` 推给调用方的正是 `Event`——此前想按受众路由推送，得手工把
  Event 拼回一个 Effect。手里拿着 Effect（`EndPhase` 的返回值）时用 `Effect.ToEvent()`。

- **事件类型的编号改成三段。** 内部段此前写成「≥ 100」，与「自定义取值从 1000 起」
  这条约定直接打架：第三方定义的每一个事件类型都会被判成引擎内部事件，
  于是白痴翻牌、狼王自爆这类本该全场可见的事情，扩展**根本发不出去**。
  现在 `1..99` 是引擎外部事件、`100..999` 是引擎内部状态变更、`1000` 起是扩展的
  地盘（照常推给 `OnEvent`，`AudienceOf` 回答「不知道」）。

- **快照版本 4 → 6**，`PlayerSnapshot` 与 `RoundCtxSnapshot` 各增加 `Vars`。

### 新增

- **`WithRoleInfo`：角色专属信息由角色自己回答。** 「谁额外看得到什么」此前是引擎里
  一个认得所有内置角色的 `switch`——狼人给队友、女巫给刀口，别的角色什么都没有。
  加一个盗贼（要看两张底牌）就得改引擎，而**加一个角色不该要求改引擎**。
  内置女巫现在走的就是这条路，可以被换掉；`PlayerView.KillTarget` 与
  `RolePhaseInfo.KillTarget` 两个具名字段随之改成 `RoleInfo` map。
- **`WithVictoryChecker`：胜负条件可以换了。** 判定此前写死在引擎里、只认好人与
  狼人两边，丘比特的情侣、第三方阵营这类板子根本没有地方表达。内置判定导出为
  `DefaultVictoryChecker`，包一层就能在原规则之上再加一条。
- **`RoundVar`：回合级的自定义状态。** 与 `PlayerVar` 的分工是「跟着玩家走一整局」
  与「每回合自动清零」。`RoundContext` 的刀口、被守、被救、被毒是同一件事，
  只是它们有专门的字段——`PendingTriggers` 的注释里已经记着这个教训
  （猎人专属字段改成队列），但当时只泛化了死亡触发那一项。
- **`PlayerVar`：第三方角色的状态有地方放了。** 读走 `GameView.PlayerVar`，
  写走 `NewSetPlayerVarEffect`，随快照走、回放能重建。此前扩展只能把状态藏在
  自己的 Resolver 里——而 `Resolver` 接口白纸黑字要求「只能通过返回 Effect 表达
  状态变更」，藏在字段里等于只能违反那条不变量，恢复出来的对局是错的还不报错。
  `PlayerInfo.Vars` 只出现在上帝视角，不进面向玩家的 `SelfInfo`。
- `example/extension`：第三个真实使用者，加了一个**白痴**——被投票放逐时翻牌、
  不出局、此后失去投票权。它的形状与狼王完全不同（不是「死后触发」而是
  「阻止一次死亡再改变往后的能力」），走的是包装内置解析器、否决效果、
  自定义事件类型这条路。上面那条事件编号的 bug 就是写它时撞出来的。
- `example/netserver`：TCP 长连接的服务端，库的第二个真实使用者。命令行主持台
  碰不到的那半边——事件推送、每条连接一份视图、并发、断线重连、超时真的触发——
  由它来压，七条端到端测试全在 `-race` 下跑。

## 路线：v2 把内核与规则拆开

到 v1.4.0 为止，引擎里已经没有「第三方做不到、内置角色能做」的事了，
但内核与狼人杀规则仍混在同一批文件里——`config.go` 一半是阶段机、一半是
9 个狼人杀阶段；`state.go` 一半是状态机、一半是女巫的药与守卫的记录。
粗算内核约 2700 行、狼人杀规则约 1000 行。

v2 要做的是把它们分开：内核不认识任何角色、技能、阶段的含义，狼人杀成为
用纯公开 API 组装出来的第一个规则包。这是 dogfooding 的最强形式——内置六
角色如果能用公开 API 完整表达，扩展性就被证明了；如果不能，缺什么当场暴露。

**这会改模块路径**：Go 的模块规则要求 v2 及以上带 `/v2` 后缀，
即 `github.com/Zereker/werewolf/v2`。发版 workflow 里有一道闸专门拦这个，
不改 `go.mod` 就发不出去。

第一步已经走了：角色的初始状态（见「未发布」一节）。挑它开头是因为它是
整个拆分的最小完整实例——一个内置角色的状态从引擎字段变成规则包写进内核
变量、再经通用投射出去。先把这一刀切干净，「类型化的外投放哪儿」这个问题
就在动包结构之前有了答案，比先搬文件再发现投射没地方放便宜得多。

## v1.3.0 — 2026-08-22

本版含破坏性变更。按 Go 的模块规则，v2 及以上需要更换模块路径（`/v2`），
本项目刻意不走那条路——模块路径保持 `github.com/Zereker/werewolf`，
破坏性变更放在 minor 版本里。升级前请通读下面这一节。

### 破坏性变更

- **移除 protobuf 依赖，改用原生 Go 类型。** 这个库从未真正序列化过 protobuf
  （没有 `Marshal`、没有 gRPC），protobuf 的全部作用是声明 6 个枚举和 1 个 struct。
  现在依赖归零，`go.sum` 也随之删除。

  ```go
  pb.RoleType_ROLE_TYPE_WEREWOLF       →  werewolf.RoleWerewolf
  pb.PhaseType_PHASE_TYPE_NIGHT_GUARD  →  werewolf.PhaseNightGuard
  pb.SkillType_SKILL_TYPE_ANTIDOTE     →  werewolf.SkillAntidote
  ```

  `pb.Event` 变成原生的 `Event`，字段 `SourceId`/`TargetId` 按 Go 约定改为
  `SourceID`/`TargetID`，并带上 json tag。枚举数值逐个保持不变，快照格式不受影响。
  `String()` 的输出去掉类型前缀（`PHASE_TYPE_NIGHT_GUARD` → `NIGHT_GUARD`）。

- **删除四个冗余的导出 API**，各有替代：

  | 删除 | 用什么代替 |
  |---|---|
  | `Engine.SetLogger` | `WithLogger` 构造选项 |
  | `Engine.SetMetrics` | `WithMetrics` 构造选项 |
  | `Engine.RegisterResolver` | `WithResolver` 构造选项 |
  | `NewGameError` | `WrapError`（且它会正确挂上哨兵，`errors.Is` 可比对） |

- `IsErrorCode` → `HasCode`，`ErrorCode(err)` → `CodeOf(err)`（与 `ErrorCode` 类型重名）。
- `AudienceOf` 返回 `([]string, bool)`，第二个值表示引擎是否认得该事件类型。
- `PlayerView.Self` 的类型从 `PlayerInfo` 改为 `SelfInfo`，不再包含只有上帝该知道的
  `Protected` 字段。
- `NewEngine` / `MustNewEngine` / `RestoreEngine` / `ReplayEngine` 接受构造选项。
- 快照版本 2 → 3（`PlayerSnapshot` 增加 `LastProtectedRound`）。

### 新增

- `EventVoteTied`：平票有了自己的事件类型（此前挂在 `UNSPECIFIED` 上，受众为空）。
- `PhaseReadiness.Optional`：列出「可以动但还没动」的人。`Ready`/`Pending` 只管必需
  行动，只看它们驱动游戏的话，默认配置下守卫、女巫、预言家一整局都不会被叫到。
- `PhaseStep.Group`：互斥备选组，猎人的「开枪」与「不开枪」提交任一即算完成。
- `GameConfig.PhaseTimeout(phase)`：取某个阶段的建议超时。
- `Event.Canceled` / `Event.Reason`：被规则否决的行动不再与成功的无法区分。
- `example/cli`：一个能真的从头玩完一局的命令行主持台。
- 包文档（`doc.go`）、基准测试、本更新日志。

### 修复

- **`PhaseInfo` 里的队友按角色判，自定义狼队角色拿不到。** `case RoleWerewolf` 只认
  内置狼人，经 `AddCustomPlayer` 加进来的狼王在主持人用来组织流程的那份名单里
  没有队友——而 `PlayerView` 与 `WolfTeammates` 两条路都是对的，只有这一份漏了。
  改成按阵营。
- **多女巫板子上，用掉解药的女巫仍能看到刀口。** 旧实现按「场上还有谁持有解药」
  判（`anyAliveWitchHasAntidote`），现在按人判。

- **快照漏掉 `LastProtectedRound`，存一次档守卫的连守限制就失效**——原引擎判连守
  取消、恢复后的引擎放行。这个字段是连守那轮加的，加进了 `PlayerState`、
  `PlayerSnapshot` 与 `restorePlayer`，唯独漏了 `snapshotPlayers`。
  随机对局的快照往返只比阶段与回合，两边照样能同步地走完一局，只是规则判定
  不一样了——现在改成逐字节比对两边导出的快照。

- **回合边界写死了守卫阶段。** 阶段环里没有它时（没有守卫的板子），回合数永远停在 1、
  回合上下文永不重置——女巫用掉的那瓶解药会一夜又一夜地把同一个人救回来。
- **第三方 Resolver 返回的切片里混进 `nil` 会 panic。**
- **`EndPhase()` 的返回值里没有 `GAME_ENDED`**，照着返回值路由的调用方不知道谁赢了。
- **被否决的用毒会广播给全场**，女巫当场暴露。
- **出局的女巫仍能读到今晚的刀口。**
- **被守护的玩家能从自己的视图里读出来。**
- `Validate()` 补上：漏填 `NextPhase`、越界的 `VictoryMode`、`UNSPECIFIED` 与具体角色
  的步骤重叠、跨角色的互斥备选组。
- 指向未配置阶段的死亡触发就地否决，而不是把游戏带进一个空阶段静默收场。
- `IsErrorCode`/`ErrorCode` 改用 `errors.As`，`GameError` 实现 `Unwrap`——此前有五个
  预定义哨兵从未出现在任何返回路径上。
- `PhaseInfo` 的行动者名单顺序不再随机。
- 毒杀结算按 ID 排序，效果流可确定地比对。
- 狼队判定改为按阵营，`AddCustomPlayer` 加入的狼王、狼美人现在真的算狼队。
- `WithData` 撞 nil map、`applyEffect(nil)` 的 panic。

### 测试

- 新增随机对局的不变量测试（`fuzz_test.go`）：随机板子、规则开关、胜负方式、阶段环
  与自定义角色，每一步核对 9 条不变量——视图一致性、快照往返、效果流回放、身份不泄漏、
  受众不外扩、名单顺序稳定、回合边界。5000 局约 5 秒。
- 覆盖率 91.4% → 94.2%，`golangci-lint` 覆盖全部代码（不再排除任何路径）。

## v1.2.0

### 破坏性变更

- 导出方法去掉 `Get` 前缀（`GetPhaseInfo` → `PhaseInfo` 等，共 16 个）。
- `NewEngine` 返回 `error`；`State` 收进包内不再导出。
- Resolver 拿到的是只读的 `GameView` 而非可变状态。

### 新增

- `Engine.PlayerView` / `Engine.AudienceOf`：信息边界收进库内。
- `Engine.Snapshot` / `RestoreEngine`：局面存档与恢复。
- `Engine.EffectLog` / `ReplayEngine`：效果流与回放。
- `Engine.PhaseReadiness`：还差谁行动。
- `VictoryMode`（屠边 / 屠城）与 `RoleCategory`（神职 / 平民 / 狼人）。
- 死亡触发抽象：引擎不再认识猎人，只认识「谁、去哪个阶段结算」。
- `WitchCanUseBothPotions` / `GuardSaveTogetherDies` 等规则开关。

### 修复

- 规则以维基百科「狼人殺」条目为基准逐条核对并固化为测试（`rules_test.go`）。
- 连守判定、猎人开枪的触发条件、屠边计数等多处与规则不符的判定。
- `State` 与 `Engine` 的嵌套双锁；回调改为在锁外执行。
- 主干上必然 panic 的 example；CI 增加实际运行示例的步骤。

## v1.0.0

首个版本：声明式阶段配置、六个内置角色、消息系统、猎人被动触发、Logger/Metrics 接口。
