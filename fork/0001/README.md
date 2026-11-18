# 0001 loader: use upstream Go net, os and internal/poll packages

**Change.** `loader/goroot.go` `pathsToOverride` drops four entries (`internal/poll/`,
`net/`, `net/http/`, `os/`). The merged GOROOT then takes Go's own `net`, `os` and
`internal/poll` instead of TinyGo's reduced versions. `TINYGOROOT/src` is unchanged; on
hosted Linux, TinyGo's `src/os` and `src/net` are simply no longer used.

**Why.** gVisor's `runsc` and `runsc/cmd/sentry` do not type-check against TinyGo's `net`/`os`.
They need `net.DialUnix`, `UnixConn.ReadMsgUnix`/`WriteMsgUnix`/`File`, `os.File.ReadFrom`
and `os.Process.WithHandle` (campaign docs/p2/P2.md, Finding 2). With upstream packages both
roots emit complete modules (Finding 3).

**Dependency.** Upstream `os` and `internal/poll` link to runtime functions that stock TinyGo
does not provide: `os.runtime_beforeExit`, `internal/poll.runtime_pollReset` and
`runtime_pollWait`. The others are `panic("todo")`. So with this commit alone, a program that
reaches `os.Exit` or a pollable fd compiles but does **not link**. This was observed: the
campaign smoke program fails with "linker could not find symbol os.runtime_beforeExit". Commit
0002 supplies these functions. 0001 and 0002 must go upstream together, or 0002 first.

**Test.** `loader/goroot_upstream_test.go` checks that the four entries are gone and that
neighbouring entries are unchanged. `fork/tests/0001-upstream-net-os` uses
`ReadMsgUnix`/`WriteMsgUnix` with SCM_RIGHTS, `UnixConn.File`, `os.File.ReadFrom` and
`net.DialUnix`. At this commit it must compile to IR (mode `ll`); 0002 turns it into a run test.

**Upstream PR plan.** Title: "loader: use upstream net, os and internal/poll on hosted Linux".
Upstream likely wants this gated to linux (and darwin?) hosted targets, not removed for every
target: wasm/baremetal still need TinyGo's `os`. The PR should make the four entries
conditional on `!needsSyscallPackage(tags) && GOOS == "linux"`. The fork keeps the unconditional
form (as approved), which is only valid for the campaign's linux/amd64 target. Not filed: we
have no GitHub credentials, and publication waits on Aditi (Q9, coord 587823).
