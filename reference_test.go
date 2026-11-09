package main

import (
	"fmt"
	"os"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"
)

// The reference of the rex command, spec/reference/cli.md, is made from
// the code: its commands, their flags, and the help that tells of them.
// The spec's functional items say what the commands are for; this file
// says what they take, and cannot drift: the test fails when it differs
// from the code. Remake it with: mise run spec-ref
const referenceFile = "spec/reference/cli.md"

type refCommand struct {
	path    string // "split", "session new"
	fn      uintptr
	flags   [][]string
	probed  bool
	aliases []string
}

// These do their work before they read any flag, so the probe does not
// run them: they take no flags.
var noFlags = map[string]bool{"server run": true, "server start": true, "server logs": true, "skill show": true, "skill hook": true, "completion": true}

func probeFlags(fn func([]string) error) (got [][]string, ok bool) {
	done := make(chan struct{})
	flagProbe = func(f *flags) { got, ok = f.order, true }
	defer func() { flagProbe = nil }()
	stdout := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() { os.Stdout = stdout }()
	go func() {
		defer close(done)
		_ = fn(nil)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		panic("a command did not reach its flags in 3 s: add it to noFlags")
	}
	return got, ok
}

func reference(t *testing.T) string {
	// A command that acts before it reads its flags acts here, not on
	// the user's files or server.
	home := t.TempDir()
	for k, v := range map[string]string{"GOREX_DIR": home + "/gorex", "HOME": home, "XDG_CONFIG_HOME": home + "/.config",
		"REX_BLOCK": "", "REX_SESSION": "", "REX_WINDOW": "", "REX_CLIENT": "", "CLAUDECODE": ""} {
		t.Setenv(k, v)
	}
	opts.autostart = false
	defer func() { opts.autostart = true }()
	var cmds []*refCommand
	add := func(path string, fn func([]string) error) {
		var fl [][]string
		ok := noFlags[path]
		if !ok {
			fl, ok = probeFlags(fn)
		}
		cmds = append(cmds, &refCommand{path: path, fn: reflect.ValueOf(fn).Pointer(), flags: fl, probed: ok})
	}
	for name, fn := range cliCommandFuncs() {
		add(name, fn)
	}
	for g, sub := range groups() {
		for name, fn := range sub {
			add(g+" "+name, fn)
		}
	}
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].path < cmds[j].path })
	// Commands that run the same function are one command by two names.
	seen := map[uintptr]*refCommand{}
	var out []*refCommand
	for _, c := range cmds {
		if first, ok := seen[c.fn]; ok {
			first.aliases = append(first.aliases, c.path)
			continue
		}
		seen[c.fn] = c
		out = append(out, c)
	}

	var b strings.Builder
	b.WriteString("# rex: the reference\n\n")
	b.WriteString("Made from the code by `mise run spec-ref`; do not edit it. Every command\n")
	b.WriteString("of `rex`, the flags it takes, and what its help leaves out. What the\n")
	b.WriteString("commands are for is in `spec/functional/`; `spec check` reports a command\n")
	b.WriteString("that no functional item names.\n")
	for _, c := range out {
		fmt.Fprintf(&b, "\n## `rex %s`\n\n", c.path)
		if len(c.aliases) > 0 {
			var as []string
			for _, a := range c.aliases {
				as = append(as, "`rex "+a+"`")
			}
			fmt.Fprintf(&b, "Also: %s.\n\n", strings.Join(as, ", "))
		}
		topic := helpTopic(c.path)
		if topic != "" {
			fmt.Fprintf(&b, "Help: `rex help %s`.\n\n", topic)
		}
		switch {
		case !c.probed:
			b.WriteString("Flags: it reads its arguments itself.\n")
			continue
		case len(c.flags) == 0:
			b.WriteString("Flags: none.\n")
			continue
		}
		var missing []string
		b.WriteString("| Flag | Spellings |\n|---|---|\n")
		for _, sp := range c.flags {
			var all []string
			for _, s := range sp {
				all = append(all, "`"+dashed(s)+"`")
			}
			name := sp[len(sp)-1]
			fmt.Fprintf(&b, "| `%s` | %s |\n", dashed(name), strings.Join(all, ", "))
			if topic != "" && !slices.ContainsFunc(sp, func(s string) bool { return mentions(help[topic], dashed(s)) }) {
				missing = append(missing, "`"+dashed(name)+"`")
			}
		}
		if len(missing) > 0 {
			fmt.Fprintf(&b, "\nNot in its help: %s.\n", strings.Join(missing, ", "))
		}
	}
	return b.String()
}

func dashed(s string) string {
	if len(s) == 1 {
		return "-" + s
	}
	return "--" + s
}

// mentions tells whether the help names a flag, as a word.
func mentions(text, flag string) bool {
	for i := 0; ; {
		j := strings.Index(text[i:], flag)
		if j < 0 {
			return false
		}
		end := i + j + len(flag)
		if end == len(text) || !strings.ContainsRune("abcdefghijklmnopqrstuvwxyz-", rune(text[end])) {
			return true
		}
		i = end
	}
}

// helpTopic is the help topic of a command: its own, else its group's.
func helpTopic(path string) string {
	if _, ok := help[path]; ok {
		return path
	}
	if g, _, ok := strings.Cut(path, " "); ok {
		if _, ok := help[g]; ok {
			return g
		}
	}
	return ""
}

func TestReference(t *testing.T) {
	got := reference(t)
	if os.Getenv("SPEC_UPDATE") != "" {
		if err := os.MkdirAll("spec/reference", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(referenceFile, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, _ := os.ReadFile(referenceFile)
	if string(want) != got {
		t.Fatalf("%s differs from the code: run mise run spec-ref, and review the diff", referenceFile)
	}
}

// Every name in cliCommands is a command.
func TestCLICommandsKnown(t *testing.T) {
	for _, c := range cliCommands {
		_, cmd := cliCommandFuncs()[c]
		_, grp := groups()[c]
		if !cmd && !grp && c != "help" && c != "--help" && c != "-h" {
			t.Errorf("%q is in cliCommands, and not a command", c)
		}
	}
}
