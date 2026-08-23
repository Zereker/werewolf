# hiddenrole

**English** · [中文](README.zh-CN.md)

A **kernel for social deduction games**, pure Go, **zero dependencies**. It
does not know what werewolf is.

[![Go Reference](https://pkg.go.dev/badge/github.com/Zereker/hiddenrole.svg)](https://pkg.go.dev/github.com/Zereker/hiddenrole)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

```
go get github.com/Zereker/hiddenrole
```

What it does know: there are players, there is a cycle of phases, at the end
of each phase it asks that phase's resolver what happened, and it folds the
answer into the state. Plus the hardest part of these games -- **who is
allowed to know what**.

Roles, skills, ways to die, victory, the information boundary: all of it is
installed by a rules package through public options.

## "It really does not know" is checkable

In this package's non-test source there are exactly two values of `RoleType`
(`RoleUnspecified`, `RoleSystem`), three of `PhaseType` and three of
`SkillType`, all of them in [`types.go`](types.go). Not one "witch",
"werewolf" or "NIGHT_WITCH" anywhere.

The harder evidence is **three unrelated rules packages** running on it, no
two of which share a single value:

| Rules package | What it plays | What it proves |
|---|---|---|
| [werewolf](https://github.com/Zereker/werewolf) | werewolf | elimination is the core mechanic, eight phases in a cycle |
| [werewolf/missions](https://github.com/Zereker/werewolf/tree/main/missions) | mission-based play (nominate / vote / mission / assassinate) | it runs with **nobody ever eliminated**; transitions are decided by resolution results |
| [werewolf/onenight](https://github.com/Zereker/werewolf/tree/main/onenight) | one-night card swapping | identity has **two layers**: the card dealt decides what you do at night, the card in hand decides which side you score for |

Writing the third one forced **zero breaking API changes** -- [the API is
frozen](API.md), guarded by `TestAPI_SurfaceIsPinned` and
[`testdata/api.golden`](testdata/api.golden): change a name or a signature and
the test goes red.

## Where to start reading

| To find out | Read |
|---|---|
| **which APIs exist and what each promises** | [API.md](API.md) 🔒 frozen |
| **what it should look like, and why so abstract** | [DESIGN.md](DESIGN.md) |
| how the code is organised today | [ARCHITECTURE.md](ARCHITECTURE.md) |
| how others did it, where we are ahead and where we are behind | [PRIOR-ART.md](PRIOR-ART.md) |
| what writing a rules package ran into | [missions](https://github.com/Zereker/werewolf/blob/main/missions/SCARS.md) · [onenight](https://github.com/Zereker/werewolf/blob/main/onenight/SCARS.md) |

When writing your own rules package, [`enginetest`](enginetest/) gives you
random games and seven general invariants (`RunFuzz`). Not one of them knows
any game; they check things at the kernel's level: does what was stored read
back the same, does replay arrive at the same board, is somebody the engine
says cannot act really unable to act.

## A state machine that knows nothing

The engine `NewEngine` builds can advance phases, but will never decide a
winner, recognises no role, and draws no information boundary. Below is a
complete ruleset that fits on two pages: red team and blue team, one public
vote per round, most votes is eliminated, one side wiped out ends it.

```go
const (
	phaseVote = hiddenrole.PhaseType("VOTE")
	roleRed   = hiddenrole.RoleType("RED")
	roleBlue  = hiddenrole.RoleType("BLUE")
	skillVote = hiddenrole.SkillType("VOTE")
	eventOut  = hiddenrole.EventType("OUT")
	campRed   = hiddenrole.Camp("RED")
	campBlue  = hiddenrole.Camp("BLUE")
)

// What happens when this phase ends. Reads GameView only, returns Effects only.
type vote struct{}

func (vote) Resolve(uses []*hiddenrole.SkillUse, _ hiddenrole.GameView) []*hiddenrole.Effect {
	tally := map[string]int{}
	for _, u := range uses {
		if u.Skill == skillVote {
			tally[u.Target()]++
		}
	}
	out, best := "", 0
	for id, n := range tally {
		if n > best || (n == best && id < out) { // the order must be decided by the board alone
			out, best = id, n
		}
	}
	if out == "" {
		return nil
	}
	return []*hiddenrole.Effect{
		hiddenrole.NewEffect(eventOut, "", out),  // the rules' name for what happened
		hiddenrole.NewSetAliveEffect(out, false), // the one that actually changes state
	}
}

// One side wiped out ends it.
type lastSideStanding struct{}

func (lastSideStanding) CheckVictory(view hiddenrole.GameView) (bool, hiddenrole.Camp) {
	red, blue := 0, 0
	for _, p := range view.AlivePlayers() {
		if p.Role == roleRed {
			red++
		} else {
			blue++
		}
	}
	switch {
	case blue == 0:
		return true, campRed
	case red == 0:
		return true, campBlue
	}
	return false, hiddenrole.CampUnspecified
}

func main() {
	cfg := &hiddenrole.Config{
		StartPhase: phaseVote,
		Phases: map[hiddenrole.PhaseType]*hiddenrole.PhaseConfig{
			phaseVote: {
				Type: phaseVote,
				Steps: []hiddenrole.PhaseStep{
					{Role: roleRed, Skill: skillVote, Required: true, Multiple: true},
					{Role: roleBlue, Skill: skillVote, Required: true, Multiple: true},
				},
				NextPhase:       phaseVote, // a cycle: back to itself
				EndsRound:       true,      // this phase ending is one round
				ClearsRoundVars: true,      // and it begins from a clean board
			},
		},
	}

	e := hiddenrole.MustNewEngine(cfg,
		hiddenrole.WithResolver(phaseVote, vote{}),
		hiddenrole.WithVictoryChecker(lastSideStanding{}))

	_ = e.AddPlayer("r1", roleRed)
	_ = e.AddPlayer("r2", roleRed)
	_ = e.AddPlayer("b1", roleBlue)
	_ = e.Start()

	for _, id := range []string{"r1", "r2", "b1"} {
		_ = e.SubmitSkillUse(&hiddenrole.SkillUse{PlayerID: id, Skill: skillVote, Targets: []string{"b1"}})
	}
	effects, _ := e.EndPhase()
	for _, ef := range effects {
		fmt.Println(ef.Type, ef.TargetID) // OUT b1 / SET_ALIVE b1 / GAME_ENDED
	}
	st := e.Status()
	fmt.Println("over:", st.Over, "winner:", st.Winner) // true RED
}
```

Leave out `WithVictoryChecker` and the game never ends; leave out
`WithResolver` and `Start()` returns an error. **The kernel knows nothing** is
something you can verify this way, not a slogan.

## The single write point

```
SubmitSkillUse  ->  Resolver.Resolve  ->  []*Effect  ->  applyEffect
collect skills      judge (pure func)   describe changes   the only writer
```

A `Resolver` is handed a read-only `GameView` and can express a state change
only by returning an `Effect`. The constraint is held up by the **signature**
rather than by convention: every change to state goes through one write point,
which is what makes snapshots, replay and auditing possible at all.

`Resolve` and `CheckVictory` are both called while the engine holds its lock,
so an implementation must not call back into any `Engine` method. The order of
the effects returned must be decided by the board alone (which is what the
`id < out` above is for), or replay and snapshot comparison lose their
determinism.

## The two primitives the state machine recognises

| Constructor | Changes | Read back with |
|---|---|---|
| `NewSetAliveEffect(id, alive)` | aliveness | `GameView.Player(id).Alive` |
| `NewSetVarEffect(scope, k, v)` | one piece of custom state | `GameView.Var(scope, k)` |

A scope is a 2x2 table -- lifetime crossed with ownership -- and the four
cells fall out of two values crossed with one method (see `VarScope`):

| | unowned | owned by a player |
|---|---|---|
| **whole game** | `ScopeGame` | `ScopeGame.Of(id)` |
| **this round** | `ScopeRound` | `ScopeRound.Of(id)` |

The table used to exist only in a comment, and the code had eight flat names
(four constructors and four readers) -- so nothing forced it to be complete,
and "whole game, unowned" was missing for a long time before the mission-based
rules ran into it. A missing cell is now not expressible.

There is one more, `NewDetourEffect(id, phase)`, which files a debt (the
hunter's shot after being killed is exactly this).

Variable values are strings, and **an empty string is equivalent to deletion
at the write point**, so a has-it/hasn't-it state needs nothing more than one
non-empty value (`VarPresent` by convention).

"Wolf kill", "exile" and "shoot" are the rules' names for what happened, and
the state machine does not recognise them -- a `KILL` effect on its own
**kills nobody**. For the rules to eliminate someone, they emit a `SET_ALIVE`
alongside it. Two effects, two things: the first for the audience and the
effect log, the second for the state machine. `OUT` and `SET_ALIVE` appearing
as a pair in the example above is exactly this.

## Who is allowed to know what

```go
e.PlayerView(id)      // everything one player is entitled to know, sendable as-is
e.AudienceOf(event)   // which players should be told about something
```

The rules draw the lines: `AudienceProvider` (who should be told about
something), `TeammateProvider` (who is on whose side, asymmetry allowed) and
`SpeechProvider` (who hears a player speak).

At this layer the kernel holds one line, and it is **not configurable**: its
own state primitives never leave the building. They are the state machine's
bookkeeping, and pushing them to a player is handing out the god's view.

The player-facing `PlayerView` / `AudienceOf` and the god's-view `PhaseInfo` /
`PlayerInfo` are two different sets of readers; do not mix them up. The first
can be sent to a player, the second cannot.

## Eight extension points

| To add | Use |
|---|---|
| how a phase resolves | `WithResolver(phase, resolver)` |
| what a role sits down with | `WithRoleSetup(role, setup)`, written into that player's `Vars` |
| how winning works | `WithVictoryChecker(checker)` |
| role-specific information | `WithRoleInfo(role, provider)`, appears in `PlayerView.RoleInfo` |
| who should be told about something | `WithAudience(provider)` |
| who is on whose side | `WithTeammates(provider)` |
| who hears a player speak | `WithSpeech(provider)` |
| logging | `WithLogger(l)` |

Plus two that are not options: a state change during play goes through an
`Effect` primitive, and a host-level state change goes through `Engine.Apply`
(the same single write point, but bypassing phase resolution -- a sharp
knife).

**All eight can be installed with a plain function**: `ResolverFunc` /
`VictoryFunc` / `RoleSetupFunc` / `GameSetupFunc` / `RoleInfoFunc` /
`AudienceFunc` / `TeammateFunc` / `SpeechFunc`. The first two were added
later -- they were the only two without an adapter, for no reason but
history, which meant installing a three-line resolver first required
declaring an empty struct.

All of them can only be given at construction: once the engine is in the
caller's hands, they no longer change. All four entry points accept them --
`NewEngine`, `MustNewEngine`, `RestoreEngine` and `ReplayEngine`.

## Two decisions the kernel refuses to make for the rules

Two decisions about how a game proceeds have answers **only the rules know**:

| Decision | Who decides |
|---|---|
| which phase comes next | `PhaseConfig.NextPhase` is the default exit; the rules can override it during resolution with `NewGotoPhaseEffect` |
| whether a new round begins after this step | declared by `PhaseConfig.EndsRound` |

The kernel used to decide both: the exit came from a static graph, and the
round boundary was guessed as "looping back to the start phase counts". In
werewolf both guesses happen to hold (night -> day -> night); in another
ruleset they do not.

The test is one sentence: **can the kernel judge this correctly without
knowing what game it is?** "Did the state change" it can judge, so that
belongs to the kernel; "is this a new round" it cannot, so that belongs to the
rules.

```go
// Go to the mission if the vote passed, back to nomination otherwise -- the
// outcome is computed by this phase's resolution, and a static graph cannot
// express it.
if approved {
	effects = append(effects, hiddenrole.NewGotoPhaseEffect(phaseMission))
} else {
	effects = append(effects, hiddenrole.NewGotoPhaseEffect(phasePropose))
}
```

Exit priority: **a pending detour queue > `GOTO_PHASE` > `NextPhase`**.
Detours come first because the queue has to drain -- the victory check and the
round boundary are both waiting on it, and jumping away mid-queue would drop a
death ability that was never settled. A destination absent from the
configuration is logged as an error and falls back to the default exit.

## Who may act in this phase

Two layers, highest priority first:

| | Who |
|---|---|
| the players the rules named | `NewSetActorsEffect(phase, ids...)`, or the list a death detour writes on entering the phase. **Aliveness is the rules' business**, and the kernel does not veto a second time |
| the default | the **living** players matching `PhaseStep.Role` |

Skill validation, `AllowedSkills`, `PhaseReadiness` and `PhaseInfo` all share
the single `actorsForStep` read point -- four questions with one source is
what keeps "the kernel accepted his submission while telling everyone else he
should not be acting" from arising.

A detour (`NewDetourEffect`) used to be a **third layer** here, answering the
same question as naming with a nearly word-for-word identical implementation.
It no longer answers "who may act": on entering the phase it is owed in, the
kernel writes the head of the queue as that phase's actor list, and everything
after that follows the naming path. It is written on entering the phase rather
than at the effect's write point because the queue may hold several detours
pointing at the same phase (two hunters eliminated on one night), and writing
at the effect would have them overwrite each other, leaving only the last one
able to act.

## Extension points must not call back into the engine

All eight extension points are called synchronously **while the engine holds
its lock**. Calling any `Engine` method from inside one **hangs**, it does not
error -- Go's RWMutex is not reentrant, and that game stops responding for
good.

They do not need to call back: everything they could want is in the arguments.
The signatures are deliberately narrow, an extension point never receives an
`*Engine`, and getting around the constraint means stashing the engine in a
struct yourself, which is a deliberate act.

To ask the engine something from a callback, use an `OnEvent` / `OnMessage`
handler -- events and messages are both published **outside the lock**:

```go
e.OnEvent(func(ev *hiddenrole.Event) {
	audience, known := e.AudienceOf(ev) // safe: no lock is held here
	if !known {
		return // a third-party event type the engine does not know; route it yourself, do not broadcast by default
	}
	for _, id := range audience {
		send(id, ev)
	}
})
```

Wiring the engine into a server is exactly this; see `example/netserver` in
the werewolf repository.

## Unit-testing your own resolver

No need to run a whole game. `Board` lets you lay one out by hand:

```go
b := hiddenrole.Board{
	Players: []hiddenrole.PlayerInfo{
		hiddenrole.Seat("r1", roleRed, true),
		hiddenrole.Seat("b1", roleBlue, true),
	},
	Round: 1,
	Phase: phaseVote,
}

effects := vote{}.Resolve([]*hiddenrole.SkillUse{
	{PlayerID: "r1", Skill: skillVote, Targets: []string{"b1"}},
}, b.View())

after := b.Apply(effects)          // fold the effects back in
p, _ := after.Player("b1")
// p.Alive == false
```

`Seat(id, role, alive, vars...)` places a player, `Mark(p, keys...)` puts this
round's markers on them, and `Board.Var(scope, k)` reads any one of the four
cells.

## Saving, replay and errors

```go
snap := e.Snapshot()                                   // plain data, json.Marshal it directly
e2, err := hiddenrole.RestoreEngine(cfg, snap, opts...) // the options must match those used to create the game

log := e.EffectLog()                                   // the complete effect log since the game was created
e3, err := hiddenrole.ReplayEngine(cfg, log, opts...)  // rebuild from the log
```

The effect log is **history**, a snapshot is **state**: persist with
`Snapshot`, and use `EffectLog` for in-process replay, post-game analysis and
investigation. A snapshot carries a version (`SnapshotVersion`), and a format
it does not understand is explicitly rejected rather than guessed at.

Errors all carry a code, and both `errors.Is` and `HasCode` classify them:

```go
if err := e.SubmitSkillUse(use); err != nil {
	switch {
	case errors.Is(err, hiddenrole.ErrPlayerDead):
		...
	case hiddenrole.HasCode(err, hiddenrole.CodeSkillNotAllowed):
		...
	}
}
```

Report your own rules' errors with `WrapError(code, format, args...)`, the
same machinery the kernel uses.

## What the kernel does not do

- **It keeps no clock.** `PhaseConfig.Timeout` is advice, when `EndPhase` is
  called is entirely up to the caller, and `PhaseReadiness()` tells you who is
  still missing.
- **No networking, no lobbies, no matchmaking.**
- **No storage.** `Snapshot` exports the board and `RestoreEngine` rebuilds
  it; where it is stored is the user's business.
- **It knows no game's rules.** That is a rules package's job.

## The full API

```
go doc github.com/Zereker/hiddenrole
```

The package documentation is in [`doc.go`](doc.go). For a real, running rules
package see [Zereker/werewolf](https://github.com/Zereker/werewolf) -- every
entry point it uses is one you can use too.

## License

MIT License. See [LICENSE](LICENSE).
