package rex

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// The model is what the server keeps of the user's work, after Rex's
// taxonomy: sessions, each a group of related work; windows, the tabs of a
// session, each a layout of blocks; and blocks, each a terminal. Every
// change bumps Revision, and clients mirror the model rather than keep
// layouts of their own, so that the CLI and the app see the same thing.

// Model is the state of every session, as clients see it.
type Model struct {
	Revision uint64     `json:"revision"`
	Sessions []*Session `json:"sessions"`
	// Blocks describes every block of every session, by its ID.
	Blocks map[string]*BlockMeta `json:"blocks"`
}

// Session is a group of windows.
type Session struct {
	ID      string    `json:"session_id"`
	Label   string    `json:"label,omitempty"`
	Created time.Time `json:"created"`
	Windows []*Window `json:"windows"`
	// Active is the ID of the window the session shows.
	Active string `json:"active_window_id,omitempty"`
}

// Window is a tab of a session: a layout of blocks.
type Window struct {
	ID     string  `json:"window_id"`
	Label  string  `json:"label,omitempty"`
	Layout *Layout `json:"layout"`
	// Focus is the block with the focus, and Zoom the block that fills
	// the window, when one does.
	Focus string `json:"focused_block_id,omitempty"`
	Zoom  string `json:"zoomed_block_id,omitempty"`
	// Recent lists the blocks by when they had the focus last, the
	// latest last: closing a block gives the focus back to the one before.
	Recent []string `json:"recent,omitempty"`
}

// Layout is a block, or a split of two layouts.
type Layout struct {
	Block string `json:"block_id,omitempty"`
	Split string `json:"split_id,omitempty"`
	// Vertical stacks A above B; otherwise A is left of B. Ratio is A's
	// share of the room.
	Vertical bool    `json:"vertical,omitempty"`
	Ratio    float32 `json:"ratio,omitempty"`
	A        *Layout `json:"a,omitempty"`
	B        *Layout `json:"b,omitempty"`
}

// BlockMeta is what the model says of a block; BlockInfo adds what runs in
// it now.
type BlockMeta struct {
	ID       string    `json:"block_id"`
	Label    string    `json:"label,omitempty"`
	Session  string    `json:"session_id"`
	Window   string    `json:"window_id"`
	Shell    string    `json:"shell"`
	Command  []string  `json:"command,omitempty"`
	PID      int       `json:"pid"`
	Cols     int       `json:"cols"`
	Rows     int       `json:"rows"`
	KeepOpen bool      `json:"keep_open,omitempty"`
	Exited   bool      `json:"exited,omitempty"`
	ExitCode int       `json:"exit_code,omitempty"`
	Created  time.Time `json:"created"`
	// Restored tells a block whose program the server started again, in
	// the directory it had, as it started after one that ended.
	Restored bool `json:"restored,omitempty"`
}

// IsLeaf reports whether the layout is a block.
func (l *Layout) IsLeaf() bool { return l != nil && l.A == nil && l.B == nil }

// Blocks returns the blocks of a layout in order, left to right and top to
// bottom.
func (l *Layout) Blocks() []string {
	if l == nil {
		return nil
	}
	if l.IsLeaf() {
		return []string{l.Block}
	}
	return append(l.A.Blocks(), l.B.Blocks()...)
}

// find returns the leaf of block id and its parent.
func (l *Layout) find(id string) (leaf, parent *Layout) {
	if l == nil {
		return nil, nil
	}
	if l.IsLeaf() {
		if l.Block == id {
			return l, nil
		}
		return nil, nil
	}
	for _, c := range []*Layout{l.A, l.B} {
		if c.IsLeaf() && c.Block == id {
			return c, l
		}
		if leaf, parent := c.find(id); leaf != nil {
			return leaf, parent
		}
	}
	return nil, nil
}

// findSplit returns the split of the ID.
func (l *Layout) findSplit(id string) *Layout {
	if l == nil || l.IsLeaf() {
		return nil
	}
	if l.Split == id {
		return l
	}
	if s := l.A.findSplit(id); s != nil {
		return s
	}
	return l.B.findSplit(id)
}

// Clone returns a deep copy.
func (l *Layout) Clone() *Layout {
	if l == nil {
		return nil
	}
	c := *l
	c.A, c.B = l.A.Clone(), l.B.Clone()
	return &c
}

// Direction is where a block goes next to another.
type Direction string

const (
	Right Direction = "right"
	Left  Direction = "left"
	Above Direction = "above"
	Below Direction = "below"
)

// ParseDirection takes Rex's words for the sides of a block, and the
// synonyms people type.
func ParseDirection(s string) (Direction, bool) {
	switch strings.ToLower(s) {
	case "right", "r", "beside", "horizontal":
		return Right, true
	case "left", "l":
		return Left, true
	case "above", "up", "top", "u":
		return Above, true
	case "below", "down", "bottom", "d", "vertical":
		return Below, true
	}
	return "", false
}

