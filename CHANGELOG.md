# 更新日志

本项目遵循 [语义化版本](https://semver.org/lang/zh-CN/)，破坏性变更在每个版本的开头单独列出。

> 发版由 GitHub Actions 完成：**Actions → Release → Run workflow**，填版本号即可。
> tag 与 Release 都由 workflow 创建，发布说明取自本文件对应的小节——
> 没有小节就发不出去。

> 公开的 tag 只有 `v1.0.0` 与 `v1.2.0`。`v1.0.0` 到 `v1.2.0` 之间的全部改动
> 都归入 `v1.2.0` 一节——对使用者而言，中间没有可取用的版本。

## 未发布

### 根包的再导出砍成一个刻意的小集合

**破坏性变更**：根包 `werewolf` 此前把内核的公开 API 整个再导出了一遍
（`alias.go` 约 250 行，一百多个名字）。现在只留二十来个。被移除的那些
仍然存在，位置变了：`import "github.com/Zereker/werewolf/engine"`。

收录规则只有一条：**根包自己的导出 API 用得到的名字，才留在根包**。

| 留下 | 去内核取 |
|---|---|
| `PhaseType` `RoleType` `SkillType` `EventType` `Camp` | `Event` `Message` `PlayerView` `PlayerInfo` `PhaseInfo` `PhaseReadiness` … |
| `PhaseStart` `PhaseEnd` `RoleGod` `SkillSkip` `SkillAnnounce` `VarCamp` `VarPresent` | `NewEngine` `MustNewEngine` `RestoreEngine` `ReplayEngine` |
| `GameConfig` `PhaseConfig` `PhaseStep` | 八个 `With*` 选项与 `Resolver` `RoleSetup` `AudienceProvider` 等扩展点类型 |
| `Engine` `EngineOption` `SkillUse` `GameView` `Effect` `Snapshot` | 六个 `New*Effect` 构造函数 |
| | `Logger` `Metrics` `Field` 与字段助手 |
| | 全部错误码、哨兵错误、`WrapError` / `CodeOf` / `HasCode` |
| | `Board` `NewGameView` `Seat` `Mark`（解析器单测） |
| | 快照子结构与 `SnapshotVersion` |

理由：那份完整镜像等于宣称「两层拆分与使用者无关」，可它恰恰是这个库
最想说的事。砍完之后，**开一局狼人杀仍然只 import 根包**，而一旦要改
规则——自己写解析器、换胜负判定、接日志、按错误码分支——调用点上就会
出现 `engine.` 这个前缀。边界因此在代码里看得见，不只在文档里。

规则包自己带头这么写：`resolver.go`、`rolesetup.go`、`victory.go`、
`wolfboundary.go` 全部改成显式的 `engine.` 调用。三个 example 同理。

没有任何行为变化：全部是别名的增删与调用点的限定符。验证方式是把两个
确定性示例（`example`、`example/extension`）改前改后的输出逐字节比对，
完全一致；`make check` 与 5000 局随机对局照常通过，覆盖率仍是 94.6%。

### 修掉 example/cli 的 `-seed` 复现不了一整局

比对示例输出时发现的：`-seed` 只喂给了发牌，托管（`auto`）挑技能与目标
走的是全局 `rand`。同一个种子跑两次结果不一样——拿它复现一个 bug 是
复现不出来的。现在牌桌持有那个随机源，发牌与托管共用，同种子逐字节可复现
（连跑三次比对确认）。

### 内核有了自己的 README

新增 [`engine/README.md`](engine/README.md)：内核是什么、唯一的写入点、
四条状态原语、信息边界、八个扩展点、怎么用 `Board` 单测自己的解析器、
内核不做什么。附一套两页纸的完整规则（红蓝公投）作为可运行的最小例子——
文中的两段代码都是先跑通再抄进来的。

顺带修掉几处不实的说法：README 与 `doc.go` 里「把 grep 指向 `engine/`，
那里没有一个『女巫』『狼人』这样的取值」是查不实的——测试夹具与注释里
就有。换成能查的说法：内核的词汇表只有五个非空取值（`START` `END`
`GOD` `SKIP` `ANNOUNCE`），全在 `engine/types.go` 里。`WithRoleSetup`
的文档示例还停留在整数枚举时代（`RoleType = 1001`），一并改掉。README
里那段存档恢复的示例引用了一个从未定义的 `config` 变量，编译不过——
现在它把配置留在手上再传回去，正好把「配置必须与保存时一致」演示出来。
README 的两段完整示例这次是真的跑过的。

### 清掉三笔为兼容留下的折中

拆包时有几处是为了「不破坏刚发的 v1.5.0」才留的折中。这个库目前没有已知的
引用者，那个顾虑不成立，清掉：

- **`RoleCategory` 移出内核。** 当初留在内核是为了不删 `SelfInfo.Category`——
  我当时就写明了「这是个折中，不是纯粹解」。神职/平民是狼人杀为了屠边判定
  才需要的细分，内核只该认「这名玩家站哪一边」（`VarCamp`）。现在
  `RoleCategory` 与 `VarCategory` 整个属于规则包，`SelfInfo.Category` 删除。
- **`GameConfig` 在内核里改名 `Config`。** 它配的是阶段机，不是「一局游戏」，
  名不副实。根包仍叫 `GameConfig`（那边还有一个 `Rules` 要区分），
  是同一个类型的别名。
- **`VictoryMode` 改成字符串。** 其余枚举上一版全改了，只剩它是 `int + iota`。
  顺带修掉一个真问题：**零值恰好是屠边**，「没填」与「选了屠边」在结构体里
  长得一模一样，填错了也发现不了。现在零值是空串，`Rules.Validate` 会拦下来，
  测试也跟着补了这一条。

### 内核与规则物理拆成两个包

**破坏性变更**：`Engine.WolfTeammates(id)` 更名为 `Engine.Teammates(id)`；
`Engine.NightKillTarget()` 从方法变成包级函数 `werewolf.NightKillTarget(e)`；
`NewEngine(nil, ...)` 不再回退默认配置而是报错；`GameConfig.StartPhase`
成为必填项；`PhaseStep` 增加 `AllowDeadTarget`；快照格式未变。

上一版做到的是**逻辑**分离——内核不装任何狼人杀默认值，规则经公开选项
装上去。但那件事当时靠自觉：`Options()` 用的还是同包内的未导出符号，
「规则只用公开 API」没有任何东西强制。

现在是两个包：

| 包 | 是什么 | 行数 |
|---|---|---|
| `github.com/Zereker/werewolf/engine` | 内核 | ~2500 |
| `github.com/Zereker/werewolf` | 狼人杀规则 | ~1900 |

**由编译器保证。** 想验证的话把 grep 指向 `engine/`：非测试代码里
没有一个 `WEREWOLF`、`NIGHT_WITCH` 这样的取值，也没有一句话认得
「女巫能不能自救」。

`go get` 的路径不变——子包不需要 `/vN` 后缀，那只跟主版本走。

#### 使用者基本不受影响

内核的公开 API 在根包全部再导出了一遍（`alias.go`）：`werewolf.Effect`
与 `engine.Effect` 是**同一个类型**，不需要转换，也不必 import 两个包。
写自己的规则包时直接 import `werewolf/engine`。

拆包只动了两个名字，都是因为它们是狼人杀的说法却挂在内核类型上：

- `Engine.WolfTeammates(id)` → `Engine.Teammates(id)`。它本来就走
  `TeammateProvider`，与「狼」无关，名字是旧的。
- `Engine.NightKillTarget()` → `werewolf.NightKillTarget(e)`。
  `Engine` 住在内核里，本包没有办法给别人的类型加方法。
  等价写法：`e.RoundVar(werewolf.RoundVarKillTarget)`。

曾想过让规则包套一层自己的 `Engine` 好把这两个留成方法。没有那么做：
两个同名类型互相不能赋值，使用者迟早会在 `RestoreEngine` 的返回值上
撞到，那比改两个调用点糟糕得多。

#### 拆的过程中补上的内核 API

不是为了让测试编译过——每一个都是规则包作者真的会撞到的需求：

- **`Board` / `NewGameView` / `Board.Apply`**：手工摆一副局面，转成
  `GameView` 喂给自己的解析器，再把产出的效果折回去看局面变成了什么样。
  没有它，规则的解析器就只能整局跑起来才测得动——那测的是集成，
  不是这个解析器本身。
- **`Engine.Apply(effects...)`**：直接施加效果，绕开阶段结算。宿主真的
  会遇到「玩家掉线判死」「管理员踢人」这类不属于任何阶段的状态变更。
  它走的仍是同一个写入点，因此存档与回放不会失真。
- **`Engine.View()`**：当前局面的只读视图。宿主想自己算一次什么时用它。
- **`Engine.Winner()`**：这局的赢家。谁赢是结束那一刻定下的事实，
  此后不再变——之后换掉判定器也不会改写已经结束的这一局。
- **`Engine.RoundVar(key)`**：读本回合的一项状态。规则用它提供自己的
  便利读法。

#### 内核补齐的两处

- **`PhaseStep.AllowDeadTarget`。** 「解药可以指向已出局的玩家」此前是
  内核校验里的一句 `use.Skill != SkillAntidote`——内核认得解药。
  现在它是规则声明出来的数据。
- **`GameConfig.StartPhase` 必填。** 留空此前会退回 `NIGHT_GUARD`，
  而那是狼人杀的第一个阶段。内核没有资格替任何规则挑一个默认值，
  `NewEngine(nil)` 同理，现在直接报错。

#### 覆盖率的统计口径

拆包之后 `go test -cover ./engine` 只有 39.2%——内核大部分代码由规则包的
测试驱动，而那是另一个包了。这是**统计口径的假象**，不是覆盖真的掉了。
CI 与 `make test-cover` 现在都按两个库包合并统计（`-coverpkg`），
数字是 **94.5%**，与拆包前一致。示例不计入：它们是使用者，不是被测对象。

## v1.5.0 — 2026-08-22

这一版把**通用内核**与**狼人杀规则**分开了：引擎的代码路径里没有一处认得
具体角色、阵营或死法，狼人杀的一整套由 `werewolf.Options` 经公开选项装上去——
与第三方注册自定义角色走的是同一批入口，没有后门。

**这是 API 的冻结点。** 本版含大量破坏性变更（下面逐条列出），此后公开 API
是承诺：真要再破坏时，才付更换模块路径的账。

**模块路径不变**，`go get github.com/Zereker/werewolf` 照旧。这是刻意的：
Go 的模块规则要求主版本 ≥ 2 的模块带 `/vN` 后缀，而那会让每个使用者的
import 路径都变一次。既然这个库目前没有已知的引用者
（pkg.go.dev: "No known importers"），把破坏一次付清、留在 v1 线上，
比让所有人换路径划算。往后要拆的**包**（`werewolf/engine`）是子包，
子包不需要 `/vN`。

### 接线倒过来：规则组装内核，不是内核认得规则

**破坏性变更**：`Resolver.Resolve` 去掉 `config *GameConfig` 参数；
`GameConfig` 上的 7 个规则开关搬到新的 `Rules` 上；
`NewGuardResolver` / `NewWitchResolver` / `NewNightResolveResolver` 现在收一个
`Rules`；新增 `New` / `NewWith` / `MustNew` / `MustNewWith` / `Restore` / `Replay`
作为狼人杀的组装入口。

`NewEngine` 此前会替你装上狼人杀的一整套：九个阶段的解析器、屠边判定、
受众划分、队友判定、发言范围、六个角色的初始状态。也就是说**内核认得规则**。

现在它造出来的是一台什么都不认识的状态机，狼人杀的一切由 `Options(rules)`
经公开选项装上去——与第三方注册自定义角色走的是同一批入口，没有后门：

```go
// 这就是 werewolf.New 的全部内容
NewEngine(DefaultGameConfig(), append(Options(rules), extra...)...)
```

- **`Resolver.Resolve` 少一个参数。** 解析器的配置是「它是什么」的一部分，
  本来就该在构造时定死，而不是每次结算重新递一遍整个 `GameConfig`。
  第三方解析器也不用再从一个自己看不懂的结构体里挑字段。
- **`GameConfig` 只剩阶段机的配置**（起始阶段、阶段图、建议超时）。
  「女巫能不能自救」搬进 `Rules`——内核不该认得女巫。
- **`VictoryMode` 的校验跟着搬进 `Rules.Validate`。**
- **内核的缺省胜负判定是「永不结束」**（`neverEnds`），不是 nil。
  一台只装了内核的引擎应该能推进阶段、只是永不分出胜负，而不是在第一次
  `Start` 空指针崩掉——这个洞是拆分过程中撞出来的。

#### 一条守着这件事的测试

变异验证时发现一个空白：把内核的缺省判定换回 `DefaultVictoryChecker`，
**一条测试都不会红**——默认规则下行为完全一样。于是补了
`TestBareEngine_KnowsNothing`：逐项断言裸内核没有解析器、不会判出胜负、
不认得任何角色、不划分信息边界。再做同样的变异，它会红。

`TestWerewolfOptions_GoThroughThePublicDoor` 从另一头比：装上 `Options` 之后
有的东西，正是内核缺的那些，中间没有第二条路径。

变异验证：内核装回狼人杀胜负判定 → 2 条红；内核装回受众划分 → 1 条红；
`newPhaseManager` 装回九个解析器 → 1 条红。

### 枚举改成字符串，`enumjson.go` 整个消失

**破坏性变更**：`PhaseType` / `RoleType` / `SkillType` / `EventType` /
`ErrorCode` 的底层从 `int32` 改为 `string`（`Camp` / `RoleCategory` 上一步已改）；
事件类型的三段编号约定取消；**快照版本 9 → 10**。

编号是 protobuf 时代留下的：那时它们由 `.proto` 生成，编号进了线格式。
protobuf 拆掉之后编号就只剩负担——快照按名字写，日志按名字打，于是每个类型
都得额外挂一张「编号到名字」的对照表和一对 JSON 方法，**127 行代码只为把值
翻译回它本来的样子**。

名字直接就是值之后，那些全部消失：

- **删除 `enumjson.go`**（127 行）。JSON 不再需要任何自定义的
  Marshal/Unmarshal——`{"role":"WEREWOLF"}` 是类型本身的形态。
- **`String()` 从查表变成一行**，五张 names 对照表一起删掉。
- **零值即「未指定」。** 整数时代零值是 `0`，而 `0` 恰好也是 UNSPECIFIED 的
  编号——两者相等是巧合，不是设计。现在零值是空串，而空串就是 UNSPECIFIED 本身。
- **「自定义取值从 1000 起」这条约定不再需要。** 字符串不会撞号，第三方用
  `RoleType("KNIGHT")` 即可，与内置取值没有身份差别。
- **内部事件的判定从编号区间改成一张表。** 那条旧约定自己咬到过自己：内部段
  写成「>= 100」，而第三方取值从 1000 起，于是**扩展定义的每一个事件类型都被
  判成内核的内部事件，扩展的事件根本发不出去**。现在内核只认自己那七个原语。

快照的**线格式基本没变**（枚举从 v4 起就按名字写），唯一的区别是第三方的
自定义取值：此前没有名字、按编号写（`"role":1000`），现在与内置的一样是名字
（`"role":"WOLF_KING"`）。这就是 v9 到 v10 的全部内容。

#### 一处真实的取舍

JSON 反序列化时不再校验名字。此前不认识的名字会直接报错，现在
`{"role":"WEREWOLFF"}` 会安静地变成一个合法但无人认得的角色。

这不是疏忽，是**内核不可能知道有效集合**——第三方的角色就是任意字符串。
可观察的后果有限：没有登记 `RoleSetup` 的角色不属于任何阵营，也就不参与胜负
计数，`RestoreEngine` 对缺解析器的自定义阶段仍然会报错。

#### 穷尽性检查换了实现

`TestAudienceOf_CoversEveryPublicEvent` 此前遍历「编号到名字」那张表，
保证新增事件类型时不会忘了划分受众。表没有了，改成**直接从 `event.go` 的源码
扫出全部取值**——手写一份清单挡不住「新加了类型但忘了同步清单」，而那恰恰是
这个测试要挡的东西。变异验证过：往 `event.go` 里加一个新事件类型不划分受众，
测试当场报 `外部事件 NEW_THING 没有划分受众`。

变异验证：把第三方事件重新判成内部事件（复现旧 bug）→ 4 条用例红。

覆盖率 94.8% → 94.9%。

### 开源门面

代码之外的东西，为对外做准备：

- **`LICENSE` 换成 MIT。** 此前 `LICENSE` 文件是 Apache-2.0，README badge 写的是
  MIT，Apache 附录里的 Copyright 行还是模板占位。授权是一个项目唯一不能含糊的事。
- **新增 `README.en.md`。** 不是全文翻译，是一份面向两类使用者的门面：做狼人杀
  产品的，和做 LLM 社会推理 benchmark 的（后者正是现在真实存在的需求——那批项目
  每一个都自己糊了一份规则引擎，而它们要的确定性、逐玩家信息边界、可回放，
  这个库都有）。
- **新增 `CONTRIBUTING.md`。** 写下了这个仓库实际在用的那条规矩：**测试通过不等于
  测试有用**，行为改动要做变异验证（把改动反向去掉，确认测试真的会红），
  结果写进提交信息，好让别人能核对而不是只能相信。
- **`Makefile` 与 CI 对齐**，补上 `lint` / `vet` / `fmt-check` / `race` /
  `examples` / `check`。此前 CONTRIBUTING 里提到的命令 Makefile 里根本没有。
- **两份 README 与 `doc.go` 都加了项目状态**：当前版本是什么、还差什么、
  什么时候起 API 是承诺。

顺带纠正一句自己写错的话：`doc.go` 里原本写「内核与规则虽在同一个包，但依赖方向
已经是单向的」——查了一下**不成立**。行为确实分干净了，但接线还是反的：`NewEngine`
直接装上狼人杀的默认实现，`phase.go` 里还有一张阶段到解析器的表。两份 README 与
`doc.go` 现在都照实说明拆到了哪一步，以及接下来要把接线倒过来。

### 阵营与角色类别下放到规则包

**破坏性变更**：`Engine.AddCustomPlayer` 删除，入座只剩 `AddPlayer(id, role)`；
`PlayerState` / `PlayerInfo` / `PlayerSnapshot` 上的 `Camp` 与 `Category`
字段删除；`CampOf` / `CategoryOf` 删除；`ErrNoWerewolf` / `ErrNoGoodPlayer`
删除，改为 `ErrBoardAlreadyDecided`；`Camp` 的底层从 `int32` 改为 `string`；
`RoleCategory` 同理；**快照版本 8 → 9**。

内核此前知道「一局游戏分好人与狼人两边、好人还分神职与平民」。那是狼人杀
的分法：阿瓦隆是正义与邪恶，血染钟楼还有单独结算的旅行者。

现在阵营与类别只是玩家身上的两项状态（`VarCamp` / `VarCategory`），由角色的
`RoleSetup` 在入座时发放——与女巫的两瓶药同一份存储、同一条写入路径。

- **`Camp` 变成内核的不透明标签**，底层是字符串，只留一个 `CampUnspecified`
  空值。`CampGood` / `CampEvil` 是狼人杀定义的两个（`wolfcamp.go`），
  和第三方定义的「情侣阵营」没有身份差别——「自定义取值从 1000 起」这类
  避让约定对它不再需要。
- **入座只剩一个入口。** `AddCustomPlayer` 多两个参数专供扩展角色显式给出
  阵营与类别，于是「这个角色属于哪一边」的答案取决于调用方在**每一处入座**
  时记得填对。现在它写在角色自己身上，填不错也漏不掉。
- **`Start` 的板子校验改成问胜负判定器。** 它此前写成「必须有狼人、必须有
  好人」，而内核不认识阵营。判定器本来就是「这一刻分出胜负了吗」的唯一权威，
  开局前问一次即可——顺带覆盖了原来漏掉的情况：**屠城模式下 2 狼对 2 好人，
  第一次结算就是狼人胜，旧校验放它过关**。
- 没有登记 `RoleSetup` 的角色不属于任何阵营，也就不参与胜负计数。这是刻意的：
  内核没有默认阵营可给，而「悄悄算作好人」比「不算」更难查。
- `enumjson.go` 少了两个类型的编号对照表——字符串枚举的名字本身就是值。

顺带修掉随机对局里一个会被忽略的损失：新的板子校验会拒掉 8% 的随机板子
（屠城模式下狼人不比好人少），而统计里看不出来，「跑了 5000 局」就成了一句
不准确的话。生成器现在补平人数，5000 局全部成局。

变异验证：内置角色不发阵营 → 148 条用例红；`Start` 不再校验板子 → 2 条红；
入座不发初始状态 → 145 条红。

覆盖率 94.4% → 94.8%。

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

下一步是把内核与狼人杀规则拆开（见 v1.5.0）。

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
