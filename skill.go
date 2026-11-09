package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gorex/internal/rex"
)

// "rex skill" installs, as Rex's does, a skill that teaches agents the rex
// command, and a hook that tells them, as a session starts, that they run
// in a terminal of GoRex and which.

// skillMarker marks the files rex skill installed, which it updates and
// removes, from others of the same name it leaves alone.
const skillMarker = "<!-- installed by rex skill setup (GoRex) -->"

// hookCommand is in the command of rex's hooks, of every version.
const hookCommand = " skill hook "

// agent is an agent rex can set up.
type agent struct {
	name, title string
	// home is the agent's directory, and bin its program, which tell that
	// it is installed.
	home, bin string
	// skill is the directory of the skill; settings the file of its hooks,
	// "" for an agent without hooks rex knows.
	skill, settings string
}

func agents(project bool) []agent {
	home, _ := os.UserHomeDir()
	claude := os.Getenv("CLAUDE_CONFIG_DIR")
	if claude == "" {
		claude = filepath.Join(home, ".claude")
	}
	codex := os.Getenv("CODEX_HOME")
	if codex == "" {
		codex = filepath.Join(home, ".codex")
	}
	shared := filepath.Join(home, ".agents", "skills", "rex")
	claudeSkill, claudeSettings := filepath.Join(claude, "skills", "rex"), filepath.Join(claude, "settings.json")
	if project {
		wd, _ := os.Getwd()
		root := repoRoot(wd)
		shared = filepath.Join(root, ".agents", "skills", "rex")
		claudeSkill, claudeSettings = filepath.Join(root, ".claude", "skills", "rex"), filepath.Join(root, ".claude", "settings.json")
	}
	return []agent{
		{name: "claude", title: "Claude Code", home: claude, bin: "claude", skill: claudeSkill, settings: claudeSettings},
		{name: "codex", title: "Codex", home: codex, bin: "codex", skill: shared},
		{name: "opencode", title: "OpenCode", home: filepath.Join(home, ".config", "opencode"), bin: "opencode", skill: shared},
		{name: "pi", title: "Pi", home: filepath.Join(home, ".pi"), bin: "pi", skill: shared},
	}
}

// repoRoot is the directory of the repository dir is in, or dir.
func repoRoot(dir string) string {
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		if filepath.Dir(d) == d {
			return dir
		}
	}
}

func (g agent) found() bool {
	if fi, err := os.Stat(g.home); err == nil && fi.IsDir() {
		return true
	}
	_, err := exec.LookPath(g.bin)
	return err == nil
}

// skillOptions are the flags of setup, status and remove.
type skillOptions struct {
	agents        []string
	project       bool
	dryRun        bool
	force         bool
	sessionNaming bool
}

// skillItem is what happened, or would, to a skill or a hook.
type skillItem struct {
	Agent string `json:"agent"`
	Item  string `json:"item"`
	State string `json:"state"`
	Path  string `json:"path"`
}

type skillReport struct {
	Items []skillItem `json:"items"`
	Notes []string    `json:"notes,omitempty"`
}

func (r skillReport) summary() string {
	var done []string
	for _, it := range r.Items {
		if it.State != "unchanged" && it.State != "unsupported" {
			done = append(done, it.Agent+" "+it.Item+" "+it.State)
		}
	}
	if len(done) == 0 {
		return "Everything is up to date."
	}
	return strings.Join(done, ", ") + ". Start a new agent session to use it."
}

func (r skillReport) print(w io.Writer) {
	last := ""
	for _, it := range r.Items {
		if it.Agent != last {
			if last != "" {
				fmt.Fprintln(w)
			}
			fmt.Fprintln(w, it.Agent)
			last = it.Agent
		}
		fmt.Fprintf(w, "  %-5s  %-12s %s\n", it.Item, it.State, tildePath(it.Path))
	}
	for _, n := range r.Notes {
		fmt.Fprintln(w, "\n"+n)
	}
}

