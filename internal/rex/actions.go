//go:build darwin || linux || windows

package rex

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

var errUnknownAction = errors.New("unknown action")

// startBlock starts a block's program, the block going nowhere yet. The
// lock is held.
func (s *Server) startBlock(id string, o CreateOptions, session string) (*block, error) {
	b, err := newBlock(id, o, blockEnv{block: id, session: session})
	if err != nil {
		return nil, err
	}
	// The emulator tells of bells and titles as it takes the block's
	// output, its lock held: the server hears of them aside, as its lock
	// comes before the block's.
	b.onExit = func(code int) { s.blockExited(id, code) }
	b.onBell = func() { go s.signal(id, "bell", nil) }
	b.onTitle = func(t string) { go s.signal(id, "title_changed", map[string]any{"title": t}) }
	b.onNotify = func(title, body string) {
		go s.signal(id, "desktop_notification", map[string]any{"title": title, "body": body})
	}
	b.onStatus = func(changed *StatusRecord, removed []string, reason string) {
		if changed != nil {
			go s.signal(id, "program_status_changed", map[string]any{"id": changed.ID, "record": *changed, "reason": reason})
		}
		if len(removed) > 0 {
			go s.signal(id, "program_status_removed", map[string]any{"ids": removed, "reason": reason})
		}
	}
	s.blocks[id] = b
	b.start()
	return b, nil
}

// signal is an event of a block's terminal.
func (s *Server) signal(id, name string, payload map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ss, _ := s.model.blockPlace(id); ss != nil {
		s.emitLocked(blockEvent(ss.ID, id, name, payload))
	}
}

// blockExited closes the block whose program ended, unless it is kept
// open, keeping its exit code for "rex wait".
func (s *Server) blockExited(id string, code int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := s.blocks[id]
	if s.stopping || b == nil {
		return
	}
	s.tombs[id] = tomb{code: code, at: time.Now()}
	ss, _ := s.model.blockPlace(id)
	if ss != nil {
		payload := map[string]any{"exit_code": code}
		if code != 0 {
			payload["error"] = fmt.Sprintf("exit status %d", code)
		}
		s.emitLocked(blockEvent(ss.ID, id, "child_exited", payload))
	}
	if b.keepOpen {
		if ss != nil {
			s.changed(ss.ID)
		}
		return
	}
	s.closeBlock(id)
}

// closeBlock takes a block out of its window and ends it: a window left
// with none closes, and a session left with no window ends. The lock is
// held.
func (s *Server) closeBlock(id string) {
	ss, w := s.model.blockPlace(id)
	if b := s.blocks[id]; b != nil {
		delete(s.blocks, id)
		if t, ok := s.tombs[id]; !ok || t.at.IsZero() {
			b.mu.Lock()
			if b.exited {
				s.tombs[id] = tomb{code: b.code, at: time.Now()}
			}
			b.mu.Unlock()
		}
		go b.kill()
	}
	if w == nil {
		return
	}
	w.Layout = w.Layout.remove(id)
	w.forget(id)
	s.emitLocked(sessionEvent(ss.ID, "block_closed", map[string]any{"block_id": id}))
	if w.Layout == nil {
		s.closeWindow(ss, w)
		return
	}
	s.changed(ss.ID)
}

// closeWindow closes a window and ends its blocks; a session left with no
// window ends. The lock is held.
func (s *Server) closeWindow(ss *Session, w *Window) {
	for _, id := range w.Layout.Blocks() {
		if b := s.blocks[id]; b != nil {
			delete(s.blocks, id)
			go b.kill()
		}
		s.emitLocked(sessionEvent(ss.ID, "block_closed", map[string]any{"block_id": id}))
	}
	i := ss.windowIndex(w.ID)
	if i < 0 {
		return
	}
	ss.Windows = slices.Delete(ss.Windows, i, i+1)
	s.emitLocked(sessionEvent(ss.ID, "window_closed", map[string]any{"window_id": w.ID}))
	if len(ss.Windows) == 0 {
		s.endSession(ss)
		return
	}
	if ss.Active == w.ID {
		ss.Active = ss.Windows[max(i-1, 0)].ID
		s.emitLocked(sessionEvent(ss.ID, "active_window_changed", map[string]any{"window_id": ss.Active}))
	}
	s.changed(ss.ID)
}

// endSession ends a session and its blocks. The lock is held.
func (s *Server) endSession(ss *Session) {
	for _, w := range ss.Windows {
		for _, id := range w.Layout.Blocks() {
			if b := s.blocks[id]; b != nil {
				delete(s.blocks, id)
				go b.kill()
			}
		}
	}
	s.model.Sessions = slices.DeleteFunc(s.model.Sessions, func(o *Session) bool { return o == ss })
	s.emitLocked(Event{Type: "session_destroyed", Payload: map[string]any{"session_id": ss.ID}})
	s.changed()
}

// Targets.

