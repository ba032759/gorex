package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

// rex's help. The prose is written by hand, here and in the commands'
// declarations (cli.go); the synopses, the lists of commands and of
// flags, and the completion come from the declarations.

const overviewHelp = `rex drives GoRex: sessions of tabs (windows) of terminals (blocks), kept
by a server that outlives the app. Inside a GoRex terminal it knows where it
runs, from REX_SESSION and REX_BLOCK.
`

const overviewEnd = `Targets (-s SESSION, -w WINDOW, -b BLOCK): a full ID, a label, a window's
position from 1, or the last 4+ characters of an ID.

Exit status: 0 ok, 1 failed, 2 usage, 3 target not found or ambiguous,
4 server unreachable, 130 interrupted; wait and --wait exit with the
program's status.

Run "rex help COMMAND" for one command.
`

const placeHelp = `split splits a block (yours by default) and places a terminal beside it;
run places one in a new tab, or beside a block with --split. Without a
command, the terminal runs your shell.`

const captureHelp = `Prints a terminal's screen and scrollback, a line for each row: a line the
terminal wrapped comes as its rows, unless --unwrap joins them. A terminal
keeps about 64 MiB of scrollback; once full, its oldest lines go.`

const sendKeyHelp = `Keys as Rex writes them: modifiers (ctrl, alt, shift) and a key joined by
"+": enter, tab, escape, backspace, space, delete, up, down, left, right,
home, end, pageup, pagedown, f1-f12, or a character. [KeyA] names a key by
its place on a US keyboard.`

const skillHelp = `Installs a skill that teaches agents rex (Claude Code: ~/.claude/skills/rex;
Codex, OpenCode, Pi: ~/.agents/skills/rex) and, in Claude Code, a
SessionStart hook in ~/.claude/settings.json that tells the agent which
session and terminal it runs in. The rest of the settings file stays as it
was. Run setup again to update; remove takes out only what rex installed.`

const keymapHelp = `Lists the key bindings in effect: the preset's and keymap.json's, beside
settings.json in GoRex's data directory. keymap.json:

  {
    "preset": "linux",
    "bindings": [
      {"key": "ctrl+b>c", "action": "client.tab.new"},
      {"key": "ctrl+b>%", "action": "pane.split", "args": {"direction": "right"}},
      {"key": "ctrl+shift+k", "action": "none"}
    ]
  }

Presets: linux (Ctrl+Shift, the default), macos (the Mac's shortcuts with
Ctrl or Super for Cmd), none. Keys: modifiers (ctrl, alt, shift, super or
cmd) and a key joined by "+", sequences joined by ">". "none" unbinds a
key. "rex actions" lists the actions; "rex config reload" applies changes.`

const statusHelp = `Program status (the OSC 7501 protocol): what a program in a terminal is
doing, which tabs, headers and notifications show. A program can print it
to its terminal:

  printf '\033]7501;state=working:app=build:msg=%s\033\\' "$(printf 'Building' | base64)"

or report it with rex status, as a hook that has no terminal does. STATE
is idle, working, blocked (with --kind permission, question or auth),
done, error, or clear. working and blocked end with the program, or when
the shell shows its prompt; done and error stay until the user looks at
them. Without STATE it prints the block's records. rex skill setup hooks
Claude Code in, so that its tab tells when it works, waits for permission,
or finished.`

const doHelp = `Asks the GoRex app to perform one of its actions, as its keys do:
  rex do pane.split direction=below
  rex do session.select session=work
  rex -C CLIENT do client.tab.goto index=2
"rex actions" lists them. GoRex has no Lua: scripts are shell scripts.`

const eventsHelp = `Prints events as they happen: sessions created and destroyed, and with -s,
everything in one session, its terminals' events too (bell, child_exited,
title_changed, pwd_changed, process_changed). --all prints every event.`

const stopHelp = `Stops the server, which ends every program in its sessions. The server saves
the sessions first, and the next one starts them again, as shells in the
same directories, unless --discard. With --if-idle, it stops only if it has
no session and no other client, else it goes on and rex exits with 1.`

const waitHelp = `Waits for a block's program to exit, and exits with its status; or for a
window or session to close.`

// printHelp prints the help of a topic: a command's words, or nothing
// for the overview, which is also the help of a topic rex does not have.
func printHelp(topic string) {
	c := rexCommands
	for _, w := range strings.Fields(topic) {
		if c = c.find(w); c == nil {
			break
		}
	}
	if c == nil || c == rexCommands {
		fmt.Print(overview())
		return
	}
	fmt.Print(commandHelp(c))
}

