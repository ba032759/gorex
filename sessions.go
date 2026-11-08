package main

import (
	"github.com/egoist/mygo/ui"
)

// sessionChip shows the session the window shows, and its tabs' activity
// in the other sessions; a click changes to another.
func (a *App) sessionChip(c *ui.Context, k *colors) {
	ss := a.currentSession()
	if ss == nil || a.model == nil {
		return
	}
	name := a.sessionName(ss)
	others := len(a.model.Sessions) - 1
	chip := ui.Row(c).Gap(7).Padding(3, 8, 3, 8).Radius(9).AlignItems(ui.Center).Cursor(ui.CursorPointer).
		Role(ui.RoleButton).Label("Session " + name).MaxWidth(200).Shrink(0).
		Tooltip(tip("Change Session", &cmdSwitchSession))
	if chip.Hovered() {
		chip.Background(k.hover)
	}
	if chip.Clicked() {
		a.openSessions()
	}
	// A dot tells of a pane asking for attention, or printing, in another
	// session.
	attention, busy := false, false
	for _, o := range a.model.Sessions {
		if o.ID == ss.ID {
			continue
		}
		at, b := a.sessionActivity(c, o)
		attention, busy = attention || at, busy || b
	}
	chip.Children(func() {
		ui.Icon(c, icon("layout-grid")).Size(15, 15).TextColor(k.textMuted)
		ui.Text(c, name).FontSize(13).FontWeight(600).TextColor(k.text.Alpha(0.85)).SingleLine().Ellipsis("…").MinWidth(0)
		if others > 0 {
			ui.Text(c, "+"+itoa(others)).FontSize(11.5).TextColor(k.textFaint).Shrink(0)
		}
		switch {
		case attention:
			ui.Box(c).Size(7, 7).Radius(4).Background(k.attention).Tooltip("A pane of another session needs attention")
		case busy:
			activityDot(c, k.busy, 5.5)
		}
		ui.Icon(c, icon("chevron-down")).Size(13, 13).TextColor(k.textFaint)
	})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}
