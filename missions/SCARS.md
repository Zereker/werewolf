# The scars the missions package left on the kernel

This file records **every awkwardness that could not be sidestepped while
forcing the mission-based rules onto the kernel's existing API**.

The rule for writing it: **every entry needs runnable evidence**. The matching
tests in `scars_test.go` assert *what the current implementation gets wrong*,
not the desired behaviour -- once the kernel grows the missing capability they
go red, and that is the moment to rewrite them as positive assertions.

Not one line of the kernel was changed. That was deliberate: let the
workarounds leave scars first, then let the shape of the scars decide what to
add. Before that, any API proposal is a guess -- the kernel had been validated
by exactly one ruleset, and with a sample size of one the easiest mistake to
make is freezing your imagination of the second use case into a promise.

```
go test -run TestScar -v ./missions/
```

---

## Scar 1: actors can only be picked by role, never by a runtime list -- **closed**

**Symptom.** Only the players on the team may vote success or failure in the
mission phase, and the team is not settled until the nomination phase. The
kernel decides "who may submit this skill" from the triple
*(phase, role, skill)* (`stepFor` in the kernel's `phase.go`), which has no way
to say "these particular people".

**The workaround (now retired).** Open the mission phase to every role and have
`missionResolver` drop submissions from non-members. Same for the leader's
nomination.

**The fix.** The kernel grew `NewSetActorsEffect(phase, playerIDs...)`: the
rules say directly "these people act in that phase". All three questions
(skill validation, `AllowedSkills`, `PhaseReadiness`) now read from one place
-- `actorsForStep`, with three levels of priority:

	pending detours     the queue has to drain first
	the named list      NewSetActorsEffect
	PhaseStep.Role      the default: work it out from roles

**Later cut down to two.** The top level answered the same question naming
does, and the implementations were nearly word-for-word identical
(`triggerActorFor` and `namedActorsFor` both mean "of the people picked out,
who carries this role's step") -- one concept, two implementations, both to be
kept in step by hand. The detour queue no longer answers "who may act"; it
writes a list from the head of the queue **on entering the phase**, and
everything after that goes through naming:

	the named list      NewSetActorsEffect, or the one a detour wrote on entry
	PhaseStep.Role      the default: work it out from roles

The three things the queue still does have no substitute: steer the phase to
where the detour wants to go, hold off the victory check and the round boundary
until it has drained, and take entries one at a time from the head.

**The kernel blocks it in `SubmitSkillUse` itself**, rather than accepting the
submission and letting the rules discard it afterwards -- that part is lifted
from boardgame.io (`if (!isPlayerActive(...)) return` in `master.ts`), reasoning
in [PRIOR-ART.md](https://github.com/Zereker/hiddenrole/blob/master/PRIOR-ART.md).
This package's resolvers dropped two filters as a result, and **none are left**.

The list names a phase rather than just applying to the current one because it
is usually computed in **an earlier phase** (the mission team is chosen during
nomination). Once the phase resolves, that list is consumed.

**A second extension point came along with it: `GameSetup`.** There is no phase
before the first phase, so who names its actors? `WithGameSetup` runs once
inside `Start()`, and its effects go through the same single write path. It
pairs with `RoleSetup`: that one covers what a player brings when they sit
down, this one covers the state the whole game opens with. This package uses it
to name the first leader and to initialise the game-long counters explicitly.

That need only turned up halfway through the implementation -- it was not
foreseen in the design; writing it revealed that "the first nomination falls
back to role-based resolution, i.e. the whole table may nominate", which is
exactly what this scar exists to kill.

**The cost.** This was the most expensive of the four, because **the cost lands
directly on what players are shown**:

```
scar 1: c is not on the mission, yet AllowedSkills offers [MISSION_SUCCESS MISSION_FAIL]
scar 1: c's fail vote was accepted by the kernel; only the resolver can discard it
scar 1: PhaseReadiness thinks it is still waiting on [a b d e] (the team is only a and b)
```

`AllowedSkills` and `PlayerView` tell a player who is not on the mission "you
may vote", and `PhaseReadiness` waits for a crowd who cannot possibly act. And
"what can this player do right now" and "who are we still waiting on" are
exactly what this library sells.

**This abstraction has now been escaped twice.** The first time was the
werewolf hunter's shot -- the kernel opened `peekTrigger` as a special case for
it, pinning a phase to a single player; the second is here. An abstraction
routed around twice is usually not strong enough, rather than being misused.

---

## Scar 2: phase transitions are static, with no conditional branch -- **closed**

**Symptom.** These rules need "go to the mission if the vote passes, otherwise
back to nomination". `PhaseConfig.NextPhase` is a fixed value, and the only
dynamic jump is the detour queue.

**The workaround (now retired).** Have the vote phase flow unconditionally into
the mission phase, and have the mission phase do nothing when the vote failed.
The cost is that every rejected nomination spins the mission phase for nothing,
walking players into a phase where nothing can happen.

**The fix.** The kernel grew `NewGotoPhaseEffect(phase)`: the rules can name the
exit at resolution time. `PhaseConfig.NextPhase` is demoted from law to
default. The vote resolver now computes its own exit -- the mission on success,
back to nomination otherwise. See
`TestRejectedProposalGoesStraightBackToPropose` and
`TestApprovedProposalGoesToMission`.

Priority: **pending detours > GOTO > NextPhase**. Detours come first because
the queue has to drain -- the victory check and the round boundary are both
waiting on it, and jumping away midway would drop a debt that has not been
settled. That rule started out written only in the docs; mutation testing found
that "moving GOTO ahead of detours turns not a single test red", which is what
prompted adding `TestGotoPhase_TriggerQueueWins` to guard it.

**An observation on the side.** The queue is called "detour" in the docs, but
what it actually means is "**who, and to which phase**" -- this package uses it
to schedule the assassination phase, a perfect fit, and gets "hold the victory
check until it resolves" thrown in, which is precisely what is needed. A name
that does not match the real generality is itself a design smell.

---

## Scar 3: `Round` counts laps around the phase loop, not rounds in the game's sense -- **closed**

**Symptom.** The kernel welds "bump the round counter and clear the round
variables" onto "the phase ring wrapped back to the starting phase" (`nextPhase`
in the kernel's `state.go`). In werewolf the two happen to coincide (night ->
day -> night), so nothing looks wrong.

**The cost.** These rules go round the ring once per nomination:

```
scar 3: the engine says round 3, the rules say we are still on mission 1
scar 3: and PlayerView.Round hands that 3 to the player unchanged
```

One mission can take up to five nominations, so the two numbers can differ by a
factor of five. `PlayerView.Round` ships that meaningless number straight out
to players.

**Note that this one differs in kind from the previous two.** Those were "a
missing capability"; this one is "**the kernel invented a game concept when it
was really an implementation detail**". There is nothing wrong with the
lifetime of round variables; the problem is that it was welded to one lap of
the ring and then given a name with game semantics.

**The fix.** `PhaseConfig.EndsRound bool`: which phase's resolution counts as a
round is declared by the board itself, and the kernel stops guessing. This
package declares it on the mission phase, and `Round` becomes "which mission we
are on".

**The two scars are coupled, and that is itself a finding.** Adding `EndsRound`
alone closes this one only halfway: a rejected nomination still spins the
mission phase (scar 2), and the mission phase declares `EndsRound`, so that
empty lap advances the round anyway. **It only closes properly together with
scar 2's `GOTO_PHASE`.** That the two shared a root was reasoning before; now it
is measured -- fix one and the other stays half-broken.

**Handing over the decision bought back checkability.** While the kernel was
**guessing** the round boundary there was no way to check the guess; once the
rules **declare** it, `Config.Validate` can check -- "no phase declares
`EndsRound`" is now an illegal configuration, rejected when the game is built.
The consequence of that configuration is round state that never resets (in
werewolf, the witch's spent antidote would revive the same player night after
night), and previously it only showed up half a game in. That is a hard measure
of whether an abstraction got better, not a feeling.

---

## Scar 4: the 2x2 table of variable scopes was missing a cell -- **closed**

**Symptom.** The kernel had three scopes:

|                    | unowned      | owned by a player |
|--------------------|--------------|-------------------|
| **the whole game** | **(missing)**| `PlayerVar`       |
| **this round**     | `RoundVar`   | `PlayerRoundVar`  |

These rules have five things to remember that last the whole game and belong to
nobody: which mission we are on, how many succeeded, how many failed, how many
consecutive rejections, and whose turn it is to lead. The only unowned scope was
`RoundVar`, and that gets cleared between rounds.

**The workaround (now retired).** Hang all of it on the `PlayerVar` of whichever
player has the lexicographically smallest ID, as a ledger.

**The fix.** The kernel filled in the fourth cell -- written with
`NewSetVarEffect(ScopeGame, key, value)`, read with `GameView.Var(ScopeGame, key)`
/ `Engine.Var(ScopeGame, key)`, carried in the snapshot, replayable. (At the
time these were named `NewSetGameVarEffect` / `GameVar`; once all four cells
existed the four separate names collapsed into the single `VarScope` grid,
which is what the kernel exports today.) This package's ledger went away
entirely, and `gamestate.go` lost three functions.

That cell is not a back door opened for this ruleset; it completes the table --
the four cells are every combination of "time scale x has an owner", and
missing one was an oversight, not a deliberate blank. Werewolf never ran into
it because its game-long state all happens to hang off people.

**The cost.**

```
scar 4: player a's private state carries 4 fields that have nothing to do with them:
     [missions.success missions.mission missions.leader missions.rejects]
```

Global facts recorded in one person's private state; that player's `PlayerView`
sprouting fields unrelated to them; "who is the ledger" held together by
convention, so a third-party extension can overwrite it by accident.

---

## Scar 5: `SkillUse` assumes one action has one target -- **closed**

**Symptom.** `SkillUse.TargetID` was a single string. A nomination here names
2-5 people at once.

**The workaround.** The leader submits `PROPOSE` N times, and the resolver
dedupes by submission order and takes the first N.

**The cost.** Readiness cannot say how many nominations are still owed -- it
only knows whether the leader has submitted at all. A 7-player game needs 2 on
the first mission; if the leader names only 1, `PhaseReadiness` reports
`Ready=true`.

**This is the same class of problem as scar 1**: the kernel telling players
something untrue. I first rated it "the lightest of the lot" and meant to leave
it -- until the probe ran and printed `Ready=true`. Since scar 1 was fixed on
the standard "lying to players is the most expensive thing", this one has to be
fixed on the same standard, or the standard is fake.

**The fix.** `SkillUse.TargetID string` became `Targets []string`, with
`Target()` as the single-target reading. One submission carries the whole team,
and nomination and readiness become the same thing.

The change touched 212 call sites, every one of them pointed out by the
compiler. The cost really was not small, but what it buys is consistency of the
standard -- and a standard with an exception stops being a standard.

---

## Scar 6: `Alive` is a privileged concept, and this ruleset has no use for it -- **closed (but not by deleting it)**

**Symptom.** **Nobody is ever eliminated** in this ruleset. One of the kernel's
four state primitives is `SET_ALIVE`, `GameView` has `AlivePlayers()` /
`AlivePlayerIDsByRole()`, skill validation rejects dead players up front,
`PhaseStep` has a dedicated `AllowDeadTarget` field, and there is a whole detour
queue on top.

**The evidence.** Count the kernel entry points this package uses:

```
of the four state primitives, this package uses three:
  NewSetPlayerVarEffect       yes
  NewSetRoundVarEffect        yes
  NewSetPlayerRoundVarEffect  yes
  NewSetAliveEffect           no   not once
```

(Those three were later unified into `NewSetVarEffect` plus a `VarScope`; see
scar 4. The count is what it was at the time.)

**This is not a missing capability, it is a question.** If the kernel really is
a kernel for social deduction games, what makes the alive bit more entitled to
be hard-coded as a primitive than the witch's antidote? Both are rules
concepts. Demote it to an ordinary `PlayerVar` interpreted by the rules and the
kernel loses one primitive, one `AllowDeadTarget`, and a special case in skill
validation.

The cost is real too: elimination is a concept nearly every social deduction
game shares, a typed home for it genuinely is convenient, and the detour queue
(see the observation under scar 2) is a good thing.

**Went and looked at how others do it. They disagree, and the disagreement has
a pattern:**

| | first-class "eliminated"? |
|---|---|
| boardgame.io | **no**. The game tracks it in `G` and skips them in the turn order |
| OpenSpiel | **no**. `current_player()` is computed from state, so the dead never come up |
| PettingZoo AEC | **yes**. `terminations` has one entry per agent, and the `agents` list shrinks |

The split maps onto "what does the framework need it for": PettingZoo's API is
**a loop that asks each agent in turn**, so the framework has to know when to
stop asking; the other two do not need it, because "who may act" is computed
from state anyway.

**And we had become the latter an hour earlier** (`SetActors`). So the question
was never "should `Alive` exist", it is **should it still get the final word**.

**That found the real problem, and it is self-contradictory:**

	via the detour queue   the dead may act (the hunter shoots after being killed) -- the kernel's own mechanism, let through
	via the rules' naming  the dead are filtered out by the kernel -- the rules' mechanism, blocked

One kernel, permitting **its own** mechanism to step over `Alive` and refusing
the **rules'**. That is not "conceptually impure", that is the kernel deciding
on the rules' behalf whether the dead can act -- and that is the rules'
business.

What it blocks is play that really exists: **in Blood on the Clocktower the
dead keep a ghost vote**, and werewolf has a last-words phase.

**The fix: demote `Alive` from law to default.** The three decision points
(submission validation, `AllowedSkills`, `PhaseReadiness`) now share one
three-level scheme, lined up entry for entry with `actorsForStep`:

	pending detours   only the triggering player, even if eliminated
	the rules' list   whoever is named; whether they are alive is the rules' problem
	the default       the living

(Those three levels were later cut to two, see scar 1: the triggering player
now goes through the "named" level as well.)

Speech went the same way: `SendMessage` used to reject eliminated players
outright, leaving `SpeechProvider` no say; now, with a provider installed the
provider decides, and only without one does it fall back to "the dead do not
speak".

Not one field of `Alive` was deleted -- as a **default** it is right, and it is
a useful one. Deleting it would mean pushing "the eliminated cannot act by
default" out to be rewritten by every ruleset.

## Scar 7: the kernel has no randomness, and randomness is part of replayability -- **closed**

**This one was not hit by these rules; it only became visible against the prior
art**, and it is recorded here because it is the same kind as the others: the
kernel missing something only it can provide.

A `Resolver` has to be a pure function of the board -- that constraint is
correct in itself (it is the premise of replay). But the kernel had **no
randomness at all**: rolling dice meant the host rolling outside the engine,
with the result never entering the effect stream, so **that part could not be
replayed**.

boardgame.io's answer is to store the PRNG state in the game state (a `seed`
and a `prngstate` field), writing the new state back after every draw. Replay
can therefore reproduce exactly the same random sequence while moves stay pure.

Both werewolf and this ruleset dodge the problem -- the randomness in both
happens before the game is built (dealing), and none is needed during play. But
any ruleset with randomness mid-game (dice, drawing cards, random events)
either cannot be built on this kernel or, if it is, loses replayability, which
is one of this library's headline properties.

**This one proves that "write a second rules package" and "read what others
did" are two different tests, and neither can be skipped.** The second rules
package can only tell you what it ran into itself; the comparison is what tells
you about the thing **neither of you thought of**.

**The fix, and smaller than the prior art's.** `GameView.Rand()` returned a
random stream determined uniquely by **(seed, current round, current phase)**,
with the seed given in `Config.Seed` and carried in the snapshot.

The prior art stores the PRNG's internal state in the game state and writes it
back after each draw -- forced on it by its own shape: its moves are arbitrary
code and can draw any number of random values anywhere within a game, so
without remembering the position it cannot reproduce them.

We did not need that. The constraint here is stronger: **resolution is a pure
function of the board**, and the same board in must produce the same effects out
(there have long been tests holding that). So as long as the stream itself is
determined by the board, reproduction follows for free -- replay reaching the
same board rolls the same numbers. **No PRNG position needs storing**, and the
snapshot needs no extra mutable field for randomness.

The cost, stated plainly: resolving the same phase of the same round twice rolls
the same numbers. For this engine that is exactly what is wanted.

See [PRIOR-ART.md](https://github.com/Zereker/hiddenrole/blob/master/PRIOR-ART.md).

**And then it was taken back out.** `GameView.Rand` and `Config.Seed` have been
removed from the kernel. Not because the design was wrong -- the reasoning above
still holds -- but because **it had not one user**: the randomness in both
werewolf and these rules happens before the game is built, and neither package
rolls anything. It was filled in against somebody else's comparison table, not
hit by any actual game.

So this entry has to be recorded twice over: **a comparison can tell you what
you are missing, but not whether you need it.** If a third rules package really
does need to roll mid-game, add it back -- the design is written down here and
rewriting it is under fifty lines; until then, a public API with no users is a
liability, especially before a freeze.

## Where the kernel held up

Scars get recorded, and so does what held up, or this file leaves the false
impression of a kernel full of holes.

**The "who knows what" half had zero friction.** All three of this ruleset's
information asymmetries were written once and passed once:

- **Merlin** knows every bad guy (except Mordred), but that is not "same
  side" -- that knowledge goes through `RoleInfoProvider`, not
  `TeammateProvider`. Two extension points, each doing its own job.
- **Oberon** neither knows his fellows nor is known to them -- `TeammateProvider`
  explicitly supports asymmetry, with Blood on the Clocktower as the example in
  its docs, and this ruleset benefits directly.
- **Percival** sees Merlin and Morgana without telling them apart -- the
  implementation of "without telling them apart" is handing over both IDs
  sorted, with no distinguishing mark. The kernel does not need to know about
  this.

**The anonymity of fail votes needs no help from the kernel.** The table may
learn only "how many fail votes there were", never who cast them -- implemented
by **not producing an event per vote at all**, only one aggregate event carrying
the count. The resolver decides what to produce, the kernel ships it, and the
boundary holds by construction.

**The victory check does not assume winning means wiping somebody out.** Nobody
is ever eliminated here; `VictoryChecker` takes only a `GameView` and returns
only *(finished, winner)*, with not one place needing a workaround.

**The detour queue generalises better than its name.** Once the good side has
three successes the game must enter the assassination phase, and phase
transitions are static (scar 2). `NewDetourEffect(assassinID, PhaseAssassin)`
schedules a "who, to which phase" and the kernel takes the game there -- **and
holds the victory check until it resolves**, which is exactly what these rules
need (three won missions is not yet a win; the assassination comes first).
Measured:

```
after three successes: phase=ASSASSIN finished=false effects=[MISSION_SUCCEEDED ... DETOUR]
```

Not one line of kernel code changed. The docs call this mechanism "detour"; what
it can actually do is much larger than that name.

**The single write path, the effect stream and snapshot replay** were equally
usable as-is, and **survived scar 4's workaround**: hanging the game-long
progress off some player's `PlayerVar` was ugly, but the snapshot carried it and
the effect stream replayed it, and a restored engine plays on to the same result
as the original game (`TestSnapshotAndReplay`). The workaround was ugly, but it
broke none of the kernel's promises.

## What this version got working

All four outcome paths of a game work, in 996 lines of non-test code:

| Path | Test |
|---|---|
| good wins three, the assassin points at the wrong player -> good wins | `TestFullGame_GoodWinsThreeThenSurvivesAssassination` |
| good wins three, the assassin finds Merlin -> evil steals it | `TestFullGame_AssassinFindsMerlin` |
| evil sabotages three -> evil wins, no assassination | `TestFullGame_EvilWinsThreeMissions` |
| five nominations rejected in a row -> evil wins | `TestHammer_FiveRejectionsEndTheGame` |

This package uses 45 kernel entry points, **all of them exported**, and has zero
coupling to the werewolf package (`grep werewolf\.` hits two comments). "Rules
use only the public API" now has two independent rules packages attesting to it
at once.

---

## Conclusion: what the shape of the scars says

The seven scars fall into four classes. **All seven are closed**, and the
classification matters more than the count.

**Missing capability (scars 1, 4, 5, all closed)** -- something the kernel
should have had and did not. The fix is adding: pick actors by a runtime list,
fill in the fourth scope cell, let one action have several targets. The shape is
clear and the cost is calculable (the fourth cell moves the snapshot format, so
`SnapshotVersion` has to be bumped).

**Conceptual mismatch (scars 2, 3, closed)** -- the kernel **invented a game
concept where it really had an implementation detail**. `Round` was only "the
ring went round once" wearing a game's name; phase transitions were built as a
static graph, so every conditional branch had to go out through the "detour"
back door -- and that door's real meaning has nothing to do with death (it
generalises well, the name just lies).

The fix was not adding fields, it was **handing the decision back to the
rules**, and both places said it with **a new word in the existing language**: a
round boundary is a static fact, so it is a configuration field (`EndsRound`);
changing the exit is a runtime decision, so it is an effect (`GOTO_PHASE`). No
third language was invented -- needing to invent a new mechanism usually means
the thinking went wrong.

**Needless privilege (scar 6, closed)** -- not by deleting `Alive` but by
**demoting it to a default**. The problem was never that the concept should not
exist, it is that it was deciding on the rules' behalf. The test is the same
line as always: can the kernel decide "may the dead act" on its own, without
knowing what game this is -- it cannot, so it hands it over.

**Only visible against prior art (scar 7, closed)** -- the kernel had no
randomness. Both rulesets happened to dodge it, and it only shows up when held
against comparable projects. It says that testing a kernel needs two legs:
writing a second rules package, and reading other people's implementations.

In one sentence: **we cleaned werewolf's vocabulary out of the kernel, and
proved it; we never cleaned out werewolf's shape.** `grep -r WEREWOLF` over the
kernel comes back empty, and all that proves is that the kernel does not
recognise the **word** "witch"; whether it recognised the **structure** of
"night follows day, a few players die each round" was unverified by anyone until
this package was written.

The answer is: **half of it did, half of it did not.** The "who knows what" half
is genuinely general; the "how does a game make progress" half was still
werewolf-shaped.
