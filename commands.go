package main

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo"

	"gorex/internal/rex"
)

// command is an action of the menus, the command palette, the keymap and
// "rex do", by the name Rex gives it.
type command struct {
	Title  string
	Action string
	Run    func(a *App)
	// Hidden keeps it out of the palette.
	Hidden bool
}

// Keys is how the palette shows the command's keys.
func (c *command) Keys() string {
	return keys.keysOf(c.Action)
}

var (
	cmdNewTab         = command{Title: "New Tab", Action: "client.tab.new", Run: func(a *App) { a.newTab(a.currentDir()) }}
	cmdSplitRight     = command{Title: "Split Right", Action: "pane.split.right", Run: func(a *App) { a.splitDir("right") }}
	cmdSplitDown      = command{Title: "Split Down", Action: "pane.split.down", Run: func(a *App) { a.splitDir("below") }}
	cmdSplitLeft      = command{Title: "Split Left", Action: "pane.split.left", Run: func(a *App) { a.splitDir("left") }}
	cmdSplitUp        = command{Title: "Split Up", Action: "pane.split.up", Run: func(a *App) { a.splitDir("above") }}
	cmdClosePane      = command{Title: "Close Pane", Action: "pane.close", Run: func(a *App) { a.closeFocused() }}
	cmdCloseOthers    = command{Title: "Close Other Panes", Action: "pane.close_others", Run: func(a *App) { a.closeOtherPanes() }}
	cmdCloseTab       = command{Title: "Close Tab", Action: "client.tab.close", Run: func(a *App) { a.closeActiveTab() }}
	cmdCloseOtherTabs = command{Title: "Close Other Tabs", Action: "client.tab.close_others", Run: func(a *App) { a.closeOtherTabs() }}
	cmdZoom           = command{Title: "Zoom Pane", Action: "pane.zoom", Run: func(a *App) { a.toggleZoom() }}
	cmdEqualize       = command{Title: "Equalize Panes", Action: "pane.balance", Run: func(a *App) { a.equalize() }}
	cmdNextTab        = command{Title: "Next Tab", Action: "client.tab.next", Run: func(a *App) { a.cycleTab(1) }}
	cmdPrevTab        = command{Title: "Previous Tab", Action: "client.tab.previous", Run: func(a *App) { a.cycleTab(-1) }}
	cmdMoveTabRight   = command{Title: "Move Tab Right", Action: "client.tab.move.forward", Run: func(a *App) { a.moveActiveTab(1) }}
	cmdMoveTabLeft    = command{Title: "Move Tab Left", Action: "client.tab.move.backward", Run: func(a *App) { a.moveActiveTab(-1) }}
	cmdRenameTab      = command{Title: "Rename Tab…", Action: "client.tab.rename", Run: func(a *App) { a.startRename() }}
	cmdPalette        = command{Title: "Command Palette…", Action: "client.palette.toggle", Run: func(a *App) { a.togglePalette() }, Hidden: true}
	cmdPalette2       = command{Title: "Go to Pane…", Action: "pane.go_to", Run: func(a *App) { a.openPalette() }, Hidden: true}
	cmdFocusLeft      = command{Title: "Focus Pane Left", Action: "pane.focus.left", Run: func(a *App) { a.moveFocus(-1, 0) }}
	cmdFocusRight     = command{Title: "Focus Pane Right", Action: "pane.focus.right", Run: func(a *App) { a.moveFocus(1, 0) }}
	cmdFocusUp        = command{Title: "Focus Pane Above", Action: "pane.focus.up", Run: func(a *App) { a.moveFocus(0, -1) }}
	cmdFocusDown      = command{Title: "Focus Pane Below", Action: "pane.focus.down", Run: func(a *App) { a.moveFocus(0, 1) }}
	cmdFocusNext      = command{Title: "Focus Next Pane", Action: "pane.focus_next", Run: func(a *App) { a.focusDir("next") }}
	cmdFocusPrev      = command{Title: "Focus Previous Pane", Action: "pane.focus_previous", Run: func(a *App) { a.focusDir("prev") }}
	cmdMoveLeft       = command{Title: "Move Pane Left", Action: "pane.move.left", Run: func(a *App) { a.swapDir("left") }}
	cmdMoveRight      = command{Title: "Move Pane Right", Action: "pane.move.right", Run: func(a *App) { a.swapDir("right") }}
	cmdMoveUp         = command{Title: "Move Pane Up", Action: "pane.move.up", Run: func(a *App) { a.swapDir("up") }}
	cmdMoveDown       = command{Title: "Move Pane Down", Action: "pane.move.down", Run: func(a *App) { a.swapDir("down") }}
	cmdGrowLeft       = command{Title: "Move Divider Left", Action: "pane.resize.left", Run: func(a *App) { a.resizeFocused(-1, 0) }}
	cmdGrowRight      = command{Title: "Move Divider Right", Action: "pane.resize.right", Run: func(a *App) { a.resizeFocused(1, 0) }}
	cmdGrowUp         = command{Title: "Move Divider Up", Action: "pane.resize.up", Run: func(a *App) { a.resizeFocused(0, -1) }}
	cmdGrowDown       = command{Title: "Move Divider Down", Action: "pane.resize.down", Run: func(a *App) { a.resizeFocused(0, 1) }}
	cmdToNewTab       = command{Title: "Move Pane to New Tab", Action: "pane.move_to_new_tab", Run: func(a *App) { a.breakPane() }}
	cmdRenamePane     = command{Title: "Rename Pane…", Action: "pane.rename", Run: func(a *App) { a.renamePane() }}
	cmdCopyID         = command{Title: "Copy Pane ID", Action: "pane.copy_id", Run: func(a *App) { a.copyPaneID() }}
	cmdClear          = command{Title: "Clear Screen and Scrollback", Action: "pane.clear", Run: func(a *App) { a.clearFocused() }, Hidden: true}
	cmdClearScroll    = command{Title: "Clear Scrollback", Action: "pane.clear_scrollback", Run: func(a *App) { a.clearFocused() }}
	cmdRestart        = command{Title: "Restart Shell in Pane", Action: "pane.restart", Run: func(a *App) { a.restartFocused() }}
	cmdBigger         = command{Title: "Bigger Text", Action: "client.font.increase", Run: func(a *App) { a.setFontSize(termFont.Size + 1) }}
	cmdSmaller        = command{Title: "Smaller Text", Action: "client.font.decrease", Run: func(a *App) { a.setFontSize(termFont.Size - 1) }}
	cmdActualSize     = command{Title: "Actual Size", Action: "client.font.reset", Run: func(a *App) { a.setFontSize(defaultFontSize) }}
	cmdLight          = command{Title: "Appearance: Light", Action: "client.appearance.light", Run: func(a *App) { a.setAppearance("light") }}
	cmdDark           = command{Title: "Appearance: Dark", Action: "client.appearance.dark", Run: func(a *App) { a.setAppearance("dark") }}
	cmdSystem         = command{Title: "Appearance: System", Action: "client.appearance.system", Run: func(a *App) { a.setAppearance("") }}
	cmdNewSession     = command{Title: "New Session", Action: "session.new", Run: func(a *App) { a.newSession() }}
	cmdSwitchSession  = command{Title: "Change Session…", Action: "session.switch", Run: func(a *App) { a.openSessions() }}
	cmdNextSession    = command{Title: "Next Session", Action: "session.next", Run: func(a *App) { a.cycleSession(1) }}
	cmdPrevSession    = command{Title: "Previous Session", Action: "session.previous", Run: func(a *App) { a.cycleSession(-1) }}
	cmdRenameSession  = command{Title: "Rename Session…", Action: "session.rename", Run: func(a *App) { a.renameSession() }}
	cmdCloseSession   = command{Title: "Close Session", Action: "session.close", Run: func(a *App) { a.closeSession() }}
	cmdReloadKeymap   = command{Title: "Reload Keymap", Action: "client.config.reload", Run: func(a *App) { a.reloadKeymapAndTell() }}
	cmdOpenKeymap     = command{Title: "Open Keymap File", Action: "client.config.open", Run: func(a *App) { a.openKeymapFile() }}
	cmdAgents         = command{Title: "Install Rex for Agents", Action: "client.agents.setup", Run: func(a *App) { a.setupAgents() }}
	cmdEndAll         = command{Title: "Quit and End All Sessions", Action: "client.quit_and_end", Run: func(a *App) { a.quitAndEnd() }}
)

