package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gorex/internal/rex"
)

// paletteMode is what the palette lists.
type paletteMode int

const (
	// paletteAll lists the sessions, the panes and the commands.
	paletteAll paletteMode = iota
	// paletteSessions lists the sessions, to change to one.
	paletteSessions
	// palettePrompt asks for a line of text.
	palettePrompt
)

// prompt is a question the palette asks.
type prompt struct {
	title, hint string
	done        func(string)
}

func (a *App) openPalette() {
	a.paletteOpen, a.paletteMode, a.paletteQuery, a.paletteSel = true, paletteAll, "", 0
	a.focusReq = nil // the palette takes the focus
}

func (a *App) togglePalette() {
	if a.paletteOpen {
		a.paletteOpen = false
		if t := a.tab(); t != nil {
			a.focusReq = t.Focus
		}
		return
	}
	a.openPalette()
}

// openSessions opens the palette on the sessions, the next one after the
// one shown chosen, as a switcher does.
func (a *App) openSessions() {
	a.openPalette()
	a.paletteMode = paletteSessions
	if a.model == nil {
		return
	}
	n := len(a.model.Sessions)
	for i, ss := range a.model.Sessions {
		if ss.ID == a.session && n > 1 {
			a.paletteSel = (i + 1) % n
		}
	}
}

// openPrompt asks for a line of text in the palette, the field holding
// value, and gives done what the user typed.
func (a *App) openPrompt(title, hint, value string, done func(string)) {
	a.openPalette()
	a.paletteMode, a.paletteQuery = palettePrompt, value
	a.prompt = &prompt{title: title, hint: hint, done: done}
}

// paletteItem is a row of the command palette: a session, a pane to go
// to, or a command.
type paletteItem struct {
	title, detail, keys string
	glyph               string
	run                 func()
}

func (a *App) paletteItems() []paletteItem {
	var items []paletteItem
	q := strings.ToLower(strings.TrimSpace(a.paletteQuery))
	match := func(s string) bool { return q == "" || fuzzy(strings.ToLower(s), q) }
	if a.paletteMode == palettePrompt {
		p := a.prompt
		text := a.paletteQuery
		return []paletteItem{{title: p.title, detail: p.hint, glyph: "pencil", run: func() { p.done(strings.TrimSpace(text)) }}}
	}
	if a.model != nil && (a.paletteMode == paletteSessions || len(a.model.Sessions) > 1 || q != "") {
		for i, ss := range a.model.Sessions {
			name := a.model.SessionName(ss)
			detail := fmt.Sprintf("%d tabs", len(ss.Windows))
			if len(ss.Windows) == 1 {
				detail = "1 tab"
			}
			if ss.ID == a.session {
				detail += " · shown"
			}
			if !match("session " + name) {
				continue
			}
			id := ss.ID
			items = append(items, paletteItem{
				title: name, detail: detail, glyph: "layout-grid", keys: fmt.Sprintf("Session %d", i+1),
				run: func() { a.showSession(id) },
			})
		}
		if a.paletteMode == paletteSessions {
			if match("new session") {
				items = append(items, paletteItem{title: "New Session", glyph: "plus", keys: cmdNewSession.Keys(), run: a.newSession})
			}
			return items
		}
	}
	for ti, t := range a.tabs {
		for _, p := range t.panes() {
			name, detail := p.label()
			if !match(fmt.Sprintf("%s %s tab %d", name, detail, ti+1)) {
				continue
			}
			items = append(items, paletteItem{
				title: name, detail: detail, glyph: paneProgram(p).Glyph,
				keys: fmt.Sprintf("Tab %d", ti+1),
				run: func() {
					a.selectTab(ti)
					a.setFocus(p)
					a.focusReq = p
				},
			})
		}
	}
	// The panes of the other sessions, when the query asks for them.
	if a.model != nil && q != "" {
		for _, ss := range a.model.Sessions {
			if ss.ID == a.session {
				continue
			}
			sname := a.model.SessionName(ss)
			for _, w := range ss.Windows {
				for _, id := range w.Layout.Blocks() {
					p := a.panes[id]
					if p == nil {
						continue
					}
					name, detail := p.label()
					if !match(name + " " + detail + " " + sname) {
						continue
					}
					sid, bid := ss.ID, id
					items = append(items, paletteItem{
						title: name, detail: detail, glyph: paneProgram(p).Glyph, keys: sname,
						run: func() {
							a.showSession(sid)
							a.act("block.focus", rex.Args{Block: bid})
						},
					})
				}
			}
		}
	}
	for _, cmd := range paletteCommands {
		if cmd.Hidden || !match(cmd.Title) {
			continue
		}
		items = append(items, paletteItem{title: cmd.Title, keys: cmd.Keys(), glyph: "command", run: func() { cmd.Run(a) }})
	}
	return items
}

// fuzzy reports whether the letters of q appear in s, in order.
func fuzzy(s, q string) bool {
	if strings.Contains(s, q) {
		return true
	}
	i := 0
	for _, r := range s {
		if i < len(q) && byte(r) == q[i] {
			i++
		}
	}
	return i == len(q)
}

