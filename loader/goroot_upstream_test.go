package loader

import (
	"testing"

	"github.com/tinygo-org/tinygo/compileopts"
)

// TestPathsToOverrideUpstreamNetOsPoll checks fork patch 0001: on a hosted Linux target the
// merged GOROOT takes Go's own net, os and internal/poll (no TinyGo override entry), while
// the other overrides stay in place.
func TestPathsToOverrideUpstreamNetOsPoll(t *testing.T) {
	config := &compileopts.Config{
		Options: &compileopts.Options{},
		Target:  &compileopts.TargetSpec{GOOS: "linux", GOARCH: "amd64"},
	}
	paths := pathsToOverride(config)
	for _, p := range []string{"net/", "net/http/", "os/", "internal/poll/"} {
		if v, ok := paths[p]; ok {
			t.Errorf("pathsToOverride has %q = %v; want no entry (upstream package)", p, v)
		}
	}
	for p, want := range map[string]bool{"runtime/": false, "internal/": true, "sync/": true, "internal/task/": false} {
		if got, ok := paths[p]; !ok || got != want {
			t.Errorf("pathsToOverride[%q] = %v (present %v); want %v", p, got, ok, want)
		}
	}
}