// paletteCommands are those the command palette lists, after the panes,
// and commands all of them; init makes them, as their actions use them.
var paletteCommands, commands []*command

func init() {
	paletteCommands = []*command{
		&cmdNewTab, &cmdSplitRight, &cmdSplitDown, &cmdSplitLeft, &cmdSplitUp, &cmdZoom, &cmdEqualize,
		&cmdClosePane, &cmdCloseOthers, &cmdCloseTab, &cmdCloseOtherTabs,
		&cmdNextTab, &cmdPrevTab, &cmdMoveTabLeft, &cmdMoveTabRight, &cmdRenameTab,
		&cmdNewSession, &cmdSwitchSession, &cmdNextSession, &cmdPrevSession, &cmdRenameSession, &cmdCloseSession,
		&cmdFocusLeft, &cmdFocusRight, &cmdFocusUp, &cmdFocusDown, &cmdFocusNext, &cmdFocusPrev,
		&cmdMoveLeft, &cmdMoveRight, &cmdMoveUp, &cmdMoveDown,
		&cmdGrowLeft, &cmdGrowRight, &cmdGrowUp, &cmdGrowDown,
		&cmdToNewTab, &cmdRenamePane, &cmdCopyID,
		&cmdBigger, &cmdSmaller, &cmdActualSize, &cmdLight, &cmdDark, &cmdSystem,
		&cmdClearScroll, &cmdRestart, &cmdReloadKeymap, &cmdOpenKeymap, &cmdAgents, &cmdEndAll,
	}
	commands = append(slices.Clone(paletteCommands), &cmdPalette, &cmdPalette2, &cmdClear)
}