func tildePath(p string) string {
	if home, _ := os.UserHomeDir(); home != "" && strings.HasPrefix(p, home+string(os.PathSeparator)) {
		return "~" + p[len(home):]
	}
	return p
}

// chosen returns the agents to act on: those named, else those found, and
// with none found, the shared skill alone.
func chosen(o skillOptions, all bool) ([]agent, error) {
	list := agents(o.project)
	if len(o.agents) > 0 {
		var out []agent
		for _, n := range o.agents {
			i := slices.IndexFunc(list, func(g agent) bool { return g.name == n })
			if i < 0 {
				return nil, usagef("unknown agent %q: claude, codex, opencode or pi", n)
			}
			out = append(out, list[i])
		}
		return out, nil
	}
	if all {
		return list, nil
	}
	var out []agent
	for _, g := range list {
		if g.found() || o.project {
			out = append(out, g)
		}
	}
	if len(out) == 0 {
		out = []agent{{name: "agents", title: "Agents", skill: list[1].skill}}
	}
	return out, nil
}

// skillFile is the SKILL.md rex installs.
func skillFile() string {
	return `---
name: rex
description: Drive the terminal you run in with the rex command (GoRex, a Rex-compatible terminal) - open tabs and split panes, run commands beside you and wait for them, type into other terminals and read their screens, switch sessions. Use when asked to open, split, run something in, read, or close panes, tabs or sessions, or when a long-running command (dev server, tests, logs) deserves a terminal of its own.
---
` + skillMarker + "\n\n" + skillGuide
}

// skillGuide is what the skill teaches, which "rex skill show" prints.
const skillGuide = `# rex: drive the terminal you run in

You run inside GoRex, a terminal that keeps sessions in a server and
follows Superlogical Rex's model and CLI. The ` + "`rex`" + ` command is on PATH.

## Where you are

- A **session** groups related work; a **window** is a tab of a session; a
  **block** is one terminal (a pane) in a window.
- ` + "`$REX_SESSION`" + ` is your session, ` + "`$REX_BLOCK`" + ` your own terminal. With no
  flags, commands act there: ` + "`rex split`" + ` splits your own terminal.
- Target something else with ` + "`-s SESSION`" + `, ` + "`-w WINDOW`" + `, ` + "`-b BLOCK`" + `. A target
  is a full ID, a label (any case), a window's position from 1, or the
  last 4+ characters of an ID. Prefer IDs in scripts; labels can change.
- Exit status: 0 ok, 1 failed, 2 bad command line, 3 target not found or
  ambiguous, 4 server unreachable. ` + "`rex wait`" + ` and ` + "`--wait`" + ` exit with the
  program's own status.

## Look before acting

    rex ls                    # sessions
    rex window ls             # tabs of your session (-A for all)
    rex block ls              # terminals of your session: @ is you, * focused
    rex block inspect -b ID   # pid, program, dir, size of one terminal
    rex list                  # the whole tree

## Open terminals

    rex split -- htop                          # right of you, focus moves there
    rex split --split=below --ratio 30 --focus=false --label logs -- tail -f app.log
    rex run -- npm run dev                     # a new tab running a command
    rex run --split=right -b editor -- lazygit # beside the block labeled editor
    rex new api --cwd ~/src/api -- nvim        # a new session
    rex window new tests                       # a new tab with a shell

Add ` + "`--json`" + ` to get IDs: ` + "`rex split --json -- make | jq -r '.block_ids[0]'`" + `.
Use ` + "`--focus=false`" + ` to leave the user's focus where it is, and
` + "`--keep-open`" + ` to keep a terminal after its command exits. Commands run with
the login shell's environment, not its aliases: use ` + "`sh -c '...'`" + ` for pipes.

## Run and wait

    rex split --wait --focus=false -- go test ./...   # exits with the tests' status
    B=$(rex split --json --keep-open --focus=false -- make | jq -r '.block_ids[0]')
    rex wait --for 10m "$B"; echo "make exited $?"

## Type and read

    rex send -b logs 'make test'     # types text, no Enter
    rex send-key -b logs enter       # keys: enter, ctrl+c, escape, up, shift+tab, f5…
    printf 'y\n' | rex send -b logs  # from stdin
    rex capture -b logs --trim       # the screen and scrollback as text

Full-screen programs (vim, htop, less) show only their screen to capture.
Poll ` + "`rex capture`" + ` until the output you expect appears rather than sleeping
long.

## Arrange

    rex focus left|right|up|down|next|prev [--wrap]   rex focus logs   rex focus 2
    rex zoom [--on|--off] -b ID     rex move ID below editor --ratio 30
    rex swap A B                    rex resize ID left --amount 10
    rex block rename ID logs        rex window rename 2 build   rex session rename api
    rex close ID                    rex window close 2          rex kill SESSION

## Status

Tell the user's tabs what a long job does, and what it waits for: they
show it, and notify when the user is away.

    rex status working --app build --msg "Building" --progress 40
    rex status blocked --kind question --msg "Deploy to prod?"
    rex status done --msg "12 tests passed"     # or: error, idle, clear
    rex status -b ID                             # what a block's programs report

Claude Code reports its own status through rex's hooks; you need not.

## The app

    rex do ACTION key=value     # an action of the GoRex window, as the user's keys run
    rex actions                 # every action: session.select, client.tab.goto…
    rex events -s "$REX_SESSION" --json   # what happens, one JSON object a line

## Manners

- Act where the user means: "beside you" or nothing is your own block; "the
  focused pane" is the focused block of the session the user views; a name
  is the block, tab or session with that label.
- Close what you opened when you are done with it, and only that.
- Don't steal focus for background work: use --focus=false.
- Never type into a terminal you have not looked at first (rex capture).
`

