// GoRex is a terminal of sessions, tabs and split panes, after
// Superlogical's Rex, in MyGo's native UI. Its shells run in a session
// server of their own, so that they outlive the app: quit it, open it
// again, and every session, tab and pane is back as it was, its programs
// still running. The rex command drives the same server, from any of its
// terminals or from scripts and agents.
//
//	go tool mygo dev    the app, rebuilt as the code changes
//	go run . -server    the session server alone, in the foreground
//	go run . help       the rex command
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gorex/internal/rex"
)

func main() {
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "-server" {
		setDir()
		log.SetPrefix("[server] ")
		if err := rex.Serve(); err != nil {
			log.Fatal(err)
		}
		return
	}
	// Run as rex, or with a command of rex's, it is the CLI.
	if invokedAsRex() || len(args) > 0 && isCLICommand(args[0]) {
		setDir()
		os.Exit(runCLI(args))
	}
	setDir()
	registerFonts()
	loadSettings()
	a := &App{}
	mygo.App.SetMenu(a.menu())
	mygo.App.WhenReady(a.open)
	mygo.App.OnActivate(func(hasVisibleWindows bool) {
		if !hasVisibleWindows && a.win == nil {
			a.open()
		}
	})
	mygo.App.OnWindowAllClosed(func() {
		if runtime.GOOS != "darwin" {
			mygo.App.Quit()
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// setDir sets GOREX_DIR, the directory of the server's state that the
// server takes it from: the app's data directory, "GoRex", or "GoRex Dev"
// under mygo dev, which keeps the installed app's sessions apart. A
// terminal of GoRex has it set already, so that its rex talks to the
// server that runs it.
func setDir() {
	if os.Getenv("GOREX_DIR") == "" {
		if dir, err := mygo.App.Path(mygo.PathUserData); err == nil {
			os.Setenv("GOREX_DIR", dir)
		}
	}
}

// invokedAsRex tells that the executable runs by the name rex, as the
// link the server puts in its terminals' PATH.
func invokedAsRex() bool {
	name := strings.TrimSuffix(strings.ToLower(filepath.Base(os.Args[0])), ".exe")
	return name == "rex"
}

// open opens the window, on the sessions the server keeps.
func (a *App) open() {
	client, err := rex.Connect()
	if err != nil {
		log.Print(err)
		mygo.Dialog.Error("GoRex could not start its session server", err.Error())
		mygo.App.Quit()
		return
	}
	h, err := client.Hello()
	switch {
	case err != nil:
	case h.Version != rex.ProtocolVersion:
		// A server of another version of GoRex, which this app cannot
		// drive: restarting it ends what runs in its sessions, which
		// start again as shells in the same directories.
		res, _ := mygo.Dialog.Message(mygo.MessageOptions{
			Type:    mygo.MessageWarning,
			Title:   "Restart the session server?",
			Message: "The session server runs another version of GoRex.",
			Detail: "Restarting it ends the programs running in its sessions. Your sessions, tabs and panes come back, " +
				"with shells in the directories they were in.",
			Buttons:            []string{"Restart Server", "Quit"},
			DestructiveButtons: []int{0},
		})
		if res.Button != 0 {
			mygo.App.Quit()
			return
		}
		client, err = client.Restart(h.PID)
	case mygo.IsDev() && staleServer(h):
		// A server a build before this one started runs that build's
		// code: start one of this build's, its sessions starting again.
		client, err = client.Restart(h.PID)
	}
	if err == nil {
		a.reset()
		err = a.connect(client)
	}
	if err != nil {
		log.Print(err)
		mygo.Dialog.Error("GoRex could not connect to its session server", err.Error())
		mygo.App.Quit()
		return
	}
	if a.model == nil || len(a.model.Sessions) == 0 {
		home, _ := os.UserHomeDir()
		a.act("session.new", rex.Args{Create: &rex.CreateOptions{Dir: home}})
	}
	if a.session != "" {
		go client.View(a.session)
	}
	opts := mygo.WindowOptions{
		Title:                "GoRex",
		Width:                1000,
		Height:               620,
		MinWidth:             560,
		MinHeight:            340,
		StateKey:             "main",
		TitleBarStyle:        mygo.TitleBarHidden,
		TrafficLightPosition: &mygo.Point{X: 16, Y: 15},
		BackgroundColor:      "light-dark(#efe1e6, #231e27)",
		Content:              ui.View(a.view),
	}
	if ownControls {
		// The title bar draws the window's controls (windowControls).
		opts.TitleBarStyle, opts.Frameless = mygo.TitleBarDefault, true
	}
	win := mygo.NewWindow(opts)
	a.win = win
	go a.poll(win, client)
	a.debugHook()
	if problems := keys.problems; len(problems) > 0 {
		a.err = "keymap.json: " + strings.Join(problems, "; ")
	}
	win.OnClosed(func() {
		a.quitting = true
		for _, p := range a.panes {
			p.closed = true
			if p.term != nil {
				p.term.Close() // detaches: the block goes on
			}
		}
		a.client.Close()
		a.win = nil
	})
}

// reset forgets the state of a window that closed, for the next window.
func (a *App) reset() {
	a.model, a.session, a.tabs, a.active, a.focusReq = nil, "", nil, 0, nil
	a.panes, a.tabOf = map[string]*Pane{}, map[string]*Tab{}
	a.paletteOpen, a.hostOpen, a.renaming, a.prompt = false, false, nil, nil
	a.quitting, a.title, a.err = false, "", ""
}

// staleServer reports whether the server runs another build of the app
// than this one, as after mygo dev rebuilt it.
func staleServer(h rex.Hello) bool {
	exe, at := rex.Executable()
	return h.Exe != exe || !h.ExeTime.Equal(at)
}

// fatalf prints an error of the CLI.
func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}
