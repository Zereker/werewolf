# The scars One Night left on the kernel

> This is a product of phase 1 of the
> [implementation plan](../../docs/ROADMAP.md). The protocol is strict: **while
> writing a rules package, not one line of the kernel may change**. Whatever
> it runs into gets recorded here, with all four columns -- what it hit, the
> way around it, the guessed fix, and the classification.
>
> The "way around it" column is the important one: **if you can work around
> it, it is an ergonomics problem, not an abstraction problem.** Two of the
> missions package's seven scars were ultimately judged "the workaround is
> acceptable", and the kernel was not touched.

**The conclusion up front**: the kernel's `git diff` is empty, 1176 lines of
non-test code, and all four outcome paths work. **Five scars, one of which is
"we got it wrong"** -- and that one matters more than the other four.

| # | What it hit | Classification | Anticipated? |
|---|---|---|---|
| 1 | a target must be a player, so "look at a card" is inexpressible | missing capability | ✅ [DESIGN §8.2](https://github.com/Zereker/hiddenrole/blob/master/DESIGN.md) called it |
| 2 | `Validate` demands a round boundary, and this ruleset has no rounds | needless enforcement | ❌ not foreseen |
| 3 | "wake up and look" has no counterpart | conceptual gap | ❌ not foreseen |
| 4 | `VarCamp` projects automatically, and here the camp is a secret | needless privilege | ❌ not foreseen |
| 5 | victory can only have one winner | missing capability | ✅ [DESIGN §8.2](https://github.com/Zereker/hiddenrole/blob/master/DESIGN.md) called it |
| 6 | the snapshot does not carry the winner | **bug** | ❌ turned up while investigating scar 5 |
| — | **"identity is fixed at seating" blocks card-swapping games** | **wrong call** | ❌ backwards |

**Disposition**: scars 2, 3 and 6 are fixed, and **the three together changed
zero exported names**. Scars 1, 4 and 5 were judged not-for-now, each with its
trigger condition written down.

Scar 3 really was just the mirror of `RoleSystem` -- the fix was giving
`PhaseStep.Skill`'s zero value a meaning, with no new name added at all.

> The test is the one at the top of this file: **if you can work around it, it
> is an ergonomics problem, not an abstraction problem.** In the previous round
> I recommended doing all three; that was without holding this ruler up to
> them. Held up, only one was left.

---

## Scar 0: we judged "identity is fixed at seating" a gap, and we were wrong

[DESIGN.md §8.1](https://github.com/Zereker/hiddenrole/blob/master/DESIGN.md)
listed "`playerState.Role` has no write path" among the **abstraction gaps
with evidence**, guessed it would block card-swapping games, and named One
Night specifically. Half the reason for picking this ruleset as the third one
was to test that claim.

**The test came back backwards: not only does it not block them, actually
adding a writable RoleType would break the game.**

Identity in One Night **already has two layers**:

```
the card you were dealt    decides what you do at night     never changes
the card in your hand now  decides which side you score for gets swapped around
```

A robber who steals the werewolf card does **not** become a wolf and does not
wake with them -- what they do at night is decided by the card they were
dealt; but when the game is scored they count as the wolf team. That is not an
implementation detail, it is the game's pivot.

The kernel provides one layer (`RoleType`, fixed at seating) and the rules
provide the other (a piece of game-long state owned by one player), which is
**exactly enough**. Had the kernel made `RoleType` writable, a rules package
would reach for it to mean "the card in hand now", the two layers would
collapse into one, the robber would immediately wake with the wolves, and the
game would fall apart on the spot.

> **Immutability is the value here.** "Identity is fixed" is not a defect; it
> carries exactly the layer of "what you were dealt" -- and that layer was
> never supposed to change.

### What this scar actually teaches

It confirms the second half of the methodology in
[the implementation plan §0](../../docs/ROADMAP.md):

| The test | What it can tell you |
|---|---|
| comparing against someone else's implementation | what you are **missing** |
| writing a third rules package | whether you **need it**, and **that the gap you thought you had should not be filled** |

`Rand` was "we added something we did not need"; this one was "we nearly broke
something that was right". The two lessons point different ways, and have the
same root: **changing the kernel before a real game has run into it**.

**Disposition**: that line in `DESIGN.md §8.1` needs rewriting -- not a gap,
but deliberate, and just validated.

---

## Scar 1: a skill's target must be a player, so "look at a card" is inexpressible -- **not for now**

**What it hit.** The seer may "look at two centre cards", the drunk "swaps
with a centre card", and a lone wolf "looks at one centre card". These actions
point at **cards**, not people.

And the kernel's target validation only knows players: every entry of
`SkillUse.Targets` is passed to `getPlayer`, and anything that does not match
is `ErrTargetNotFound`. Submitting `Targets: ["center-0"]` is rejected on the
spot.

**The way around it.** Encode the index into the skill name:

```go
SkillPeekCenter0  SkillPeekCenter1  SkillPeekCenter2      // a lone wolf looks at one
SkillSeerCenter01 SkillSeerCenter02 SkillSeerCenter12     // the seer looks at two
SkillDrinkCenter0 SkillDrinkCenter1 SkillDrinkCenter2     // the drunk swaps one
```

Nine skills doing what is really three things, with the index read back off
the end of the name in `centerIndexes()`.

**The workaround works, but it has a cost**, and the cost grows
**combinatorially** with the number of cards: two of three cards is three
combinations; with more public cards, as in the Daybreak expansion, or a role
that looks at any two, this column would grow beyond writing.

**The guessed fix.** Let a step declare what kind of target it takes:

```go
type PhaseStep struct {
    // ...
    TargetKind TargetKind  // TargetPlayer by default; TargetFree skips the player check
}
```

The kernel gains one branch: skip `getPlayer` when `TargetFree`. What the
target string means is the rules' to interpret, exactly like a `Var` key --
the kernel only carries it.

**Classification: missing capability.**
[DESIGN §8.2](https://github.com/Zereker/hiddenrole/blob/master/DESIGN.md)
listed this in advance ("a target can only be a player ID"), marked
"speculative, no game has run into it". **Now one has.**

### But judged not-for-now

Hold it up against the test at the top of this file: **if you can work around
it, it is an ergonomics problem.**

An earlier draft of this scar said something stronger: "`AllowedSkills` lists
four skills for the seer, when they really have two choices". **That sentence
was wrong**: two out of three centre cards really is three combinations, and
with "look at one player" that makes four. The encoding is ugly, but it
**tells no lie**.

As for the combinatorial explosion, that is a hypothetical fourth ruleset (the
Daybreak expansion, a role that looks at any two). In this project's own
words: **a generalisation from a sample of two is not a generalisation, it is
a guess** -- and here there is not even a two. The actual cost is 9 constants
instead of 3, plus 15 lines of parsing.

**Trigger**: whenever the encoding really does explode in some rules package,
or really does start lying (the options `AllowedSkills` lists stop matching the
options a player actually has), change it.

---

## Scar 2: `Validate` demands a round boundary, and this ruleset has no rounds at all -- **fixed**

**What it hit.** `Config.Validate()` required **at least one phase marked
`EndsRound` and at least one marked `ClearsRoundVars`**. Those two checks have
a history: they guard against "the round stays at 1 forever and round-scoped
variables are never cleared", which was a real bug.

But this ruleset has **exactly one round in the whole game**. One night, one
discussion, one vote, ending at VOTE. `Round` is 1 from start to finish, and
**that is exactly right**.

So the kernel, guarding against one class of misconfiguration, was forcing a
correct configuration to lie.

**The way around it.** Hang both markers on VOTE, even though no round follows
it:

```go
PhaseVote: {
    Type: PhaseVote, NextPhase: hiddenrole.PhaseEnd,
    EndsRound:       true,  // no round follows; marked only to satisfy Validate
    ClearsRoundVars: true,  // likewise
},
```

It runs without any problem -- `EndsRound` does not take effect when the game
ends anyway (the kernel has the `!endNow` guard). But **the configuration is
lying**: anyone reading it would assume this ruleset has a round cycle.

**The guessed fix.** What those two checks really mean is "do not let round
state go uncleared forever". And **a game with no round cycle has no such
risk** -- whether the phase graph has a cycle is something the kernel can see
for itself (walk `NextPhase` from `StartPhase` and ask whether it returns to a
phase already visited). Change the check to: **require a round boundary only
when the phase graph loops.**

Or, more simply: add an explicit `SingleRound bool` to `Config` and let the
rules say "I have no rounds". The former is something the kernel can judge on
its own, which fits the first question of
[the test](https://github.com/Zereker/hiddenrole/blob/master/DESIGN.md) better.

**Classification: needless enforcement.** The same class as the missions
package's scar 6 (`Alive`'s privilege) -- the kernel making a judgement on the
rules' behalf that it is not equipped to make. **Not foreseen.**

**Fixed, and the only one of the three candidates with zero API change.**
`Config.Validate()` now requires a round boundary only of a phase graph that
**loops** (`Config.loops()`: walk `NextPhase` from `StartPhase`, and failing
to reach `PhaseEnd` within `len(Phases)` steps means it loops). Not one byte
of the exported surface moved, and the validation only got looser -- anything
that passed before still passes.

This package therefore dropped the two fake markers: its phase graph now
**declares no round boundary at all**, because it genuinely needs none.

Why the judgement belongs to the kernel rather than the rules: "does the phase
graph loop" is something the kernel can compute **without knowing what game
this is**, which is exactly the first question of
[the test](https://github.com/Zereker/hiddenrole/blob/master/DESIGN.md).

---

## Scar 3: "wake up and look" has no counterpart in the kernel -- **fixed**

**What it hit.** In the minion, mason and insomniac steps, the player **only
receives information and takes no action**. The minion opens their eyes, sees
who the wolves are, and closes them -- no submission, no target, no state
change.

The kernel describes a phase with `Steps []PhaseStep`, and a `PhaseStep` is
`{Role, Skill, ...}` -- **one action**. "This role wakes, but takes no action"
is inexpressible.

A phase with no steps at all (like the day) does not work either:
`PhaseInfo.ActiveRoles` would be empty and the host would not know who to wake.

**The way around it.** Hang a `SkillSkip` step on it as a placeholder:

```go
PhaseNightMinion: {
    Type:  PhaseNightMinion,
    Steps: step(RoleMinion, hiddenrole.SkillSkip),  // only so the host knows who to call
    ...
},
```

It works, but the meaning is wrong -- `SKIP` means "declining to act", and the
minion is not declining, they **never had an action to decline**.
`AllowedSkills` therefore tells them "you may SKIP" when the correct answer is
"you need do nothing; open your eyes and look".

**The guessed fix.** Add a marker to `PhaseStep` for "this step receives
information, it is not an action", or make a step with an empty `Skill` legal
(the zero value already means "unspecified"). Readiness skips it,
`AllowedSkills` does not list it, but `ActiveRoles` includes it.

**This one may not need a kernel change at all.** There is a smaller reading:
the kernel already has `RoleSystem` + `SkillAnnounce` ("no player carries this
step"), and what is wanted here is its mirror -- "this step has a player, who
does not act". Together the two make a complete pair.

**Classification: conceptual gap.** Not foreseen, and the cheapest of the
five.

### Fixed, zero exported names changed

The fix is exactly the smaller reading guessed above: **an empty
`PhaseStep.Skill` is legal**. The zero value already meant "unspecified", and
now it has a meaning -- "this role wakes, but takes no action".

It mirrors `RoleSystem`: that one is "this step has no player", this one is
"this step has a player, who does not act". Together they complete the four
combinations of what a step in a phase can be.

An empty step:

| | |
|---|---|
| does not appear in `AllowedSkills` | there is nothing they can submit |
| cannot be submitted | `SkillUnspecified` is explicitly blocked by `stepFor`, or it would match the empty step exactly |
| does not enter readiness | there is nothing to satisfy, or the phase would never be ready |
| **appears in `PhaseInfo.ActiveRoles`** | the host has to know who to wake -- **the whole point** |

This package therefore replaced its three placeholder `SkillSkip`s with empty
skills (`watch()` in `board.go`).

Each of the four properties has a mutation verified to turn it red, the last
one especially: skipping `ActiveRoles` would make the feature pointless while
the other three stayed green.

---

## Scar 4: `VarCamp` projects automatically, and in this ruleset the camp is a secret

**What it hit.** The kernel recognises a canonical key `VarCamp` and carries
its value into `PlayerInfo.Camp` and `SelfInfo.Camp`, so that "which side is
this player on" need not be dug out of `Vars` by every caller. Both earlier
rules packages use it, and it serves them well.

This one **cannot use it**. The drunk swaps their own card with a centre card
**without looking**; the two players the troublemaker swapped do not know
either. Which side they now count for is **a secret from themselves** -- which
is the whole of what those roles are. And `SelfInfo.Camp` is a field in the
player's own view, so writing it simply tells them.

**The way around it.** Never write `VarCamp`, and compute the camp in the host
at the moment cards are revealed (this package exports `CampOf(role)`). The
cost is that `SelfInfo.Camp` is empty all game -- a convenience the kernel
offers that this ruleset never once uses.

**I consider this workaround acceptable**, and what it exposes is smaller than
it looks: whether to write `VarCamp` is **the rules' own decision**, and not
writing it means nothing is projected. The kernel forces nobody to write it.

**But there is one real problem**: `VarCamp` is a **one-way automatic
projection** -- once the rules write it, there is no way to let only the god's
view see it while keeping it from the player. "Who knows their own camp" is a
real design dimension in this class of game (werewolf: everyone; One Night:
nobody is sure after the swaps; Blood on the Clocktower: the poisoned get
false information).

**The guessed fix.** None. This is recorded and waits for a fourth rules
package to hit it again -- **one ruleset not being able to use a convenience
is not a reason to change it**. If it were to change, the direction would be
turning "fill it into SelfInfo" from the kernel's automatic behaviour into the
rules' explicit choice, but that would cost both earlier packages an extra
line for no gain.

**Classification: needless privilege, but judged an acceptable workaround.**
Handled the same way as the missions package's two such scars.

---

## Scar 5: victory can only have one winner -- **not for now, wait for a second collision**

**What it hit.** `VictoryChecker`'s signature is:

```go
CheckVictory(view GameView) (over bool, winner Camp)
```

**One** `Camp`. And this ruleset can have **two winners** -- the official
rules verbatim:

> The tanner wins only by being eliminated themselves. Eliminated with no wolf
> eliminated -> the wolves do not win; **eliminated with a wolf also
> eliminated -> the village wins too.**

The tanner and the village winning together is a common outcome of this game.

**The way around it.** `Camp` is a string underneath and the kernel does not
interpret values, so packing several into one works:

```go
"TANNER+VILLAGE"
```

Joined in lexicographic order, so the result is deterministic, with this
package exporting `Winners(camp) []Camp` and `Won(camp, want) bool` to take it
apart again.

**The workaround works, but it is a string encoding, not a type.** The
encoding and decoding rules can only be carried by the rules package -- the
kernel does not know `+` is a separator, `Engine.Status().Winner` hands the
caller a compound string they cannot read, and `AudienceOf` has no way to
speak on it.

**The guessed fix.** Make `Camp` a set:

```go
CheckVictory(view GameView) (over bool, winners []Camp)
```

The kernel still does not interpret values; "one" simply becomes "a set".
`Status.Winner` follows into `Winners []Camp`, and a single winner is a slice
of length 1. The change is small, and it touches one place in each of the
three rules packages.

**Classification: missing capability.**
[DESIGN §8.2](https://github.com/Zereker/hiddenrole/blob/master/DESIGN.md)
listed this in advance ("victory has a single Camp"), marked "speculative",
with the reason given as "Blood on the Clocktower's travellers score
separately". **Now a real ruleset has hit it**, and as a routine outcome of the
base game, not an expansion corner case.

### But judged not-for-now, waiting for a second collision

The evidence here is the **strongest** of the three: it was not only hit, the
workaround **genuinely lies** -- `Camp` is documented as "the label of one
side", and `"TANNER+VILLAGE"` is not one side, it is two. Scar 1's encoding is
ugly without being false; this one is false.

There is only one reason not to fix it, and it is hard enough: **it is the
only breaking signature change among the three candidates** (the
`VictoryChecker` interface, `VictoryFunc` and the `Status.Winner` field all
move together), and afterwards the 99% of games with exactly one winner would
**permanently** pay for a slice, and every host would write `[0]` one more
time. **One ruleset hitting it is not enough to move a signature that was just
frozen** -- consistent with scar 4 ("one ruleset not being able to use a
convenience is not a reason to change it").

**Trigger**: a second rules package hitting the same thing. Blood on the
Clocktower's travellers scoring separately is most likely the second. At that
point change it to `winners []Camp` together; the change is small and touches
one place per rules package.

---

## Scar 6: the snapshot does not carry the winner -- **fixed**

**Not something this ruleset hit; it turned up while deciding whether to fix
scar 5.**

**What it hit.** Save a game that has **already ended** and restore it:

```
original: Over=true  Winner="VILLAGE"
restored: Over=true  Winner="UNSPECIFIED"
```

`Snapshot` does not carry the winner. And who won is settled by the
`VictoryChecker` **at the moment the game ends** and does not change
afterwards, and a restored engine does not run the check again -- so the answer
is simply lost.

The previous round had just merged `Phase` / `Round` / `Over` / `Winner` into
`Status`, on the grounds that "they have to come from one instant", and wrote
`TestStatus_IsAtomic` for it. But that test only asserted **one** direction,
"not over yet has a winner"; the reverse, **"over with no winner"**, was
nobody's job -- and on the restore path it had been wrong all along.

**What this scar teaches**: when an invariant has two directions, testing one
guards half of it.

**Fixed.** `Snapshot` gained a `Winner` field (`SnapshotVersion` 12 -> 13),
written back on restore. `TestStatus_IsAtomic` gained the reverse assertion,
and `TestStatus_SurvivesSnapshot` was added to watch the save round trip
specifically. Two mutations (not writing it to the snapshot, not reading it on
restore) were each verified to turn them red.

---

## Where the kernel held

Scars get recorded, and so does what held. This ruleset is **the opposite of
the first two in every respect**, and the following cost no friction at all:

**The variable-scope 2x2 table: three cells out of four used.** The card in
each hand is "whole game, one player", the three centre cards are "whole game,
unowned", and who saw what is "whole game, one player". **The "whole game,
unowned" cell is the one the missions package ran into and had added** -- less
than one rules package later, the third one used it again. The two
round-scoped cells went unused, and that is precisely the evidence that a
scope should be two axes rather than a list.

**The information boundary never leaked once.** The asymmetry here is denser
than in either earlier package: wolves recognise each other, the minion sees
the wolves one-way, the masons recognise each other, what the seer saw goes
stale, the robber knows what they took while the player robbed does not, the
troublemaker swaps two people with all three left in the dark, and the drunk
does not even know their own card. **All of it worked first time.**

This one especially: the kernel **deliberately does not hand `Vars` to
players** (what a player should see has to be projected explicitly through a
`RoleInfoProvider`). Had the kernel handed `Vars` over by default, **the drunk
as a role would not work at all** -- the card in their hand is a `Vars` entry.
That rule had only just gained a test the previous round
(`TestPlayerView_CarriesNoFreeFormState`), and this round it saved a role.

**The non-configurable floor, "state primitives never leave the building",
held too.** "Player 3 now holds the werewolf card" is a `SET_VAR`. This
package's `AudienceProvider` blocks it whether or not it says anything about
it.

**The "two effects, two things" split held as well.** `ROBBED` is the account
of what happened, and the `SET_VAR` beside it is what actually swaps the
cards. A lone `ROBBED` moves nobody's card.

**`SetActors` was never used.** Every actor in this ruleset is computed by
role -- and a role is the card you were dealt, which never changes. The
previous round spent a refactor bringing "who may act" from three layers down
to two, and this ruleset only ever touched the bottom layer, without any
awkwardness.

---

## Against the first two

The kernel recognises not one word below.

| | Werewolf | The missions package | One Night |
|---|---|---|---|
| phase graph | 8, a cycle | 3 in a loop + 1 entered conditionally | **10, a straight line** |
| rounds | one per night | one per mission | **one for the whole game** |
| who may act | by role | computed at runtime (`SetActors`) | by role |
| elimination | the core mechanic | never used | **once, at the very last moment** |
| identity | one layer, fixed | one layer, fixed | **two layers, one fixed and one not** |
| how long information stays true | a check result holds forever | the list holds all game | **what you saw goes stale, and you do not know it** |
| victory | wipe out one side | three missions + the assassination | **there can be two winners** |
| randomness | dealt before the game | dealt before the game | dealt before the game |

That last row is worth calling out on its own: **three rules packages, and not
one of them needs randomness during play.** The previous round's decision to
delete `Rand` was confirmed once more by the third.
