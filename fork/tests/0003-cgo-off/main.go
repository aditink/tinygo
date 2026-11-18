// Patch 0003 test: the file set follows CGO_ENABLED. fork/test.sh builds this program with
// CGO_ENABLED=0 (see the `env` file); with that setting the cgo-tagged file must be excluded
// and the !cgo file selected.
package main

import "fmt"

func main() {
	if cgoOn {
		fmt.Println("FAIL: cgo-tagged file selected under CGO_ENABLED=0")
		return
	}
	fmt.Println("fork-0003 ok: !cgo file set selected")
}
