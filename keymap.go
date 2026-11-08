package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"gorex/internal/rex"
)

// The keymap binds keys to actions, by the names Rex gives them
// ("pane.split.right", "client.tab.new"). The defaults come from a preset,
// and keymap.json, beside settings.json, changes them:
//
//	{
//	  "bindings": [
//	    {"key": "ctrl+b>c", "action": "client.tab.new"},
//	    {"key": "ctrl+b>%", "action": "pane.split", "args": {"direction": "right"}},
//	    {"key": "ctrl+shift+k", "action": "none"}
//	  ]
//	}
//
// Keys are written as Rex writes them: modifiers (cmd or super, ctrl,
// alt, shift) and a key joined by "+", without shift applied (plus is
// "ctrl+shift+="), and a sequence of keys joined by ">". The action "none"
// unbinds a key, which then goes to the terminal. The newest binding of a
// key wins.

// chord is a key with modifiers.
type chord struct {
	mods ui.Modifiers
	key  ui.Key
}

// keyBinding is a key, or a sequence of keys, and what it does.
type keyBinding struct {
	spec   string
	seq    []chord
	action string
	args   map[string]any
	source string
}

// keymap is the bindings in effect, and what was wrong with the file.
type keymap struct {
	bindings []keyBinding
	problems []string
	path     string
	exists   bool
}

// keymapFile is keymap.json.
type keymapFile struct {
	// Preset picks the defaults: "linux" (Control and Shift, as other
	// terminals of Linux and Windows), "macos" (the macOS shortcuts with
	// Control or Super for Command, for keyboards remapped as a Mac's), or
	// "none". It follows settings.json's macosKeybindings when empty.
	Preset   string `json:"preset,omitempty"`
	Bindings []struct {
		Key    string         `json:"key"`
		Action string         `json:"action"`
		Args   map[string]any `json:"args,omitempty"`
	} `json:"bindings"`
}

func keymapPath() string { return filepath.Join(rex.Dir(), "keymap.json") }

// keys is the keymap in effect.
var keys = &keymap{}

// loadKeymap reads the keymap: the preset's bindings, then the file's.
func loadKeymap(path string) *keymap {
	km := &keymap{path: path}
	var f keymapFile
	if b, err := os.ReadFile(path); err == nil {
		km.exists = true
		if err := json.Unmarshal(b, &f); err != nil {
			km.problems = append(km.problems, fmt.Sprintf("error %s: %v", filepath.Base(path), err))
		}
	}
	preset := f.Preset
	if preset == "" {
		preset = defaultPreset()
	}
	for _, d := range presetBindings(preset) {
		_ = km.bind(d[0], d[1], nil, "default") // the presets' keys are tested
	}
	if preset != "linux" && preset != "macos" && preset != "none" && preset != "darwin" {
		km.problems = append(km.problems, fmt.Sprintf("error %s: unknown preset %q: linux, macos or none", filepath.Base(path), preset))
	}
	for i, b := range f.Bindings {
		src := fmt.Sprintf("%s:bindings[%d]", filepath.Base(path), i)
		if b.Action != "none" && b.Action != "" && !knownAction(b.Action) {
			km.problems = append(km.problems, fmt.Sprintf("warning %s: unknown action %q", src, b.Action))
		}
		if err := km.bind(b.Key, b.Action, b.Args, src); err != nil {
			km.problems = append(km.problems, fmt.Sprintf("error %s: %v", src, err))
		}
	}
	return km
}

// bind binds a key, replacing what it did; action "none" or "" unbinds it.
func (km *keymap) bind(spec, action string, args map[string]any, source string) error {
	seq, err := parseKeySeq(spec)
	if err != nil {
		return err
	}
	km.bindings = slices.DeleteFunc(km.bindings, func(b keyBinding) bool { return slices.Equal(b.seq, seq) })
	if action == "" || action == "none" {
		return nil
	}
	km.bindings = append(km.bindings, keyBinding{spec: spec, seq: seq, action: action, args: args, source: source})
	return nil
}

// keysOf returns how the keymap writes the first key of an action, for
// menus and the palette: "" when none is bound.
func (km *keymap) keysOf(action string) string {
	for _, b := range km.bindings {
		if b.action == action && len(b.args) == 0 {
			return showKeys(b.seq)
		}
	}
	return ""
}

func parseKeySeq(spec string) ([]chord, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, fmt.Errorf("no key")
	}
	var seq []chord
	for _, part := range strings.Split(spec, ">") {
		c, err := parseChord(part)
		if err != nil {
			return nil, err
		}
		seq = append(seq, c)
	}
	return seq, nil
}