// skillSetup installs the skill and hooks.
func skillSetup(o skillOptions) (skillReport, error) {
	var r skillReport
	list, err := chosen(o, false)
	if err != nil {
		return r, err
	}
	written := map[string]string{}
	for _, g := range list {
		state, err := installSkill(g.skill, o, written)
		if err != nil {
			return r, err
		}
		r.Items = append(r.Items, skillItem{g.title, "skill", state, g.skill})
		if g.settings == "" {
			if g.name != "agents" {
				r.Items = append(r.Items, skillItem{g.title, "hook", "unsupported", "-"})
			}
			continue
		}
		state, err = installHook(g, o)
		if err != nil {
			return r, err
		}
		r.Items = append(r.Items, skillItem{g.title, "hook", state, g.settings})
	}
	r.Notes = append(r.Notes, "Start a new agent session inside GoRex to use it.")
	if slices.ContainsFunc(list, func(g agent) bool { return g.name == "claude" }) {
		r.Notes = append(r.Notes, "Claude Code runs no hooks in a folder it does not trust yet.")
	}
	return r, nil
}

// installSkill writes SKILL.md into dir; written keeps the directories
// done already, which agents share.
func installSkill(dir string, o skillOptions, written map[string]string) (string, error) {
	if s, ok := written[dir]; ok {
		return s, nil
	}
	path := filepath.Join(dir, "SKILL.md")
	want := skillFile()
	state := "created"
	if b, err := os.ReadFile(path); err == nil {
		switch {
		case string(b) == want:
			written[dir] = "unchanged"
			return "unchanged", nil
		case !bytes.Contains(b, []byte(skillMarker)) && !o.force:
			written[dir] = "foreign"
			return "foreign", nil
		}
		state = "updated"
	} else if fi, err := os.Stat(dir); err == nil && fi.IsDir() && !o.force {
		if entries, _ := os.ReadDir(dir); len(entries) > 0 {
			written[dir] = "foreign"
			return "foreign", nil
		}
	}
	if o.dryRun {
		state = "would be " + state
	} else {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			return "", err
		}
	}
	written[dir] = state
	return state, nil
}

