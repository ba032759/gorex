package main

import (
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gorex/internal/rex"
)

// What programs report of themselves (OSC 7501, or an agent's hooks
// through rex status): working, blocked on the user, done, failed. Panes
// show it in their headers, tabs and the session chip with dots, and a
// change the user is not looking at posts a notification.

// failedRed is the color of a program that failed.
var failedRed = ui.Hex("#e5484d")

// programStatus returns the record a pane shows: the one that says most,
// nil when there is none, or when it is a result the user has seen.
func (p *Pane) programStatus() *rex.StatusRecord {
	st := rex.StatusSummary(p.info.Status)
	if st == nil || st.State == "idle" {
		return nil
	}
	if (st.State == "done" || st.State == "error") && !st.Updated.After(p.statusSeen) {
		return nil
	}
	return st
}

// appTitle names the program of a record: "claude-code" is Claude Code.
func appTitle(app, fallback string) string {
	switch app {
	case "":
		if fallback != "" {
			return fallback
		}
		return "A program"
	case "claude-code", "claude":
		return "Claude Code"
	}
	if p := programOf(app); p.Name != "" {
		return p.Name
	}
	return app
}

// statusWords says what a record waits for, or what it is, in a few
// words.
func statusWords(st *rex.StatusRecord) string {
	switch st.State {
	case "blocked":
		switch st.Kind {
		case "permission":
			return "Needs permission"
		case "question":
			return "Has a question"
		case "auth":
			return "Needs a login"
		}
		return "Waiting for you"
	case "working":
		return "Working"
	case "done":
		return "Done"
	case "error":
		return "Failed"
	}
	return ""
}

// statusChip shows a pane's reported status in its header.
func (a *App) statusChip(c *ui.Context, k *colors, p *Pane) bool {
	st := p.programStatus()
	if st == nil {
		return false
	}
	col := k.busy
	switch st.State {
	case "blocked", "done":
		col = k.attention
	case "error":
		col = failedRed
	}
	words := statusWords(st)
	if st.Progress >= 0 {
		words += " " + itoa(st.Progress) + "%"
	}
	tip := words
	if st.Msg != "" {
		tip += ": " + st.Msg
	}
	ui.Row(c).Gap(5).Padding(1, 7, 1, 6).Radius(8).Background(col.Alpha(0.14)).AlignItems(ui.Center).
		MinWidth(0).Shrink(1).Tooltip(tip).Children(func() {
		if st.State == "working" {
			activityDot(c, col, 5)
		} else {
			ui.Box(c).Size(6, 6).Radius(3).Background(col).Shrink(0)
		}
		ui.Text(c, words).FontSize(11.5).FontWeight(600).TextColor(col).SingleLine().Shrink(0)
		if st.Msg != "" && st.State != "working" {
			ui.Text(c, st.Msg).FontSize(11.5).TextColor(k.text.Alpha(0.75)).SingleLine().Ellipsis("…").MinWidth(0).Shrink(1)
		}
	})
	return true
}

// statusChanged acts on a pane's status as the server tells it: a pane
// newly blocked, done or failed out of sight asks for attention, and a
// notification tells of it when the window is not in front.
func (a *App) statusChanged(p *Pane, was, now []rex.StatusRecord) {
	prev, cur := rex.StatusSummary(was), rex.StatusSummary(now)
	if cur == nil || prev != nil && prev.State == cur.State && prev.Kind == cur.Kind && prev.ID == cur.ID {
		return
	}
	if cur.State != "blocked" && cur.State != "done" && cur.State != "error" {
		return
	}
	shown := a.tab()
	seen := p.Tab != nil && p.Tab == shown && shown.Focus == p && a.focusedWin
	if seen {
		p.statusSeen = time.Now()
		return
	}
	p.attention = true
	if a.focusedWin && p.Tab == shown {
		return // in sight, the dot says enough
	}
	// One notification per pane every two seconds, however fast a
	// program changes.
	if time.Since(p.lastNotify) < 2*time.Second {
		return
	}
	p.lastNotify = time.Now()
	name, _ := p.programLabel()
	title := appTitle(cur.App, name)
	switch cur.State {
	case "blocked":
		title += " " + strings.ToLower(statusWords(cur)[:1]) + statusWords(cur)[1:]
	case "done":
		title += " finished"
	case "error":
		title += " failed"
	}
	body := cur.Msg
	if where := a.whereIs(p); where != "" {
		if body != "" {
			body += "\n"
		}
		body += where
	}
	mygo.NewNotification(mygo.NotificationOptions{Title: title, Body: body}).Show()
}

// whereIs names the session and tab of a pane, for a notification: which
// terminal it came from.
func (a *App) whereIs(p *Pane) string {
	if a.model == nil {
		return ""
	}
	for _, ss := range a.model.Sessions {
		for i, w := range ss.Windows {
			for _, id := range w.Layout.Blocks() {
				if id == p.SID {
					where := a.model.SessionName(ss) + " · tab " + itoa(i+1)
					if w.Label != "" {
						where = a.model.SessionName(ss) + " · " + w.Label
					}
					return where
				}
			}
		}
	}
	return ""
}

// blockedPane returns a pane of the tab blocked on the user.
func (t *Tab) blockedPane() *Pane {
	for _, p := range t.panes() {
		if st := p.programStatus(); st != nil && st.State == "blocked" {
			return p
		}
	}
	return nil
}