func parseChord(spec string) (chord, error) {
	mods, name, err := rex.ParseKey(spec)
	if err != nil {
		return chord{}, err
	}
	var c chord
	if mods&rex.ModShift != 0 {
		c.mods |= ui.Shift
	}
	if mods&rex.ModCtrl != 0 {
		c.mods |= ui.Ctrl
	}
	if mods&rex.ModAlt != 0 {
		c.mods |= ui.Alt
	}
	if mods&rex.ModSuper != 0 {
		c.mods |= ui.Super
	}
	if len(name) == 1 && name[0] >= 'A' && name[0] <= 'Z' {
		name = strings.ToLower(name)
	}
	k, ok := keyNames[name]
	switch {
	case ok:
	case len(name) == 1 && name[0] >= 'a' && name[0] <= 'z':
		k = ui.KeyA + ui.Key(name[0]-'a')
	case len(name) == 1 && name[0] >= '0' && name[0] <= '9':
		k = ui.Key0 + ui.Key(name[0]-'0')
	default:
		// A shifted character stands for its key with shift: "%" is
		// shift+5, as a US keyboard has it.
		if len(name) == 1 {
			const from, to = "!@#$%^&*()_+{}|:\"<>?~", "1234567890-=[]\\;',./`"
			if i := strings.IndexByte(from, name[0]); i >= 0 {
				if c2, err := parseChord(string(to[i])); err == nil {
					c2.mods |= c.mods | ui.Shift
					return c2, nil
				}
			}
		}
		return chord{}, fmt.Errorf("unknown key %q", name)
	}
	c.key = k
	return c, nil
}

var keyNames = map[string]ui.Key{
	"enter": ui.KeyEnter, "escape": ui.KeyEscape, "backspace": ui.KeyBackspace, "tab": ui.KeyTab,
	"space": ui.KeySpace, "delete": ui.KeyDelete, "insert": ui.KeyInsert, "home": ui.KeyHome, "end": ui.KeyEnd,
	"pageup": ui.KeyPageUp, "pagedown": ui.KeyPageDown, "left": ui.KeyLeft, "right": ui.KeyRight,
	"up": ui.KeyUp, "down": ui.KeyDown,
	"f1": ui.KeyF1, "f2": ui.KeyF2, "f3": ui.KeyF3, "f4": ui.KeyF4, "f5": ui.KeyF5, "f6": ui.KeyF6,
	"f7": ui.KeyF7, "f8": ui.KeyF8, "f9": ui.KeyF9, "f10": ui.KeyF10, "f11": ui.KeyF11, "f12": ui.KeyF12,
	"-": ui.KeyMinus, "=": ui.KeyEqual, ",": ui.KeyComma, ".": ui.KeyPeriod, "/": ui.KeySlash,
	";": ui.KeySemicolon, "'": ui.KeyQuote, "[": ui.KeyBracketLeft, "]": ui.KeyBracketRight,
	"\\": ui.KeyBackslash, "`": ui.KeyBackquote,
}

// showKeys writes a sequence as the palette shows it: macOS's symbols on
// macOS, "Ctrl+Shift+T" elsewhere.
func showKeys(seq []chord) string {
	var parts []string
	for _, c := range seq {
		parts = append(parts, c.String())
	}
	return strings.Join(parts, " ")
}

func (c chord) String() string {
	mac := runtime.GOOS == "darwin"
	var b strings.Builder
	type mod struct {
		m          ui.Modifiers
		mac, other string
	}
	order := []mod{{ui.Ctrl, "⌃", "Ctrl+"}, {ui.Alt, "⌥", "Alt+"}, {ui.Shift, "⇧", "Shift+"}, {ui.Super, "⌘", "Super+"}}
	for _, m := range order {
		if c.mods&m.m != 0 {
			if mac {
				b.WriteString(m.mac)
			} else {
				b.WriteString(m.other)
			}
		}
	}
	b.WriteString(keyLabel(c.key, mac))
	return b.String()
}

func keyLabel(k ui.Key, mac bool) string {
	switch {
	case k >= ui.KeyA && k <= ui.KeyZ:
		return string(rune('A' + k - ui.KeyA))
	case k >= ui.Key0 && k <= ui.Key9:
		return string(rune('0' + k - ui.Key0))
	case k >= ui.KeyF1 && k <= ui.KeyF12:
		return fmt.Sprintf("F%d", k-ui.KeyF1+1)
	}
	if mac {
		if s, ok := map[ui.Key]string{ui.KeyEnter: "↩", ui.KeyLeft: "←", ui.KeyRight: "→", ui.KeyUp: "↑",
			ui.KeyDown: "↓", ui.KeyTab: "⇥", ui.KeyBackspace: "⌫", ui.KeyEscape: "⎋"}[k]; ok {
			return s
		}
	}
	for name, v := range keyNames {
		if v == k {
			if len(name) == 1 {
				return name
			}
			return strings.ToUpper(name[:1]) + name[1:]
		}
	}
	return "?"
}