func (s *Server) sessionItems() []labeled {
	items := make([]labeled, len(s.model.Sessions))
	for i, ss := range s.model.Sessions {
		items[i] = labeled{id: ss.ID, label: ss.Label, name: s.model.SessionName(ss)}
	}
	return items
}

func (s *Server) resolveSession(t string) (*Session, error) {
	i, err := match("session", t, s.sessionItems(), true)
	if err != nil {
		return nil, err
	}
	return s.model.Sessions[i], nil
}

// defaultSession is the session a request acts in: the one it names, else
// the one the client runs in, else the one the app shows, else the first.
func (s *Server) defaultSession(a *Args) (*Session, error) {
	if a.Session != "" {
		return s.resolveSession(a.Session)
	}
	if ss := s.model.session(a.CtxSession); ss != nil {
		return ss, nil
	}
	if ss, _ := s.model.blockPlace(a.CtxBlock); ss != nil {
		return ss, nil
	}
	var viewed *conn
	for c := range s.conns {
		if c.viewing != "" && s.model.session(c.viewing) != nil && (viewed == nil || c.active.After(viewed.active)) {
			viewed = c
		}
	}
	if viewed != nil {
		return s.model.session(viewed.viewing), nil
	}
	if len(s.model.Sessions) > 0 {
		return s.model.Sessions[0], nil
	}
	return nil, fmt.Errorf("%w: there are no sessions", ErrNoTarget)
}

func (s *Server) windowItems(ss *Session) []labeled {
	items := make([]labeled, len(ss.Windows))
	for i, w := range ss.Windows {
		name := w.Label
		if name == "" {
			name = fmt.Sprintf("window %d of %s", i+1, s.model.SessionName(ss))
		}
		items[i] = labeled{id: w.ID, label: w.Label, name: name}
	}
	return items
}

// resolveWindow finds a window in a session, or by its ID in any.
func (s *Server) resolveWindow(ss *Session, t string) (*Session, *Window, error) {
	i, err := match("window", t, s.windowItems(ss), true)
	if err == nil {
		return ss, ss.Windows[i], nil
	}
	if strings.HasPrefix(t, "window:") {
		if os, w := s.model.window(t); w != nil {
			return os, w, nil
		}
	}
	return nil, nil, err
}

// scopeWindow is the window a request acts in: the one it names, else the
// one the client's block is in, else its session's active one.
func (s *Server) scopeWindow(a *Args) (*Session, *Window, error) {
	if a.Window != "" && a.Session == "" && strings.HasPrefix(a.Window, "window:") {
		if ss, w := s.model.window(a.Window); w != nil {
			return ss, w, nil
		}
	}
	if a.Session == "" && a.Window == "" {
		if ss, w := s.model.blockPlace(a.CtxBlock); w != nil {
			return ss, w, nil
		}
	}
	ss, err := s.defaultSession(a)
	if err != nil {
		return nil, nil, err
	}
	if a.Window != "" {
		return s.resolveWindow(ss, a.Window)
	}
	if w := ss.ActiveWindow(); w != nil {
		return ss, w, nil
	}
	return nil, nil, fmt.Errorf("%w: the session has no windows", ErrNoTarget)
}

func (s *Server) blockItems(ws []*Window) []labeled {
	var items []labeled
	for _, w := range ws {
		for _, id := range w.Layout.Blocks() {
			name, label := "", ""
			if b := s.blocks[id]; b != nil {
				label, name = b.label, b.shell
			}
			if label != "" {
				name = label
			}
			items = append(items, labeled{id: id, label: label, name: name})
		}
	}
	return items
}

// resolveBlock finds a block by target: in the window or session the
// request names, else in the client's session, then anywhere.
func (s *Server) resolveBlock(a *Args, t string) (*Session, *Window, string, error) {
	var scopes [][]*Window
	switch {
	case a.Window != "":
		_, w, err := s.scopeWindow(a)
		if err != nil {
			return nil, nil, "", err
		}
		scopes = append(scopes, []*Window{w})
	case a.Session != "":
		ss, err := s.resolveSession(a.Session)
		if err != nil {
			return nil, nil, "", err
		}
		scopes = append(scopes, ss.Windows)
	default:
		if ss, err := s.defaultSession(a); err == nil {
			scopes = append(scopes, ss.Windows)
		}
		var all []*Window
		for _, ss := range s.model.Sessions {
			all = append(all, ss.Windows...)
		}
		scopes = append(scopes, all)
	}
	var last error
	for _, ws := range scopes {
		items := s.blockItems(ws)
		i, err := match("block", t, items, false)
		if err == nil {
			ss, w := s.model.blockPlace(items[i].id)
			return ss, w, items[i].id, nil
		}
		last = err
		var te *TargetError
		if errors.As(err, &te) && len(te.Matches) > 1 {
			return nil, nil, "", err
		}
	}
	return nil, nil, "", last
}

