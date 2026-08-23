# How the kernel is put together

## Scope

This engine is a pure state machine library: **no clock, no IO, no concurrency
scheduling**. It does not decide "when day breaks"; it answers "in the current
phase, who may do what, and what does the world look like once they have".
Timing, networking, persistence and AI decisions all belong to the caller.

That boundary decides everything below it.

> This describes the structure of the code as it stands. For what it *should*
> look like and why, see [DESIGN.md](DESIGN.md); for how it compares with
> other engines, see [PRIOR-ART.md](PRIOR-ART.md); for the frozen surface, see
> [API.md](API.md).

## Principles

### 1. Phase-centric, not role-centric

Rules hang off **phases**, not off roles. "Who the guard may protect" is not a
method on a guard class; it is the configuration of the `NIGHT_GUARD` phase
plus the judgement of its resolver.

The benefit is that adding a role does not mean editing the engine: a new role
is a new `PhaseConfig` plus a new `Resolver`.

### 2. Every state change goes through an Effect

A `Resolver` is handed a read-only `GameView` and can only produce `Effect`s
describing what it wants to happen; a single internal write point lands them.
The constraint is **held up by the signature**, not by convention -- it used
to live only in the documentation while a Resolver received a mutable state
object, so any implementation could bypass the whole pipeline.

```
SkillUse ──► Resolver ──► []*Effect ──► applyEffect ──► new state
 (input)    (pure func)  (description)  (the only writer)
```

Three benefits fall out directly:

- **Testable**: a Resolver is a pure function; a given input always yields the
  same list of effects.
- **Auditable**: a game is a sequence of effects -- serialisable, replayable,
  reportable.
- **Cancellable**: `Effect.Canceled` plus `Reason` make "why it did not
  happen" a first-class thing (protected, no antidote left, a
  consecutive-protection limit -- all of them reasons, not silent drops).

A historical lesson: a wolf resolver once produced no kill effect at all when
the target was protected, which made the "guarded and healed on the same
night" situation impossible to express in the engine. The kill is now always
recorded and life-and-death converges on the resolution phase --
**concentrate the judgement, do not discard information early**.

### 3. Configuration is data, not code

Phase transitions, the skills available per phase and rules variants are all
serialisable data structures.

### 4. One source of truth

"What skills may this player use right now" has one source: `PhaseConfig.Steps`.
Validation in `SubmitSkillUse`, the outward declaration in `PhaseInfo`, the
projection in `PlayerView` and the decision in `PhaseReadiness` are all
derived from it.

A historical lesson: two of those used to hard-code their own skill lists, so
`PhaseInfo` announced that the hunter could `SKIP` while `SubmitSkillUse`
rejected `SKIP`.

### 5. The information boundary belongs to the engine

A caller acting as the host needs the god's view, but it should not be forced
to implement the projection itself -- that is the most safety-critical logic
in the whole game, and leaving it outside the library means every user rewrites
it, and getting it wrong once voids the game. `PlayerView` and `AudienceOf`
pull it back inside.

### 6. The engine recognises no specific role

The hunter was once hard-coded into the phase transitions: the next-phase
computation had a branch for it, and the round context had a field of its own.
Every additional death-triggered role meant editing the engine again. The
engine now recognises only the abstraction "who, and to which phase", and who
that is is the Resolver's decision.

## The shape of it

```
                     ┌──────────────────────────────┐
   caller            │           Engine             │
 (timing/net/AI) ───►│  ┌────────────────────────┐  │
                     │  │ SubmitSkillUse         │  │  collect skills
                     │  │   └─ validateSkillUse  │  │  (validated against Steps)
                     │  ├────────────────────────┤  │
                     │  │ EndPhase               │  │  drive the game
                     │  │   1. Resolver.Resolve  │──┼──► []*Effect
                     │  │   2. applyEffect       │  │
                     │  │   3. detour pending?   │  │
                     │  │   4. CheckVictory      │  │
                     │  │   5. next phase        │  │
                     │  ├────────────────────────┤  │
                     │  │ PhaseInfo              │──┼──► who acts, with what
                     │  │ SendMessage            │──┼──► speech routed by phase
                     │  └────────────────────────┘  │
                     └───────┬──────────────┬───────┘
                             │              │
                     ┌───────▼──────┐  ┌────▼──────────────┐
                     │  gameState   │  │   phaseManager    │
                     │ players/round│  │ config + resolvers│
                     └──────────────┘  └───────────────────┘
```

## The modules

### Engine (`engine.go`)

A lightweight state machine whose public surface falls into four groups:

| Group | Methods | Notes |
|---|---|---|
| setup | `AddPlayer` | only before `Start`; returns an error |
| driving | `Start` / `EndPhase` | `EndPhase` is the sole way forward |
| input | `SubmitSkillUse` / `SendMessage` | skills and speech are two independent channels |
| reading | `Status` / `PhaseInfo` / `PlayerInfo` / `View` … | always read-only copies |

**Invalid input always returns an error and never takes effect silently**: a
duplicate ID, an empty ID, seating the system role, adding a player after the
start, or a board that is already decided before play begins are all rejected
at the entry point. Camp and category are not parameters; a role hands them
out as initial state through `WithRoleSetup`.

**Concurrency**: every exported method may be called concurrently. User
callbacks (`OnEvent` / `OnMessage`) always run **after the lock is released**,
and the handler list is snapshotted while it is held -- so there is no
deadlock (a callback may safely call Engine methods) and no race with a
concurrent `OnEvent` registration. A panic in one handler is isolated, logged
at error level, and does not affect the others.

### State (`state.go`)

Holds all game state and is the **single write point** (`applyEffect`).

- Custom state is a 2x2 table (`VarScope`): lifetime (whole game / this round)
  crossed with ownership. The two owned cells live in the unexported
  `playerState`'s `Vars` and `RoundVars`; the two unowned ones in
  `gameState.Vars` and `RoundContext.Vars`. The keys are the rules' own and
  the kernel only stores them -- the witch's potions, who the guard protected
  last round and the missions package's score all take this one route. Write
  with `NewSetVarEffect(scope, k, v)`, read with `GameView.Var(scope, k)`;
  outside code only ever sees read-only copies.
- `RoundContext` holds this round's unowned state and the pending detour
  queue. The queue governs three things -- routing the phase to where the debt
  is, holding off the victory check and the round boundary until it drains,
  and taking them one at a time from the head. It does **not** answer "who may
  act": on entering the phase it writes an actor list, and everything after
  that takes exactly the same path as `NewSetActorsEffect`. The whole
  RoundContext is rebuilt whenever a new round begins.

**An actor list is one-shot**, and so is the detour at the head of the queue.
Both are consumed by `leavePhase` when the phase is left. Without that, the
next visit to the same phase inherits the previous round's list, or the same
player is dragged back to use a skill again and again.

### Phase (`phase.go`)

The lookup entry point for phase configuration, the resolver registry, and
skill validation (`validateSkillUse`).

### Resolver (`resolver.go`)

One per phase, with one signature:

```go
Resolve(uses []*SkillUse, view GameView) []*Effect
```

It reads `GameView` only and expresses state changes only by returning
effects. It is called while the engine holds its lock, so it must not call
back into any `Engine` method -- the consequence is a hang, not an error.

### Effect (`effect.go`)

The description of a state change. Event types fall into classes decided by
**who owns the name**, recorded in the `kernelEvents` table, not by a numeric
range:

| Class | What it is | Sent out? |
|---|---|---|
| `kindStateWrite` | `SET_ALIVE` / `SET_VAR` / `SET_ACTORS` / `DETOUR` | never |
| `kindControl` | `GOTO_PHASE` | never |
| `kindReplay` | `PLAYER_ADDED` / `PHASE_CHANGED` | never |
| `kindRuleEvent` | anything else -- the rules' name for what happened | pushed to `OnEvent` |

The numbered era had three ranges (1..99 external, 100..999 internal, 1000 up
third-party) and that convention bit itself: every third-party event type
landed inside the "internal" range, so an extension's events could not be sent
at all.

## Data flow

### One phase's full life

```
1. the caller reads PhaseInfo()   ──► who acts this phase, and with what
2. the caller collects decisions  ──► (timeouts, AI, networking: all theirs)
3. SubmitSkillUse() x N           ──► validated, then queued
4. the caller calls EndPhase()
   ├─ Resolver.Resolve(pendingUses) ──► []*Effect
   ├─ applyEffect for each effect
   ├─ leavePhase: consume the actor list and the head detour
   ├─ work out the next phase (detour queue > GOTO_PHASE > NextPhase)
   ├─ CheckVictory -- deferred while a detour is still pending
   └─ dispatch outwardly visible events outside the lock
5. back to 1
```

**Why the victory check is deferred**: the hunter, killed, may already have
completed a wipe-out, but his shot may take the last wolf and hand the game to
the villagers instead. The check has to come after the death ability resolves.

## Extension points

Everything is installed at construction, and all four entry points accept the
options (`NewEngine`, `MustNewEngine`, `RestoreEngine`, `ReplayEngine`):

