# 0003 goenv/loader: honour CGO_ENABLED

**Change.** There are two hunks:
- `goenv/goenv.go`: `Get("CGO_ENABLED")` returns the caller's non-empty `CGO_ENABLED`.
  Otherwise it returns `"1"`, so stock behaviour is unchanged.
- `loader/list.go`: the `go list` subprocess gets `CGO_ENABLED=` + `goenv.Get("CGO_ENABLED")`
  instead of a hard-coded `CGO_ENABLED=1`.

The patch is 30 lines, sha256 `aacc9dcf…`.

**Why.** gVisor is built and shipped with `CGO_ENABLED=0`, which changes the file set (for
example `net`, `os/user` and the seccomp rule files). With stock TinyGo the emitted module
always used the cgo-on file set, so it was not the deployed configuration. Campaign
docs/p2/CGO_OFF.md (D-cgo option (a), MR !420) compares the file sets. With 0001+0003 and
`CGO_ENABLED=0`, there were zero non-GOROOT file-set differences against stock
`CGO_ENABLED=0 go list` for `./runsc` and `./runsc/cmd/sentry`. That comparison is still
[hyp] until it is re-run with the committed comparator.

**Test.**
- `goenv/cgo_enabled_test.go` covers unset → `1`, `0` → `0` and `1` → `1`.
- `fork/tests/0003-cgo-off` is built with `CGO_ENABLED=0` (its `env` file). It must select
  the `!cgo` file. [obs] With the 0002-level binary, the test **fails** ("cgo-tagged file
  selected"); with this commit it passes. So the test tells the two versions apart.

**Upstream PR plan.** Title: "goenv: honour CGO_ENABLED from the environment". Upstream
forces cgo on because targets such as macOS and the rp2040 need it. The PR should keep `"1"`
as the default, and probably reject `0` on targets that require cgo. Not filed: Q9, coord
587823.