// scopeBlock is the block a request acts on: the one it names, else the
// client's own, else the focused block of the window it acts in.
func (s *Server) scopeBlock(a *Args) (*Session, *Window, string, error) {
	if a.Block != "" {
		return s.resolveBlock(a, a.Block)
	}
	if a.Session == "" && a.Window == "" {
		if ss, w := s.model.blockPlace(a.CtxBlock); w != nil {
			return ss, w, a.CtxBlock, nil
		}
	}
	ss, w, err := s.scopeWindow(a)
	if err != nil {
		return nil, nil, "", err
	}
	id := w.Focus
	if id == "" {
		if bs := w.Layout.Blocks(); len(bs) > 0 {
			id = bs[0]
		}
	}
	if id == "" {
		return nil, nil, "", fmt.Errorf("%w: the window has no blocks", ErrNoTarget)
	}
	return ss, w, id, nil
}

// targetBlock returns the block a request acts on.
func (s *Server) targetBlock(a *Args) (*block, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _, id, err := s.scopeBlock(a)
	if err != nil {
		return nil, err
	}
	if b := s.blocks[id]; b != nil {
		return b, nil
	}
	return nil, &TargetError{Kind: "block", Target: id}
}

// Actions.

// act performs an action on the model.
func (s *Server) act(name string, a *Args) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch name {
	case "session.resolve":
		ss, err := s.defaultSession(a)
		if err != nil {
			return nil, err
		}
		return map[string]any{"session_id": ss.ID}, nil
	case "window.resolve":
		ss, w, err := s.scopeWindow(a)
		if err != nil {
			return nil, err
		}
		return map[string]any{"session_id": ss.ID, "window_id": w.ID}, nil
	case "block.resolve":
		ss, w, id, err := s.scopeBlock(a)
		if err != nil {
			return nil, err
		}
		return map[string]any{"session_id": ss.ID, "window_id": w.ID, "block_id": id}, nil
	case "session.new":
		return s.newSession(a)
	case "session.rename":
		ss, err := s.defaultSession(a)
		if err != nil {
			return nil, err
		}
		ss.Label = strings.TrimSpace(a.Label)
		s.emitLocked(sessionEvent(ss.ID, "session_label_changed", map[string]any{"label": ss.Label}))
		s.changed(ss.ID)
		return map[string]any{"session_id": ss.ID, "label": ss.Label}, nil
	case "session.kill":
		ss, err := s.defaultSession(a)
		if err != nil {
			return nil, err
		}
		s.endSession(ss)
		return map[string]any{"session_id": ss.ID}, nil
	case "session.move":
		ss, err := s.defaultSession(a)
		if err != nil {
			return nil, err
		}
		items := s.model.Sessions
		to, err := s.position(a, len(items), slices.Index(items, ss), func(t string) (int, error) {
			o, err := s.resolveSession(t)
			return slices.Index(items, o), err
		})
		if err != nil {
			return nil, err
		}
		s.model.Sessions = moveItem(items, slices.Index(items, ss), to)
		s.changed(ss.ID)
		return map[string]any{"session_id": ss.ID, "index": slices.Index(s.model.Sessions, ss) + 1}, nil

	case "window.new":
		ss, err := s.defaultSession(a)
		if err != nil {
			return nil, err
		}
		return s.newWindow(ss, a)
	case "window.close":
		ss, w, err := s.scopeWindow(a)
		if err != nil {
			return nil, err
		}
		s.closeWindow(ss, w)
		return map[string]any{"window_id": w.ID}, nil
	case "window.rename":
		ss, w, err := s.scopeWindow(a)
		if err != nil {
			return nil, err
		}
		w.Label = strings.TrimSpace(a.Label)
		s.emitLocked(sessionEvent(ss.ID, "window_label_changed", map[string]any{"window_id": w.ID, "label": w.Label}))
		s.changed(ss.ID)
		return map[string]any{"window_id": w.ID, "label": w.Label}, nil
	case "window.focus":
		var ss *Session
		var w *Window
		var err error
		switch strings.ToLower(a.Target) {
		case "next", "prev", "previous":
			ss, w, err = s.scopeWindow(a)
			if err != nil {
				return nil, err
			}
			d := 1
			if a.Target != "next" {
				d = -1
			}
			i, n := ss.windowIndex(w.ID)+d, len(ss.Windows)
			if a.Wrap {
				i = (i%n + n) % n
			}
			if i < 0 || i >= n {
				return map[string]any{"window_id": w.ID}, nil
			}
			w = ss.Windows[i]
		default:
			if a.Target != "" {
				a2 := *a
				a2.Window = a.Target
				ss, w, err = s.scopeWindow(&a2)
			} else {
				ss, w, err = s.scopeWindow(a)
			}
			if err != nil {
				return nil, err
			}
		}
		s.activate(ss, w)
		return map[string]any{"window_id": w.ID, "session_id": ss.ID}, nil
	case "window.move":
		ss, w, err := s.scopeWindow(a)
		if err != nil {
			return nil, err
		}
		from := ss.windowIndex(w.ID)
		to, err := s.position(a, len(ss.Windows), from, func(t string) (int, error) {
			_, o, err := s.resolveWindow(ss, t)
			if err != nil {
				return 0, err
			}
			return ss.windowIndex(o.ID), nil
		})
		if err != nil {
			return nil, err
		}
		ss.Windows = moveItem(ss.Windows, from, to)
		s.changed(ss.ID)
		return map[string]any{"window_id": w.ID, "index": ss.windowIndex(w.ID) + 1}, nil
	case "window.balance":
		ss, w, err := s.scopeWindow(a)
		if err != nil {
			return nil, err
		}
		w.Layout.balance()
		s.changed(ss.ID)
		return map[string]any{"window_id": w.ID}, nil

	case "block.split":
		return s.split(a)
	case "block.close":
		_, _, id, err := s.scopeBlock(a)
		if err != nil {
			return nil, err
		}
		s.closeBlock(id)
		return map[string]any{"block_id": id}, nil
	case "block.rename":
		ss, _, id, err := s.scopeBlock(a)
		if err != nil {
			return nil, err
		}
		if b := s.blocks[id]; b != nil {
			b.label = strings.TrimSpace(a.Label)
		}
		s.changed(ss.ID)
		return map[string]any{"block_id": id, "label": strings.TrimSpace(a.Label)}, nil
	case "block.focus":
		return s.focusBlock(a)
	case "focus":
		return s.focus(a)
	case "block.zoom":
		ss, w, id, err := s.scopeBlock(a)
		if err != nil {
			return nil, err
		}
		on := w.Zoom != id
		switch a.Zoom {
		case "on":
			on = true
		case "off":
			on = false
		}
		if on {
			w.Zoom = id
			w.setFocus(id)
		} else if w.Zoom == id || a.Zoom == "" {
			w.Zoom = ""
		}
		s.changed(ss.ID)
		return map[string]any{"block_id": id, "zoomed": w.Zoom == id}, nil
	case "block.move":
		return s.move(a)
	case "block.swap":
		return s.swap(a)
	case "block.resize":
		ss, w, id, err := s.scopeBlock(a)
		if err != nil {
			return nil, err
		}
		dir, ok := ParseDirection(a.Direction)
		if !ok {
			return nil, usageError("direction must be left, right, up or down")
		}
		amount := a.Amount
		if amount <= 0 {
			amount = 5
		}
		if !w.Layout.resize(id, dir, amount/100) {
			return nil, usageError("no divider to move that way")
		}
		s.changed(ss.ID)
		return map[string]any{"block_id": id}, nil
	case "block.restart":
		return s.restart(a)
	case "block.break":
		// The block leaves its window for a new one, after it.
		ss, w, id, err := s.scopeBlock(a)
		if err != nil {
			return nil, err
		}
		if len(w.Layout.Blocks()) == 1 {
			return map[string]any{"block_id": id, "window_id": w.ID}, nil
		}
		w.Layout = w.Layout.remove(id)
		w.forget(id)
		nw := &Window{ID: newID("window"), Layout: &Layout{Block: id}}
		nw.setFocus(id)
		ss.Windows = slices.Insert(ss.Windows, ss.windowIndex(w.ID)+1, nw)
		s.emitLocked(sessionEvent(ss.ID, "window_created", map[string]any{"window_id": nw.ID}))
		if !a.NoFocus {
			s.activate(ss, nw)
		}
		s.changed(ss.ID)
		return map[string]any{"block_id": id, "window_id": nw.ID}, nil
	case "split.ratio":
		for _, ss := range s.model.Sessions {
			for _, w := range ss.Windows {
				if sp := w.Layout.findSplit(a.Split); sp != nil {
					sp.Ratio = ClampRatio(a.Ratio)
					s.changed(ss.ID)
					return map[string]any{"split_id": a.Split}, nil
				}
			}
		}
		return nil, &TargetError{Kind: "split", Target: a.Split}
	}
	return nil, errUnknownAction
}

