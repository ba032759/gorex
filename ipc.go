package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"sync"

	"gorex/internal/rex"
)

type ipcServer struct {
	app      *App
	listener net.Listener
	closed   chan struct{}
	once     sync.Once
}

type ipcRequest struct {
	Op        string   `json:"op"`
	TargetSID string   `json:"target_sid,omitempty"`
	Vertical  bool     `json:"vertical,omitempty"`
	Command   []string `json:"command,omitempty"`
	Dir       string   `json:"dir,omitempty"`
	SID       string   `json:"sid,omitempty"`
}

type ipcResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	SID   string `json:"sid,omitempty"`
	PID   int    `json:"pid,omitempty"`
	Data  any    `json:"data,omitempty"`
}

func (a *App) startIPC() *ipcServer {
	sock := rex.AppSocketPath()
	os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		log.Printf("rex app ipc: listen failed: %v", err)
		return nil
	}
	os.Chmod(sock, 0o600)

	srv := &ipcServer{app: a, listener: ln, closed: make(chan struct{})}
	go srv.serve()
	return srv
}

func (s *ipcServer) close() {
	s.once.Do(func() {
		close(s.closed)
		if s.listener != nil {
			s.listener.Close()
			os.Remove(rex.AppSocketPath())
		}
	})
}

func (s *ipcServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.closed:
				return
			default:
				return
			}
		}
		go s.handle(conn)
	}
}

func (s *ipcServer) handle(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req ipcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			b, _ := json.Marshal(ipcResponse{Error: err.Error()})
			conn.Write(append(b, '\n'))
			return
		}

		res := s.dispatch(req)
		b, _ := json.Marshal(res)
		conn.Write(append(b, '\n'))
	}
}

func (s *ipcServer) dispatch(req ipcRequest) ipcResponse {
	type result struct {
		res ipcResponse
	}
	ch := make(chan result, 1)

	action := func() {
		switch req.Op {
		case "split":
			opts := rex.CreateOptions{
				Command: req.Command,
				Dir:     req.Dir,
			}
			p, err := s.app.splitWithOptions(req.TargetSID, req.Vertical, opts)
			if err != nil {
				ch <- result{res: ipcResponse{Error: err.Error()}}
				return
			}
			ch <- result{res: ipcResponse{OK: true, SID: p.SID, PID: p.info.PID}}

		case "newTab":
			opts := rex.CreateOptions{
				Command: req.Command,
				Dir:     req.Dir,
			}
			_, p := s.app.newTabWithOptions(opts)
			ch <- result{res: ipcResponse{OK: true, SID: p.SID, PID: p.info.PID}}

		case "list":
			data := s.app.listTabsAndPanes()
			ch <- result{res: ipcResponse{OK: true, Data: data}}

		case "focus":
			ok := s.app.focusPane(req.SID)
			if !ok {
				ch <- result{res: ipcResponse{Error: "pane not found"}}
				return
			}
			ch <- result{res: ipcResponse{OK: true}}

		case "close":
			ok := s.app.closePaneBySID(req.SID)
			if !ok {
				ch <- result{res: ipcResponse{Error: "pane not found"}}
				return
			}
			ch <- result{res: ipcResponse{OK: true}}

		default:
			ch <- result{res: ipcResponse{Error: fmt.Sprintf("unknown op: %s", req.Op)}}
		}
	}

	if s.app.win != nil {
		s.app.win.Update(action)
	} else {
		action()
	}

	return (<-ch).res
}