// insert puts block id next to the leaf of anchor, toward dir, the block
// taking share of the room (0.5 when out of range). It returns false when
// anchor is not in the layout.
func (l *Layout) insert(anchor, id string, dir Direction, share float32, splitID string) bool {
	leaf, _ := l.find(anchor)
	if leaf == nil {
		return false
	}
	if share <= 0 || share >= 1 {
		share = 0.5
	}
	old := &Layout{Block: leaf.Block}
	nw := &Layout{Block: id}
	leaf.Block, leaf.Split = "", splitID
	leaf.Vertical = dir == Above || dir == Below
	if dir == Left || dir == Above {
		leaf.A, leaf.B, leaf.Ratio = nw, old, share
	} else {
		leaf.A, leaf.B, leaf.Ratio = old, nw, 1-share
	}
	return true
}

// remove takes block id out of the layout, the other side of its split
// taking the split's place. It returns the layout that is left, nil when
// the block was all of it.
func (l *Layout) remove(id string) *Layout {
	if l.IsLeaf() {
		if l.Block == id {
			return nil
		}
		return l
	}
	leaf, parent := l.find(id)
	if leaf == nil {
		return l
	}
	other := parent.A
	if other == leaf {
		other = parent.B
	}
	*parent = *other
	return l
}

// balance gives every split equal shares, by the blocks on either side.
func (l *Layout) balance() {
	var walk func(n *Layout) (cols, rows int)
	walk = func(n *Layout) (int, int) {
		if n.IsLeaf() {
			return 1, 1
		}
		ac, ar := walk(n.A)
		bc, br := walk(n.B)
		if n.Vertical {
			n.Ratio = float32(ar) / float32(ar+br)
			return max(ac, bc), ar + br
		}
		n.Ratio = float32(ac) / float32(ac+bc)
		return ac + bc, max(ar, br)
	}
	if l != nil {
		walk(l)
	}
}

// Rect is a block's place in its window, in shares of the window: from 0
// to 1 on both axes.
type Rect struct{ X, Y, W, H float32 }

// Rects returns where each block of the layout is.
func (l *Layout) Rects() map[string]Rect {
	out := map[string]Rect{}
	var walk func(n *Layout, r Rect)
	walk = func(n *Layout, r Rect) {
		if n == nil {
			return
		}
		if n.IsLeaf() {
			out[n.Block] = r
			return
		}
		a, b := r, r
		if n.Vertical {
			a.H = r.H * n.Ratio
			b.Y, b.H = r.Y+a.H, r.H-a.H
		} else {
			a.W = r.W * n.Ratio
			b.X, b.W = r.X+a.W, r.W-a.W
		}
		walk(n.A, a)
		walk(n.B, b)
	}
	walk(l, Rect{W: 1, H: 1})
	return out
}

// Neighbor returns the block next to from toward dx, dy: the nearest, then
// the one most in line with from's center. With wrap, it looks from the
// other side when there is none.
func Neighbor(rects map[string]Rect, order []string, from string, dx, dy int, wrap bool) string {
	f, ok := rects[from]
	if !ok {
		return ""
	}
	const eps = 1e-4
	cx, cy := f.X+f.W/2, f.Y+f.H/2
	best, bestScore := "", float32(1e9)
	for _, id := range order {
		if id == from {
			continue
		}
		r := rects[id]
		var dist, overlap float32
		switch {
		case dx > 0:
			dist, overlap = r.X-(f.X+f.W), min(r.Y+r.H, f.Y+f.H)-max(r.Y, f.Y)
		case dx < 0:
			dist, overlap = f.X-(r.X+r.W), min(r.Y+r.H, f.Y+f.H)-max(r.Y, f.Y)
		case dy > 0:
			dist, overlap = r.Y-(f.Y+f.H), min(r.X+r.W, f.X+f.W)-max(r.X, f.X)
		default:
			dist, overlap = f.Y-(r.Y+r.H), min(r.X+r.W, f.X+f.W)-max(r.X, f.X)
		}
		if dist < -eps || overlap <= eps {
			continue
		}
		off := r.X + r.W/2 - cx
		if dx != 0 {
			off = r.Y + r.H/2 - cy
		}
		if off < 0 {
			off = -off
		}
		if score := dist*4 + off; score < bestScore {
			best, bestScore = id, score
		}
	}
	if best == "" && wrap {
		// From the far side: as if from just past the other edge.
		g := f
		switch {
		case dx > 0:
			g.X = -f.W
		case dx < 0:
			g.X = 1
		case dy > 0:
			g.Y = -f.H
		default:
			g.Y = 1
		}
		m := map[string]Rect{}
		for k, v := range rects {
			m[k] = v
		}
		m["\x00from"] = g
		if n := Neighbor(m, order, "\x00from", dx, dy, false); n != from {
			return n
		}
	}
	return best
}

// shortID is how listings show an ID: its last six characters.
func shortID(id string) string {
	if _, hex, ok := strings.Cut(id, ":"); ok {
		id = hex
	}
	if len(id) > 6 {
		return "…" + id[len(id)-6:]
	}
	return id
}

// ShortID is shortID, for the CLI.
func ShortID(id string) string { return shortID(id) }