// defaultPreset is the keymap's defaults when the file names none.
func defaultPreset() string {
	switch {
	case runtime.GOOS == "darwin":
		return "darwin"
	case prefs.macosKeybindings():
		return "macos"
	}
	return "linux"
}

// presetBindings are the default bindings of a preset, as key and action.
// On macOS the menus' shortcuts do the rest.
func presetBindings(preset string) [][2]string {
	common := [][2]string{
		{"ctrl+tab", "client.tab.next"},
		{"ctrl+shift+tab", "client.tab.previous"},
	}
	switch preset {
	case "linux":
		b := append(common, [][2]string{
			{"ctrl+shift+t", "client.tab.new"},
			{"ctrl+shift+d", "pane.split.right"},
			{"ctrl+shift+e", "pane.split.down"},
			{"ctrl+shift+w", "pane.close"},
			{"ctrl+shift+enter", "pane.zoom"},
			{"ctrl+pagedown", "client.tab.next"},
			{"ctrl+pageup", "client.tab.previous"},
			{"ctrl+shift+pagedown", "client.tab.move.forward"},
			{"ctrl+shift+pageup", "client.tab.move.backward"},
			{"ctrl+shift+r", "client.tab.rename"},
			{"ctrl+shift+p", "client.palette.toggle"},
			{"ctrl+shift+n", "session.new"},
			{"ctrl+shift+s", "session.switch"},
			{"alt+left", "pane.focus.left"},
			{"alt+right", "pane.focus.right"},
			{"alt+up", "pane.focus.up"},
			{"alt+down", "pane.focus.down"},
			{"alt+shift+left", "pane.resize.left"},
			{"alt+shift+right", "pane.resize.right"},
			{"alt+shift+up", "pane.resize.up"},
			{"alt+shift+down", "pane.resize.down"},
			{"ctrl+shift+k", "pane.clear"},
			{"ctrl+=", "client.font.increase"},
			{"ctrl+-", "client.font.decrease"},
			{"ctrl+0", "client.font.reset"},
		}...)
		for i := 1; i <= 9; i++ {
			b = append(b, [2]string{fmt.Sprintf("ctrl+shift+%d", i), fmt.Sprintf("client.tab.goto.%d", i)})
		}
		return b
	case "macos":
		// The macOS shortcuts, with Command as Control or Super: keyboards
		// remapped to work as a Mac's send either. Control and a letter
		// then are GoRex's, not the terminal's.
		var b [][2]string
		both := func(key, action string) {
			b = append(b, [2]string{"ctrl+" + key, action}, [2]string{"super+" + key, action})
		}
		b = append(b, common...)
		both("t", "client.tab.new")
		both("shift+t", "client.tab.new")
		both("d", "pane.split.right")
		both("shift+d", "pane.split.down")
		b = append(b, [2]string{"ctrl+shift+e", "pane.split.down"})
		both("w", "pane.close")
		both("shift+w", "client.tab.close")
		both("shift+enter", "pane.zoom")
		both("p", "client.palette.toggle")
		both("shift+p", "client.palette.toggle")
		both("k", "pane.clear")
		b = append(b, [2]string{"ctrl+shift+k", "pane.clear"})
		both("shift+r", "client.tab.rename")
		both("shift+n", "session.new")
		both("shift+s", "session.switch")
		both("=", "client.font.increase")
		b = append(b, [2]string{"ctrl+shift+=", "client.font.increase"})
		both("-", "client.font.decrease")
		both("0", "client.font.reset")
		b = append(b,
			[2]string{"ctrl+pagedown", "client.tab.next"}, [2]string{"ctrl+pageup", "client.tab.previous"},
			[2]string{"super+shift+]", "client.tab.next"}, [2]string{"super+shift+[", "client.tab.previous"},
		)
		for _, d := range []string{"left", "right", "up", "down"} {
			b = append(b,
				[2]string{"ctrl+alt+" + d, "pane.focus." + d}, [2]string{"alt+" + d, "pane.focus." + d},
				[2]string{"super+alt+" + d, "pane.focus." + d},
				[2]string{"ctrl+shift+" + d, "pane.resize." + d}, [2]string{"alt+shift+" + d, "pane.resize." + d},
			)
		}
		b = append(b, [2]string{"ctrl+alt+=", "pane.balance"}, [2]string{"super+ctrl+=", "pane.balance"})
		for i := 1; i <= 9; i++ {
			a := fmt.Sprintf("client.tab.goto.%d", i)
			b = append(b, [2]string{fmt.Sprintf("ctrl+%d", i), a}, [2]string{fmt.Sprintf("ctrl+shift+%d", i), a},
				[2]string{fmt.Sprintf("super+%d", i), a})
		}
		return b
	case "darwin":
		return common
	}
	return nil
}