// position is where a move puts an item: before or after another, or at
// an index from one; an index past the end is the last place.
func (s *Server) position(a *Args, n, from int, find func(string) (int, error)) (int, error) {
	switch {
	case a.Before != "":
		i, err := find(a.Before)
		if err != nil {
			return 0, err
		}
		if from < i {
			i--
		}
		return i, nil
	case a.After != "":
		i, err := find(a.After)
		if err != nil {
			return 0, err
		}
		if from > i {
			i++
		}
		return i, nil
	case a.Index > 0:
		return min(a.Index-1, n-1), nil
	}
	return 0, usageError("one of --before, --after or --index is needed")
}

// moveItem moves the item at from to to.
func moveItem[T any](items []T, from, to int) []T {
	if from < 0 || from == to {
		return items
	}
	it := items[from]
	items = slices.Delete(items, from, from+1)
	return slices.Insert(items, min(to, len(items)), it)
}

// activate makes w its session's active window.
func (s *Server) activate(ss *Session, w *Window) {
	if ss.Active != w.ID {
		ss.Active = w.ID
		s.emitLocked(sessionEvent(ss.ID, "active_window_changed", map[string]any{"window_id": w.ID}))
	}
	s.changed(ss.ID)
}

// createOptions are the options of a new block, sized and placed as the
// block it goes beside.
func (s *Server) createOptions(a *Args, beside string) CreateOptions {
	o := CreateOptions{}
	if a.Create != nil {
		o = *a.Create
	}
	if b := s.blocks[beside]; b != nil {
		if o.Dir == "" {
			o.Dir = b.info().Dir
		}
		if o.Cols <= 0 || o.Rows <= 0 {
			b.mu.Lock()
			o.Cols, o.Rows = b.cols, b.rows
			b.mu.Unlock()
		}
	}
	return o
}