// TargetError is a target that matched nothing, or more than one thing.
type TargetError struct {
	Kind, Target string
	Matches      []string
}

func (e *TargetError) Error() string {
	if len(e.Matches) > 1 {
		var b strings.Builder
		fmt.Fprintf(&b, "%s target %q is ambiguous", e.Kind, e.Target)
		for _, m := range e.Matches {
			b.WriteString("\n  " + m)
		}
		return b.String()
	}
	return fmt.Sprintf("%s %q not found", e.Kind, e.Target)
}

// ErrNoTarget is a command with nothing to act on: no sessions, or no
// block in the window.
var ErrNoTarget = errors.New("nothing to act on")

// labeled is a session, window or block as targets match it.
type labeled struct {
	id, label, name string
}

// match finds target among items: by full ID, then label (without case),
// then position from one (when positional), then the end of the ID, at
// least four characters long. It stops at the first kind that matches.
func match(kind, target string, items []labeled, positional bool) (int, error) {
	for i, it := range items {
		if it.id == target {
			return i, nil
		}
	}
	var found []int
	for i, it := range items {
		if it.label != "" && strings.EqualFold(it.label, target) {
			found = append(found, i)
		}
	}
	if len(found) == 0 && positional {
		if n, err := strconv.Atoi(target); err == nil && n >= 1 && n <= len(items) {
			return n - 1, nil
		}
	}
	if len(found) == 0 && len(target) >= 4 {
		t := strings.TrimPrefix(target, "…")
		for i, it := range items {
			_, hex, _ := strings.Cut(it.id, ":")
			if strings.HasSuffix(it.id, t) || hex == t {
				found = append(found, i)
			}
		}
	}
	switch len(found) {
	case 0:
		return -1, &TargetError{Kind: kind, Target: target}
	case 1:
		return found[0], nil
	}
	e := &TargetError{Kind: kind, Target: target}
	for _, i := range found {
		e.Matches = append(e.Matches, fmt.Sprintf("%-8s %s", shortID(items[i].id), items[i].name))
	}
	return -1, e
}

// session returns the session of the ID.
func (m *Model) session(id string) *Session {
	for _, s := range m.Sessions {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// Session returns the session of the ID, for clients.
func (m *Model) Session(id string) *Session { return m.session(id) }

// window returns the window of the ID and its session.
func (m *Model) window(id string) (*Session, *Window) {
	for _, s := range m.Sessions {
		for _, w := range s.Windows {
			if w.ID == id {
				return s, w
			}
		}
	}
	return nil, nil
}

// blockPlace returns the session and window of a block.
func (m *Model) blockPlace(id string) (*Session, *Window) {
	for _, s := range m.Sessions {
		for _, w := range s.Windows {
			if leaf, _ := w.Layout.find(id); leaf != nil {
				return s, w
			}
		}
	}
	return nil, nil
}

// ActiveWindow returns the window the session shows.
func (s *Session) ActiveWindow() *Window {
	for _, w := range s.Windows {
		if w.ID == s.Active {
			return w
		}
	}
	if len(s.Windows) > 0 {
		return s.Windows[0]
	}
	return nil
}

func (s *Session) windowIndex(id string) int {
	return slices.IndexFunc(s.Windows, func(w *Window) bool { return w.ID == id })
}

// setFocus gives a block the focus of its window.
func (w *Window) setFocus(id string) {
	w.Focus = id
	w.Recent = slices.DeleteFunc(w.Recent, func(b string) bool { return b == id })
	w.Recent = append(w.Recent, id)
}

// forget takes a block out of the window's focus, giving it to the one
// that had it before.
func (w *Window) forget(id string) {
	w.Recent = slices.DeleteFunc(w.Recent, func(b string) bool { return b == id })
	if w.Zoom == id {
		w.Zoom = ""
	}
	if w.Focus == id {
		w.Focus = ""
		if k := len(w.Recent); k > 0 {
			w.Focus = w.Recent[k-1]
		} else if bs := w.Layout.Blocks(); len(bs) > 0 {
			w.setFocus(bs[0])
		}
	}
}

// Name is how a session shows when it has no label: its position.
func (m *Model) SessionName(s *Session) string {
	if s.Label != "" {
		return s.Label
	}
	return fmt.Sprintf("Session %d", slices.Index(m.Sessions, s)+1)
}

// Clone returns a deep copy of the model, for a client to keep.
func (m *Model) Clone() *Model {
	c := &Model{Revision: m.Revision, Blocks: make(map[string]*BlockMeta, len(m.Blocks))}
	for _, s := range m.Sessions {
		cs := *s
		cs.Windows = nil
		for _, w := range s.Windows {
			cw := *w
			cw.Layout = w.Layout.Clone()
			cw.Recent = slices.Clone(w.Recent)
			cs.Windows = append(cs.Windows, &cw)
		}
		c.Sessions = append(c.Sessions, &cs)
	}
	for k, b := range m.Blocks {
		cb := *b
		cb.Command = slices.Clone(b.Command)
		c.Blocks[k] = &cb
	}
	return c
}
