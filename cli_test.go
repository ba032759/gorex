package main

import (
	"strings"
	"testing"
)

// GoRex runs as rex with a command of rex's, after rex's global options.
//
// spec: F-100
func TestGoRexRunsAsRex(t *testing.T) {
	for args, want := range map[string]bool{
		"ls":                          true,
		"--timeout 5s ls":             true,
		"-C app --autostart=false ls": true,
		"--timeout=5s help":           true,
		"--timeout x ls":              true, // rex says what is wrong
		"-S host ls":                  true,
		"":                            false,
		"--timeout 5s":                false,
		"--nope ls":                   false,
		"--timeout 5s nope":           false,
		"-psn_0_1234":                 false,
	} {
		if got := isCLI(strings.Fields(args)); got != want {
			t.Errorf("GoRex %s: rex %v, want %v", args, got, want)
		}
	}
}

// --autostart is a boolean as every other flag.
//
// spec: F-101
func TestAutostartIsBoolean(t *testing.T) {
	for v, want := range map[string]bool{"": true, "=true": true, "=1": true, "=false": false, "=0": false} {
		var g global
		if _, err := globalFlags(&g).parseLeading([]string{"--autostart" + v, "ls"}); err != nil || g.autostart != want {
			t.Errorf("--autostart%s: %v %v", v, g.autostart, err)
		}
	}
	for _, v := range []string{"yes", "no", "on", "2"} {
		var g global
		if _, err := globalFlags(&g).parseLeading([]string{"--autostart=" + v, "ls"}); exitCode(err) != exitUsage {
			t.Errorf("--autostart=%s: %v", v, err)
		}
	}
}

// capture's formats are the values --format takes: another is a wrong
// command line, before rex connects.
//
// spec: F-164
func TestCaptureFormats(t *testing.T) {
	t.Setenv("GOREX_DIR", t.TempDir()) // no server runs there
	for _, f := range []string{"html", "foo", "TEXT"} {
		if out, code := cli(t, nil, "--autostart=false", "capture", "--format", f); code != exitUsage || !strings.Contains(out, "text or vt") {
			t.Errorf("--format %s: %d %s", f, code, out)
		}
	}
	if _, code := cli(t, nil, "--autostart=false", "capture", "--format", "vt"); code != exitNoServer {
		t.Errorf("--format vt without a server: %d", code)
	}
}