func (s *Server) newSession(a *Args) (any, error) {
	ss := &Session{ID: newID("session"), Label: strings.TrimSpace(a.Label), Created: time.Now()}
	o := s.createOptions(&Args{Create: a.Create}, "")
	id := newID("block")
	if _, err := s.startBlock(id, o, ss.ID); err != nil {
		return nil, err
	}
	w := &Window{ID: newID("window"), Label: a.WindowLabel, Layout: &Layout{Block: id}}
	w.setFocus(id)
	ss.Windows, ss.Active = []*Window{w}, w.ID
	s.model.Sessions = append(s.model.Sessions, ss)
	s.emitLocked(Event{Type: "session_created", SessionID: ss.ID, Payload: map[string]any{
		"session": map[string]any{"session_id": ss.ID, "label": ss.Label}}})
	s.emitLocked(sessionEvent(ss.ID, "window_created", map[string]any{"window_id": w.ID, "label": w.Label}))
	s.emitLocked(sessionEvent(ss.ID, "block_created", map[string]any{"block_id": id}))
	s.changed(ss.ID)
	return map[string]any{"session_id": ss.ID, "window_id": w.ID, "block_id": id, "block_ids": []string{id},
		"label": ss.Label, "revision": s.model.Revision}, nil
}

// newWindow opens a window after the session's active one.
func (s *Server) newWindow(ss *Session, a *Args) (any, error) {
	beside := ""
	if aw := ss.ActiveWindow(); aw != nil {
		beside = aw.Focus
	}
	o := s.createOptions(a, beside)
	id := newID("block")
	if _, err := s.startBlock(id, o, ss.ID); err != nil {
		return nil, err
	}
	w := &Window{ID: newID("window"), Label: a.WindowLabel, Layout: &Layout{Block: id}}
	w.setFocus(id)
	at := len(ss.Windows)
	if i := ss.windowIndex(ss.Active); i >= 0 {
		at = i + 1
	}
	ss.Windows = slices.Insert(ss.Windows, at, w)
	s.emitLocked(sessionEvent(ss.ID, "window_created", map[string]any{"window_id": w.ID, "label": w.Label}))
	s.emitLocked(sessionEvent(ss.ID, "block_created", map[string]any{"block_id": id}))
	if !a.NoFocus || ss.ActiveWindow() == nil || ss.Active == "" {
		ss.Active = w.ID
		s.emitLocked(sessionEvent(ss.ID, "active_window_changed", map[string]any{"window_id": w.ID}))
	}
	s.changed(ss.ID)
	return map[string]any{"session_id": ss.ID, "window_id": w.ID, "block_id": id, "block_ids": []string{id},
		"revision": s.model.Revision}, nil
}

// split opens a block beside another, its anchor.
func (s *Server) split(a *Args) (any, error) {
	ss, w, anchor, err := s.scopeBlock(a)
	if err != nil {
		return nil, err
	}
	dir := Right
	if a.Direction != "" {
		d, ok := ParseDirection(a.Direction)
		if !ok {
			return nil, usageError(fmt.Sprintf("unknown direction %q: right, left, above or below", a.Direction))
		}
		dir = d
	}
	o := s.createOptions(a, anchor)
	share := a.Ratio
	if share <= 0 || share >= 1 {
		share = 0.5
	}
	share = ClampRatio(share)
	if dir == Left || dir == Right {
		o.Cols = max(int(float32(o.Cols)*share), 10)
	} else {
		o.Rows = max(int(float32(o.Rows)*share), 2)
	}
	id := newID("block")
	if _, err := s.startBlock(id, o, ss.ID); err != nil {
		return nil, err
	}
	splitID := newID("split")
	w.Layout.insert(anchor, id, dir, share, splitID)
	w.Zoom = ""
	if !a.NoFocus {
		w.setFocus(id)
		s.activate(ss, w)
	} else {
		w.Recent = append([]string{id}, w.Recent...)
	}
	s.emitLocked(sessionEvent(ss.ID, "block_created", map[string]any{"block_id": id}))
	s.changed(ss.ID)
	return map[string]any{"anchor_block_id": anchor, "block_ids": []string{id}, "block_id": id,
		"split_ids": []string{splitID}, "window_id": w.ID, "session_id": ss.ID, "revision": s.model.Revision}, nil
}

// directions are the words "focus" takes for moving the focus.
var directions = map[string][2]int{"left": {-1, 0}, "right": {1, 0}, "up": {0, -1}, "down": {0, 1}}

