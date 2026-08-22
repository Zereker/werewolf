# 对照：boardgame.io 的引擎，和我们的

写这份文档的起因是一句话：查完三个同类框架之后发现，**「谁可以行动」这件事，
没有一个把它写进静态配置，只有我们这么做**。那句话值得追问——是我们想得不一样，
还是我们根本没想过。

于是把 [boardgame.io](https://github.com/boardgameio/boardgame.io) 的引擎整个读了一遍，
逐条对照。选它是因为它和我们做的是同一件事（回合制多人游戏的状态内核 + 信息边界），
而且开源、文档与源码都可查。

读的是这几个文件，结论都能指回去：

- `src/core/turn-order.ts` —— 行动者集合
- `src/core/flow.ts` —— 阶段流转与行动权判定
- `src/core/reducer.ts` —— 写入路径与历史
- `src/plugins/random/random.ts` —— 随机
- `docs/documentation/{stages,phases,secret-state}.md`

---

## 一张表

任何这类引擎都要回答同一批问题。两边的答案并排：

| 问题 | boardgame.io | 我们 | 判定 |
|---|---|---|---|
| 状态怎么改 | 单一 reducer，move 用 immer 改 `G` | 单一写入点 `applyEffect`，规则只能返回 `[]*Effect` | **我们更强** |
| 发生过什么怎么记 | `deltalog` + `_undo/_redo` 栈 + `_stateID` | `EffectLog`（进出都是副本）+ 快照 + 回放 | 相当 |
| **谁可以行动** | `ctx.activePlayers` 运行时集合，在状态里 | `SetActors` 运行时名单，在状态里 | 已补，形状相同 |
| 能做什么 | `GetMove` 分层：stage → phase → 全局 | `PhaseStep.Skill` 按阶段列举 | 相当 |
| 下一步去哪 | `next` 可以是字符串或函数 | `NextPhase` 默认 + `GOTO_PHASE` 效果改写 | 相当（我们刚补） |
| 一回合是什么 | **没有这个概念**，只有 turn 与 phase | `Round`，由 `EndsRound` 声明 | 我们多一个，存疑 |
| 谁能看到什么 | `playerView(G, ctx, playerID) -> G'` 一个函数 | 结构化 `PlayerView` + `AudienceOf` + 不可配置底线 | **我们明显更强** |
| 随机怎么办 | PRNG 状态进游戏状态，回放确定 | 流由「种子+回合+阶段」推导，不存进度 | 已补，做法更小 |
| 阶段何时结束 | `endIf` / `maxMoves` 自动结束，框架管 | 不管，`EndPhase` 由调用方调 | 刻意不同，我们对 |
| 扩展怎么加 | plugin 系统 + 配置 | 七个具体扩展点 + 效果原语 | 不同路子 |
| 谁在跑 | 自带 client/server/master + transport | 不做 | 刻意不同 |

---

## 我们更强的两处

### 写入约束是签名保证的，不是约定

他们的 move 可以任意改 `G`——immer 只是把改动变成不可变更新，**没有阻止 move
乱改的机制**。我们的 `Resolver.Resolve(uses, view) []*Effect` 在**类型上**就拿不到
可变状态：只读 `GameView` 进，`Effect` 出。

这不是风格差异。「状态的每一次改变都经由同一个写入点」是快照、回放、审计三样
收益的前提；他们靠纪律维持，我们靠编译器。

### 信息边界不是一个删字段的函数

他们的 `playerView` 是「你自己写一个函数，把不该看的字段删掉」，
默认实现 `PlayerView.STRIP_SECRETS` 靠**命名约定**——删掉叫 `secret` 的键。

我们这边是三样东西：

- **结构化的 `PlayerView`**——`Self` / `Players` / `RoleInfo` / `Teammates`，
  每一格的语义都定死了，不是「一个自由的对象删掉几个键」；
- **`AudienceOf` 单独一条路**——「一件事该告诉谁」和「一个人现在能看到什么」
  是两个问题，他们只解决了后者；
- **一条不可配置的底线**——状态原语永不外发，规则改不了它。

阿瓦隆写下来的时候这一整块**零摩擦**（梅林的单向可见、奥伯伦的双向隔离、
派西维尔的二选一、任务失败票的匿名）。这是这个库最值钱的部分。

---

## 我们欠债的两处

### 一、谁可以行动（[SCARS.md](../avalon/SCARS.md) 疤 1）

他们的判定就三行：

```javascript
function IsPlayerActive(_G, ctx, playerID): boolean {
  if ((ctx._removedPlayers || []).includes(playerID)) return false;
  if (ctx.activePlayers) return playerID in ctx.activePlayers;   // 运行时集合优先
  return ctx.currentPlayer === playerID;                          // 否则退回默认
}
```

三个要点，每一个都在打我们的脸：

1. **`ctx.activePlayers` 是状态**，在序列化的 `ctx` 里，因此进存档、能回放。
   他们特意**没有**把它放进 `G`（那是放任意游戏状态的地方），而是给了 `ctx`
   一个专属位置，还配了生命周期（`_prevActivePlayers` 栈、集合空了自动出栈）。
   → **行动者集合值得当一等概念，不是一个普通变量。**
2. **默认值 + 运行时改写的分层**——有运行时集合就用，没有就退回默认。
   这正是我们刚给 `GOTO_PHASE` 用的形状。
3. **框架自己拦**，在 `master.ts` 的入口：
   `if (!this.game.flow.isPlayerActive(...)) { logging.error('player not active'); return; }`
   ——不是让游戏逻辑事后过滤。

我们的做法是 `PhaseStep{Role, Skill}` 静态匹配，外加 `peekTrigger` 一个单人特例。
后果是内核对没资格行动的玩家说谎（`AllowedSkills` 说他能动、`PhaseReadiness`
等着他）。

**顺带一个发现**：我们的 `NewAbilityTriggerEffect(playerID, phase)` 语义是
「这一个人、去哪个阶段行动」——**它本来就是「谁在阶段 X 能行动」的单人特例**。
补这条能力不是加新机制，是把已有机制从一个人推广到一组人，
`validateSkillUse` 里那个特判大概率能一起删掉。

### 二、随机（此前没记过）

他们把 PRNG 状态存进游戏状态：`seed` 与 `prngstate` 两个字段，每次取随机数
之后把新的 PRNG 状态写回去。于是**回放能重现完全相同的随机序列**，move 也
「stay pure」。

我们的内核**根本没有随机**。`Resolver` 必须是局面的纯函数，要随机只能宿主在
外面摇，摇完的结果不进效果流——于是那一部分**回放不出来**。

狼人杀和阿瓦隆都躲过了这个问题（发牌在建局之前，局中不需要随机）。但任何
局中带随机的规则——掷骰、摸牌、随机事件——在我们这儿建不起来，或者建起来了
就失去可回放性，而那是这个库的招牌之一。

**这条是阿瓦隆没撞到、对照才看出来的。** 它证明「写第二套规则包」和
「看别人怎么做」是两种不同的检验，都不能省。

---

## 出局：三个来源分歧，而分歧有规律

|  | 有没有一等的「出局」 |
|---|---|
| boardgame.io | **没有**。游戏自己在 `G` 里记，在轮转里跳过（`playOrderPos` 的 `next` 函数里跳） |
| OpenSpiel | **没有**。`current_player()` 由 state 算，死人自然轮不到 |
| PettingZoo AEC | **有**。`terminations` 每个 agent 一份，`agents` 列表会缩短 |

分歧对应的是**框架需要它干什么**：PettingZoo 的 API 是循环问每个 agent，
框架必须知道什么时候不再问；另两个不需要，因为「谁能行动」本来就从状态算。

我们补上 `SetActors` 之后变成了后者。于是问题从「要不要 `Alive`」变成
「它还该不该说了算」——答案是不该，见 [SCARS.md](../avalon/SCARS.md) 疤 6。
`Alive` 保留为**默认**，规则点名时由规则负责。

## 存疑的一处：`Round` 到底要不要

boardgame.io **没有回合这个概念**，只有 turn（谁在行动）与 phase（游戏的段落）。

我们有 `Round`。它此前是内核猜出来的（绕回起始阶段就算），阿瓦隆撞出疤 3
之后改成了由板子声明（`PhaseConfig.EndsRound`）。现在它不是错的——规则说了算，
阿瓦隆的 `Round` 等于第几轮任务，狼人杀等于第几夜。

但对照之下仍要问一句：**它值不值一个内核概念？** 它现在的全部作用是给
`RoundVar` / `PlayerRoundVar` 划一个生命周期。如果变量的生命周期能由规则
直接声明，`Round` 这个计数器就只是个方便读数——那它更该属于规则。

暂不动。记在这里，等第三套规则包再说。

---

## 有一样明确不抄

他们的 `endIf`、`minMoves`、`maxMoves` 会**自动结束**阶段或 stage。

我们刻意不管这件事：引擎不计时，`PhaseConfig.Timeout` 只是建议值，
什么时候 `EndPhase` 完全由调用方决定，`PhaseReadiness` 只回答「还差谁」。
这条写在 `engine/doc.go` 的「内核不做什么」里，是有意的边界，不是缺失。

所以补行动者集合的时候，**只取「谁能行动」，不取那套计数与自动结束**。

---

## 结论

对照下来，我们不是「架构整体有问题」，而是**一边领先一边欠债**：

- 「谁知道什么」这半边比对照对象**强一个量级**，而且是这个库的核心价值；
- 「一局游戏怎么推进」那半边欠两笔——行动者集合、随机——两笔都是
  「内核替规则做了决定」或「内核干脆没提供」，而不是做错了。

而欠债的形状是清楚的，因为对照给出了可抄的答案：**一等状态 + 规则用效果设置
+ 内核在入口强制 + 未设置时退回默认**。我们已经用这个形状解决过两次
（`EndsRound`、`GOTO_PHASE`），这是第三次。
