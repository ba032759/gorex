package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"
)

// The reference of the rex command, spec/reference/cli.md, is made from
// the commands' declarations: their words, their flags, what the flags
// take and their defaults. The spec's functional items say what the
// commands are for; this file says what they take, and cannot drift: the
// test fails when it differs from the code. Remake it with: mise run spec-ref
const referenceFile = "spec/reference/cli.md"

// allCommands are the commands of the tree, without groups and without
// the commands that are another's under another path.
func allCommands(c *cliCmd) []*cliCmd {
	var out []*cliCmd
	for _, s := range c.sub {
		switch {
		case s.of != nil:
		case s.sub != nil:
			out = append(out, allCommands(s)...)
		default:
			out = append(out, s)
		}
	}
	return out
}

func reference() string {
	cmds := allCommands(rexCommands)
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].path < cmds[j].path })
	var b strings.Builder
	b.WriteString("# rex: the reference\n\n")
	b.WriteString("Made from the code by `mise run spec-ref`; do not edit it. Every command\n")
	b.WriteString("of `rex`, the flags it takes, the values they take and their defaults,\n")
	b.WriteString("as the commands declare them; `rex help COMMAND` prints the same. What the\n")
	b.WriteString("commands are for is in `spec/functional/`; `spec check` reports a command\n")
	b.WriteString("that no functional item names.\n")
	b.WriteString("\n## Options before the command\n\n")
	flagTable(&b, globalFlags(&global{}).flags)
	for _, c := range cmds {
		fmt.Fprintf(&b, "\n## `rex %s`\n\n", c.path)
		if others := otherPaths(c); len(others) > 0 {
			fmt.Fprintf(&b, "Also: `rex %s`.\n\n", strings.Join(others, "`, `rex "))
		}
		fl := c.flags()
		switch {
		case c.raw != nil:
			b.WriteString("Flags: none; it reads its arguments itself.\n")
		case len(fl) == 0:
			b.WriteString("Flags: none.\n")
		default:
			flagTable(&b, fl)
		}
	}
	return b.String()
}

func flagTable(b *strings.Builder, fl []*flagDef) {
	b.WriteString("| Flag | Value | Default | Help |\n|---|---|---|---|\n")
	for _, d := range fl {
		var sp []string
		for _, n := range d.names {
			sp = append(sp, "`"+dashed(n)+"`")
		}
		val := d.placeholder()
		if val != "" {
			val = "`" + val + "`"
		}
		if v := d.values(); v != "" {
			val += ", " + v
		}
		def := "`" + d.def + "`"
		switch {
		case d.computed:
			def = d.def
		case d.def == "":
			def = "none"
		}
		help := d.text()
		if d.ignored {
			help += "; taken, and ignored"
		}
		fmt.Fprintf(b, "| %s | %s | %s | %s |\n", strings.Join(sp, ", "), val, def, strings.ReplaceAll(help, "|", `\|`))
	}
}

func TestReference(t *testing.T) {
	got := reference()
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

// Each command's help lists every flag it declares, and tells its use.
//
// spec: F-103
func TestHelpListsFlags(t *testing.T) {
	for _, c := range allCommands(rexCommands) {
		if c.short == "" {
			t.Errorf("rex %s has no line for the overview", c.path)
		}
		h := commandHelp(c)
		for _, d := range c.flags() {
			if d.usage == "" {
				t.Errorf("rex %s %s has no help", c.path, d.name())
			}
			if !strings.Contains(h, d.spellings()) {
				t.Errorf("rex help %s does not list %s", c.path, d.spellings())
			}
		}
	}
	// A group's help lists its commands.
	if h := commandHelp(rexCommands.find("session")); !strings.Contains(h, "rename [SESSION] LABEL") {
		t.Errorf("rex help session:\n%s", h)
	}
	// Every command of the overview's sections is in the overview.
	o := overview()
	for _, c := range rexCommands.sub {
		if c.section != "" && !strings.Contains(o, "  "+c.name+" ") {
			t.Errorf("the overview lacks %s", c.name)
		}
	}
}

// The completion completes every command and every flag.
//
// spec: F-124
func TestCompletion(t *testing.T) {
	for _, sh := range []string{"bash", "zsh", "fish"} {
		out, code := cli(t, nil, "completion", sh)
		if code != 0 {
			t.Fatalf("completion %s: %d %s", sh, code, out)
		}
		for _, w := range []string{"split", "send-key", "ratio", "session"} {
			if !strings.Contains(out, w) {
				t.Errorf("completion %s lacks %s", sh, w)
			}
		}
	}
	if _, err := exec.LookPath("bash"); err != nil {
		return
	}
	script, _ := cli(t, nil, "completion", "bash")
	for line, want := range map[string]string{
		"rex spl":            "split",
		"rex split --ra":     "--ratio",
		"rex session re":     "rename",
		"rex window ls --al": "--all",
		"rex block move --r": "--ratio",
	} {
		words := strings.Fields(line)
		if strings.HasSuffix(line, " ") {
			words = append(words, "")
		}
		prog := script + fmt.Sprintf("COMP_WORDS=(%s); COMP_CWORD=%d; _rex; echo \"${COMPREPLY[*]}\"\n", strings.Join(words, " "), len(words)-1)
		out, err := exec.Command("bash", "-c", prog).CombinedOutput()
		if err != nil || !strings.Contains(" "+string(out), " "+want) {
			t.Errorf("bash completes %q to %q (%v), not %s", line, out, err, want)
		}
	}
}
