# The kernel's API

> ## 🔒 Frozen
>
> **What is frozen is this document plus
> [`testdata/api.golden`](testdata/api.golden).** The latter is the
> machine-readable baseline, guarded by `TestAPI_SurfaceIsPinned`: change a
> name or a signature and the test goes red.
>
> **Frozen does not mean "never changes"**, it means three disciplines:
>
> 1. **A breaking change needs a specific reason somebody ran into** -- some
>    rules package could not be written because of it, or the way around it
>    would tell a lie. "I think this is nicer" does not count.
> 2. **Adding is harder than removing.** What is added cannot be taken back;
>    before removing, you have to answer "who uses this".
> 3. **A change cannot happen quietly.** Changing the exported surface means
>    updating the golden baseline and this document at the same time, and that
>    step is explicit.
>
> The state at the freeze: **three independent rules packages** (werewolf,
> mission-based, one-night card swapping), a kernel of 55 types / 24
> package-level functions / 56 methods / 20 interface methods / 62 constants
> and variables, and **not one exported name without a user**.
>
> What would reopen it: see [§15](#15-what-would-reopen-the-freeze).

> **This document is the thing that is frozen.** It lists **every** exported
> name in `github.com/Zereker/hiddenrole` and says what each one promises.
>
> The design intent is in [DESIGN.md](DESIGN.md) and the implementation order
> in [ROADMAP.md](https://github.com/Zereker/werewolf/blob/main/docs/ROADMAP.md).
> This document is about the **contract** only: what exists, how to use it,
> and whether it changes.
>
> Current size: **55 types, 24 package-level functions, 56 methods, 62
> constants and variables**, plus 6 names in the public sub-package
> `enginetest` (see Appendix B). Appendix A is the complete listing, used for
> comparison after the freeze.

---

## 0. Two layers; work out which one you are on

| What you want to do | What to import |
|---|---|
| run a game of werewolf and read its state | `werewolf` alone |
| **change the rules**: write a resolver, replace the victory check, add a role, take a snapshot apart, branch on error codes | also `Zereker/hiddenrole` |

The `werewolf` package's `alias.go` is a **deliberately short** list of
aliases -- only the names its own exported API needs. The moment you want to
change the rules, the `hiddenrole.` prefix shows up at the call site. **That
is not an omission, it is making the boundary visible in the code.**

Everything below is the `hiddenrole` package.

---

## 1. The whole thing at a glance

| Group | Names | When to use |
|---|---|---|
| **vocabulary** | `PhaseType` `RoleType` `SkillType` `EventType` `Camp` | define which phases, roles and skills your game has |
| **setup** | `Config` `PhaseConfig` `PhaseStep` | describe how phases flow and who acts in each |
| **construction** | `NewEngine` `MustNewEngine` `RestoreEngine` `ReplayEngine` `EngineOption` | build an engine |
| **driving** | `Engine.AddPlayer` `.Start` `.SubmitSkillUse` `.EndPhase` `.Apply` | run a game |
| **reading the board** | `Engine.Status` `.Var` `.PlayerInfo` `.AlivePlayerIDs` `.RoundContext` `.PhaseInfo` `.PhaseReadiness` `.View` | ask the engine how things stand |
| **player views** | `Engine.PlayerView` `.AllowedSkills` `.Teammates` `.AudienceOf` | what to send a player |
| **speech** | `Engine.SendMessage` `.MessageReceivers` `Message` | players talking |
| **callbacks** | `Engine.OnEvent` `.OnMessage` `EventHandler` `MessageHandler` `Event` | events pushed to the host |
| **what the rules write** | `Resolver` `GameView` `Effect` `VarScope` `SkillUse` | how a phase resolves |
| **the eight extension points** | eight `With*` + eight interfaces + eight `*Func` | add a role, replace a decision, change a boundary |
| **saving and replay** | `Engine.Snapshot` `.EffectLog` + five `*Snapshot` types | save, review, audit |
| **errors** | `ErrorCode` `GameError` 20 `Err*` 18 `Code*` `HasCode` `CodeOf` `WrapError` | branch on error codes |
| **test helpers** | `Board` `Seat` `Mark` | unit-test your own resolver |

---

## 2. The vocabulary

**Five types, and the kernel owns almost no values.** All strings underneath.
The zero value is the empty string and means "unspecified".

```go
type PhaseType string   // a phase: NIGHT_WOLF / PROPOSE / …
type RoleType  string   // a role: WEREWOLF / MERLIN / …
type SkillType string   // a skill: KILL / APPROVE / …
type EventType string   // what happened: KILL / MISSION_FAIL / …
type Camp      string   // one side: GOOD / EVIL / …
```

All five have `String()`, printing `"UNSPECIFIED"` when unset.

### The values the kernel does own

**Only these. This table may only get shorter.** The reason for each is in
[DESIGN.md §7](DESIGN.md).

| Value | What it is |
|---|---|
| `PhaseUnspecified` `PhaseStart` `PhaseEnd` | the state machine's lifecycle. `AddPlayer` is only allowed inside `START` |
| `RoleUnspecified` | on a `PhaseStep`, "every role" |
| `RoleSystem` + `SkillAnnounce` | "no player carries this step" -- a marker, not an identity. Readiness does not count it. For a role literally named god, name it in your rules package (`werewolf.RoleGod` is exactly that) |
| `SkillUnspecified` `SkillSkip` | `SKIP` is the one move that needs no target |
| `CampUnspecified` | not decided yet, or this player belongs to no side |
| `VarCamp = "camp"` | a canonical key: its value fills `PlayerInfo.Camp` / `SelfInfo.Camp` |
| `VarPresent = "1"` | the agreed value for has-it/hasn't-it state (an empty string is deletion, so a non-empty one is needed) |
| nine `Event*` | the kernel's own events, see §9 |

---

## 3. Setup

```go
type Config struct {
	StartPhase     PhaseType                   // the phase play begins in
	Phases         map[PhaseType]*PhaseConfig  // every phase
	DefaultTimeout time.Duration               // advice; the engine does not time by it
}
func (c *Config) Validate() error
func (c *Config) PhaseTimeout(phase PhaseType) time.Duration
```

```go
type PhaseConfig struct {
	Type      PhaseType
	Steps     []PhaseStep
	Timeout   time.Duration
	NextPhase PhaseType   // the default exit, overridable by GOTO_PHASE

	EndsRound       bool  // when this phase ends, the round number goes up
	ClearsRoundVars bool  // before entering this phase, round variables are cleared
}
```

**Only a looping phase graph needs a round boundary.** `Validate()` walks
`NextPhase` from `StartPhase`: reaching `PhaseEnd` means it is a straight
line, each phase is visited once, there is no second round, and nothing is
required. One Night has exactly such a graph (one night, one discussion, one
vote in the whole game). A looping graph still needs at least one of each.

**`EndsRound` and `ClearsRoundVars` are two things, declared separately.**
Most boards have them coincide (in werewolf, the vote phase ending is both a
new round and the right moment to clear); the mission-based games do not --
team markers live until the next nomination while the round number tracks
which mission it is. `Validate()` requires **at least one phase declaring
each**, or the round stays at 1 forever and round variables are never
cleared.

```go
type PhaseStep struct {
	Role  RoleType   // which role; RoleUnspecified means every role, RoleSystem means no player carries it
	Skill SkillType  // **empty means "this role wakes, but takes no action"**, see below

	Required bool    // unsatisfied means not ready (affects PhaseReadiness only, never EndPhase)
	Multiple bool    // true = every eligible actor must submit; false = any one of them
	Group    string  // a mutually exclusive group: submitting any member completes it (shoot / do not shoot)

	AllowDeadTarget bool  // may this skill target an eliminated player (the witch's antidote)
}
```

**An empty `Skill` means "this role wakes, but takes no action"** -- it only
receives information and submits nothing. The One Night minion opening their
eyes to see the wolves, the masons recognising each other and the insomniac
looking at their own card are all this kind of step.

It mirrors `RoleSystem`: that one is "this step has no player", this one is
"this step has a player, who does not act".

| An empty step | |
|---|---|
| `AllowedSkills` | **excludes** it -- there is nothing they can submit |
| `SubmitSkillUse` | **rejects** a `SkillUnspecified` submission |
| `PhaseReadiness` | **does not enter** it -- there is nothing to satisfy |
| `PhaseInfo.ActiveRoles` | **includes** it -- the host has to know who to wake |

---

## 4. Construction

```go
func NewEngine(config *Config, opts ...EngineOption) (*Engine, error)
func MustNewEngine(config *Config, opts ...EngineOption) *Engine
func RestoreEngine(config *Config, snap *Snapshot, opts ...EngineOption) (*Engine, error)
func ReplayEngine(config *Config, log []*Effect, opts ...EngineOption) (*Engine, error)

type EngineOption func(*Engine) error
```

All four entry points take the same options. **Options can only be given at
construction**: once the engine is in the caller's hands, the resolvers, the
victory checker and the four providers no longer change.

`RestoreEngine` and `ReplayEngine` **must** be given the same `config` and
`opts` as were in force during recording -- a snapshot and an effect log
record what happened, not the rules.

---

## 5. Engine

23 methods. **All of them may be called concurrently.**

### 5.1 Driving

```go
func (e *Engine) AddPlayer(id string, role RoleType) error  // only in the START phase
func (e *Engine) Start() error
func (e *Engine) SubmitSkillUse(use *SkillUse) error
func (e *Engine) EndPhase() ([]*Effect, error)
func (e *Engine) Apply(effects ...*Effect) []*Effect
```

`SubmitSkillUse` **blocks at submission time** rather than accepting and
leaving the rules to throw it away afterwards -- otherwise `AllowedSkills`
lies to unqualified players and `PhaseReadiness` waits on a crowd who cannot
possibly act.

`EndPhase` is the whole cycle: resolve the skills -> apply the effects ->
check for victory -> transition. It returns every effect this phase produced,
vetoed ones included.

`Apply` bypasses phase resolution and applies effects directly. **This is a
tool with an edge**, and a necessary one: a host really does meet "the player
disconnected, count them dead" and "an admin kicked someone". It still goes
through the **same write point** -- effects enter the effect log, vetoed ones
do not take hold, and kernel primitives are not sent out.

### 5.2 Reading the board

```go
func (e *Engine) Status() Status   // the summary, all four read under one lock
type Status struct {
	Phase  PhaseType
	Round  int
	Over   bool
	Winner Camp
}

func (e *Engine) Var(scope VarScope, key string) string
func (e *Engine) PlayerInfo(playerID string) (PlayerInfo, bool)
func (e *Engine) AlivePlayerIDs() []string
func (e *Engine) RoundContext() *RoundContext
func (e *Engine) PhaseInfo() *PhaseInfo          // the god's view
func (e *Engine) PhaseReadiness() PhaseReadiness // who has yet to act
func (e *Engine) View() GameView                 // the whole read-only board (clones)
```

**Why `Status` is one struct and not four methods**: four methods each take
their own read lock, a host rendering "the day of round 3" has to ask twice,
and if another goroutine resolves a phase in between, it reads a combination
of values that **never held at the same time**. Four scalars, one lock, no
allocation.

**Why there are cheap readers besides `View()`**: `View()` clones the whole
board, and asking "which round is it" should not cost that. This is a
performance tier, not duplication.

### 5.3 Player views and the boundary

```go
func (e *Engine) PlayerView(playerID string) *PlayerView
func (e *Engine) AllowedSkills(playerID string) []SkillType
func (e *Engine) Teammates(playerID string) []string
func (e *Engine) AudienceOf(event *Event) ([]string, bool)
```

`AudienceOf`'s second result separates two things that must stay separable:

| Returns | Means | What the caller should do |
|---|---|---|
| `(nil, true)` | **definitely shown to nobody** (a kernel state primitive) | send nothing |
| `(ids, true)` | definitely to these people | send it to them |
| `(nil, false)` | **don't know** (the rules installed no provider) | route it yourself |

### 5.4 Speech

```go
func (e *Engine) SendMessage(senderID, content string) error
func (e *Engine) MessageReceivers(senderID string) []string
type Message struct { /* sender, content, phase, round */ }
```

Speech **does not go through the skill channel**. The audible range is decided
by a `SpeechProvider`; with no provider installed it falls back to "every
living player hears it".

### 5.5 Callbacks

```go
func (e *Engine) OnEvent(handler EventHandler)
func (e *Engine) OnMessage(handler MessageHandler)

type EventHandler   func(event *Event)
type MessageHandler func(msg *Message, receiverIDs []string)
```

**Callbacks always run after the lock is released**, and the handler list is
snapshotted while it is held -- so there is no deadlock (a callback may safely
call `Engine` methods) and no race with a concurrent registration. A panic in
one handler is isolated, logged at error level, and does not affect the
others.

### 5.6 Saving and replay

```go
func (e *Engine) Snapshot() *Snapshot   // plain data, json.Marshal it directly
func (e *Engine) EffectLog() []*Effect  // the complete effect log since the game was created
```

**The division of labour**: a snapshot is **state**, an effect log is
**history**. For persistence use a snapshot -- `Effect.Data` is a
`map[string]interface{}` whose types degrade on a JSON round trip, and the
effect log is designed for in-process replay and auditing, not as a storage
format.

---

## 6. What the rules write

### 6.1 Resolver

```go
type Resolver interface {
	Resolve(uses []*SkillUse, view GameView) []*Effect
}
type ResolverFunc func(uses []*SkillUse, view GameView) []*Effect
```

**This signature is the most important line in the library**: it can read
`GameView` only and return `Effect`s only. "Every state change goes through an
Effect" is therefore held up by the **signature**, not by convention.

**Do not call any `Engine` method from an implementation** -- extension points
are called synchronously while the engine holds its lock, Go's RWMutex is not
reentrant, and the consequence is **a hang, not an error**. The signatures are
deliberately narrow: an extension point never receives an `*Engine`.

### 6.2 GameView

```go
type GameView interface {
	Player(id string) (PlayerInfo, bool)
	AlivePlayers() []PlayerInfo
	AllPlayers() []PlayerInfo          // the eliminated included: a wipe-out check has to count the dead
	AlivePlayerIDsByRole(role RoleType) []string
	RoundContext() RoundContext
	Var(scope VarScope, key string) string
	Round() int
	Phase() PhaseType
}
```

**A view offers facts, never judgements.** "How many special roles are left"
is the rules' judgement; count them yourself.

### 6.3 VarScope: a 2x2 table

```go
type VarScope struct{ /* unexported fields */ }

var ScopeGame  VarScope  // whole game, unowned
var ScopeRound VarScope  // this round, unowned

func (s VarScope) Of(playerID string) VarScope  // bind to a player, lifetime unchanged
func (s VarScope) String() string               // game / round / game:p1 / round:p1
```

| | unowned | owned by a player |
|---|---|---|
| **whole game** | `ScopeGame` | `ScopeGame.Of(id)` |
| **this round** | `ScopeRound` | `ScopeRound.Of(id)` |

```go
// write
hiddenrole.NewSetVarEffect(hiddenrole.ScopeGame,        "score",    "3")
hiddenrole.NewSetVarEffect(hiddenrole.ScopeGame.Of(id), "antidote", "used")
hiddenrole.NewSetVarEffect(hiddenrole.ScopeRound,       "kill",     target)
hiddenrole.NewSetVarEffect(hiddenrole.ScopeRound.Of(id),"guarded",  hiddenrole.VarPresent)

// read
view.Var(hiddenrole.ScopeRound.Of(id), "guarded")
```

**Values are strings, an empty string is deletion, identically in all four
cells.** The keys are the rules' own.

The four cells fall out of two values crossed with one method, so **a missing
cell is not expressible** -- the table used to exist only in a comment while
the code had eight flat names, and "whole game, unowned" was missing for a
long time before anyone noticed.

### 6.4 Effect

```go
type Effect struct {
	Type     EventType
	SourceID string
	TargetID string
	Data     map[string]interface{}
	Canceled bool
	Reason   string
}

func NewEffect(eventType EventType, sourceID, targetID string) *Effect
func (e *Effect) WithData(key string, value interface{}) *Effect
func (e *Effect) Cancel(reason string)
func (e *Effect) ToEvent() *Event
```

**Six constructors, two kinds of thing:**

| Constructor | Kind | Recognised by the state machine? |
|---|---|---|
| `NewEffect` | the rules naming what happened | no; pushed to `OnEvent` |
| `NewSetAliveEffect(id, alive)` | changes state | ✅ |
| `NewSetVarEffect(scope, k, v)` | changes state | ✅ |
| `NewSetActorsEffect(phase, ids...)` | changes state (writes the actor list) | ✅ |
| `NewDetourEffect(id, phase)` | a directive (files a detour debt) | ✅ |
| `NewGotoPhaseEffect(phase)` | a directive (overrides the next phase) | ✅ but changes no state |

**A wolf kill kills nobody.** A lone `KILL` means nothing to the state
machine. For the rules to eliminate someone, they emit a `SET_ALIVE` alongside
it. Two effects, two things -- the first for the audience and the effect log,
the second for the state machine.

**Two inspection methods**, for an extension that wants to intercept a class
of change:

```go
func (e *Effect) SetsAlive() (alive, ok bool)
func (e *Effect) SetsVar() (scope VarScope, key, value string, ok bool)
```

The idiot surviving an exile by flipping their card works by vetoing the
lethal primitive. **Intercepting the primitive rather than the word "exile"**
makes it independent of the cause -- one piece of code stops a wolf kill, a
poisoning, a gunshot and any third-party ruleset's way of dying.

### 6.5 SkillUse

```go
type SkillUse struct {
	PlayerID string
	Skill    SkillType
	Targets  []string
	Phase    PhaseType  // filled in by the engine
	Round    int        // filled in by the engine
}
func (u *SkillUse) Target() string  // the single-target reader: Targets[0], or the empty string
```

---

## 7. The eight extension points

**All eight can be installed with a plain function. Built-in roles hold no
privilege** -- they go through the same doors.

| To add | Option | Interface | Function adapter |
|---|---|---|---|
| how a phase resolves | `WithResolver(phase, r)` | `Resolver` | `ResolverFunc` |
| how winning works | `WithVictoryChecker(c)` | `VictoryChecker` | `VictoryFunc` |
| what a role sits down with | `WithRoleSetup(role, s)` | `RoleSetup` | `RoleSetupFunc` |
| what the board looks like at the start | `WithGameSetup(s)` | `GameSetup` | `GameSetupFunc` |
| who should be told about something | `WithAudience(p)` | `AudienceProvider` | `AudienceFunc` |
| who is on whose side | `WithTeammates(p)` | `TeammateProvider` | `TeammateFunc` |
| who hears a player speak | `WithSpeech(p)` | `SpeechProvider` | `SpeechFunc` |
| what a role additionally sees | `WithRoleInfo(role, p)` | `RoleInfoProvider` | `RoleInfoFunc` |

Plus one that is not an extension point: `WithLogger(l)` / `Logger` / `Field`
-- that is the host's wiring.

**The division between `RoleSetup` and `GameSetup`**: the first is "what this
role sits down with" (the witch's two potions), the second is "what the whole
board should look like at the moment play begins" -- it can see a `GameView`,
so it can do what `RoleSetup` cannot, such as "which seat leads first".

**Asymmetry is allowed**: the demon knows its minions and the reverse does not
hold; the missions package's Oberon neither knows his fellows nor is known to
them. The kernel does not assume symmetry.

---

## 8. The information boundary's types

### One player's three faces

```go
type PlayerInfo struct {           // god / the rules
	ID    string
	Role  RoleType
	Alive bool
	Vars      map[string]string
	RoundVars map[string]string
}

type SelfInfo struct {             // themselves
	ID    string
	Role  RoleType
	Alive bool
	Camp  Camp
}                                  // note: no Vars

type PublicPlayerInfo struct {     // everyone else
	ID    string
	Alive bool
	Role  RoleType  // filled in only where revealed to this view
}
```

**These three do not get merged.** `PublicPlayerInfo` **structurally cannot
hold `Vars`** -- so "should they be shown this" is a question about
signatures, not a runtime judgement.

Private state a player should see is **projected explicitly** by the role
through a `RoleInfoProvider`.

### PlayerView

```go
type PlayerView struct {
	PlayerID string
	Round    int
	Phase    PhaseType
	Self          SelfInfo
	Players       []PublicPlayerInfo  // sorted by ID
	AllowedSkills []SkillType         // never nil; an empty slice means it is not my turn
	Teammates     []string
	RoleInfo      map[string]string   // what the role projected explicitly
}
```

### The god's view

```go
type PhaseInfo struct { /* phase, round, active roles, per-role information */ }
func (p *PhaseInfo) NeedsGodAnnouncement() bool
func (p *PhaseInfo) GodAnnouncementStep() *PhaseStep
func (p *PhaseInfo) PlayerActionSteps() []PhaseStep

type RolePhaseInfo struct { /* available skills, who acts, teammates, role information */ }

type PhaseReadiness struct {
	Phase    PhaseType
	Round    int
	Ready    bool             // is every Required step satisfied
	Pending  []PendingAction  // who still **must** act
	Optional []PendingAction  // who **may** act but has not
	Acted    []string
}
type PendingAction struct { PlayerID string; Role RoleType; Skill SkillType }
```

**`Pending` and `Optional` are separate** because "who still must act" and
"who may act in this phase" are different questions: in a default
configuration only the wolf kill and the vote are `Required`, while the guard,
the witch, the seer and the hunter may all decline. Drive the game off
`Pending` alone and those roles are never called on for a whole game.

`Ready == false` **does not** make `EndPhase` refuse -- the engine keeps no
clock, and whether to advance on a timeout is the caller's decision.

---

## 9. Events

```go
type Event struct { /* type, source, target, data, cancelled, reason */ }
```

**Nine kernel events. The first seven are never sent out, and that is not
configurable.**

| Event | Class |
|---|---|
| `EventSetAlive` `EventSetVar` `EventSetActors` `EventDetour` | changes state |
| `EventGotoPhase` | a control directive (changes no state) |
| `EventPlayerAdded` `EventPhaseChanged` | replay bookkeeping |
| `EventGameStarted` `EventGameEnded` | **public**, players should see them |

**What decides is the kernel's table, not a numeric range and not a name
prefix.** It used to read ">= 100 is internal", which collided head-on with
"third-party values start at 1000" -- every event an extension defined was
judged internal, so an extension's events could not be sent at all.

Any other `EventType` the rules define is an external event, and its audience
goes to the `AudienceProvider`.

---

## 10. Saving

```go
const SnapshotVersion = 13

type Snapshot struct { /* version, phase, round, game vars, actors, winner, players, round context, unresolved submissions */ }
type PlayerSnapshot struct{ ... }
type RoundCtxSnapshot struct{ ... }
type SkillUseSnapshot struct{ ... }
type DetourSnapshot struct{ ... }
```

**The five `*Snapshot` shadow types exist deliberately**: a snapshot is a
format written to storage, its field names must stay stable, and they must not
drift with an internal refactor. This is the one place where "the same data in
two types" is explicitly endorsed.

**A snapshot does not contain** the `Config`, the `Logger` or the callbacks --
the same options have to be passed back in on restore.

**A snapshot does contain the winner** (since v13). Who won is settled by the
`VictoryChecker` at the moment the game ends and does not change afterwards,
and a restored engine does not run the check again -- without it, a finished
game restores as `Over=true` with an empty `Winner`, and `Status`, which
claims its four fields come from one instant, does not line up on this path.
That was a real bug, fixed in v13.

---

## 11. Errors

```go
type ErrorCode string
type GameError struct{ Code ErrorCode; Message string; ... }

func WrapError(code ErrorCode, format string, args ...interface{}) *GameError
func HasCode(err error, code ErrorCode) bool
func CodeOf(err error) ErrorCode
```

18 `Code*` values and 20 `Err*` sentinels. **Both styles are supported**:

```go
if errors.Is(err, hiddenrole.ErrPlayerDead) { ... }            // by sentinel
if hiddenrole.HasCode(err, hiddenrole.CodePlayerDead) { ... }  // by code (friendlier across processes)
```

An error carrying context is still seen through by `errors.Is`, via
`Unwrap()`.

---

## 12. Test helpers

```go
type Board struct {
	Players   []PlayerInfo
	Round     int
	Phase     PhaseType
	Vars      map[string]string  // whole game, unowned
	RoundVars map[string]string  // this round, unowned
}
func (b Board) View() GameView
func (b Board) Apply(effects []*Effect) Board
func (b Board) Player(id string) (PlayerInfo, bool)
func (b Board) Var(scope VarScope, key string) string

func Seat(id string, role RoleType, alive bool, vars ...string) PlayerInfo
func Mark(p PlayerInfo, keys ...string) PlayerInfo
```

**The names do not start with `Test` because this is genuine public API**: a
rules package sits outside the kernel and cannot reach its internal state, and
without this entry point its resolvers could only be exercised by running a
whole game -- which tests the integration.

```go
b = b.Apply(resolver.Resolve(uses, b.View()))
```

It goes through **exactly the same** write point as the engine, so "the effect
never landed" shows up in a unit test.

---

## 13. Stability promises

### What will not change (changing it needs a specific reason somebody ran into)

1. **`Resolver`'s signature** -- read `GameView` only, return `Effect`s only
2. **The information boundary's floor** -- kernel state primitives are never
   sent out, and that is not configurable
3. **The three faces do not merge** -- `PublicPlayerInfo` not being able to
   hold `Vars` is a compile-time guarantee
4. **The five `*Snapshot` types stay decoupled from the internal structures**
5. **The vocabulary has types only; the values live in the rules packages**
6. **Extension points can only be given at construction**
7. **`Effect` is the only write path** (`Engine.Apply` is the same write
   point, not a second one)

### What will change ([ROADMAP.md phase 2](https://github.com/Zereker/werewolf/blob/main/docs/ROADMAP.md))

| How it changes | Who it affects |
|---|---|
| `PlayerInfo.Alive` / `.Role` go from stored fields to **derived fields** | reading is unchanged; writing moves from `SET_ALIVE` into `SET_VAR` |
| `SnapshotVersion` is bumped and the snapshot format changes | old saves become unreadable (currently zero users) |
| ~~the detour queue's naming~~ | **done** (§14, item 3) |

### What is not promised

- **Performance.** No real workload says it is slow, and optimisation comes
  after measurement.
- **The specific keys in `Effect.Data`.** They are the kernel's internal
  convention; read effects with methods like `SetsAlive()` / `SetsVar()`
  rather than digging into `Data`.

---

## 14. Clearing the books before the freeze (**seven items, all handled**)

Seven inconsistencies found by going through the API line by line while
writing this document. All cleared.

| # | Problem | What was done |
|---|---|---|
| 1 | `CodeInvalidPlayerId` and `ErrInvalidPlayerID` disagreed on case | unified as `CodeInvalidPlayerID` |
| 2 | `PlayerInfo.Var(key)` / `.RoundVar(key)` did not take a `VarScope`, unlike every other reader | **both methods deleted**. `Vars` / `RoundVars` are exported fields, reading a nil map is safe in Go anyway, and these two were zero-value sugar -- their only effect was making `Var` mean different things on two types |
| 3 | `PendingTrigger` / `NewAbilityTriggerEffect`'s docs still said "death ability" | renamed to `Detour` / `NewDetourEffect`, event value `ABILITY_TRIGGERED` -> `DETOUR`, snapshot field `pending_triggers` -> `detours`, `SnapshotVersion` 11 -> 12 |
| 4 | `RoleGod`'s name implied the identity of a host | renamed in the kernel to `RoleSystem` (value `"GOD"` -> `"SYSTEM"`). "God" is werewolf's name for this marker, and it lives in the rules package (`werewolf.RoleGod`) |
| 5 | `Engine.SendMessage`'s docs said a dead player is an error | rewritten: that is the **default when no `SpeechProvider` is installed**; with one, the provider decides |
| 6 | `Engine.PlayerInfo`'s comment said "(recommended)" | rewritten to say what it actually means: the god's view, `Vars` included, **not** for showing a player |
| 7 | Nothing pinned this listing of exported names | **`TestAPI_SurfaceIsPinned`**: `go/ast` parses every non-test source file in the package, collects the exported names, and compares against `testdata/api.golden` |

### Item 7 is the enforcement mechanism

Without it this document would certainly drift away from the code -- the same
wound as every other "rule that lives only in a comment" in this project.

What is pinned is **names plus signatures**, interface method sets included.
Pinning names alone would let a change like "`CheckVictory` returns a set of
`Camp`s instead of one" slip through -- not one exported name added or
removed, and every implementer fails to compile. Renaming a parameter is not a
change; changing its **type** is.

It does not judge whether the API is good, only that **a change cannot happen
quietly**:

```
$ go test .
--- FAIL: TestAPI_SurfaceIsPinned
    the kernel's exported surface changed.
    added:   [func SneakyExport]
    removed: []

    This is not an error, it is a reminder: the exported surface is what
    API.md declares frozen.
    Confirm the change is intended, then do two things together --
      1. go test . -run TestAPI_SurfaceIsPinned -update-api-golden
      2. update API.md (the body and Appendix A)
```

All three directions -- a quiet addition, a quiet removal, a quiet signature
change -- were verified to go red. The last used a mutation that **still
compiles** (adding a variadic parameter to `CodeOf`, which leaves every
existing call compiling), because a mutation that does not compile proves
nothing about the test itself.

---

## 15. What would reopen the freeze

A freeze has to be overturnable, or it is only a slogan. Any one of the four
below reopens the corresponding part:

| Trigger | What reopens | Where it stands |
|---|---|---|
| **a second rules package runs into "victory has exactly one winner"** | `VictoryChecker`'s signature becomes `winners []Camp` | one-night card swapping has run into it once (the tanner winning alongside the villagers). Blood on the Clocktower's travellers scoring separately is most likely the second |
| **the "a target must be a player" encoding starts lying, or combinatorially explodes** | `PhaseStep` gains a `TargetKind` | one-night card swapping ran into it, but the way around it is ugly without being false, at a cost of 15 lines |
| **some ruleset cannot be written because "aliveness is one bit"** | `Alive` is demoted to a canonical key | not yet. Blood on the Clocktower's poisoned/drunk are the candidates |
| **a fourth and fifth rules package still cannot use `RoleSystem`** | it moves into the werewolf package | one of the three uses it |

### The first criterion has not been tested by itself yet

The freeze's first criterion is "**the next rules package no longer forces a
breaking API change**". That criterion was only phrased this way after the
third rules package was written -- **the third was written under the old
criterion** -- so strictly speaking the new one has not yet been tested
against a real rules package.

Writing this down is not a discount on the freeze, it is being clear about its
reach: **the fourth rules package is this criterion's first real exam**. If it
forces a breaking change, the freeze came too early; if it forces only changes
with zero exported names (as the third did), the freeze holds.

---

## Appendix B: `enginetest` -- the test harness for rules packages

A public sub-package of the same module, in the same position as
`net/http/httptest`: **a test harness for users of the library, not the thing
under test.**

```go
func RunFuzz(t *testing.T, spec FuzzSpec)

type FuzzSpec struct {
    Games    int      // how many games to run
    MaxSteps int      // most steps per game
    Setup    Setup    // how to lay a game out (the same rng must lay out the same game)
    Act      Act      // how to take a turn; nil means only advance phases
    WantEnd  bool     // whether every game must finish within MaxSteps
    MustSee  []string // none of these labels may be zero, or the randomisation has degenerated
}

type Game struct { Config *hiddenrole.Config; Options []hiddenrole.EngineOption; Seats []Seat; Labels []string }
type Seat struct { ID string; Role hiddenrole.RoleType }
type Setup func(rng *rand.Rand) Game
type Act   func(e *hiddenrole.Engine, rng *rand.Rand)
```

The rules package supplies "how to lay a game out and how to take a turn", and
`RunFuzz` supplies **seven general invariants**, not one of which knows any
game:

| | |
|---|---|
| snapshot round trip | compare snapshots byte for byte, **and compare behaviour** (who may act, readiness, the god's-view lists) |
| effect-log replay | the same two |
| three paths agree | `AllowedSkills` and `PlayerView.AllowedSkills` must match |
| lists are stable | querying the same board repeatedly gives the same order |
| `Status` is coherent | over means stopped at `PhaseEnd` with a winner; not over means no winner |
| over stays over | once the game is over the board stops changing, and it still survives a save round trip |
| primitives are not sent out | not one kernel state primitive reaches `OnEvent` |

**The "and compare behaviour" clause was forced out by mutation
verification**: the first version compared snapshot bytes only, and when the
snapshot serialiser itself drops a field it drops it on both sides, so the
comparison is blind -- the "snapshot loses `Actors`" mutation survived on the
spot. With behaviour comparison added, the first run caught three real bugs.

It used to be called `internal/gamefuzz`. `internal/` can only be imported
from within the same module, and the engine had to become its own library --
the rules packages then live in another module and could not use a line of it.
Being public, it is pinned by `TestAPI_SurfaceIsPinned` along with everything
else, or it would be a back door around the freeze.

---

## Appendix A: the complete listing of exported names

**The freeze baseline.** Guarded by `TestAPI_SurfaceIsPinned` and
`testdata/api.golden` -- change a **name or a signature** and the test goes
red.

In total **55 types / 24 package-level functions / 56 methods / 20 interface
methods / 62 constants and variables**. They are listed by name below; the
complete listing with signatures is in `testdata/api.golden`.

### Types (55)

```
AudienceFunc  AudienceProvider  Board  Camp  Config  Detour
DetourSnapshot  Effect  Engine  EngineOption  ErrorCode  Event
EventHandler  EventType  Field  GameError  GameSetup  GameSetupFunc
GameView  Logger  Message  MessageHandler  PendingAction  PhaseConfig
PhaseInfo  PhaseReadiness  PhaseStep  PhaseType  PlayerInfo
PlayerSnapshot  PlayerView  PublicPlayerInfo  Resolver  ResolverFunc
RoleInfoFunc  RoleInfoProvider  RolePhaseInfo  RoleSetup  RoleSetupFunc
RoleType  RoundContext  RoundCtxSnapshot  SelfInfo  SkillType  SkillUse
SkillUseSnapshot  Snapshot  SpeechFunc  SpeechProvider  Status
TeammateFunc  TeammateProvider  VarScope  VictoryChecker  VictoryFunc
```

### Package-level functions (24)

```
CodeOf  HasCode  Mark  MustNewEngine  NewDetourEffect  NewEffect
NewEngine  NewGotoPhaseEffect  NewSetActorsEffect  NewSetAliveEffect
NewSetVarEffect  ReplayEngine  RestoreEngine  Seat  WithAudience
WithGameSetup  WithLogger  WithResolver  WithRoleInfo  WithRoleSetup
WithSpeech  WithTeammates  WithVictoryChecker  WrapError
```

### Methods (56, by receiver)

```
Engine(23)  AddPlayer  AlivePlayerIDs  AllowedSkills  Apply  AudienceOf  EffectLog  EndPhase  MessageReceivers  OnEvent  OnMessage  PhaseInfo  PhaseReadiness  PlayerInfo  PlayerView  RoundContext  SendMessage  Snapshot  Start  Status  SubmitSkillUse  Teammates  Var  View
Effect(5)  Cancel  SetsAlive  SetsVar  ToEvent  WithData
Board(4)  Apply  Player  Var  View
PhaseInfo(3)  GodAnnouncementStep  NeedsGodAnnouncement  PlayerActionSteps
Config(2)  PhaseTimeout  Validate
GameError(2)  Error  Unwrap
VarScope(2)  Of  String
AudienceFunc(1)  Audience
Camp(1)  String
ErrorCode(1)  String
EventType(1)  String
GameSetupFunc(1)  Setup
PhaseType(1)  String
ResolverFunc(1)  Resolve
RoleInfoFunc(1)  RoleInfo
RoleSetupFunc(1)  Setup
RoleType(1)  String
SkillType(1)  String
SkillUse(1)  Target
SpeechFunc(1)  Receivers
TeammateFunc(1)  Teammates
VictoryFunc(1)  CheckVictory
```

### Constants (41)

```
CampUnspecified  CodeGameAlreadyStarted  CodeGameEnded
CodeGameNotStarted  CodeInvalidBoard  CodeInvalidConfig
CodeInvalidEffectLog  CodeInvalidPhase  CodeInvalidPlayerID
CodeInvalidRole  CodeInvalidSnapshot  CodeMessageNotAllowed
CodePlayerDead  CodePlayerExists  CodePlayerNotFound  CodeSkillNotAllowed
CodeTargetDead  CodeTargetNotFound  CodeUnspecified  DefaultPhaseTimeout
EventDetour  EventGameEnded  EventGameStarted  EventGotoPhase
EventPhaseChanged  EventPlayerAdded  EventSetActors  EventSetAlive
EventSetVar  EventUnspecified  PhaseEnd  PhaseStart  PhaseUnspecified
RoleSystem  RoleUnspecified  SkillAnnounce  SkillSkip  SkillUnspecified
SnapshotVersion  VarCamp  VarPresent
```

### Variables (21)

```
ErrBoardAlreadyDecided  ErrGameAlreadyStarted  ErrGameEnded
ErrGameNotStarted  ErrInvalidBoard  ErrInvalidConfig  ErrInvalidEffectLog
ErrInvalidPhase  ErrInvalidPlayerID  ErrInvalidRole  ErrInvalidSnapshot
ErrMessageNotAllowed  ErrNilSnapshot  ErrPlayerDead  ErrPlayerExists
ErrPlayerNotFound  ErrSkillNotAllowed  ErrTargetDead  ErrTargetNotFound
ScopeGame  ScopeRound
```