// hookEvents are the Claude Code events rex hooks: the session's start,
// which tells the agent where it runs, and those that tell its status
// (OSC 7501) to GoRex: working, blocked on the user, done.
var hookEvents = []string{"SessionStart", "UserPromptSubmit", "Notification", "PermissionRequest", "PostToolUse", "Stop", "SessionEnd"}

// hookEntry is rex's hook of a Claude Code event. The hook acts in GoRex
// alone, whose terminals have REX_BIN_DIR: it runs the server's own rex,
// whatever order a shell profile puts PATH in.
func hookEntry(event string, naming bool) any {
	cmd := `[ -n "$REX_BIN_DIR" ] && "$REX_BIN_DIR/rex" skill hook claude ` + event
	if naming && event == "SessionStart" {
		cmd += " --session-naming"
	}
	return map[string]any{"hooks": []any{map[string]any{"type": "command", "command": cmd + " || true"}}}
}

// installHook adds rex's hooks to Claude Code's settings, leaving the rest
// of the file as it was.
func installHook(g agent, o skillOptions) (string, error) {
	doc, err := readSettings(g.settings)
	if err != nil {
		return "unreadable", nil //nolint:nilerr // reported as the hook's state, not as a failure of setup
	}
	hooks, _ := doc.get("hooks").(*ordered)
	if hooks == nil {
		hooks = &ordered{}
	}
	naming := o.sessionNaming
	if !naming {
		starts, _ := hooks.get("SessionStart").([]any)
		for _, e := range starts {
			if strings.Contains(rexHookCommand(e), "--session-naming") {
				naming = true // keep the choice installed
			}
		}
	}
	state, touched := "unchanged", false
	for _, ev := range hookEvents {
		entries, _ := hooks.get(ev).([]any)
		kept := entries[:0:0]
		existing := ""
		for _, e := range entries {
			if c := rexHookCommand(e); c != "" {
				existing = c
				continue
			}
			kept = append(kept, e)
		}
		want := hookEntry(ev, naming)
		if existing == rexHookCommand(want) {
			continue
		}
		if existing != "" || touched && state == "updated" {
			state = "updated"
		} else if state == "unchanged" {
			state = "created"
		}
		touched = true
		hooks.set(ev, append(kept, want))
	}
	if !touched {
		return "unchanged", nil
	}
	if o.dryRun {
		return "would be " + state, nil
	}
	doc.set("hooks", hooks)
	return state, writeSettings(g.settings, doc)
}

// rexHookCommand returns the command of a SessionStart entry that is rex's.
func rexHookCommand(entry any) string {
	var hs []any
	switch e := entry.(type) {
	case *ordered:
		hs, _ = e.get("hooks").([]any)
	case map[string]any:
		hs, _ = e["hooks"].([]any)
	}
	for _, h := range hs {
		var cmd string
		switch h := h.(type) {
		case *ordered:
			cmd, _ = h.get("command").(string)
		case map[string]any:
			cmd, _ = h["command"].(string)
		}
		if strings.Contains(cmd, "rex") && strings.Contains(cmd, hookCommand) {
			return cmd
		}
	}
	return ""
}

func readSettings(path string) (*ordered, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) || err == nil && len(bytes.TrimSpace(b)) == 0 {
		return &ordered{}, nil
	}
	if err != nil {
		return nil, err
	}
	v, err := decodeOrdered(json.NewDecoder(bytes.NewReader(b)))
	if err != nil {
		return nil, err
	}
	o, ok := v.(*ordered)
	if !ok {
		return nil, errors.New("not an object")
	}
	return o, nil
}

