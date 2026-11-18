package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"time"
)

func check(name string, ok bool) {
	if ok {
		fmt.Println("PASS:", name)
	} else {
		fmt.Println("FAIL:", name)
		os.Exit(1)
	}
}

func main() {
	// --- socketpair test ---
	fds, err := syscallSocketpair()
	if err != nil {
		fmt.Println("FAIL: socketpair:", err)
		os.Exit(1)
	}
	a := os.NewFile(uintptr(fds[0]), "a")
	b := os.NewFile(uintptr(fds[1]), "b")

	// 1. read deadline in the past -> timeout
	past := time.Now().Add(-time.Second)
	if err := a.SetReadDeadline(past); err != nil {
		fmt.Println("FAIL: SetReadDeadline:", err)
		os.Exit(1)
	}
	buf := make([]byte, 16)
	start := time.Now()
	_, rerr := a.Read(buf)
	elapsed := time.Since(start)
	check("socketpair read with past deadline returns ErrDeadlineExceeded", errors.Is(rerr, os.ErrDeadlineExceeded))
	check("timeout returned quickly (<2s)", elapsed < 2*time.Second)

	// 2. clear deadline, write on b, read on a
	if err := a.SetReadDeadline(time.Time{}); err != nil {
		fmt.Println("FAIL: clear deadline:", err)
		os.Exit(1)
	}
	msg := []byte("hello-netpoller")
	if _, werr := b.Write(msg); werr != nil {
		fmt.Println("FAIL: write:", werr)
		os.Exit(1)
	}
	n, rerr2 := a.Read(buf)
	check("socketpair read after write succeeds", rerr2 == nil && n == len(msg) && string(buf[:n]) == string(msg))

	// 3. read deadline in the future, no data -> timeout after ~200ms
	if err := a.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		fmt.Println("FAIL: SetReadDeadline future:", err)
		os.Exit(1)
	}
	start = time.Now()
	_, rerr3 := a.Read(buf)
	elapsed = time.Since(start)
	check("socketpair read with 200ms deadline times out", errors.Is(rerr3, os.ErrDeadlineExceeded))
	check("timeout took >=150ms", elapsed >= 150*time.Millisecond)
	check("timeout took <2s", elapsed < 2*time.Second)

	// 4. after timeout, write then read with fresh deadline works
	if err := a.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		fmt.Println("FAIL: SetReadDeadline 2s:", err)
		os.Exit(1)
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		b.Write([]byte("late"))
	}()
	n, rerr4 := a.Read(buf)
	check("socketpair read woken by late write", rerr4 == nil && n == 4 && string(buf[:n]) == "late")

	// --- regression: >=3 fds ready at once, batched in one epoll_pwait (svp
	// blocker 1: packed epollEvent layout misparsed events with a 16-byte
	// stride; with the packed 12-byte layout each event's Data must be read at
	// offset 4). 8 reader goroutines park FIRST (each with its own 2s deadline,
	// armed just before its Read so the deadline cannot expire while an
	// earlier reader is still blocked), then all 8 writes happen at once, so
	// one epoll_pwait returns several events in a single batch and each parked
	// reader must be woken with ITS OWN fd's event. A misattributed event
	// leaves some reader parked until its deadline (ErrDeadlineExceeded)
	// instead of returning its byte. On the old 16-byte layout this fails on
	// 90632d3 under both schedulers: readers 1,3,5,7 time out (the old strict
	// check got EOF on every fd) [obs, ran on 90632d3]. ---
	const nfds = 8
	var readers, writers [nfds]*os.File
	for i := 0; i < nfds; i++ {
		f, err := syscallSocketpair()
		if err != nil {
			fmt.Println("FAIL: socketpair", i, err)
			os.Exit(1)
		}
		readers[i] = os.NewFile(uintptr(f[0]), fmt.Sprintf("r%d", i))
		writers[i] = os.NewFile(uintptr(f[1]), fmt.Sprintf("w%d", i))
	}
	// per-reader result: byte read (want 'A'+i) or error. Each reader uses its
	// own buffer: they run concurrently and share nothing.
	type res struct {
		fd  int
		b   byte
		ok  bool
		err error
	}
	resCh := make(chan res, nfds)
	goStart := make(chan struct{})
	for i := 0; i < nfds; i++ {
		go func(i int) {
			<-goStart
			// arm the deadline just before reading, so it cannot expire while
			// an earlier reader is still parked
			readers[i].SetReadDeadline(time.Now().Add(2 * time.Second))
			rbuf := make([]byte, 16)
			n, rerr := readers[i].Read(rbuf)
			r := res{fd: i, err: rerr}
			if n == 1 {
				r.b, r.ok = rbuf[0], true
			}
			resCh <- r
		}(i)
	}
	// let all readers park, then make all fds readable at once
	time.Sleep(100 * time.Millisecond)
	close(goStart)
	time.Sleep(100 * time.Millisecond) // all 8 readers are now parked in pollWait
	werrCh := make(chan error, nfds)
	for i := 0; i < nfds; i++ {
		go func(i int) {
			_, werr := writers[i].Write([]byte{byte('A' + i)})
			werrCh <- werr
		}(i)
	}
	for i := 0; i < nfds; i++ {
		if werr := <-werrCh; werr != nil {
			fmt.Println("FAIL: multi-fd write", i, werr)
			os.Exit(1)
		}
	}
	multiOK := true
	for k := 0; k < nfds; k++ {
		r := <-resCh
		if r.err != nil || !r.ok || r.b != byte('A'+r.fd) {
			fmt.Printf("  multi-fd read %d: ok=%v byte=%q err=%v (want byte %q, nil)\n", r.fd, r.ok, r.b, r.err, byte('A'+r.fd))
			multiOK = false
		}
	}
	check("8 parked readers each get their own byte from one batched epoll_pwait (packed epollEvent)", multiOK)
	// strict: a second read on each fd must time out (its one byte was consumed),
	// proving events were not cross-attributed between fds. Deadline armed per
	// read so it cannot expire early. On the old 16-byte layout this fails with
	// EOF on every fd [obs, ran on 90632d3].
	strictOK := true
	for i := 0; i < nfds; i++ {
		readers[i].SetReadDeadline(time.Now().Add(150 * time.Millisecond))
		_, rerr := readers[i].Read(buf)
		if !errors.Is(rerr, os.ErrDeadlineExceeded) {
			fmt.Printf("  strict read %d: err=%v (want ErrDeadlineExceeded)\n", i, rerr)
			strictOK = false
		}
	}
	check("8 fds: second read on each times out (no cross-fd event misattribution)", strictOK)

	// --- regression: concurrent Close while a read is parked (svp blocker 4 /
	// reviewer item 1: semrelease 0->1 without a wake used to hang Close) ---
	cfds, err := syscallSocketpair()
	if err != nil {
		fmt.Println("FAIL: socketpair close:", err)
		os.Exit(1)
	}
	ca := os.NewFile(uintptr(cfds[0]), "ca")
	cb := os.NewFile(uintptr(cfds[1]), "cb")
	if err := ca.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		fmt.Println("FAIL: SetReadDeadline close:", err)
		os.Exit(1)
	}
	closeDone := make(chan error, 1)
	go func() {
		time.Sleep(100 * time.Millisecond)
		closeDone <- ca.Close()
	}()
	_, rerrC := ca.Read(buf) // parked; Close must unblock it
	cerr := <-closeDone
	check("parked read unblocked by concurrent Close (ErrClosed)", errors.Is(rerrC, os.ErrClosed))
	check("concurrent Close returned nil within 10s", cerr == nil)
	cb.Close()

	// --- pipe test ---
	pr, pw, perr := os.Pipe()
	if perr != nil {
		fmt.Println("FAIL: pipe:", perr)
		os.Exit(1)
	}
	if err := pr.SetReadDeadline(past); err != nil {
		fmt.Println("FAIL: pipe SetReadDeadline:", err)
		os.Exit(1)
	}
	_, perr2 := pr.Read(buf)
	check("pipe read with past deadline returns ErrDeadlineExceeded", errors.Is(perr2, os.ErrDeadlineExceeded))

	if err := pr.SetReadDeadline(time.Time{}); err != nil {
		fmt.Println("FAIL: pipe clear deadline:", err)
		os.Exit(1)
	}
	if _, werr := pw.Write([]byte("pipe-data")); werr != nil {
		fmt.Println("FAIL: pipe write:", werr)
		os.Exit(1)
	}
	n, perr3 := pr.Read(buf)
	check("pipe read after write succeeds", perr3 == nil && n == 9 && string(buf[:n]) == "pipe-data")

	// --- net.Pipe (in-memory, no fd) sanity: should not touch netpoller ---
	c1, c2 := net.Pipe()
	go func() { c2.Write([]byte("np")) }()
	c1.SetReadDeadline(time.Now().Add(time.Second))
	n, nerr := c1.Read(buf)
	check("net.Pipe read works", nerr == nil && n == 2 && string(buf[:n]) == "np")

	fmt.Println("fork-0002 ok: ALL TESTS PASSED")
}
