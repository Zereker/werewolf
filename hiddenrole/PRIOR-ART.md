# Prior art: boardgame.io's engine, and ours

This document started from one sentence: after reading three comparable
frameworks, **not one of them puts "who may act" into static configuration --
only we do**. That sentence deserved following up. Did we think about it
differently, or did we never think about it at all?

So [boardgame.io](https://github.com/boardgameio/boardgame.io)'s engine was
read end to end and compared point by point. It was chosen because it does the
same thing we do (a state kernel for turn-based multiplayer plus an
information boundary), and because it is open source with both docs and source
available.

These are the files that were read, and every conclusion points back into
them:

- `src/core/turn-order.ts` -- the set of active players
- `src/core/flow.ts` -- phase transitions and the may-act decision
- `src/core/reducer.ts` -- the write path and the history
- `src/plugins/random/random.ts` -- randomness
- `docs/documentation/{stages,phases,secret-state}.md`

---

## One table

Any engine of this kind has to answer the same set of questions. Both answers
side by side:

| Question | boardgame.io | Us | Verdict |
|---|---|---|---|
| how state changes | a single reducer; a move mutates `G` through immer | a single write point, `applyEffect`; the rules can only return `[]*Effect` | **we are stronger** |
| how what happened is recorded | `deltalog` + an `_undo/_redo` stack + `_stateID` | `EffectLog` (copies in and out) + snapshots + replay | comparable |
| **who may act** | `ctx.activePlayers`, a runtime set living in state | `SetActors`, a runtime list living in state | since fixed; same shape |
| what they may do | `GetMove` layered: stage -> phase -> global | `PhaseStep.Skill`, enumerated per phase | comparable |
| where to go next | `next` may be a string or a function | `NextPhase` as the default, overridden by a `GOTO_PHASE` effect | comparable (we only just fixed it) |
| what a round is | **no such concept**, only turns and phases | `Round`, declared by `EndsRound` | we have one more; questionable |
| who can see what | one function, `playerView(G, ctx, playerID) -> G'` | a structured `PlayerView` + `AudienceOf` + a non-configurable floor | **we are clearly stronger** |
| randomness | PRNG state lives in game state, so replay is deterministic | the kernel offers none | added, then removed; see below |
| when a phase ends | `endIf` / `maxMoves` end it automatically; the framework runs it | not our business; the caller calls `EndPhase` | deliberately different, and we are right |
| how extensions are added | a plugin system plus configuration | seven concrete extension points plus effect primitives | different approach |
| who runs it | ships client/server/master plus transport | we do not | deliberately different |

---

## Two places where we are stronger

### The write constraint is held up by a signature, not by convention

Their moves can mutate `G` however they like -- immer only turns the mutation
into an immutable update, and **nothing stops a move doing whatever it
wants**. Our `Resolver.Resolve(uses, view) []*Effect` cannot reach mutable
state **at the type level**: a read-only `GameView` goes in, an `Effect` comes
out.

This is not a stylistic difference. "Every change to state goes through one
write point" is the premise of all three benefits -- snapshots, replay,
auditing. They maintain it by discipline; we maintain it with the compiler.

### The information boundary is not a function that deletes fields

Their `playerView` is "write a function yourself that deletes the fields they
should not see", and the default implementation
`PlayerView.STRIP_SECRETS` relies on a **naming convention** -- it deletes
keys called `secret`.

On our side there are three things:

- **A structured `PlayerView`** -- `Self` / `Players` / `RoleInfo` /
  `Teammates`, each cell with a fixed meaning, not "a free-form object with a
  few keys deleted";
- **`AudienceOf` as a separate path** -- "who should be told about this" and
  "what can this person see right now" are two questions, and they only solved
  the second;
- **A non-configurable floor** -- state primitives never leave the building,
  and the rules cannot change that.

Writing the mission-based rules, this whole area cost **zero friction**
(Merlin's one-way visibility, Oberon's two-way isolation, Percival's
either-or, anonymous mission-fail votes). This is the most valuable part of
this library.

---

## Two places where we owe

### 1. Who may act ([SCARS.md](https://github.com/Zereker/werewolf/blob/main/missions/SCARS.md) scar 1)

Their decision is three lines:

```javascript
function IsPlayerActive(_G, ctx, playerID): boolean {
  if ((ctx._removedPlayers || []).includes(playerID)) return false;
  if (ctx.activePlayers) return playerID in ctx.activePlayers;   // the runtime set wins
  return ctx.currentPlayer === playerID;                          // otherwise fall back to the default
}
```

Three points, each of which is a slap in our face:

1. **`ctx.activePlayers` is state**, inside the serialised `ctx`, so it goes
   into saves and can be replayed. They deliberately did **not** put it in `G`
   (the bag for arbitrary game state) and gave it a dedicated place in `ctx`
   instead, with a lifecycle to match (a `_prevActivePlayers` stack, popped
   automatically when the set empties).
   -> **The active-player set deserves to be a first-class concept, not an
   ordinary variable.**
2. **A default plus a runtime override** -- use the runtime set if there is
   one, fall back to the default otherwise. Which is exactly the shape we just
   used for `GOTO_PHASE`.
3. **The framework enforces it itself**, at the entry point in `master.ts`:
   `if (!this.game.flow.isPlayerActive(...)) { logging.error('player not active'); return; }`
   -- rather than leaving the game logic to filter afterwards.

Our approach was a static `PhaseStep{Role, Skill}` match plus `peekTrigger` as
a one-player special case. The consequence was the kernel lying to unqualified
players (`AllowedSkills` saying they may act, `PhaseReadiness` waiting on
them).

**One thing that fell out along the way**: our
`NewDetourEffect(playerID, phase)` means "this one player, acting in this
phase" -- **it was already the one-player special case of "who may act in
phase X"**. Adding the capability was not adding a new mechanism, it was
generalising an existing one from one player to a set, and the special case in
`validateSkillUse` could most likely be deleted along with it.

### 2. Randomness (never recorded before)

They store the PRNG's state in the game state: two fields, `seed` and
`prngstate`, with the new PRNG state written back after every draw. So
**replay reproduces the exact same random sequence** and moves "stay pure".

Our kernel has **no randomness at all**. A `Resolver` has to be a pure
function of the board, so randomness can only be rolled by the host outside,
and the result does not enter the effect log -- so that part **cannot be
replayed**.

Both werewolf and the mission-based rules dodged this (dealing happens before
the game is created, and nothing during play needs randomness). But any
ruleset with randomness during play -- dice, drawing cards, random events --
either cannot be built on us, or is built and loses replayability, which is
one of this library's selling points.

**This is one the mission-based rules did not run into; only the comparison
surfaced it.** It proves that "write a second rules package" and "read how
somebody else did it" are two different tests, and neither can be skipped.

**Added, then removed.** We did add `GameView.Rand()` -- a stream determined
uniquely by (seed, current round, current phase), storing no PRNG progress,
one size smaller than the reference implementation. The design holds up, and
the reason is that our constraint is stronger than theirs (resolution is a
pure function of the board), so progress need not be recorded.

It was removed because **it had not a single user**. Neither rules package
rolls during play; it was added against this comparison table, not because
some game ran into it.

So this one goes into the record twice: **a comparison can tell you what you
are missing, but it cannot tell you whether you need it.** That is the
distortion built into "fill the gaps from somebody else's checklist" --
somebody else's checklist reflects somebody else's shape. If a third rules
package really does need to roll during play, adding it back is under fifty
lines, and the design is written up in `missions/SCARS.md` scar 7.

---

## Elimination: three sources disagree, and the disagreement has a pattern

|  | A first-class "eliminated"? |
|---|---|
| boardgame.io | **No.** The game records it in `G` itself and skips them in the rotation (inside `playOrderPos`'s `next` function) |
| OpenSpiel | **No.** `current_player()` is computed from state, so the dead simply never come up |
| PettingZoo AEC | **Yes.** `terminations`, one per agent, and the `agents` list shrinks |

The disagreement tracks **what the framework needs it for**: PettingZoo's API
asks each agent in a loop, so the framework has to know when to stop asking;
the other two do not, because "who may act" is computed from state anyway.

Adding `SetActors` put us in the second camp. So the question changed from
"do we need `Alive`" to "should it still be the decider" -- and the answer is
no; see [SCARS.md](https://github.com/Zereker/werewolf/blob/main/missions/SCARS.md)
scar 6. `Alive` stays as the **default**, and when the rules name actors, the
rules take responsibility.

## `Round`: why we need one and others do not

boardgame.io **has no concept of a round**, only turns and phases; neither do
OpenSpiel or PettingZoo. And more fundamentally: **none of them has any
automatically cleared scoped storage** -- all game state sits in one free-form
bag (`G`), and whatever needs clearing you clear yourself in `onBegin` /
`onEnd`.

That explains where the disagreement comes from, and the answer for us is
**yes**:

> They do not need a boundary because **they have no partitions**.

We deliberately do not offer a free-form bag: the rules can only produce
`Effect`s, and state is cut into four cells (whole game / this round x
unowned / owned), which is what makes replay and auditing work. **Once you
partition by lifetime, you must have at least one cell shorter than "the whole
game", and therefore something has to define its boundary.**

`Round` is that boundary marker. It is not a game concept we invented, it is
the boundary of a storage partition we deliberately created. The evidence is
direct: **they have nothing like a round-scoped variable at all.**

Usage says it is earning its place: 26 sites in werewolf, 5 in the
mission-based rules. And that historical bug -- the witch's antidote saving
the same person night after night -- came from getting the clearing wrong.

**Copying "no Round" would be a trap**: it looks like moving towards
precedent, and is actually throwing away the partitioning we paid for, ending
up with neither its benefits nor freedom from clearing things ourselves.

### But it used to have two things welded together

`EndsRound` did two jobs at once: increment the round number, and clear
round-scoped variables. In werewolf the two happen to coincide (a night marker
lives until the next night, and that is exactly one round), which is why
nothing looked wrong. In the mission-based rules they do not:

	team markers live until the next nomination begins   one mission may take five nominations
	the round number tracks which mission it is          or the number shown to players is meaningless

So the mission rules had to clear them by hand in the nomination resolver --
**the kernel was one lifetime short and the rules made up the difference**.
Same root cause as scar 3: the kernel welding two things together.

They are separate now: `EndsRound` governs counting only, and
`ClearsRoundVars` says "this phase of mine begins from a clean board". Each
phase declares only what is about itself, and `Validate` enforces both.

## One thing we deliberately do not copy

Their `endIf`, `minMoves` and `maxMoves` **end** a phase or stage
automatically.

We deliberately stay out of it: the engine keeps no clock,
`PhaseConfig.Timeout` is advice, when `EndPhase` is called is entirely the
caller's decision, and `PhaseReadiness` only answers "who is still missing".
This is written into "what the kernel does not do" in `doc.go`; it is a
deliberate boundary, not an omission.

So when the active-player set was added, **only "who may act" was taken, not
the counting and auto-ending that come with it**.

---

## Conclusion

The comparison says we do not have "an architecture that is wrong overall";
we are **ahead on one half and in debt on the other**:

- The "who knows what" half is **an order of magnitude stronger** than what it
  was compared against, and it is this library's core value;
- The "how a game proceeds" half owed two things -- the active-player set and
  randomness -- and both were "the kernel deciding for the rules" or "the
  kernel not offering it at all", not something done wrong. (Randomness was
  later added and removed: the design was fine, it had no users. See above.)

And the shape of the debt is clear, because the comparison hands over a
copyable answer: **first-class state + the rules set it with an effect + the
kernel enforces it at the entry point + fall back to a default when unset.**
We have solved two things with this shape already (`EndsRound`,
`GOTO_PHASE`); this was the third.