func writeSettings(path string, doc *ordered) error {
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".rex-tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// skillStatus reports what is installed.
func skillStatus(o skillOptions) (skillReport, error) {
	var r skillReport
	list, err := chosen(o, true)
	if err != nil {
		return r, err
	}
	for _, g := range list {
		title := g.title + " (not found)"
		if g.found() {
			title = g.title + " (found)"
		}
		state := "missing"
		if b, err := os.ReadFile(filepath.Join(g.skill, "SKILL.md")); err == nil {
			switch {
			case string(b) == skillFile():
				state = "current"
			case bytes.Contains(b, []byte(skillMarker)):
				state = "outdated"
			default:
				state = "foreign"
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			state = "unreadable"
		}
		r.Items = append(r.Items, skillItem{title, "skill", state, g.skill})
		if g.settings == "" {
			r.Items = append(r.Items, skillItem{title, "hook", "unsupported", "-"})
			continue
		}
		state = "missing"
		if doc, err := readSettings(g.settings); err != nil {
			state = "unreadable"
		} else if hooks, _ := doc.get("hooks").(*ordered); hooks != nil {
			found, current := 0, 0
			for _, ev := range hookEvents {
				entries, _ := hooks.get(ev).([]any)
				for _, e := range entries {
					if c := rexHookCommand(e); c != "" {
						found++
						if c == rexHookCommand(hookEntry(ev, strings.Contains(c, "--session-naming"))) {
							current++
						}
					}
				}
			}
			switch {
			case found == 0:
			case current == len(hookEvents):
				state = "current"
			default:
				state = "outdated"
			}
		}
		r.Items = append(r.Items, skillItem{title, "hook", state, g.settings})
	}
	r.Notes = append(r.Notes, `Run "rex skill setup" to install or update the skill.`)
	return r, nil
}

// skillRemove removes what rex installed, and nothing else.
func skillRemove(o skillOptions) (skillReport, error) {
	var r skillReport
	list, err := chosen(o, true)
	if err != nil {
		return r, err
	}
	done := map[string]bool{}
	for _, g := range list {
		path := filepath.Join(g.skill, "SKILL.md")
		state := "absent"
		if b, err := os.ReadFile(path); err == nil {
			if !bytes.Contains(b, []byte(skillMarker)) {
				state = "foreign, kept"
			} else if done[g.skill] {
				state = "removed"
			} else {
				state = "removed"
				if o.dryRun {
					state = "would be removed"
				} else {
					os.Remove(path)
					os.Remove(g.skill) // when empty
				}
			}
		}
		done[g.skill] = true
		r.Items = append(r.Items, skillItem{g.title, "skill", state, g.skill})
		if g.settings == "" {
			continue
		}
		state = "absent"
		if doc, err := readSettings(g.settings); err == nil {
			if hooks, _ := doc.get("hooks").(*ordered); hooks != nil {
				for _, ev := range slices.Clone(hooks.keys) {
					entries, _ := hooks.get(ev).([]any)
					kept := entries[:0:0]
					for _, e := range entries {
						if rexHookCommand(e) == "" {
							kept = append(kept, e)
						}
					}
					if len(kept) == len(entries) {
						continue
					}
					state = "removed"
					if len(kept) == 0 {
						hooks.del(ev)
					} else {
						hooks.set(ev, kept)
					}
				}
				if state == "removed" {
					if o.dryRun {
						state = "would be removed"
					} else {
						if len(hooks.keys) == 0 {
							doc.del("hooks")
						}
						if err := writeSettings(g.settings, doc); err != nil {
							return r, err
						}
					}
				}
			}
		}
		r.Items = append(r.Items, skillItem{g.title, "hook", state, g.settings})
	}
	return r, nil
}

// skillCLI is the flags of rex skill setup, status and remove.
type skillCLI struct {
	o    skillOptions
	json bool
}

func skillFlags(o *skillCLI, f *flagSet) {
	f.list(&o.o.agents, "agent", "only this `AGENT`").defaults("every agent found")
	f.boolean(&o.o.project, "project", false, "in this project, not in your home")
	f.boolean(&o.o.dryRun, "dry-run", false, "tell what it would do, and do nothing")
	f.boolean(&o.o.force, "force", false, "replace a skill of the same name that rex did not install")
	f.boolean(&o.o.sessionNaming, "session-naming", false, "also have the agent name its session after its task")
	jsonFlag(f, &o.json)
}

// cliSkill is rex skill setup, status or remove.
func cliSkill(name string, o *skillCLI) error {
	fn := map[string]func(skillOptions) (skillReport, error){"setup": skillSetup, "status": skillStatus, "remove": skillRemove}[name]
	r, err := fn(o.o)
	if err != nil {
		return err
	}
	output(o.json, r, func() { r.print(os.Stdout) })
	return nil
}

func cliSkillShow(args []string) error {
	fmt.Print(skillGuide)
	return nil
}

// cliSkillHook is the hook agents run: as a session starts, it tells
// them where in GoRex they are; as they work, it tells GoRex their status
// (OSC 7501), which tabs show and notifications tell. It does nothing
// outside GoRex, and never fails the agent.
func cliSkillHook(args []string) error {
	block := os.Getenv("REX_BLOCK")
	if block == "" {
		return nil
	}
	event, naming := "SessionStart", false
	for _, a := range args {
		switch {
		case a == "--session-naming":
			naming = true
		case a != "claude" && !strings.HasPrefix(a, "-"):
			event = a
		}
	}
	var in hookInput
	if fi, err := os.Stdin.Stat(); err == nil && fi.Mode()&os.ModeCharDevice == 0 {
		_ = json.NewDecoder(io.LimitReader(os.Stdin, 1<<20)).Decode(&in) // a hook works without its input
	}
	if event == "SessionStart" {
		fmt.Printf("You are running inside GoRex, a Rex-compatible terminal. Your session is %s and your own terminal (block) is %s. "+
			"Use the rex skill, or run `rex skill show`, to open panes and tabs, run commands beside you, and read other terminals.\n",
			os.Getenv("REX_SESSION"), block)
		if naming {
			fmt.Println("Once you know your task, name the session after it: rex session rename \"<a few words>\".")
		}
	}
	if report := hookStatus(event, in); report != "" {
		sendStatus(block, event, report)
	}
	return nil
}

// hookInput is what Claude Code gives a hook on its standard input.
type hookInput struct {
	Prompt           string `json:"prompt"`
	Message          string `json:"message"`
	NotificationType string `json:"notification_type"`
	ToolName         string `json:"tool_name"`
	LastMessage      string `json:"last_assistant_message"`
	TranscriptPath   string `json:"transcript_path"`
}

const claudeApp = "claude-code"

// hookStatus is the report an event of Claude Code makes: "" for none.
func hookStatus(event string, in hookInput) string {
	switch event {
	case "SessionStart":
		return statusReport("idle", "", "", "", claudeApp, "", "")
	case "UserPromptSubmit":
		return statusReport("working", "", "", "", claudeApp, "", oneLine(in.Prompt, 120))
	case "Notification":
		switch in.NotificationType {
		case "idle_prompt", "auth_success":
			return "" // waiting for the next prompt, as Stop said
		case "elicitation_dialog":
			return statusReport("blocked", "", "question", "", claudeApp, "", oneLine(in.Message, 200))
		}
		if in.NotificationType == "permission_prompt" || strings.Contains(strings.ToLower(in.Message), "permission") {
			return statusReport("blocked", "", "permission", "", claudeApp, "", oneLine(in.Message, 200))
		}
		return ""
	case "PermissionRequest":
		msg := "Claude needs your permission"
		if in.ToolName != "" {
			msg += " to use " + in.ToolName
		}
		return statusReport("blocked", "", "permission", "", claudeApp, "", msg)
	case "PostToolUse":
		return statusReport("working", "", "", "", claudeApp, "", "")
	case "Stop":
		msg := in.LastMessage
		if msg == "" {
			msg = lastAssistantText(in.TranscriptPath)
		}
		return statusReport("done", "", "", "", claudeApp, "", oneLine(msg, 200))
	case "SessionEnd":
		return "state=clear"
	}
	return ""
}

// sendStatus reports a status to the server that runs the block, quickly
// or not at all: a hook must not hold up the agent. A report of working
// keeps the message of the turn, and is not sent again while working.
func sendStatus(block, event, report string) {
	c, err := rex.Dial()
	if err != nil {
		return
	}
	defer c.Close()
	if event == "SessionEnd" {
		// The session's result outlives it, as the protocol has done and
		// error outlive their program: only what it was doing ends.
		var cur struct {
			Records []rex.StatusRecord `json:"records"`
		}
		if c.DoFor("status.list", &rex.Args{Block: block}, &cur, 2*time.Second) == nil {
			for _, r := range cur.Records {
				if r.ID == "" && (r.State == "done" || r.State == "error") {
					return
				}
			}
		}
	}
	if event == "PostToolUse" {
		var cur struct {
			Records []rex.StatusRecord `json:"records"`
		}
		if c.DoFor("status.list", &rex.Args{Block: block}, &cur, 2*time.Second) == nil {
			for _, r := range cur.Records {
				if r.ID != "" {
					continue
				}
				if r.State == "working" {
					return
				}
				if r.Msg != "" && r.State == "blocked" {
					// Back to work on the turn, whose prompt is gone.
					report = statusReport("working", "", "", "", claudeApp, "", "")
				}
			}
		}
	}
	c.DoFor("status.report", &rex.Args{Block: block, Data: report}, nil, 2*time.Second)
}

// oneLine makes a text one line of at most n characters.
func oneLine(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > n {
		s = string(r[:n-1]) + "…"
	}
	return s
}

// lastAssistantText reads the last text Claude wrote, from the end of a
// session's transcript.
func lastAssistantText(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	const tail = 256 << 10
	if fi, err := f.Stat(); err == nil && fi.Size() > tail {
		f.Seek(fi.Size()-tail, io.SeekStart)
	}
	b, _ := io.ReadAll(f)
	lines := bytes.Split(b, []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		var e struct {
			Type    string `json:"type"`
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(lines[i], &e) != nil || e.Type != "assistant" {
			continue
		}
		var parts []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal(e.Message.Content, &parts) == nil {
			for j := len(parts) - 1; j >= 0; j-- {
				if parts[j].Type == "text" && strings.TrimSpace(parts[j].Text) != "" {
					return parts[j].Text
				}
			}
		}
	}
	return ""
}

// ordered is a JSON object that keeps the order of its keys, so that
// editing a settings file leaves the rest of it as it was.
type ordered struct {
	keys []string
	vals map[string]any
}

func (o *ordered) get(k string) any {
	if o.vals == nil {
		return nil
	}
	return o.vals[k]
}

func (o *ordered) set(k string, v any) {
	if o.vals == nil {
		o.vals = map[string]any{}
	}
	if _, ok := o.vals[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
}

func (o *ordered) del(k string) {
	delete(o.vals, k)
	o.keys = slices.DeleteFunc(o.keys, func(s string) bool { return s == k })
}

func (o *ordered) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		b.Write(kb)
		b.WriteByte(':')
		vb, err := json.Marshal(o.vals[k])
		if err != nil {
			return nil, err
		}
		b.Write(vb)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// decodeOrdered decodes a JSON value, its objects as ordered.
func decodeOrdered(d *json.Decoder) (any, error) {
	d.UseNumber()
	tok, err := d.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			o := &ordered{}
			for d.More() {
				kt, err := d.Token()
				if err != nil {
					return nil, err
				}
				k, _ := kt.(string)
				v, err := decodeOrdered(d)
				if err != nil {
					return nil, err
				}
				o.set(k, v)
			}
			_, err := d.Token()
			return o, err
		case '[':
			arr := []any{}
			for d.More() {
				v, err := decodeOrdered(d)
				if err != nil {
					return nil, err
				}
				arr = append(arr, v)
			}
			_, err := d.Token()
			return arr, err
		}
	}
	return tok, nil
}
