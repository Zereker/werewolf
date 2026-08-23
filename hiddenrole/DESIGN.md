# The kernel's design

> This document answers one question: **what does this kernel have to look
> like to carry the whole class of social deduction games, and not just
> werewolf?**
>
> **The API is frozen** -- the contract clause by clause and the freeze
> declaration are in [API.md](API.md), and the order in which we got here is
> in [ROADMAP.md](https://github.com/Zereker/werewolf/blob/main/docs/ROADMAP.md)
> (archived). The structure of the code as it stands is in
> [ARCHITECTURE.md](ARCHITECTURE.md), and the comparison with other engines is
> in [PRIOR-ART.md](PRIOR-ART.md).

---

## 0. What this is

**A state machine for social deduction games, plus a layer of information
boundary.**

It does two things and only these two:

| | |
|---|---|
| **Drive** | who may act -> what they did -> what the board becomes -> where to go next -> is it over |
| **Conceal** | one board, and different people see different things |

The second is **the entire point** of this class of games -- without
asymmetric information there is nothing to deduce. So it is not "a feature
added along the way", it is the other half, standing alongside the state
machine.

### What it is not

- **Not a werewolf engine.** The words "witch", "werewolf" and "tonight's
  kill" do not appear in the kernel.
- **Not a general board game framework.** It assumes four things: the players
  are a group, time is divided into named steps, actions are resolved in
  batches, and some people know things others do not. A game that does not
  satisfy all four (real-time, perfect information, not turn-based) is out of
  range.
- **Not a server.** No clock, no networking, no persistence. How long a
  timeout waits, whether an eliminated player's card is turned over, what
  transport to use -- all the host's business.

---

## 1. The test: whose job is this

**This is the most important section in the document.** Every conclusion below
follows from it.

For anything at all, ask three questions in order; the first "yes" is where it
belongs:

```
1. Can the kernel judge it correctly **without knowing what game this is**?  -> the kernel
2. Judgeable from the rules, but independent of this table and this game?    -> the rules
3. Only judgeable by knowing how this particular table plays?                -> the host
```

Side by side:

| The thing | Whose | Why |
|---|---|---|
| "did this player's alive bit flip" | kernel | judgeable without knowing the game |
| "is this player on this phase's actor list" | kernel | ditto |
| "does the board replayed from the effect log match the original" | kernel | ditto |
| "is this a new round" | **rules** | a mission-based round is three phases, werewolf's is eight |
| "who won" | **rules** | the kernel does not know which sides exist |
| "who should see this message" | **rules** | wolf night chat is wolves-only, the mission-based games are public throughout |
| "may the dead act" | **rules** | the dead in Blood on the Clocktower have a ghost vote |
| "how long before it times out" | host | offline differs from online, a fast table from a slow one |
| "is an eliminated player's card turned over" | host | one ruleset, different tables, different practice |
| "WebSocket or HTTP" | host | nothing to do with the game |

**This test has been violated, and every violation left the same wound**: an
`if role == X`, a `case EventY`, or a field only one ruleset could use, inside
the kernel. Seven of them are recorded in
[`missions/SCARS.md`](https://github.com/Zereker/werewolf/blob/main/missions/SCARS.md).

### The test's other side: the kernel may offer a **default**, not make **law**

"The dead may not act" is the convention in most games, and the kernel
offering it is a good thing -- without it every ruleset would rewrite it. The
problem is not offering a default, it is **whether it can be overruled**.

So every kernel default must come with an explicit way to overrule it:

| Kernel default | The override |
|---|---|
| actors = the living players matching the role | `NewSetActorsEffect(phase, ids...)` |
| a skill may not target the dead | `PhaseStep.AllowDeadTarget` |
| the dead may not speak | `WithSpeech(provider)` |
| the next phase comes from static configuration | `NewGotoPhaseEffect(phase)` |
| the round ends at the phase marked `EndsRound` | the rules mark it in the configuration; the kernel does not guess |

**A default with no override is law.** Before adding any default behaviour,
answer "how do the rules overrule it".

---

## 2. Five invariants

These five are the kernel's load-bearing walls. They are **held up by
signatures or by tests, not by convention** -- a rule that lives only in a
comment might as well not exist.

### I1. State has one write point

`applyEffect` is the only place state changes. A `Resolver` is handed a
read-only `GameView` and can express a change only by returning `[]*Effect`.

- **Held up by**: the signature.
  `Resolve(uses []*SkillUse, view GameView) []*Effect` cannot reach any
  mutable state.
- **What breaking it costs**: snapshots, replay and auditing fail together --
  all three rest on "every change flows through one pipe".
- **What guards it**: `GameView` is an interface whose implementation is
  unexported; `TestGameView_IsReadOnly`.

### I2. Resolution is a pure function of the board

The same board in must give the same effects out, **in the same order**.

- **Held up by**: a `Resolver` is stateless (there is nowhere to put state --
  its only parameter is `view`); everywhere a map is iterated, the output is
  sorted.
- **What breaking it costs**: replay stops matching, and the effect log
  becomes meaningless.
- **What guards it**: tests in the rules packages along the lines of
  `EffectOrderIsDeterminedByTheBoard`; the replay comparison across 5000
  random games (three rules packages combined).

### I3. The information boundary has a non-configurable floor

The kernel's own state primitives (`SET_ALIVE` / `SET_VAR` / `SET_ACTORS` /
`DETOUR` / `GOTO_PHASE` / `PLAYER_ADDED` / `PHASE_CHANGED`) **are never sent
to any player**, and that **cannot be overridden by a provider**.

Everything else -- events the rules named themselves -- has its audience
decided by the rules, and the kernel's answer is "I don't know".

- **Why there is a floor**: state primitives are the god's-view bookkeeping.
  If an `AudienceProvider` that gives everything to everybody could route
  them, every hidden fact in the game would leak at once.
- **What guards it**: `TestBoundary_StatePrimitivesNeverReachPlayers`,
  `TestKernelEventTypes_AreAllClassified` (a new event type goes red until it
  is classified), `TestPlayerView_CarriesNoFreeFormState` (no free-form bag in
  a player-facing struct).

### I4. One question has one source

"Who may act in this phase" is read from exactly one place, `actorsForStep`.
Skill validation, `AllowedSkills`, `PhaseReadiness` and `PhaseInfo` all share
it.

- **What breaking it costs**: "the kernel accepted his submission while
  telling everyone else he should not be acting". That contradiction has
  arisen three times, and every time it was two places each writing their own
  version and then drifting.
- **What guards it**: cross-assertions such as
  `TestEngine_AllowedSkills_MatchesPlayerView`.

### I5. The kernel recognises no value

The vocabulary (`PhaseType` / `RoleType` / `SkillType` / `EventType` / `Camp`)
has **types only, never values**. The values all live in the rules packages.

Every one of the handful the kernel does own has to be defensible on its own
(see §7).

- **What breaking it costs**: the kernel starts knowing which game it is
  running, and the second rules package cannot be built.
- **What guards it**: the second rules package itself --
  `missions/vocab.go` and `werewolf/vocab.go` share not one value, and the
  kernel needed not one line changed.

---

## 3. The state model

### 3.1 The target shape: one variable table, one timeline

> **The kernel stores two things: a variable table, and a timeline.**
>
> Identity, life and death, and camp are **canonical keys** in the variable
> table -- the kernel recognises the key names and offers default behaviour
> from them, but **does not interpret the values**.

This is the design's central claim. Its job is to drive the number of concepts
the kernel recognises as low as possible: fewer concepts, more games carried.

### 3.2 The variable table

A scope is a 2x2 table -- lifetime crossed with ownership:

| | unowned | owned by a player |
|---|---|---|
| **whole game** | `ScopeGame` | `ScopeGame.Of(id)` |
| **this round** | `ScopeRound` | `ScopeRound.Of(id)` |

- Write: `NewSetVarEffect(scope, key, value)`
- Read: `GameView.Var(scope, key)` / `Engine.Var(scope, key)`
- Values are strings, and **an empty string is deletion**, identically in all
  four cells
- The keys are the rules' own; the kernel only stores them

**Why 2x2 and not something else**: lifetime and ownership are two orthogonal
questions, and any social deduction ruleset uses both at once. The table used
to exist only in a comment while the code had eight flat names, so nothing
forced it to be complete -- "whole game, unowned" was missing for a long time,
until the missions package's score ran into it. **A concept that needs a table
drawn to be explained has no counterpart in the code.**

**Canonical keys**: the kernel recognises a handful of key names and offers
default behaviour from them --

| Key | What the kernel does with it | What the kernel does **not** know |
|---|---|---|
| `camp` | fills `PlayerInfo.Camp` / `SelfInfo.Camp` | which sides exist, which one is good |
| aliveness (target: `alive`) | the default right to act, the default target check, the default right to speak | how they died, whether dying is losing |
| identity (target: `role`) | the default match for `PhaseStep.Role` | what this role can do |

Aliveness and identity are **struct fields on `playerState` today**, not keys
in the variable table. Demoting them to canonical keys is one of this
design's main changes; the reasoning is in §8.

### 3.3 The timeline

| Stored | What it is | Written by |
|---|---|---|
| `Phase` | the current phase | the kernel (transitions) |
| `Round` | **how many times a phase marked `EndsRound` has been passed** -- a plain counter | the kernel, with the boundary declared by the rules |
| `Actors: map[phase][]id` | the actors the rules named, spent on use | the rules (`SET_ACTORS`) or the detour queue |
| the detour queue | "someone owes an action in some phase", first in first out | the rules (`DETOUR`) |

**Why `Round` stays**: none of the engines compared has one, because their
state has no partitions. Ours does -- a round-scoped variable's lifetime needs
a boundary, and "one round" is a rules concept (three phases in the
mission-based games, eight in werewolf). So the kernel only counts, and the
rules mark the boundary in the configuration. "Round number +1" and
"round-scoped variables cleared" are two things and are declared separately
(`EndsRound` / `ClearsRoundVars`) -- most boards have them coincide, and the
mission-based games do not.

**Why `GOTO_PHASE` cannot replace the detour queue**: it governs three things,
and nothing else provides the last two --

1. routing the phase to where the debt is (`GOTO_PHASE` can do this)
2. **holding off the victory check and the round boundary until it drains**
   (the killed hunter's shot can turn the game around)
3. **taking them one at a time from the head** (two hunters eliminated on one
   night each fire their own shot)

It does **not** answer "who may act" -- on entering the phase owed to, it
writes an actor list, and everything after that takes exactly the same path as
`SET_ACTORS`.

---

## 4. Driving: how a phase runs to its end

```
SubmitSkillUse ──validate──▶ accumulate in pendingUses
                                  │
      EndPhase ───────────────────┤
                                  ▼
                            Resolver.Resolve(uses, view) ─▶ []*Effect
                                  │
                                  ▼
                            applyEffect (the only writer)
                                  │
                                  ├─▶ the effect log (replay, auditing)
                                  ├─▶ AudienceOf ─▶ OnEvent (per audience)
                                  ▼
                            next phase ─▶ victory? ─▶ transition
```

### 4.1 Who may act: two layers

| Priority | Who | How aliveness counts |
|---|---|---|
| 1 | **the players named**: `SET_ACTORS`, or the list the detour queue writes on entering the phase | the rules' business; the kernel does not veto a second time |
| 2 | **the default**: the living players matching `PhaseStep.Role` | the kernel filters on aliveness |

**Aliveness is therefore the default qualification, not the law.** Only the
kernel's own detour queue used to be able to step over it while the rules
naming actors could not -- one kernel letting its own mechanism move the dead
while forbidding the rules' mechanism from doing the same is the kernel
deciding "may the dead act" on the rules' behalf. What that blocks is real
play.

### 4.2 Submission and validation

The kernel **blocks at `SubmitSkillUse`** rather than accepting and leaving
the rules to throw it away afterwards. The reason: accept-then-discard makes
`AllowedSkills` lie to unqualified players and `PhaseReadiness` wait on a
crowd who cannot possibly act.

It validates four things, each using only information the kernel can judge:

1. is this player on the actor list (the two layers of §4.1)
2. is this skill declared in the current phase's `Steps`
3. does the target exist
4. if the target is eliminated, does this step declare `AllowDeadTarget`

### 4.3 Resolution

A `Resolver` is the whole of "how a phase resolves". It takes a read-only
board and returns effects.

**Two kinds of effect, usually both produced by one action**:

| | Example | For whom |
|---|---|---|
| **the rules' name for what happened** | `KILL` / `SHOOT` / `MISSION_FAIL` | audience decided by the rules, pushed to `OnEvent` |
| **a kernel state primitive** | `SET_ALIVE` / `SET_VAR` | for the state machine only, never sent out |

A wolf kill kills nobody -- a lone `KILL` means nothing to the state machine.
For the rules to eliminate someone, they emit a `SET_ALIVE` alongside it.
**Two effects, two things.**

The benefit of this split is that it is **independent of the cause**: an
extension that wants to intercept a death (the idiot flipping their card)
intercepts the primitive, and one piece of code stops a wolf kill, a
poisoning, a gunshot and any third-party ruleset's way of dying.

### 4.4 Where to go next: three layers of exit

```
a pending detour queue  >  GOTO_PHASE  >  PhaseConfig.NextPhase
```

Detours come first because the queue has to drain (§3.3). `GOTO_PHASE` is the
rules' runtime override (the mission-based games go to the mission when the
vote passes and back to nomination when it fails, which a static graph cannot
express). With neither, the default exit from the declarative configuration is
taken.

### 4.5 When it ends

The `VictoryChecker` is asked once after every phase transition. **The
decision is deferred while the queue is non-empty** -- a death ability can
turn the game around, the killed hunter shoots the last wolf, and the
villagers win instead.

---

## 5. Concealment: who sees what

This half is an order of magnitude stronger than what it was compared against,
and it is this library's core value. **Change it with particular care.**

### 5.1 The floor

See I3. One non-configurable rule; everything else is configurable.

### 5.2 The four providers

| Question | Answered by | The kernel's default |
|---|---|---|
| who should be told about this event | `AudienceProvider` | "I don't know" (the caller routes it) |
| who is on whose side | `TeammateProvider` | no teammates |
| who hears this speech | `SpeechProvider` | every living player |
| what does this role additionally see | `RoleInfoProvider` | nothing |

**Asymmetry is allowed**: the demon knows its minions and the reverse does not
hold; the missions package's Oberon neither knows his fellows nor is known to
them. That is the norm in social deduction games, and the kernel cannot assume
symmetry.

### 5.3 One player's three faces

| Type | Who is looking | What it has |
|---|---|---|
| `PlayerInfo` | god / the rules | everything, `Vars` included |
| `SelfInfo` | themselves | identity, camp, aliveness, **no `Vars`** |
| `PublicPlayerInfo` | everyone else | aliveness; identity only where revealed to this view |

**These three do not get merged.** `PublicPlayerInfo` **structurally cannot
hold `Vars`** -- so "should they be shown this" is a question about
signatures, not about runtime. Merging them into one type with optional fields
would demote that guarantee back into a judgement call.

Private state a player should see is **projected explicitly** by the role
through a `RoleInfoProvider`. Handing `Vars` to the player by default would
make every role work out for itself whether each entry may be shown -- exactly
the class of judgement this library sets out to take off a caller's hands.

---

## 6. The extension points

Eight, all of which can only be given at construction: once the engine is in
the caller's hands, they no longer change.

| To add | Use |
|---|---|
| how a phase resolves | `WithResolver(phase, r)` |
| how winning works | `WithVictoryChecker(c)` |
| what a role sits down with | `WithRoleSetup(role, s)` |
| what the board looks like at the start | `WithGameSetup(s)` |
| who should be told about something | `WithAudience(p)` |
| who is on whose side | `WithTeammates(p)` |
| who hears a player speak | `WithSpeech(p)` |
| what a role additionally sees | `WithRoleInfo(role, p)` |

All eight can be installed with a plain function (`ResolverFunc` /
`VictoryFunc` / ...). **Built-in roles hold no privilege** -- they go through
the same doors.

**Extension points must not call back into the engine**: they are called
synchronously while the engine holds its lock, and calling any `Engine` method
hangs (Go's RWMutex is not reentrant). The signatures are deliberately narrow:
an extension point never receives an `*Engine`. To ask the engine something
from a callback, use an `OnEvent` / `OnMessage` handler -- those run **after
the lock is released**.

---

## 7. The values the kernel owns, defended one by one

I5 says the kernel recognises no value. What follows are the exceptions, and
each has to be defensible on its own. **This table may only get shorter, never
casually longer.**

Every entry was defended again after the three rules packages were written,
and what follows carries **actual usage data** -- a defence cannot rest on
argument alone, it has to show somebody uses it.

| Value | Who uses it | Reason | Verdict |
|---|---|---|---|
| `PhaseEnd` | all three | the state machine's lifecycle end | ✅ |
| `RoleUnspecified` | all three | on a `PhaseStep` it means "every role"; a meaningful zero value saves a field | ✅ |
| `VarPresent` (`"1"`) | all three | an empty string is deletion, so has-it/hasn't-it needs an agreed non-empty value | ✅ |
| `VarCamp` | all three | so that "which side is this player on" need not be dug out of `Vars` by every caller | ✅ |
| `CampUnspecified` | all three | "not decided yet" and "belongs to no side" | ✅ |
| `PhaseStart` | one | the lifecycle start; `AddPlayer` is only allowed inside it. **The kernel needs it itself**, whether a rules package uses it or not | ✅ |
| `SkillUnspecified` | zero **by name** | but its **zero value is load-bearing**: an empty `PhaseStep.Skill` means "this role wakes but takes no action" (see §4.1). The zero value has meaning, and the name only writes it down | ✅ |
| `PhaseUnspecified` | zero | as above; the zero value exists anyway, and naming it is documentation | ⚠️ harmless, kept |
| `RoleSystem` + `SkillAnnounce` | **one** | "no player carries this step". The mission-based games have no host and neither does the one-night format -- of the three, only werewolf can use them | ⚠️ see below |
| `SkillSkip` | two | "I decline" is a move every turn-based game has, and the kernel offers a shared name so each ruleset does not invent its own | ✅ but the **privilege is gone**, see below |

### One thing this defence changed: `SkillSkip`'s kernel privilege

`validateSkillUse` used to contain:

```go
// skipping needs no target
if use.Skill == SkillSkip {
    return nil
}
```

**That branch was empty.** A submission with no target already passes target
validation (the loop never runs), and a submission that does carry a target
**should** be validated -- a `SKIP` carrying a player ID that does not exist
is a malformed submission and should not be let through quietly.

Its only real effect was to **make the kernel recognise one specific skill**,
which is precisely what I5 sets out to eliminate. It is gone. `SkillSkip`
stays as a **shared name** (which is its real value), without the privilege.

`TestSkip_HasNoKernelPrivilege` pins two things: a submission with no target
still goes through, and one with a bad target is rejected as
`ErrTargetNotFound`. Putting the privilege back turns it red immediately.

### The two only one ruleset can use: `RoleSystem` / `SkillAnnounce`

Of the three, only werewolf has a host. By the rule "an API with no users is a
liability", these look like they should go.

**They stay**, on the grounds that they are not a capability but **vocabulary**:
any ruleset needing "this step is a broadcast and waits for nobody" will use
them, and they cost nothing at runtime (readiness skips them,
`AllowedSkills` does not list them). Removed, a fourth ruleset that needed
them would have to invent a value of its own, about which the kernel would
know nothing -- and "readiness must not count it" would then have to be
reimplemented by every ruleset.

**This defence is conditional**: if a fourth and fifth rules package still
cannot use them, that says the host really is werewolf-specific, and at that
point they get deleted and moved into the werewolf package. **Writing it down
is so that it can be honoured next time.**

---

## 8. Abstraction gaps: what class of game cannot be built today

**Ordered by strength of evidence, which is directly the implementation
priority** (see [ROADMAP.md](https://github.com/Zereker/werewolf/blob/main/docs/ROADMAP.md)).

### 8.1 With evidence

| Gap | Evidence | What it blocks |
|---|---|---|
| **aliveness is a privileged bool, one bit only** | the mission-based rules use it nowhere, yet pay for it in a snapshot field, a view field and three default decisions | Blood on the Clocktower's poisoned / drunk / protected are parallel state bits; "silenced but alive" |
| ~~**identity is fixed at seating**~~ | **judged wrong, withdrawn.** The third rules package (One Night, exactly the one named here) proved the opposite: what a card-swapping game wants is not "a writable `RoleType`" but **two layers of identity** -- the card dealt (decides what you do at night, never changes) and the card in hand now (decides which side you score for, does change). One layer from the kernel and one from the rules is exactly enough; flatten them and the robber wakes up with the wolves and the game collapses on the spot. **Immutability is the value here.** See [onenight/SCARS.md scar 0](https://github.com/Zereker/werewolf/blob/main/onenight/SCARS.md) | — |
| **the detour queue's naming and docs still say "death ability"** | the concept has been generalised, the words did not follow | pure documentation debt, zero risk |

The first two share one fix: **demote them to canonical keys in the variable
table** (§3.1). That is not two changes but one -- the state model converging
from "a variable table plus two privileged fields" into "a variable table".

### 8.2 Speculative (**not one game has run into them**)

| Gap | What it might block |
|---|---|
| victory has a single `Camp` | **One Night has run into it** (the tanner can win alongside the villagers, a routine outcome of the base game). Judged as **wait for a second collision** -- it is the only breaking signature change, and one ruleset is not enough to move an interface that was just frozen |
| `SkillUse.Targets` can only hold player IDs | **One Night has run into it** (looking at a centre card). Judged as **not fixing it for now**: the way around it (encoding the index into the skill name) is ugly and tells no lie, at a cost of 15 lines |
| one `Resolver` per phase, composition by hand-wrapping | games whose phase resolution is heavily reused |
| the kernel has no randomness | games that draw or roll during play (**it was added once, and removed for having no users**) |

**None of these four moves until a game really runs into it** -- and "ran into
it" is not the same as "change it now": it still has to pass the test at the
top of [`SCARS.md`](https://github.com/Zereker/werewolf/blob/main/onenight/SCARS.md)
(**if you can work around it, it is an ergonomics problem, not an abstraction
problem**), and "should a breaking signature change wait for a second
collision". The reasoning is in the next section and in
[ROADMAP.md §0](https://github.com/Zereker/werewolf/blob/main/docs/ROADMAP.md).

---

## 9. The will-not-do list

Refusals need writing down more than acceptances do, or every one of them gets
proposed again.

| Will not | Why |
|---|---|
| **fill in APIs from a comparison table** | `Rand` came from exactly that: added against boardgame.io's table, sound design, zero users, deleted. **A comparison tells you what you are missing; it cannot tell you whether you need it.** |
| **add abstraction for "it might be useful later"** | a generalisation from a sample of two is not a generalisation, it is a guess. Wait for the third. |
| **build any role, phase or skill into the kernel** | I5. Break this and the second rules package cannot be built. |
| **merge the three faces (§5.3)** | it would demote a compile-time guarantee into a runtime judgement. |
| **hard-code a metrics set, a timeout policy or a transport for the host** | those are the host's to decide (§1, third question). `Metrics` was deleted for exactly this. |
| **open a second write point beside `Effect`** | I1. `Engine.Apply` is **the same** write point, not a second one. |
| **split "who may act" into a third path again** | I4. Going from three layers to two cost one refactor. |

---

## 10. Two consumers landing on this abstraction

The kernel does not recognise a single word below. This table is the evidence
for "one abstraction, two completely different fillings".

| Abstraction | Werewolf | The mission-based rules |
|---|---|---|
| the phase cycle | 8, static | 3 in a loop plus 1 entered conditionally, routed by `GOTO_PHASE` |
| who may act | by role (the default path) | computed at runtime (`SET_ACTORS`: the leader rotates by seat, the team comes from a nomination) |
| elimination | the core mechanic | **not used at all** |
| the round boundary | the end of the vote phase | the end of one mission, and it does **not** coincide with variable lifetime |
| whole-game unowned variables | not needed | the score, the consecutive-reject count, whose turn it is to lead |
| this-round owned variables | guarded / healed / poisoned tonight | on this round's mission team or not |
| the detour queue | the hunter's shot after being killed | not needed |
| victory | wipe out one side (counting special roles / villagers / wolves) | three successful missions, and the assassin failing to identify Merlin |
| asymmetric information | the wolves know each other; the witch sees the kill | Merlin knows the bad guys (except Mordred); Percival sees Merlin and Morgana without telling them apart; Oberon is isolated both ways |
| camps | `GOOD` / `EVIL` | `GOOD` / `EVIL` (same names, entirely different meanings and decisions) |

**The most convincing line is the elimination one**: one ruleset uses it as
the core mechanic, the other never once, and the kernel needs no change. Which
is exactly the argument in §8.1 that aliveness should be demoted to a
canonical key -- it is **one game's core concept**, not the kernel's.

---

## Appendix: how this design gets verified

Whether a design is right does not rest on review, it rests on these four
things:

1. **Two independent rules packages** -- sharing not one value. When a third
   is added, the size of the kernel change is the measure of generality.
2. **Mutation verification** -- for every rule added, break it by hand and
   confirm a test goes red. "Removing `consumeActors` turned not one test red"
   really happened, and at that point the rule was only a comment.
3. **5000 random games** (three rules packages combined) -- each one comparing
   snapshot round trips, effect-log replay and the invariants.
4. **A byte-level golden snapshot** -- drift in the save format goes red
   immediately.

**A rule cannot live only in a comment.** That is this project's one quality
slogan.