// focusBlock gives a block the focus, making its window active: the block
// a request names, or the one next to the request's block toward a
// direction.
func (s *Server) focusBlock(a *Args) (any, error) {
	if d := strings.ToLower(a.Direction); d != "" {
		a2 := *a
		a2.Block = ""
		ss, w, from, err := s.scopeBlock(&a2)
		if err != nil {
			return nil, err
		}
		to := from
		order := w.Layout.Blocks()
		switch d {
		case "next", "prev", "previous":
			i := slices.Index(order, from)
			if d == "next" {
				i++
			} else {
				i--
			}
			n := len(order)
			if a.Wrap {
				i = (i%n + n) % n
			}
			if i >= 0 && i < n {
				to = order[i]
			}
		default:
			v, ok := directions[d]
			if !ok {
				if dd, ok := ParseDirection(d); ok {
					v = directions[map[Direction]string{Right: "right", Left: "left", Above: "up", Below: "down"}[dd]]
				} else {
					return nil, usageError(fmt.Sprintf("unknown direction %q", a.Direction))
				}
			}
			if n := Neighbor(w.Layout.Rects(), order, from, v[0], v[1], a.Wrap); n != "" {
				to = n
			}
		}
		w.setFocus(to)
		s.activate(ss, w)
		return map[string]any{"block_id": to, "window_id": w.ID, "session_id": ss.ID}, nil
	}
	ss, w, id, err := s.scopeBlock(a)
	if err != nil {
		return nil, err
	}
	w.setFocus(id)
	s.activate(ss, w)
	return map[string]any{"block_id": id, "window_id": w.ID, "session_id": ss.ID}, nil
}

// focus is "rex focus": a direction, or a window or block by target.
func (s *Server) focus(a *Args) (any, error) {
	t := a.Target
	switch strings.ToLower(t) {
	case "left", "right", "up", "down", "next", "prev", "previous":
		a2 := *a
		a2.Direction = t
		return s.focusBlock(&a2)
	}
	if _, err := s.defaultSession(a); err != nil {
		return nil, err
	}
	a2 := *a
	a2.Window = t
	_, w, werr := s.scopeWindow(&a2)
	_, _, id, berr := s.resolveBlock(a, t)
	switch {
	case werr == nil && berr == nil && !strings.HasPrefix(t, "window:") && !strings.HasPrefix(t, "block:"):
		return nil, &TargetError{Kind: "focus", Target: t, Matches: []string{w.ID + " (window)", id + " (block)"}}
	case werr == nil && !strings.HasPrefix(t, "block:"):
		ws, _ := s.model.window(w.ID)
		s.activate(ws, w)
		return map[string]any{"window_id": w.ID, "session_id": ws.ID}, nil
	case berr == nil:
		a3 := *a
		a3.Block = id
		return s.focusBlock(&a3)
	}
	var te *TargetError
	if errors.As(berr, &te) && len(te.Matches) > 1 {
		return nil, berr
	}
	return nil, werr
}

// move moves a block next to an anchor, in the same session.
func (s *Server) move(a *Args) (any, error) {
	ss, w, id, err := s.scopeBlock(a)
	if err != nil {
		return nil, err
	}
	dir, ok := ParseDirection(a.Direction)
	if !ok {
		return nil, usageError("direction must be right, left, above or below")
	}
	as, aw, anchor, err := s.resolveBlock(&Args{Session: ss.ID}, a.Target)
	if err != nil {
		return nil, err
	}
	if as != ss {
		return nil, usageError("the block and its anchor must be in the same session")
	}
	if anchor == id {
		return nil, usageError("a block cannot move next to itself")
	}
	w.Layout = w.Layout.remove(id)
	w.forget(id)
	aw.Layout.insert(anchor, id, dir, a.Ratio, newID("split"))
	aw.Zoom = ""
	aw.setFocus(id)
	if w.Layout == nil {
		i := ss.windowIndex(w.ID)
		ss.Windows = slices.Delete(ss.Windows, i, i+1)
		s.emitLocked(sessionEvent(ss.ID, "window_closed", map[string]any{"window_id": w.ID}))
	}
	s.activate(ss, aw)
	return map[string]any{"block_id": id, "anchor_block_id": anchor, "window_id": aw.ID}, nil
}

