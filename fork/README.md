# gVisor isolation campaign TinyGo fork

Base: `tinygo-org/tinygo` @ `37bbd3328e3789386df1da16f8f77a232bdc39f3` (v0.44.0 + 39 commits).
One commit per patch, in order. Each commit adds its own `fork/<id>/README.md` (what, why,
upstream PR plan) and its own test. Owner decision: OD-9 (Aditi; patches 0001, 0002, 0003
accepted for the fork). Fork location: GitLab branch `toolchain/tinygo-fork` in the campaign
repository until GitHub access exists.

| id | commit subject | test |
|---|---|---|
| 0001 | loader: use upstream Go net, os and internal/poll packages | `go test -tags llvm22 ./loader -run TestPathsToOverrideUpstreamNetOsPoll` + `fork/test.sh` |

The commits are reproducible. They use a fixed author/committer
(`gVisor isolation campaign <source-lead@agents.company.invalid>`) and a fixed date
(`2026-11-18T00:00:00+0000`), so `scripts/toolchain/tinygo.sh` in the campaign repository
can recreate every SHA offline from the base commit plus the patch files and assert it.

`fork/test.sh TINYGOROOT` builds and runs the fork's run tests (`fork/tests/*`) with a
built release root. It is cumulative: each commit's run test also passes on later commits.
