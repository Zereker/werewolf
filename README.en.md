# Werewolf Game Engine

[繁中/简中 README](README.md) · English

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/Zereker/werewolf.svg)](https://pkg.go.dev/github.com/Zereker/werewolf)

A rules engine for Werewolf / Mafia, written in Go with **zero dependencies**.

It does exactly one thing: given a board and a rule set, decide what happened
in each phase and **what each player is entitled to know**. Timers, networking,
rooms, and persistence are deliberately out of scope — those belong to you.

## Why this exists

Social deduction is easy to prototype and hard to get right. The hard part is
not the state machine, it's the information boundary: the witch may see tonight's
kill only while she still holds the antidote; a vetoed poison must reach the
witch alone, or she is outed; a saved-then-guarded target may or may not die
depending on a house rule.

Getting one of those wrong does not crash anything. It silently produces a
game that is subtly unfair, and you find out from players, weeks later.

This library takes those judgements off your hands and keeps them under test.

## Who it's for

**Building a Werewolf product** — you get the rules, the information boundary,
save/restore, and replay. Wire up your own transport and UI.
[`example/netserver`](example/netserver) is a working TCP server in under 400
lines; [`example/cli`](example/cli) is a host console you can play a full game in.

**Building an LLM social-deduction benchmark** — you get determinism (same
inputs, byte-identical snapshots), a per-player view you can hand straight to an
agent as its prompt context, and a complete effect log to replay a run. Agents
cannot see what they shouldn't, because the engine never hands it to them.

## Install

```sh
go get github.com/Zereker/werewolf
```

## Quick start

```go
g, _ := werewolf.New(werewolf.DefaultRules()) // the default 9-player board

for id, role := range map[string]werewolf.RoleType{
    "w1": werewolf.RoleWerewolf, "w2": werewolf.RoleWerewolf,
    "s": werewolf.RoleSeer, "wi": werewolf.RoleWitch, "g": werewolf.RoleGuard,
    "v1": werewolf.RoleVillager, "v2": werewolf.RoleVillager,
} {
    g.AddPlayer(id, role)
}
g.Start()

// The wolves pick a target
g.SubmitSkillUse(&werewolf.SkillUse{
    PlayerID: "w1", Skill: werewolf.SkillKill, TargetID: "v1",
})
effects, _ := g.EndPhase() // resolve, then advance

// What may this player be told? Send it as-is; no filtering needed.
view := g.PlayerView("wi")
fmt.Println(view.RoleInfo[werewolf.RoleInfoKillTarget]) // "v1" — she still has the antidote

// Who should hear about a given event?
for _, e := range effects {
    audience, known := g.AudienceOf(e.ToEvent())
    _ = audience // known == false means "the engine has no opinion; you route it"
}
```

## What you get

| | |
|---|---|
| **Information boundary** | `PlayerView` is safe to send verbatim. `AudienceOf` answers "who should hear this". God-view interfaces are named separately and documented as non-forwardable. |
| **Determinism** | Same board, same inputs, byte-identical snapshots. Enforced by 5000 randomized games per test run. |
| **Save / restore** | `Snapshot()` / `Restore()`, versioned, refuses formats it cannot read. |
| **Replay** | Every state change flows through one write point, so the effect log is a complete history. `Replay()` rebuilds a game from it. |
| **Configurable rules** | Witch self-save, guard repeat-protect, guard+antidote interaction, side-wipe vs. town-wipe victory — all switches, not forks. |
| **Zero dependencies** | `go.mod` has no `require` block. |

## Extending it

The governing standard for the whole design:

> **Adding a role must not require changing a line of the engine.**

Eight extension points, all constructor options. Built-in roles have **no
privileges** — they go through exactly the same doors:

| To add | Use |
|---|---|
| A role's behaviour | `WithResolver(phase, resolver)` — wraps built-ins to reuse them |
| A role's starting state | `WithRoleSetup(role, setup)` — the witch's two potions are one row in this table |
| A role's own state | `NewSetPlayerVarEffect` / `NewSetRoundVarEffect` / `NewSetPlayerRoundVarEffect` |
| A victory condition | `WithVictoryChecker(checker)` — wrap `DefaultVictoryChecker` to add a clause |
| Role-private information | `WithRoleInfo(role, provider)` — the witch's kill target is one of these |
| Who hears an event | `WithAudience(provider)` |
| Who is on whose side | `WithTeammates(provider)` — asymmetry allowed |
| Who can hear speech | `WithSpeech(provider)` |

[`example/extension`](example/extension) adds an **Idiot** (flips their card when
voted out, survives, loses the vote thereafter) using only exported API.

### The kernel knows four state primitives

`KILL`, `POISON`, `ELIMINATE`, `SHOOT` are names the *rules* give to things that
happen. The state machine does not recognise them — emit a bare `KILL` and
nobody dies. What changes state is the primitive next to it:

```go
// Werewolf's night resolution, in full:
NewEffect(EventKill, "", target),   // what happened — for audience and the log
NewSetAliveEffect(target, false),   // what changed — for the state machine
```

That separation is what lets a rules pack invent its own deaths, marks and
items without touching the engine.

## Status

**Current release: [v1.5.0](CHANGELOG.md).** The generic kernel and the Werewolf
rules are now separated: no code path in the engine recognises a specific role,
camp, or cause of death, and the whole rule set is installed through the same
public options a third party would use. 94.5% coverage, every rule traced to the
Wikipedia article, 5000 randomized games per test run.

**v1.5.0 is the API freeze point.** It carries a large set of breaking changes
(listed in the changelog); from here on the public API is a commitment.

**The import path will not change.** Go requires a `/vN` suffix on modules at
major version 2 and above, which would change every user's import path. Paying
the breakage once and staying on the v1 line is the better trade for a library
with no known importers yet.

The two layers are two packages:

| Package | What it is |
|---|---|
| `github.com/Zereker/werewolf` | The Werewolf rules: roles, phases, resolvers, victory |
| `github.com/Zereker/werewolf/engine` | The kernel: players, a phase ring, four state primitives, the information boundary |

**"The rules only use public API" is enforced by the compiler**, not by
discipline — the rules package sits outside the kernel, and every door it uses
is a door you can use too. To check, read
[`engine/types.go`](engine/types.go): the kernel's whole vocabulary is five
non-empty values — `START`, `END`, `GOD`, `SKIP`, `ANNOUNCE` — plus a zero value
per type. There is no witch and no werewolf; those live in the root package's
[`vocab.go`](vocab.go).

**Playing a game needs the root package only.** It re-exports a small, deliberate
slice of the kernel — about twenty names ([`alias.go`](alias.go)) — under a single
admission rule: a name is there only if the root package's own exported API uses
it (`SkillUse`, `GameView`, `Effect`, `Snapshot`, the vocabulary types and their
kernel-owned values). They are plain aliases: `werewolf.Effect` and
`engine.Effect` are the same type.

**Changing the rules means writing the `engine.` prefix.** Custom resolvers, a
different victory checker, logging and metrics, branching on error codes, taking
a snapshot apart — those names live in the kernel. That is not an oversight; it
is how the boundary stays visible at the call site. The rules package writes it
that way itself (see `resolver.go`, `rolesetup.go`). The kernel's own API is
documented in [engine/README.md](engine/README.md).

## Testing

Passing tests is not the same as tests that would catch anything. Every
behavioural change in this repo is **mutation-verified**: the change is reverted
in place and the suite must go red. Those results are recorded in the changelog
entry and the commit message, so you can check the claim rather than trust it.

```sh
go test ./...          # unit + integration + 200 randomized games
go test -race ./...    # the TCP server example is exercised under -race
make lint              # golangci-lint
```

## Documentation

- [`doc.go`](doc.go) / [pkg.go.dev](https://pkg.go.dev/github.com/Zereker/werewolf) — package documentation
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — design decisions and their reasons
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — how to build, test, and what a good change looks like
- [`CHANGELOG.md`](CHANGELOG.md) — every release, every breaking change, and why

## License

MIT. See [LICENSE](LICENSE).
