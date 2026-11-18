// Patch 0001 test: with upstream net/os/internal/poll in the merged GOROOT, the APIs
// that gVisor's runsc needs (P2 Finding 2) exist and work. Before 0001 this program does not
// type-check: TinyGo's net/os overrides lack them.
package main

import (
	"io"
	"net"
	"os"
	"syscall"
)

func fail(s string, err error) {
	println("FAIL:", s, err.Error())
	syscall.Exit(1)
}

func main() {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		fail("socketpair", err)
	}
	ca, err := net.FileConn(os.NewFile(uintptr(fds[0]), "a"))
	if err != nil {
		fail("FileConn a", err)
	}
	cb, err := net.FileConn(os.NewFile(uintptr(fds[1]), "b"))
	if err != nil {
		fail("FileConn b", err)
	}
	ua, ub := ca.(*net.UnixConn), cb.(*net.UnixConn)
	// pass fd 1 (stdout) as SCM_RIGHTS along with a payload
	if _, _, err := ua.WriteMsgUnix([]byte("hi"), syscall.UnixRights(1), nil); err != nil {
		fail("WriteMsgUnix", err)
	}
	buf, oob := make([]byte, 8), make([]byte, 64)
	n, oobn, _, _, err := ub.ReadMsgUnix(buf, oob)
	if err != nil {
		fail("ReadMsgUnix", err)
	}
	msgs, err := syscall.ParseSocketControlMessage(oob[:oobn])
	if err != nil || len(msgs) != 1 {
		fail("ParseSocketControlMessage", err)
	}
	rights, err := syscall.ParseUnixRights(&msgs[0])
	if err != nil || len(rights) != 1 {
		fail("ParseUnixRights", err)
	}
	if _, err := ub.File(); err != nil { // net.UnixConn.File (runsc/console)
		fail("UnixConn.File", err)
	}
	f := os.NewFile(uintptr(rights[0]), "passed-stdout")
	if _, err := f.ReadFrom(nil2{}); err != nil { // os.File.ReadFrom (runsc/cmd/sandboxsetup)
		fail("File.ReadFrom", err)
	}
	_ = net.DialUnix // runsc/... godbus: net.DialUnix
	println("fork-0001 ok", string(buf[:n]))
}

type nil2 struct{}

func (nil2) Read(p []byte) (int, error) { return 0, io.EOF }