| To add | Use |
|---|---|
| how a phase resolves | `WithResolver(phase, resolver)` |
| what a role sits down with | `WithRoleSetup(role, setup)` |
| the board at the start of the game | `WithGameSetup(setup)` |
| how winning works | `WithVictoryChecker(checker)` |
| role-specific information | `WithRoleInfo(role, provider)` |
| who should be told about something | `WithAudience(provider)` |
| who is on whose side | `WithTeammates(provider)` |
| who hears a player speak | `WithSpeech(provider)` |
| logging | `WithLogger(l)` |

All of them live in the kernel package -- extending the rules is rewiring the
kernel, and the `hiddenrole.` prefix at the call site is deliberate.

An ability triggered on elimination is produced by a Resolver as a
`NewDetourEffect`; the engine queues it, routes to it automatically, and
defers the victory check until it resolves.

## Naming conventions

Read-only methods take **no `Get` prefix** -- the ordinary Go style (Effective
Go): `e.Status().Phase` rather than `e.GetCurrentPhase()`, `e.PlayerInfo(id)`
rather than `e.GetPlayerInfo(id)`. A method sharing its name with its return
type is fine; `time.Time.Location()` returning a `*Location` is the same
shape.

Anything of no use outside the package stays unexported. The phase manager
(`phaseManager`) has no injection point, the state object (`gameState`) is an
implementation detail, and vote tallies never leave the package -- exporting
them would only make `go doc` longer without letting anyone do one more thing.

## What it does not do

Explicitly out of scope, to avoid misunderstanding:

- **No clock**: `PhaseConfig.Timeout` is advice for the caller.
- **No role assignment**: who is a wolf is the caller's decision, told to the
  engine through `AddPlayer`.
- **No dealing or shuffling**: the mapping from seats to identities is given
  by the caller.
- **No storage**: `Snapshot` / `RestoreEngine` export and rebuild the board,
  but where and how it is stored is the caller's decision.
- **It does not decide when a phase ends**: `PhaseReadiness` tells you who is
  still missing; whether to wait or advance on a timeout is the caller's
  decision, and `EndPhase` never refuses on the grounds of not being ready.
- **No networking, no AI.**

## Saving

`Engine.Snapshot()` exports the board and `RestoreEngine(config, snap)`
rebuilds an engine.

**The snapshot types and the engine's internal types are deliberately two
separate sets**: the internal ones evolve with refactoring, while a snapshot
is a format written to storage and its field names must stay stable. The
conversion is all in `snapshot.go`, so adding or removing a field produces an
explicit compile error there rather than silently losing data.

The trade-offs:

- **It does not contain the rules configuration.** A snapshot records the
  board, and the caller supplies the `Config` on restore -- versioning the
  rules is the caller's business, and mixing them into a save leaves both
  sides unable to say what they hold.
- **It does contain unresolved skills.** `pendingUses` are exported too, so a
  save point need not sit on a phase boundary.
- **Enums serialise by name** (`"NIGHT_GUARD"`, not `21`). A save is meant to
  be read by people and possibly by other languages, and numbers do not line
  up. Since the enums became strings this needs no translation layer, and a
  third party's custom values are names like any other.
- **The output is deterministic.** Sets and player lists are sorted before
  export, so one board gives identical bytes -- which makes comparison and
  idempotent writes possible (Go's map iteration order is randomised, so
  without sorting neither is).
- **A version mismatch is rejected outright**, rather than reading old data
  through a new structure -- which would give a board that looks fine and is
  in fact scrambled. `SnapshotVersion` carries the version, and a golden test
  pins the serialised shape so that changing the structure without bumping the
  version goes red.

## The effect log

`EffectLog` accumulates every effect since the game was created, and
`ReplayEngine` rebuilds the board from it. Three events -- `PLAYER_ADDED`,
`GAME_STARTED` and `PHASE_CHANGED` -- make the log self-contained, so a full
rebuild needs no outside information.

The division of labour with snapshots: **the effect log is history, a snapshot
is state**. Persist with a snapshot (`Effect.Data` is a
`map[string]interface{}` whose types degrade on a JSON round trip), and use
the effect log for in-process replay, post-game analysis and investigation.

## Testing infrastructure

Two public pieces, both for rules packages rather than for the kernel:

- `Board` / `Seat` / `Mark` lay out one board by hand so that a rules package
  can unit-test its own resolver without running a whole game. `Board.Apply`
  goes through exactly the same write point as the engine, so "the effect
  never landed" shows up in a unit test.
- [`enginetest`](enginetest/) runs random games against seven general
  invariants (`RunFuzz`). Not one of them knows any game: they check whether
  what was stored reads back the same, whether replay arrives at the same
  board, and whether somebody the engine says cannot act really cannot.

Both are pinned by `TestAPI_SurfaceIsPinned` along with the rest of the
exported surface -- a public test API is still public API.