// palette draws the command palette, over the window.
func (a *App) palette(c *ui.Context, k *colors) {
	if !a.paletteOpen {
		return
	}
	items := a.paletteItems()
	a.paletteSel = min(max(a.paletteSel, 0), max(len(items)-1, 0))
	was := a.paletteOpen
	// reveal scrolls the selection into view in the frame the keys or the
	// query move it, and only then, so that the wheel scrolls the list.
	reveal := false
	placeholder := "Type a command, or the name of a pane…"
	switch a.paletteMode {
	case paletteSessions:
		placeholder = "Change to a session…"
	case palettePrompt:
		placeholder = a.prompt.title
	}
	ui.DialogBase(c, &a.paletteOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(k.backdrop).Justify(ui.Start).Padding(78, 0, 0, 0)
		panel.Width(560).MaxHeight(440).Radius(16).Background(k.panel).Border(1, k.panelBorder).
			Shadow(0, 24, 60, -8, ui.RGBA(0, 0, 0, 0.28)).Shadow(0, 2, 6, 0, ui.RGBA(0, 0, 0, 0.06)).Clip()
		ui.Row(c).Padding(12, 16).Gap(10).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(k.panelBorder).Children(func() {
			glyph := "search"
			if a.paletteMode == palettePrompt {
				glyph = "pencil"
			}
			ui.Icon(c, icon(glyph)).Size(17, 17).TextColor(k.textFaint)
			in := ui.TextInputBase(c, &a.paletteQuery).Grow(1).FontSize(15).Placeholder(placeholder).AutoFocus()
			if in.Changed() {
				a.paletteSel, reveal = 0, true
			}
			if in.Shortcut(0, ui.KeyDown) {
				a.paletteSel, reveal = min(a.paletteSel+1, len(items)-1), true
			}
			if in.Shortcut(0, ui.KeyUp) {
				a.paletteSel, reveal = max(a.paletteSel-1, 0), true
			}
			if in.Submitted() && len(items) > 0 {
				a.paletteOpen = false
				items[a.paletteSel].run()
			}
		})
		listH := float32(min(max(len(items), 1)*34+12, 384))
		ui.Scroll(c).Height(listH).Padding(6).Children(func() {
			if len(items) == 0 {
				ui.Text(c, "No matches").FontSize(13).TextColor(k.textFaint).Padding(14)
			}
			for i, it := range items {
				row := ui.Row(c.Key(i)).Height(34).Padding(0, 10).Gap(10).Radius(8).AlignItems(ui.Center).Cursor(ui.CursorPointer)
				if i == a.paletteSel {
					row.Background(k.panelSel)
					if reveal {
						row.ScrollIntoView()
					}
				} else if row.Hovered() {
					row.Background(k.hover)
				}
				if row.Clicked() {
					a.paletteOpen = false
					it.run()
				}
				row.Children(func() {
					ui.Icon(c, icon(it.glyph)).Size(15, 15).TextColor(k.textMuted)
					ui.Text(c, it.title).FontSize(13.5).FontWeight(500).TextColor(k.text).SingleLine().Ellipsis("…").Shrink(0.3).MinWidth(0)
					if it.detail != "" {
						ui.Text(c, it.detail).FontSize(13).TextColor(k.textFaint).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
					}
					ui.Spacer(c)
					if it.keys != "" {
						ui.Text(c, it.keys).FontSize(12).TextColor(k.textFaint).Padding(2, 7).Radius(6).Background(k.hover).Shrink(0)
					}
				})
			}
		})
	})
	if was && !a.paletteOpen {
		a.prompt = nil
		if t := a.tab(); t != nil && a.focusReq == nil {
			a.focusReq = t.Focus
		}
	}
}

// reloadKeymapAndTell reads the keymap again and shows what is wrong with
// it.
func (a *App) reloadKeymapAndTell() {
	if problems := a.reloadKeymap(); len(problems) > 0 {
		a.err = "keymap.json: " + strings.Join(problems, "; ")
	}
}

// openKeymapFile opens keymap.json, writing an example first when there
// is none.
func (a *App) openKeymapFile() {
	path := keymapPath()
	if _, err := os.Stat(path); err != nil {
		os.MkdirAll(rex.Dir(), 0o700)
		if err := os.WriteFile(path, []byte(keymapExample), 0o644); err != nil {
			a.err = "Could not create " + path + ": " + err.Error()
			return
		}
	}
	if err := mygo.Shell.OpenPath(path); err != nil {
		a.err = err.Error()
	}
}

// keymapExample is the keymap.json "Open Keymap File" starts.
const keymapExample = `{
  "bindings": [
    {"key": "ctrl+b>c", "action": "client.tab.new"},
    {"key": "ctrl+b>%", "action": "pane.split", "args": {"direction": "right"}},
    {"key": "ctrl+b>\"", "action": "pane.split", "args": {"direction": "down"}},
    {"key": "ctrl+b>z", "action": "pane.zoom"}
  ]
}
`

// setupAgents installs Rex's skill and hooks into the agents found, as
// "rex skill setup" does.
func (a *App) setupAgents() {
	report, err := skillSetup(skillOptions{})
	if err != nil {
		a.err = err.Error()
		return
	}
	mygo.NewNotification(mygo.NotificationOptions{Title: "Rex for Agents", Body: report.summary()}).Show()
}
