# What objects are hiding inside the API (archived)

> **This is a one-off audit record. It is archived and no longer updated
> alongside the code.**
>
> It was written before the API converged, and its job was to lay out the
> places where "a concept exists but has no counterpart in the code". Of five
> findings, four have since converged and one was judged **a mistaken call
> that should not be acted on**; each section is annotated below. The numbers
> in it are the numbers **at the time**, and it is normal for them not to
> match the code today.
>
> For the API as it stands, see [API.md](API.md) -- that one is guarded by
> `TestAPI_SurfaceIsPinned` and always agrees with the code. For design
> intent, see [DESIGN.md](DESIGN.md).
>
> It is kept because **it records the judgement process**: the test it used
> ("a concept that needs a table drawn, or a naming prefix, to be explained
> has no counterpart in the code") gets used again later, and finding 4, "I
> judged this wrong", is especially worth keeping -- not every audit's
> conclusion is right.


This document proposes no fixes. It does one thing: **lay out the places in
the current API where a concept exists but has no counterpart in the code**,
so that what to change, if anything, can be decided.

The data comes from `go doc -all ./engine`: **53 types, 25 package-level
functions, 59 methods** (after scopes converged; before that it was 52 / 28 /
57, the same total).

There is one test:

> **A concept that needs a table drawn, or a naming prefix, to be explained
> has no counterpart in the code.**

---

## 1. Variable scope: a 2x2 table flattened into eight names -- converged

This one is the most glaring, because **the evidence is documentation we
wrote ourselves** -- `state.go`, `view.go`, `SCARS.md` and the CHANGELOG all
drew this table:

|  | unowned | owned by a player |
|---|---|---|
| **whole game** | `GameVar` | `PlayerVar` |
| **this round** | `RoundVar` | `PlayerRoundVar` |

And in the code it was eight unrelated names:

|  | write | read |
|---|---|---|
| whole game, unowned | `NewSetGameVarEffect(k,v)` | `GameVar(k)` |
| whole game, one player | `NewSetPlayerVarEffect(id,k,v)` | `PlayerVar(id,k)` |
| this round, unowned | `NewSetRoundVarEffect(k,v)` | `RoundVar(k)` |
| this round, one player | `NewSetPlayerRoundVarEffect(id,k,v)` | `PlayerRoundVar(id,k)` |

**The symptom is checkable**: scar 4 is "a missing cell" precisely because
nothing forced the table to be complete. Were a scope a type, a missing cell
would not be expressible; being four functions, one left unwritten is nobody's
job to notice -- and in fact one was, until the mission-based rules ran into
it.

"The concept" = a scope (lifetime x ownership).
"Its counterpart in the code" = none.

**Converged**: a scope is now the `VarScope` type, the four cells fall out of
two values crossed with one method, writes go through
`NewSetVarEffect(scope, k, v)` and reads through `Var(scope, k)`:

|  | unowned | owned by a player |
|---|---|---|
| **whole game** | `ScopeGame` | `ScopeGame.Of(id)` |
| **this round** | `ScopeRound` | `ScopeRound.Of(id)` |

The same fault turned out to be in two more places: `Engine` had only two
cells (the owned ones could not be read), and `Board` was missing "whole game,
unowned" (so a board with a score could not be laid out). Both were filled in.

The total number of names did not drop (the names describing this table went
from 15 to 11, and the kernel's exported total is still 137). What was bought
is **completeness**: the four cells can be enumerated, and a missing one is
caught by a test rather than by the next rules package.

---

## 2. Effect constructors: six free functions mixing two kinds of thing

```
NewEffect              the rules naming what happened
NewSetAliveEffect      changes state
NewSetVarEffect        changes state
NewDetourEffect        a directive: queue someone into some phase
NewGotoPhaseEffect     a directive: where to go next
NewSetActorsEffect     a directive: who may act in some phase
```

(Down from nine to six once the four Var constructors converged into one, but
this finding's fault is unchanged: two kinds of thing are still flattened
together.)

Two kinds of thing side by side with no type distinguishing them: **those that
change state** and **those that give a directive**.

The consequence had already shown up once: `GOTO_PHASE` was in the
`kernelPrimitives` table, whose documentation read "they are the state
machine's bookkeeping (whose alive bit flipped, who gained a marker)", while
`GOTO_PHASE` **has no branch in `applyEffect` at all and changes no state**.
The behaviour was right (never sent out); the classification was wrong.

**The classification has converged; the constructors have not.**
`kernelPrimitives` (`map[EventType]bool`) became `kernelEvents`
(`map[EventType]eventKind`), with three classes:

```
kindStateWrite   SET_ALIVE / SET_VAR / SET_ACTORS / DETOUR
kindControl      GOTO_PHASE                    -- changes no state, only where to go next
kindReplay       PLAYER_ADDED / PHASE_CHANGED  -- only meaningful on the replay path
```

The original two-way split (changes state / gives a directive) was not
accurate either: `PLAYER_ADDED` and `PHASE_CHANGED` are neither, they are
replay bookkeeping.

With the class as a value, that comment becomes assertable: every
`kindStateWrite` is tried against a clean state and one that cannot change
anything is misclassified; every non-`kindStateWrite` must leave the state
identical field by field. Putting `GOTO_PHASE` back into `kindStateWrite`
(that is, today's mistake) turns it red immediately.

`eventKind` is **unexported**: no caller outside needs it, and
`isInternalEvent` is its only exit. The concept has a counterpart without
widening the public API along the way.

**The constructor side is untouched**: six `NewXxxEffect` functions, still
flat. Whether to give them types too (splitting `Effect` into two types, say)
is the next question -- at the cost of every rules package's return
signatures.

"The concept" = what the rules say to the kernel, in three classes.
"Its counterpart in the code" = `eventKind` (unexported); the constructors are
still flat.

---

## 3. Extension points: eight things, twenty-four names -- half fixed

Eight extension points, each spread across 2-3 names:

| Extension point | Interface | Func adapter | With option |
|---|---|---|---|
| `Resolver` | ✓ | ✓ | ✓ |
| `VictoryChecker` | ✓ | ✓ | ✓ |
| `AudienceProvider` | ✓ | ✓ | ✓ |
| `TeammateProvider` | ✓ | ✓ | ✓ |
| `SpeechProvider` | ✓ | ✓ | ✓ |
| `RoleInfoProvider` | ✓ | ✓ | ✓ |
| `RoleSetup` | ✓ | ✓ | ✓ |
| `GameSetup` | ✓ | ✓ | ✓ |

**Filled in**: `Resolver` and `VictoryChecker` had no Func adapter while the
other six did. That asymmetry had no reason but history, and `ResolverFunc`
and `VictoryFunc` have been added --
`TestExtensionPoints_AllHaveFuncAdapters` installs eight function literals
straight into one engine, so a missing adapter fails to compile.

**The other half remains**: an extension point is still three names (an
interface, an adapter, an option). Whether to change that is another question
-- the three names each do their own job (the interface gives a type, the
adapter gives convenience, the option gives assembly), unlike scopes, where
one thing was spread out.

"The concept" = one extension point.
"Its counterpart in the code" = an interface, an adapter and an option
function: three names.

---

## 4. Shadow types: three shapes for one set of data -- judged wrong, enforcement added instead

The same game state wears three faces in the code:

| Internal | Read-only outward | Save |
|---|---|---|
| `playerState` (unexported) | `PlayerInfo` / `PublicPlayerInfo` / `SelfInfo` | `PlayerSnapshot` |
| `RoundContext` | `RoundContext` | `RoundCtxSnapshot` |
| `SkillUse` | `SkillUse` | `SkillUseSnapshot` |
| `Detour` | `Detour` | `DetourSnapshot` |

The four `*Snapshot` shadow types exist **deliberately** (a snapshot is a
format written to storage, its field names must stay stable and must not drift
with an internal refactor -- that is written in `snapshot.go`, and I still
think it is right).

The view column I originally judged to be the same problem: `PlayerInfo` /
`PublicPlayerInfo` / `SelfInfo` all describe "one player, exposing different
fields depending on who is looking", the "who is looking" dimension has no
counterpart, and so it became three type names.

**That call was wrong, and they should not be merged.** Unlike scopes -- whose
four cells behave identically and differ only in where they hang, so they
merge -- these three are **three different contracts**:

```
PlayerInfo         god's view      ID Role Alive Vars RoundVars
SelfInfo           their own       ID Role Alive Camp
PublicPlayerInfo   everyone else   ID Alive Role(only where revealed to this view)
```

`PublicPlayerInfo` **structurally cannot hold** `Vars`. That is not a naming
coincidence, it is a compile-time guarantee of the same rank as "a `Resolver`
can only return `Effect`s": merge them into one type with optional fields and
"should they be shown this" falls back from a question about signatures to a
question about runtime.

**What was actually missing was not a type but enforcement.** "What goes into
it is up to the role, and handing it to the player by default would make every
role work out for itself whether each entry may be shown" -- that is written
on `PlayerInfo.Vars` and is the entire reason the three faces are kept apart,
and it used to be **only a comment**. Anyone adding a
`Vars map[string]string` to `SelfInfo` would send how many potions the witch
has left to the whole table, and nothing would make a sound. The same class of
problem as `GOTO_PHASE`: a rule written in a comment.

`TestPlayerView_CarriesNoFreeFormState` now walks `PlayerView`'s entire type
graph by reflection and treats any `map[string]string` as a leak, with only
`PlayerView.RoleInfo` on the allow-list (a deliberate disclosure the role
projects **explicitly**). Adding a bag to `SelfInfo` or `PublicPlayerInfo`
turns it red immediately.

Its companion `TestPlayerView_ShapeTestActuallyWalks` watches that test
itself: walking a type graph by reflection is easy to short-circuit with one
early return, after which it checks nothing and is green forever.

---

## 5. `Engine`'s methods -- the summary group has converged

There were 27, and a run of them were **the same thing at different
granularities**: `Phase()`, `Round()`, `Var()`, `PlayerInfo()`,
`AlivePlayerIDs()` and `RoundContext()` ask the same set of questions as
`View()`.

The earlier defence was "`View()` clones the whole state, and asking `which
round is it` should not cost that" -- a performance tier, not duplication.
That defence holds, but it explains **why there are two paths**, not **why the
cheap path is spread across seven methods**.

**The group that converged did so not because there were too many names, but
because it could tear.** `Phase()` / `Round()` / `IsGameOver()` / `Winner()`
each took their own read lock: a host rendering "the day of round 3" had to
ask twice, and if another goroutine resolved a phase in between, it read a
combination of values that **never held at the same time**. The four merged
into one `Status()`, four scalars read under one lock with no allocation.

```
Status{ Phase, Round, Over, Winner }
```

`TestStatus_IsAtomic` advances phases while reading concurrently, asserting
that the combination read is always legal (over means stopped at `PhaseEnd`,
not over means no winner yet). Going back to four separate locks turns it red.

**The other three are untouched**: `Var(scope, key)` / `PlayerInfo(id)` /
`AlivePlayerIDs()` take parameters or allocate; they are not summary fields,
and putting them in a struct would only make every read pay a cost it does not
need.  `View()`'s path is unchanged.

There are 23 methods now.

---

## Summary

| Concept | Its counterpart in the code | Spread across how many names |
|---|---|---|
| variable scope (2x2) | ~~none~~ -> `VarScope` | ~~8~~ -> converged |
| what the rules say to the kernel (three classes) | `eventKind` (unexported) | 6 (constructors not converged) |
| one extension point | partly (the interface, not the assembly) | 8 x 3 = 24 (now symmetric, not converged) |
| "who is looking at this data" | three types (**and it should be three**) | 3, not to be touched |
| the cheap state readers (the summary group) | `Status` | ~~4~~ -> converged |

**A substantial share of the 53 exported types come from concepts with no
counterpart being spread out.**

---

## Deliberate, and not to be touched

So that they are not damaged later:

- **The four `*Snapshot` shadow types** -- a snapshot is a format written to
  storage and must stay decoupled from the internal structures.
- **`GameView` being separate from mutable state** -- the rules get a
  read-only view and can only return `Effect`s, a constraint held up by the
  signature and one of this library's most valuable properties.
- **`Engine` and `GameView` coexisting as two read paths** -- the performance
  tier is real.
