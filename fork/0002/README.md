# 0002 runtime: epoll netpoller for internal/poll (linux/amd64)

**Change.** TinyGo `src/runtime` only:
- deletes `poll.go`, whose four `internal/poll` hooks were `panic("todo")`;
- adds `poll_epoll_linux.go`, a real epoll netpoller implementing the Go 1.27.3
  `internal/poll` linkname contract (`runtime_pollServerInit/Open/Close/Reset/Wait/
  WaitCanceled/SetDeadline/Unblock`, `IsPollDescriptor`);
- adds `poll_hook_unix.go` (scheduler hook) and `poll_stub.go`, which keeps stock behaviour
  on other targets;
- changes `runtime_unix.go`: `sleepTicks` waits in `epoll_pwait` while fds are registered,
  and `os.runtime_beforeExit` is defined;
- changes `sync.go`: `runtime_Semacquire`/`Semrelease` waiters are keyed by address, as in
  upstream `semaRoot`.

Design, memory/fault/blocking notes and review history are in the campaign doc
docs/p2/NETPOLLER.md (Q-net (a), MR !426). The patch is 842 lines, sha256 `0d15ea80…`.

**Why.** After 0001, upstream `os`/`internal/poll` call these functions. Without them, pipes,
sockets and non-blocking fds panic, and `runtime_pollReset`/`runtime_pollWait`/
`os.runtime_beforeExit` do not link.

**Test.** `fork/tests/0002-netpoller` (campaign `qnet-test`) checks, under `-scheduler=tasks`
and `threads`:
- past and 200 ms read deadlines;
- a late write waking a parked reader;
- 8 parked readers with one batched `epoll_pwait`, and no cross-fd misattribution;
- `Close` unblocking a parked read;
- pipe deadlines;
- `net.Pipe`.

At this commit, `fork/tests/0001-upstream-net-os` becomes a run test (mode `run`): it links
and passes.

**Scope.** linux/amd64 only. The test uses `-tags netgo`. That it carries over to the gVisor
build configuration is a hypothesis, not checked here.

**Upstream PR plan.** "runtime: implement internal/poll netpoller with epoll on linux". Send it
together with or before 0001. Upstream will want arm64/386 coverage, or at least a stub
fallback. Not filed (Q9, coord 587823).
