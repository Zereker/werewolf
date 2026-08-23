# Before you change this kernel

## The API is frozen

The exported surface is pinned jointly by [`API.md`](API.md) and
[`testdata/api.golden`](testdata/api.golden), guarded by
`TestAPI_SurfaceIsPinned`: **change a name or a signature and the test goes
red**. The public sub-package `enginetest` is in there too.

Changing the exported surface means doing three things together; any one of
them missing does not count:

1. Have a **specific reason you actually ran into** -- some rules package
   could not be written because of it, or the way around it would tell a lie.
   "I think this is nicer" does not count.
2. Update the golden baseline:
   `go test . -run TestAPI_SurfaceIsPinned -update-api-golden`
3. Update `API.md` (the body and Appendix A)

[`API.md` §15](API.md) lists the four conditions under which reopening the
freeze is worth it.

## A rule cannot live only in a comment

**Tests passing is not the same as tests being useful.** Every behaviour
change needs **mutation verification**: undo the change, run the tests, and
confirm they really do go red. If they do not, that rule was only a comment.

Real problems this has caught in this project:

- Removing the consumption of the actor list turned **not one test red** --
  both rules packages name actors again every time, so a stale list was always
  overwritten.
- The first version of the random games compared snapshot bytes only, and when
  the snapshot serialiser itself drops a field it drops it on both sides --
  the "snapshot loses `Actors`" mutation survived on the spot.

Write what you mutated and what happened into the commit message.

## The kernel knows no game

The test is one sentence: **can the kernel judge this correctly without
knowing what game it is?**

If it cannot, it belongs to the rules. See [`DESIGN.md` §1](DESIGN.md).

The kernel owns only a handful of values, and every one of them is defended
individually in [`DESIGN.md` §7](DESIGN.md) with **usage data from three rules
packages**. **That table may only get shorter.**

## A change has to be verified against all three rules packages

The engine and the rules packages live in two modules. During development use
`replace` to work against local sources:

```
// werewolf/go.mod
replace github.com/Zereker/hiddenrole => ../hiddenrole
```

A kernel change **must** be run against all three rules packages (they are the
only evidence of generality), along with each of their random games
(`RunFuzz`, 5000 games across the three).
