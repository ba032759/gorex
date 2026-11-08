//go:build darwin || linux || windows

package rex

import (
	"encoding/json"
	"time"
)

// wideRequest records a request the server answered: who asked, what for,
// what it acted on, how it ended and how long it took, with the size of
// the server at that moment. The app's polls are left out when they
// succeed: they come twice a second and say nothing.
func (s *Server) wideRequest(c *conn, req Request, res Response, start time.Time, size int) {
	s.mu.Lock()
	kind, id, name, pid := c.kind, c.id, c.name, c.pid
	if res.Error == "" && kind == "app" && (req.Op == "list" || req.Op == "status.list" || req.Op == "model") {
		s.mu.Unlock()
		return
	}
	windows := 0
	for _, se := range s.model.Sessions {
		windows += len(se.Windows)
	}
	w := W{
		"op":          req.Op,
		"ok":          res.Error == "",
		"duration_ms": float64(time.Since(start).Microseconds()) / 1000,
		"client_kind": kind,
		"client_id":   id,
		"revision":    s.model.Revision,
		"n_sessions":  len(s.model.Sessions),
		"n_windows":   windows,
		"n_blocks":    len(s.blocks),
		"n_clients":   len(s.conns),
		"uptime_s":    time.Since(s.started).Seconds(),
	}
	s.mu.Unlock()
	a := req.Args
	w.setNonZero(
		"client_name", name,
		"client_pid", pid,
		"code", res.Code,
		"error", res.Error,
		"response_bytes", size,
		"sid", req.SID,
		"arg_session", a.Session,
		"arg_window", a.Window,
		"arg_block", a.Block,
		"arg_target", a.Target,
		"ctx_session", a.CtxSession,
		"ctx_block", a.CtxBlock,
		"arg_split", a.Split,
		"arg_direction", a.Direction,
		"arg_label", a.Label,
		"arg_ratio", a.Ratio,
		"arg_index", a.Index,
		"arg_zoom", a.Zoom,
		"arg_format", a.Format,
		"arg_trim", a.Trim,
		"arg_all", a.All,
		"arg_model", a.Model,
		"client_target", a.Client,
		"action", a.Action,
		// The lengths only: what is typed into terminals stays private.
		"data_len", len(a.Data),
		"n_keys", len(a.Keys),
	)
	if cr := req.Create; cr != nil || a.Create != nil {
		if cr == nil {
			cr = a.Create
		}
		w.setNonZero("create_shell", cr.Shell, "create_keep_open", cr.KeepOpen,
			"create_has_command", len(cr.Command) > 0, "create_label", cr.Label)
		if len(cr.Command) > 0 {
			w["create_program"] = cr.Command[0]
		}
	}
	if req.Op == "status.report" && a.Data != "" {
		// A report's state, not its text.
		if rec, _, err := parseStatus(a.Data); err == nil {
			w.setNonZero("status_state", rec.State, "status_kind", rec.Kind, "status_app", rec.App)
		}
	}
	Wide("request", w)
}

// wideEvent records an event the server emitted, whether or not anyone
// listened. A session's event and a terminal's, nested in the Rex shape,
// come flat: type is the innermost type ("block_event" names the
// terminal's in block_event), and the payloads' scalar fields are columns.
func wideEvent(ev Event) {
	w := W{"type": ev.Type, "event_type": ev.Type}
	w.setNonZero("session_id", ev.SessionID)
	p := ev.Payload
	if ev.Type == "session_event" {
		if e, ok := p["event"].(map[string]any); ok {
			w["type"], _ = e["type"].(string)
			p, _ = e["payload"].(map[string]any)
			if w["type"] == "block_event" {
				w["block_event"], _ = p["name"].(string)
				w.setNonZero("block_id", p["block_id"])
				p, _ = p["payload"].(map[string]any)
			}
		}
	}
	if rec, ok := p["record"].(StatusRecord); ok {
		// A program status record.
		w.setNonZero("status_state", rec.State, "status_kind", rec.Kind, "status_app", rec.App,
			"status_id", rec.ID, "status_msg_len", len(rec.Msg))
		if rec.Progress >= 0 {
			w["status_progress"] = rec.Progress
		}
	}
	for k, v := range p {
		switch v.(type) {
		case string, bool, float64, float32, int, int64:
			if _, taken := w[k]; !taken {
				w["p_"+k] = v
			}
		}
	}
	if b, err := json.Marshal(redact(ev.Payload)); err == nil && len(b) < 2048 {
		w["payload"] = string(b)
	}
	Wide("event", w)
}

// redact replaces the texts programs report about their work, which may
// hold what the user wrote, by their lengths.
func redact(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		switch v := v.(type) {
		case map[string]any:
			out[k] = redact(v)
		case StatusRecord:
			v.Msg, v.Title = "", ""
			out[k] = v
		case string:
			if k == "msg" {
				out[k+"_len"] = len(v)
				continue
			}
			out[k] = v
		default:
			out[k] = v
		}
	}
	return out
}