func overview() string {
	var b strings.Builder
	b.WriteString(overviewHelp)
	b.WriteString("\nUsage: rex [OPTIONS] COMMAND [ARGS]\n")
	for _, sec := range sections {
		fmt.Fprintf(&b, "\n%s\n", sec)
		for _, c := range rexCommands.sub {
			if c.section != sec {
				continue
			}
			left := c.name + " " + c.use
			if c.sub != nil {
				var names []string
				for _, s := range c.sub {
					names = append(names, s.name)
				}
				left = c.name + " " + strings.Join(names, "|")
			}
			left = strings.TrimSpace(left)
			if len(left) > 26 {
				fmt.Fprintf(&b, "  %s\n  %-27s %s\n", left, "", c.short)
				continue
			}
			fmt.Fprintf(&b, "  %-27s %s\n", left, c.short)
		}
	}
	b.WriteString("\nOptions, before the command\n")
	b.WriteString(flagList(globalFlags(&global{}).flags))
	b.WriteString("\n" + overviewEnd)
	return b.String()
}

// commandHelp is the help of a command or a group.
func commandHelp(c *cliCmd) string {
	if c.of != nil {
		c = c.of
	}
	var b strings.Builder
	use := c.use
	switch {
	case c.sub != nil:
		use = "COMMAND"
	case len(c.flags()) > 0:
		use = strings.TrimSpace("[flags] " + use)
	}
	fmt.Fprintf(&b, "Usage: rex %s %s\n", c.path, use)
	if others := otherPaths(c); len(others) > 0 {
		fmt.Fprintf(&b, "Also: rex %s\n", strings.Join(others, ", rex "))
	}
	text := c.help
	if text == "" {
		text = strings.ToUpper(c.short[:1]) + c.short[1:] + "."
	}
	fmt.Fprintf(&b, "\n%s\n", text)
	if c.sub != nil {
		b.WriteString("\nCommands\n")
		w := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)
		for _, s := range c.sub {
			fmt.Fprintf(w, "  %s\t%s\n", strings.TrimSpace(s.name+" "+s.use), s.short)
		}
		w.Flush()
	}
	if fl := c.flags(); len(fl) > 0 {
		b.WriteString("\nFlags\n")
		b.WriteString(flagList(fl))
	}
	return b.String()
}

// flagList lists flags, a line each, from their declarations.
func flagList(fl []*flagDef) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)
	for _, d := range fl {
		fmt.Fprintf(w, "  %s\t%s\n", strings.TrimSpace(d.spellings()+" "+d.placeholder()), d.text()+d.note())
	}
	w.Flush()
	return b.String()
}

// note says the values a flag takes and its default, or that it does
// nothing.
func (d *flagDef) note() string {
	if d.ignored {
		return " (taken, and ignored)"
	}
	var n []string
	if v := d.values(); v != "" && !strings.Contains(v, "again") {
		n = append(n, v)
	}
	if d.def != "" && d.def != "false" {
		n = append(n, "default "+d.def)
	}
	if len(n) == 0 {
		return ""
	}
	return " (" + strings.Join(n, "; ") + ")"
}

// otherPaths are the other ways to name a command: its other words, and
// the commands that are it under another path.
func otherPaths(c *cliCmd) []string {
	var out []string
	parent := strings.TrimSuffix(c.path, c.name)
	for _, n := range c.names {
		out = append(out, parent+n)
	}
	for _, s := range rexCommands.sub {
		if s.of == c {
			out = append(out, s.path)
		}
	}
	return out
}