// actionInfo describes an action, for "rex actions".
type actionInfo struct {
	Name  string `json:"name"`
	Title string `json:"title"`
	Args  string `json:"args,omitempty"`
}

// argActions are the actions that take arguments, beside the commands.
var argActions = []actionInfo{
	{"pane.split", "Split Pane", "direction"},
	{"pane.focus", "Focus Pane", "direction"},
	{"pane.move", "Move Pane", "direction"},
	{"pane.resize", "Resize Pane", "direction, amount?"},
	{"pane.send_key", "Send Key", "key"},
	{"client.tab.goto", "Go to Tab", "index"},
	{"session.select", "Go to Session", "session"},
}

func commandOf(action string) *command {
	for _, c := range commands {
		if c.Action == action {
			return c
		}
	}
	return nil
}

func knownAction(name string) bool {
	if commandOf(name) != nil {
		return true
	}
	if n, ok := strings.CutPrefix(name, "client.tab.goto."); ok {
		_, err := strconv.Atoi(n)
		return err == nil
	}
	return slices.ContainsFunc(argActions, func(x actionInfo) bool { return x.Name == name })
}

// placed tells whether an action acts on a block, a window or a session,
// rather than on the app as a whole.
func placed(action string) bool {
	for _, p := range []string{"pane.", "client.tab.", "session."} {
		if strings.HasPrefix(action, p) {
			return true
		}
	}
	return false
}

// actionList lists the actions, for "rex actions".
func actionList() []actionInfo {
	var out []actionInfo
	for _, c := range commands {
		out = append(out, actionInfo{Name: c.Action, Title: c.Title})
	}
	out = append(out, argActions...)
	slices.SortFunc(out, func(x, y actionInfo) int { return strings.Compare(x.Name, y.Name) })
	return out
}

// runAction performs an action by name, with its arguments: from a key,
// or from "rex do".
func (a *App) runAction(name string, args map[string]any) error {
	str := func(k string) string {
		switch v := args[k].(type) {
		case string:
			return v
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64)
		}
		return ""
	}
	num := func(k string) (float64, bool) {
		f, err := strconv.ParseFloat(str(k), 64)
		return f, err == nil
	}
	if n, ok := strings.CutPrefix(name, "client.tab.goto."); ok {
		i, err := strconv.Atoi(n)
		if err != nil {
			return fmt.Errorf("unknown action %q", name)
		}
		a.showTab(i)
		return nil
	}
	switch name {
	case "pane.split":
		d := str("direction")
		if d == "" {
			d = "right"
		}
		a.splitDir(d)
		return nil
	case "pane.focus":
		a.focusDir(str("direction"))
		return nil
	case "pane.move":
		a.swapDir(str("direction"))
		return nil
	case "pane.resize":
		amount, _ := num("amount")
		a.resizeDir(str("direction"), float32(amount))
		return nil
	case "pane.send_key":
		if t := a.tab(); t != nil && t.Focus != nil {
			return a.client.Do("sendKey", &rex.Args{Block: t.Focus.SID, Keys: []string{str("key")}}, nil)
		}
		return nil
	case "client.tab.goto":
		i, ok := num("index")
		if !ok {
			return fmt.Errorf("client.tab.goto takes an index")
		}
		a.showTab(int(i))
		return nil
	case "session.select":
		return a.selectSession(str("session"))
	}
	cmd := commandOf(name)
	if cmd == nil {
		return fmt.Errorf("unknown action %q: \"rex actions\" lists them", name)
	}
	cmd.Run(a)
	return nil
}

// clientAction performs an action a client asked for with "rex do", on
// block, the block of the request, when the action has a place.
func (a *App) clientAction(name string, params json.RawMessage, block string) (any, error) {
	var args map[string]any
	if len(params) > 0 && string(params) != "null" {
		if err := json.Unmarshal(params, &args); err != nil {
			return nil, fmt.Errorf("arguments: %w", err)
		}
	}
	if name == "client.actions" {
		return actionList(), nil
	}
	// An action of a block, a window or a session acts on the block of
	// the request, else where the focus is.
	if placed(name) {
		a.showBlock(block)
	}
	if err := a.runAction(name, args); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "action": name}, nil
}