// swap swaps the places of two blocks of a session.
func (s *Server) swap(a *Args) (any, error) {
	ss, w, id, err := s.scopeBlock(a)
	if err != nil {
		return nil, err
	}
	var os *Session
	var ow *Window
	var other string
	if a.Target == "" && a.Direction != "" {
		// The block next to it, toward the direction.
		v, ok := directions[strings.ToLower(a.Direction)]
		if dd, ok2 := ParseDirection(a.Direction); !ok && ok2 {
			v, ok = directions[map[Direction]string{Right: "right", Left: "left", Above: "up", Below: "down"}[dd]]
		}
		if !ok {
			return nil, usageError(fmt.Sprintf("unknown direction %q", a.Direction))
		}
		other = Neighbor(w.Layout.Rects(), w.Layout.Blocks(), id, v[0], v[1], false)
		if other == "" {
			return map[string]any{"block_ids": []string{id}}, nil
		}
		os, ow = ss, w
	} else {
		var err error
		if os, ow, other, err = s.resolveBlock(&Args{Session: ss.ID}, a.Target); err != nil {
			return nil, err
		}
	}
	if os != ss {
		return nil, usageError("the blocks must be in the same session")
	}
	if other == id {
		return map[string]any{"block_ids": []string{id, other}}, nil
	}
	l1, _ := w.Layout.find(id)
	l2, _ := ow.Layout.find(other)
	l1.Block, l2.Block = other, id
	if w != ow {
		for _, p := range []struct {
			w        *Window
			from, to string
		}{{w, id, other}, {ow, other, id}} {
			if p.w.Focus == p.from {
				p.w.Focus = p.to
			}
			if p.w.Zoom == p.from {
				p.w.Zoom = ""
			}
			for i, r := range p.w.Recent {
				if r == p.from {
					p.w.Recent[i] = p.to
				}
			}
		}
	}
	s.changed(ss.ID)
	return map[string]any{"block_ids": []string{id, other}}, nil
}

// restart ends a block's program and starts a shell in its place, in the
// directory it was in.
func (s *Server) restart(a *Args) (any, error) {
	ss, w, id, err := s.scopeBlock(a)
	if err != nil {
		return nil, err
	}
	old := s.blocks[id]
	o := s.createOptions(&Args{}, id)
	if old != nil {
		o.Label = old.label
	}
	nid := newID("block")
	if _, err := s.startBlock(nid, o, ss.ID); err != nil {
		return nil, err
	}
	leaf, _ := w.Layout.find(id)
	leaf.Block = nid
	if w.Focus == id {
		w.Focus = ""
	}
	w.forget(id)
	w.setFocus(nid)
	if old != nil {
		delete(s.blocks, id)
		go old.kill()
	}
	s.emitLocked(sessionEvent(ss.ID, "block_closed", map[string]any{"block_id": id}))
	s.emitLocked(sessionEvent(ss.ID, "block_created", map[string]any{"block_id": nid}))
	s.changed(ss.ID)
	return map[string]any{"block_id": nid, "replaced_block_id": id}, nil
}

// resize moves the nearest divider of a block on the axis of dir by
// amount, a share of the split.
func (l *Layout) resize(id string, dir Direction, amount float32) bool {
	vertical := dir == Above || dir == Below
	sign := float32(1)
	if dir == Left || dir == Above {
		sign = -1
	}
	// The path from the root to the block, nearest split last.
	var path []*Layout
	var walk func(n *Layout) bool
	walk = func(n *Layout) bool {
		if n.IsLeaf() {
			return n.Block == id
		}
		path = append(path, n)
		if walk(n.A) || walk(n.B) {
			return true
		}
		path = path[:len(path)-1]
		return false
	}
	if l == nil || !walk(l) {
		return false
	}
	for i := len(path) - 1; i >= 0; i-- {
		if p := path[i]; p.Vertical == vertical {
			p.Ratio = ClampRatio(p.Ratio + sign*amount)
			return true
		}
	}
	return false
}