// cliCompletion prints a script that completes rex's commands, their
// commands and their flags, from their declarations.
func cliCompletion(args []string) error {
	if len(args) != 1 {
		return usagef("usage: rex completion bash|zsh|fish")
	}
	type entry struct{ key, words string }
	var top, groups []string
	var subs, flags []entry
	for _, c := range rexCommands.sub {
		top = append(top, c.name)
		if c.sub == nil {
			flags = append(flags, entry{c.name, flagWords(c)})
			continue
		}
		groups = append(groups, c.name)
		var names []string
		for _, s := range c.sub {
			for _, n := range append([]string{s.name}, s.names...) {
				names = append(names, n)
				flags = append(flags, entry{c.name + " " + n, flagWords(s)})
			}
		}
		subs = append(subs, entry{c.name, strings.Join(names, " ")})
	}
	top = append(top, "help")
	b := &strings.Builder{}
	switch args[0] {
	case "bash":
		fmt.Fprintf(b, "_rex() {\n\tlocal cur=${COMP_WORDS[COMP_CWORD]} key=${COMP_WORDS[1]} words=\n")
		fmt.Fprintf(b, "\tcase $key in %s) key=\"$key ${COMP_WORDS[2]}\" ;; esac\n", strings.Join(groups, "|"))
		fmt.Fprintf(b, "\tif [ \"$COMP_CWORD\" -eq 1 ]; then\n\t\twords=%q\n", strings.Join(top, " "))
		fmt.Fprintf(b, "\telif [ \"$COMP_CWORD\" -eq 2 ] && [ \"$key\" != \"${COMP_WORDS[1]}\" ]; then\n\t\tcase ${COMP_WORDS[1]} in\n")
		for _, e := range subs {
			fmt.Fprintf(b, "\t\t%s) words=%q ;;\n", e.key, e.words)
		}
		fmt.Fprintf(b, "\t\tesac\n\telse\n\t\tcase $key in\n")
		for _, e := range flags {
			if e.words != "" {
				fmt.Fprintf(b, "\t\t%q) words=%q ;;\n", e.key, e.words)
			}
		}
		fmt.Fprintf(b, "\t\tesac\n\tfi\n\t# shellcheck disable=SC2207\n\tCOMPREPLY=($(compgen -W \"$words\" -- \"$cur\"))\n}\ncomplete -F _rex rex\n")
	case "zsh":
		fmt.Fprintf(b, "#compdef rex\nlocal key=$words[2]\nlocal -a w\n")
		fmt.Fprintf(b, "case $key in (%s) (( CURRENT > 3 )) && key=\"$key $words[3]\" ;; esac\n", strings.Join(groups, "|"))
		fmt.Fprintf(b, "if (( CURRENT == 2 )); then\n\tw=(%s)\n", strings.Join(top, " "))
		fmt.Fprintf(b, "elif (( CURRENT == 3 )) && [[ $key == (%s) ]]; then\n\tcase $key in\n", strings.Join(groups, "|"))
		for _, e := range subs {
			fmt.Fprintf(b, "\t(%s) w=(%s) ;;\n", e.key, e.words)
		}
		fmt.Fprintf(b, "\tesac\nelse\n\tcase $key in\n")
		for _, e := range flags {
			if e.words != "" {
				fmt.Fprintf(b, "\t(%q) w=(%s) ;;\n", e.key, e.words)
			}
		}
		fmt.Fprintf(b, "\tesac\nfi\ncompadd -- $w\n")
	case "fish":
		fmt.Fprintf(b, "complete -c rex -f\ncomplete -c rex -n __fish_use_subcommand -a %q\n", strings.Join(top, " "))
		for _, e := range subs {
			fmt.Fprintf(b, "complete -c rex -n '__fish_seen_subcommand_from %s; and not __fish_seen_subcommand_from %s' -a %q\n", e.key, e.words, e.words)
		}
		for _, e := range flags {
			cond := "__fish_seen_subcommand_from " + e.key
			if g, s, ok := strings.Cut(e.key, " "); ok {
				cond = "__fish_seen_subcommand_from " + g + "; and __fish_seen_subcommand_from " + s
			}
			for _, w := range strings.Fields(e.words) {
				opt := "-l " + strings.TrimPrefix(w, "--")
				if !strings.HasPrefix(w, "--") {
					opt = "-s " + strings.TrimPrefix(w, "-")
				}
				fmt.Fprintf(b, "complete -c rex -n '%s' %s\n", cond, opt)
			}
		}
	case "powershell":
		return usagef("rex completion powershell: GoRex runs on Linux, without PowerShell; bash, zsh or fish")
	default:
		return usagef("rex completion bash|zsh|fish")
	}
	_, err := os.Stdout.WriteString(b.String())
	return err
}

// flagWords are a command's flags, every spelling.
func flagWords(c *cliCmd) string {
	var w []string
	for _, d := range c.flags() {
		for _, n := range d.names {
			w = append(w, dashed(n))
		}
	}
	return strings.Join(w, " ")
}
