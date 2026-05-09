---
title: Built-in pack lifecycle
description: How `.gc/system/packs/` is populated, why local edits are
  ephemeral, and the correct workflow for changing a built-in pack.
---

## tl;dr

`.gc/system/packs/<name>/` is **regenerated from the `gc` binary on every
`gc init` and `gc start`**. Editing files in there directly does nothing
durable — the next gc command resets them.

To change a built-in pack, edit the source under
`examples/<...>/packs/<name>/` (or `internal/bootstrap/packs/core/`),
rebuild `gc`, then restart the city.

## Where the system packs come from

Built-in packs are compiled into the `gc` binary using Go's `//go:embed`
directive. Each pack has an `embed.go` that captures its file tree at
build time:

```go
// examples/gastown/packs/maintenance/embed.go
//go:embed pack.toml doctor formulas orders all:agents \
    template-fragments all:assets
var PackFS embed.FS
```

The list of embedded packs is registered in
[`cmd/gc/embed_builtin_packs.go`](https://github.com/gastownhall/gascity/blob/main/cmd/gc/embed_builtin_packs.go):

```go
var builtinPacks = []builtinPack{
    {fs: core.PackFS,        name: "core"},
    {fs: bd.PackFS,          name: "bd"},
    {fs: dolt.PackFS,        name: "dolt"},
    {fs: maintenance.PackFS, name: "maintenance"},
    {fs: gastown.PackFS,     name: "gastown"},
}
```

`MaterializeBuiltinPacks(cityPath)` walks each embedded FS and writes its
contents to `.gc/system/packs/<name>/`. It compares content + mode and
repairs drift with an atomic rename, so a partial write is never visible
to readers — and any hand-edited file is overwritten back to the embedded
version.

## Where the materializer runs

`MaterializeBuiltinPacks` is called from every meaningful gc command:

| Caller (cmd/gc) | When |
| --- | --- |
| `cmd_supervisor.go` | controller (supervisor) startup |
| `cmd_supervisor_city.go` | `gc start` |
| `init_provider_readiness.go` | `gc init` (provider readiness) |
| `cmd_config.go` | config loads that need system packs |
| `cmd_beads_city.go` | `gc beads` city-level operations |

In practice you can assume any `gc` invocation that touches city state
will run the materializer.

## Why local edits don't stick

The lifecycle is:

```
source tree                       gc binary                  city dir
examples/.../packs/maintenance/   ─embed─▶  PackFS  ─materialize─▶  .gc/system/packs/maintenance/
        ▲                                                                    │
        │                                                        edits here are reverted
        edit + rebuild                                           on the next gc command
```

The embedded FS is frozen at compile time, so the binary's view of the
pack does not change until you rebuild. As long as the binary keeps
running with a stale embed, every materialize call resets the on-disk
copy.

The two failure modes operators run into:

- **Manual edit drift.** You hand-edit a script in `.gc/system/packs/`
  to test a fix. It works once, then 20 minutes later (next `gc start`
  / supervisor recycle) the change is gone. The materializer rewrote it
  from the binary's stale embed.
- **Source edit without rebuild.** You change
  `examples/.../packs/.../some.sh` on `main`, but the running `gc`
  binary was built before that commit. The materialized copy still
  reflects the old source until you rebuild.

## Correct workflow

1. **Edit the source** under `examples/<...>/packs/<name>/` (or
   `internal/bootstrap/packs/core/` for `core`).
2. **Rebuild the binary**: `make gc-install` (or whatever places a fresh
   binary on `PATH`).
3. **Restart the city**: `gc stop` then `gc start`. The materializer
   runs at startup and rewrites `.gc/system/packs/<name>/` from the new
   embed.
4. **Verify**: compare a known signature in the materialized file to
   the source on disk, or check `stat` on `which gc` against the commit
   that introduced the change.

If you only need a one-off change for local debugging, prefer adding a
city-local override layer (see
[feature-parity notes](../archive/analysis/feature-parity.md) on the
resolution order) rather than editing `.gc/system/packs/` directly.
City-local overrides shadow system packs and are not touched by the
materializer.

## Why it's designed this way

The materializer is intentionally aggressive about repair. Its job is to
make `.gc/system/packs/` a function of the running binary, not a
mutable surface that operators have to maintain. That guarantees:

- Pack contents on disk match what the binary's loader expects.
- Corrupted or truncated files self-heal on the next start.
- Upgrades (new gc version) automatically replace the system packs
  without any explicit cleanup step.

The cost is what this doc exists to point out: edits inside
`.gc/system/packs/` are ephemeral. Treat that directory as a cache, not
as configuration.

## Related code

- `cmd/gc/embed_builtin_packs.go` — registry, `MaterializeBuiltinPacks`,
  `materializeFS`.
- `examples/<...>/packs/<name>/embed.go` — per-pack embed declarations.
- `internal/citylayout/layout.go` — `SystemPacksRoot` constant
  (`.gc/system/packs`).
- `cmd/gc/embed_builtin_packs_test.go` — extraction invariants and
  permission checks.