// wait waits for a block's program to end, or a window or session to
// close, and returns the exit code of a block's.
func (s *Server) wait(a *Args) (any, error) {
	s.mu.Lock()
	kind, id := "", ""
	t := a.Target
	switch {
	case a.Block != "":
		t, kind = a.Block, "block"
	case t == "" && a.Window != "":
		t, kind = a.Window, "window"
	case t == "" && a.Session != "":
		t, kind = a.Session, "session"
	}
	if _, ok := s.tombs[t]; ok {
		kind, id = "block", t
	}
	if id == "" {
		var errs []error
		try := func(k string) bool {
			if kind != "" && kind != k {
				return false
			}
			var err error
			switch k {
			case "block":
				_, _, id, err = s.resolveBlock(a, t)
			case "window":
				var w *Window
				a2 := *a
				a2.Window = t
				if _, w, err = s.scopeWindow(&a2); err == nil {
					id = w.ID
				}
			case "session":
				var ss *Session
				if ss, err = s.resolveSession(t); err == nil {
					id = ss.ID
				}
			}
			if err != nil {
				errs = append(errs, err)
				return false
			}
			kind = k
			return true
		}
		if t == "" {
			kind = "block"
			_, _, id, _ = s.scopeBlock(a)
		} else if !try("block") && !try("window") && !try("session") {
			s.mu.Unlock()
			if len(errs) > 0 {
				return nil, errs[0]
			}
			return nil, &TargetError{Kind: "target", Target: t}
		}
	}
	s.mu.Unlock()
	if id == "" {
		return nil, fmt.Errorf("%w: nothing to wait for", ErrNoTarget)
	}
	for {
		s.mu.Lock()
		switch kind {
		case "block":
			if b := s.blocks[id]; b != nil {
				b.mu.Lock()
				exited, code := b.exited, b.code
				b.mu.Unlock()
				if exited {
					s.mu.Unlock()
					return map[string]any{"block_id": id, "exit_code": code}, nil
				}
			} else {
				code := 1
				if t, ok := s.tombs[id]; ok {
					code = t.code
				}
				s.mu.Unlock()
				return map[string]any{"block_id": id, "exit_code": code}, nil
			}
		case "window":
			if _, w := s.model.window(id); w == nil {
				s.mu.Unlock()
				return map[string]any{"window_id": id, "exit_code": 0}, nil
			}
		case "session":
			if s.model.session(id) == nil {
				s.mu.Unlock()
				return map[string]any{"session_id": id, "exit_code": 0}, nil
			}
		}
		s.mu.Unlock()
		select {
		case <-s.quit:
			return nil, ErrServerGone
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// appLocked finds the app a request names, as Rex's -C does: by ID, name,
// position or end of ID; without a name, the app when exactly one is
// connected. s.mu is held.
func (s *Server) appLocked(name string) (*conn, error) {
	var apps []*conn
	for c := range s.conns {
		if c.kind == "app" {
			apps = append(apps, c)
		}
	}
	slices.SortFunc(apps, func(x, y *conn) int { return x.connected.Compare(y.connected) })
	switch {
	case len(apps) == 0:
		return nil, ErrNoClient
	case name != "":
		items := make([]labeled, len(apps))
		for i, c := range apps {
			items[i] = labeled{id: c.id, label: c.name, name: c.name}
		}
		i, err := match("client", name, items, true)
		if err != nil {
			return nil, err
		}
		return apps[i], nil
	case len(apps) == 1:
		return apps[0], nil
	}
	e := &TargetError{Kind: "client", Target: "(default)"}
	for _, c := range apps {
		e.Matches = append(e.Matches, fmt.Sprintf("%-8s %s", shortID(c.id), c.name))
	}
	return nil, e
}

// clientDo asks an app to perform one of its actions, and waits for its
// answer.
func (s *Server) clientDo(a *Args) (any, error) {
	s.mu.Lock()
	target, err := s.appLocked(a.Client)
	if err != nil {
		s.mu.Unlock()
		return nil, err
	}
	s.nextAction++
	aid := s.nextAction
	ch := make(chan Args, 1)
	s.actions[aid] = ch
	s.mu.Unlock()
	ev := Event{Type: "client_action", Time: time.Now(), Payload: map[string]any{
		"action_id": aid, "name": a.Action, "params": a.Params,
		"ctx_session": a.CtxSession, "ctx_block": a.CtxBlock,
	}}
	b, _ := json.Marshal(Response{Event: &ev})
	target.send(append(b, '\n'), true)
	select {
	case r := <-ch:
		if r.Error != "" {
			return nil, errors.New(r.Error)
		}
		if len(r.Result) == 0 {
			return nil, nil
		}
		return r.Result, nil
	case <-target.done:
		return nil, errors.New("the app went away")
	case <-time.After(30 * time.Second):
		s.mu.Lock()
		delete(s.actions, aid)
		s.mu.Unlock()
		return nil, errors.New("the app did not answer")
	}
}

// watch tells subscribers when a terminal's program or directory changes:
// it looks every second, while someone listens.
func (s *Server) watch() {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-s.quit:
			return
		case <-t.C:
		}
		s.mu.Lock()
		listening := false
		for c := range s.conns {
			if c.sub != nil && (c.sub.all || c.sub.session != "") {
				listening = true
			}
		}
		var all, transient []*block
		for _, b := range s.blocks {
			if listening {
				all = append(all, b)
			}
			b.mu.Lock()
			if len(b.status) > 0 && slices.ContainsFunc(b.status.list(), func(r StatusRecord) bool {
				return r.State == "working" || r.State == "blocked"
			}) {
				transient = append(transient, b)
			}
			b.mu.Unlock()
		}
		s.mu.Unlock()
		// A shell back at its prompt ends what a program said it was
		// doing, for shells that mark no prompts (OSC 133).
		for _, b := range transient {
			if in := b.info(); in.Idle && !in.Exited {
				b.dropStatus("prompt", "working", "blocked")
			}
		}
		seen := map[string]bool{}
		for _, b := range all {
			in := b.info()
			seen[b.id] = true
			s.mu.Lock()
			was, ok := s.last[b.id]
			s.last[b.id] = in
			s.mu.Unlock()
			if !ok || in.Exited {
				continue
			}
			if was.Dir != in.Dir && in.Dir != "" {
				s.signal(b.id, "pwd_changed", map[string]any{"pwd": in.Dir})
			}
			if was.Program != in.Program {
				s.signal(b.id, "process_changed", map[string]any{"foreground": in.Program, "args": in.Args})
			}
		}
		s.mu.Lock()
		for id := range s.last {
			if !seen[id] {
				delete(s.last, id)
			}
		}
		s.mu.Unlock()
	}
}
