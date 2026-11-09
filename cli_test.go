package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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

// An action of the app with no app connected fails: rex do and rex
// config reload exit with 1.
//
// spec: F-175, F-180
func TestNoAppFails(t *testing.T) {
	startTestServer(t)
	for _, args := range [][]string{{"do", "client.tab.new"}, {"config", "reload"}, {"config", "reload", "--json"}} {
		if out, code := cli(t, nil, args...); code != exitFailed || !strings.Contains(out, "no app is connected") {
			t.Errorf("%s: %d %s", strings.Join(args, " "), code, out)
		}
	}
}

// rex config check --json prints what it found, and exits with 1 when a
// problem is an error.
//
// spec: F-179
func TestConfigCheckJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GOREX_DIR", dir)
	path := filepath.Join(dir, "keymap.json")
	os.WriteFile(path, []byte(`{"preset": "linux", "bindings": [{"key": "ctrl+shift+k", "action": "none"}]}`), 0o644)
	out, code := cli(t, nil, "config", "check", "--json")
	var r configCheck
	if err := json.Unmarshal([]byte(out), &r); err != nil || code != 0 || r.Path != path || !r.Exists || r.Preset != "linux" || r.Problems == nil {
		t.Errorf("config check --json: %d %s %+v", code, out, r)
	}
	os.WriteFile(path, []byte(`{"preset": "nope"}`), 0o644)
	out, code = cli(t, nil, "config", "check", "--json")
	if code != exitFailed || !strings.Contains(out, `"problems": [`) || !strings.Contains(out, "unknown preset") {
		t.Errorf("config check --json of a bad file: %d %s", code, out)
	}
}
