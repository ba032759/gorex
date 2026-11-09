package main

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// A command's flags come before, between and after its arguments; what
// follows -- is a command, kept as it is; a number is an argument.
//
// spec: F-102
func TestFlagParse(t *testing.T) {
	var dir, label string
	var json, focus bool
	var ratio float64
	var index int
	var agents []string
	f := newFlagSet("flag")
	f.str(&dir, "c,cwd,d,dir", "", "`DIR`")
	f.str(&label, "label", "", "`LABEL`")
	f.boolean(&json, "json", false, "")
	f.boolean(&focus, "focus", true, "")
	f.percent(&ratio, "ratio", 50, pctRange{0, 100, false, "takes a percentage between 1 and 99"}, "`PCT`")
	f.position(&index, "index", "`N`")
	f.list(&agents, "agent", "`AGENT`")
	c, err := f.parse(strings.Fields("a -d /tmp b --json --focus=false -5 --ratio=30% --agent x --agent=y -- sh -c --json"))
	if err != nil {
		t.Fatal(err)
	}
	if dir != "/tmp" || !json || focus || ratio != 30 || !slices.Equal(agents, []string{"x", "y"}) {
		t.Errorf("dir %q json %v focus %v ratio %v agents %v", dir, json, focus, ratio, agents)
	}
	if !slices.Equal(c.args, []string{"a", "b", "-5"}) || !slices.Equal(c.command, []string{"sh", "-c", "--json"}) || !c.dashed {
		t.Errorf("args %q command %q", c.args, c.command)
	}
	// An empty value is the default.
	if _, err := f.parse([]string{"--ratio="}); err != nil || ratio != 50 {
		t.Errorf("--ratio=: %v %v", ratio, err)
	}
	for _, bad := range []struct{ args, msg string }{
		{"--nope", "unknown flag --nope"},
		{"--label", "--label needs a value"},
		{"--json=yes", "--json=yes takes true or false"},
		{"--ratio 100", "--ratio takes a percentage between 1 and 99"},
		{"--index 0", "--index takes a position from 1"},
	} {
		_, err := f.parse(strings.Fields(bad.args))
		if err == nil || err.Error() != bad.msg || exitCode(err) != exitUsage {
			t.Errorf("%s: %v", bad.args, err)
		}
	}
}

// The global options come before the command.
//
// spec: F-101
func TestGlobalOptions(t *testing.T) {
	var g global
	rest, err := globalFlags(&g).parseLeading(strings.Fields("-C app --timeout 3s --autostart=false -vv --color never ls -q"))
	if err != nil || g.client != "app" || g.timeout != 3*time.Second || g.autostart || !slices.Equal(rest, []string{"ls", "-q"}) {
		t.Errorf("%+v %q %v", g, rest, err)
	}
	if rest, _ := globalFlags(&g).parseLeading([]string{"--help"}); len(rest) != 1 {
		t.Errorf("--help is a command: %q", rest)
	}
	for _, bad := range []string{"-S host ls", "--nope ls", "--timeout x ls", "-- ls"} {
		if _, err := globalFlags(&g).parseLeading(strings.Fields(bad)); exitCode(err) != exitUsage {
			t.Errorf("%s: %v", bad, err)
		}
	}
}