// keySeqTimeout is how long a sequence waits for its next key.
const keySeqTimeout = 2 * time.Second

// shortcuts runs the actions of the keys pressed, before the terminal gets
// them: a key bound alone, or the next key of a sequence begun. A key
// reaches a shortcut only when the frame before asked for it, so every
// frame asks for every first key, and, while a sequence waits, for the
// keys that go on with it: those alone, as "c" of "ctrl+b>c" must reach
// the terminal otherwise.
func (a *App) shortcuts(c *ui.Context) {
	km := keys
	if len(a.keySeq) > 0 && c.Now().Sub(a.keySeqAt) > keySeqTimeout {
		a.keySeq = nil
	}
	if len(a.keySeq) > 0 {
		if c.Shortcut(0, ui.KeyEscape) {
			a.keySeq = nil
		} else if a.nextKeys(c, km) {
			return
		}
	}
	var fired *keyBinding
	var prefix []chord
	for i := range km.bindings {
		b := &km.bindings[i]
		first := b.seq[0]
		if c.Shortcut(first.mods, first.key) && fired == nil && prefix == nil {
			if len(b.seq) == 1 {
				fired = b
			} else {
				prefix = []chord{first}
			}
		}
	}
	switch {
	case fired != nil:
		b := *fired
		a.later(c, func() { a.runBinding(b) })
	case prefix != nil:
		a.keySeq, a.keySeqAt = prefix, c.Now()
		a.askNext(c, km)
	}
}

// nextKeys takes the next key of the sequence begun, and reports whether
// the sequence still waits.
func (a *App) nextKeys(c *ui.Context, km *keymap) bool {
	n := len(a.keySeq)
	var fired *keyBinding
	var next *chord
	for i := range km.bindings {
		b := &km.bindings[i]
		if len(b.seq) <= n || !slices.Equal(b.seq[:n], a.keySeq) {
			continue
		}
		k := b.seq[n]
		if c.Shortcut(k.mods, k.key) && fired == nil && next == nil {
			if len(b.seq) == n+1 {
				fired = b
			} else {
				next = &k
			}
		}
	}
	switch {
	case fired != nil:
		a.keySeq = nil
		b := *fired
		a.later(c, func() { a.runBinding(b) })
		return false
	case next != nil:
		a.keySeq, a.keySeqAt = append(a.keySeq, *next), c.Now()
		a.askNext(c, km)
		return true
	}
	// Another key ends the sequence, as Rex has it; the keys of the
	// sequence so far, still held, do not.
	for _, k := range c.KeysDown() {
		if k != ui.KeyUnknown && !slices.ContainsFunc(a.keySeq, func(ch chord) bool { return ch.key == k }) {
			a.keySeq = nil
			return false
		}
	}
	c.After(keySeqTimeout)
	return true
}

// askNext asks for the keys that go on with the sequence begun, for the
// next frame.
func (a *App) askNext(c *ui.Context, km *keymap) {
	n := len(a.keySeq)
	for _, b := range km.bindings {
		if len(b.seq) > n && slices.Equal(b.seq[:n], a.keySeq) {
			c.Shortcut(b.seq[n].mods, b.seq[n].key)
		}
	}
	c.Shortcut(0, ui.KeyEscape)
	c.After(keySeqTimeout)
}

func (a *App) runBinding(b keyBinding) {
	if err := a.runAction(b.action, b.args); err != nil {
		a.err = fmt.Sprintf("%s (%s): %v", b.spec, b.action, err)
	}
}

// reloadKeymap reads keymap.json again. The menus' shortcuts, macOS's
// alone, do not follow it.
func (a *App) reloadKeymap() []string {
	keys = loadKeymap(keymapPath())
	return keys.problems
}

// userKeysOf is keysOf for the bindings of keymap.json alone.
func (km *keymap) userKeysOf(action string) string {
	for _, b := range km.bindings {
		if b.action == action && len(b.args) == 0 && b.source != "default" {
			return showKeys(b.seq)
		}
	}
	return ""
}

// firstChord returns the key of an action bound to one key alone.
func (km *keymap) firstChord(action string) (chord, bool) {
	for _, b := range km.bindings {
		if b.action == action && len(b.seq) == 1 && len(b.args) == 0 {
			return b.seq[0], true
		}
	}
	return chord{}, false
}

// applyKeybindingLabels reads the keymap again, as its preset follows the
// settings.
func applyKeybindingLabels() { keys = loadKeymap(keymapPath()) }