// menu builds the menu bar, whose items run commands in the window.
func (a *App) menu() *mygo.Menu {
	item := func(cmd *command) *mygo.MenuItem {
		return &mygo.MenuItem{Label: cmd.Title, Click: func(*mygo.MenuItem, *mygo.Window) {
			a.run(cmd)
		}}
	}
	tabItems := []*mygo.MenuItem{item(&cmdNextTab), item(&cmdPrevTab), mygo.Separator()}
	for i := 1; i <= 9; i++ {
		tabItems = append(tabItems, &mygo.MenuItem{
			Label: fmt.Sprintf("Show Tab %d", i),
			Click: func(*mygo.MenuItem, *mygo.Window) {
				a.do(func() { a.showTab(i) })
			},
		})
	}
	appearance := func(label, v string) *mygo.MenuItem {
		return &mygo.MenuItem{Label: label, Type: mygo.MenuItemRadio, Checked: prefs.Appearance == v, Click: func(*mygo.MenuItem, *mygo.Window) {
			a.setAppearance(v)
		}}
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{Label: "Shell", Submenu: []*mygo.MenuItem{
			item(&cmdNewTab),
			item(&cmdNewSession),
			mygo.Separator(),
			item(&cmdSplitRight),
			item(&cmdSplitDown),
			mygo.Separator(),
			item(&cmdClosePane),
			item(&cmdCloseTab),
			item(&cmdCloseSession),
			mygo.Separator(),
			item(&cmdRestart),
			item(&cmdAgents),
			item(&cmdEndAll),
		}},
		{Label: "Edit", Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleUndo},
			{Role: mygo.RoleRedo},
			mygo.Separator(),
			{Role: mygo.RoleCut},
			{Role: mygo.RoleCopy},
			{Role: mygo.RolePaste},
			{Role: mygo.RoleSelectAll},
			mygo.Separator(),
			item(&cmdClear),
			item(&cmdCopyID),
		}},
		{Label: "View", Submenu: []*mygo.MenuItem{
			item(&cmdPalette),
			item(&cmdPalette2),
			mygo.Separator(),
			item(&cmdZoom),
			item(&cmdEqualize),
			mygo.Separator(),
			{Label: "Focus", Submenu: []*mygo.MenuItem{item(&cmdFocusLeft), item(&cmdFocusRight), item(&cmdFocusUp), item(&cmdFocusDown), item(&cmdFocusNext), item(&cmdFocusPrev)}},
			{Label: "Move", Submenu: []*mygo.MenuItem{item(&cmdMoveLeft), item(&cmdMoveRight), item(&cmdMoveUp), item(&cmdMoveDown), mygo.Separator(), item(&cmdToNewTab)}},
			{Label: "Resize", Submenu: []*mygo.MenuItem{item(&cmdGrowLeft), item(&cmdGrowRight), item(&cmdGrowUp), item(&cmdGrowDown)}},
			mygo.Separator(),
			item(&cmdBigger),
			item(&cmdSmaller),
			item(&cmdActualSize),
			{Label: "Appearance", Submenu: []*mygo.MenuItem{
				appearance("System", ""),
				appearance("Light", "light"),
				appearance("Dark", "dark"),
			}},
			mygo.Separator(),
			item(&cmdReloadKeymap),
			item(&cmdOpenKeymap),
			mygo.Separator(),
			{Role: mygo.RoleToggleFullScreen},
			{Role: mygo.RoleToggleDevTools, Label: "Inspector", Hidden: !mygo.IsDev()},
		}},
		{Label: "Sessions", Submenu: []*mygo.MenuItem{
			item(&cmdSwitchSession), item(&cmdNextSession), item(&cmdPrevSession), mygo.Separator(),
			item(&cmdNewSession), item(&cmdRenameSession), item(&cmdCloseSession),
		}},
		{Label: "Tabs", Submenu: append(tabItems, mygo.Separator(), item(&cmdRenameTab), item(&cmdMoveTabLeft), item(&cmdMoveTabRight))},
		{Role: mygo.RoleWindowMenu},
	})
}

// do changes the state on the main thread, from a menu, and draws.
func (a *App) do(fn func()) {
	if a.win == nil {
		return
	}
	a.win.Update(fn)
}

// run runs a command of the menus, on the main thread. New Tab opens the
// window again when it was closed.
func (a *App) run(cmd *command) {
	if a.win == nil {
		if cmd == &cmdNewTab {
			mygo.RunOnMain(a.open)
		}
		return
	}
	a.do(func() {
		if a.win != nil {
			cmd.Run(a)
		}
	})
}
