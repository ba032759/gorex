package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	"gorex/internal/rex"
)

func runCLI(args []string) {
	if len(args) == 0 {
		printCLIHelp()
		return
	}

	cmd := args[0]
	rest := args[1:]

	var err error
	switch cmd {
	case "split":
		err = cliSplit(rest)
	case "new", "new-tab":
		err = cliNewTab(rest)
	case "list":
		err = cliList(rest)
	case "focus":
		err = cliFocus(rest)
	case "close":
		err = cliClose(rest)
	case "send-keys":
		err = cliSendKeys(rest)
	case "help", "--help", "-h":
		printCLIHelp()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		printCLIHelp()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printCLIHelp() {
	fmt.Print(`Rex / GoRex - Terminal multiplexer CLI

Usage:
  rex split [flags] [-- command...]     Split pane (default: split right)
  rex new-tab [flags] [-- command...]   Create a new tab
  rex list [--json]                     List tabs and panes
  rex focus <sid>                       Focus a pane by session ID
  rex close [sid]                       Close a pane (defaults to current $GOREX_SESSION)
  rex send-keys [sid] <keys>            Send keystrokes to a session pty

Split Flags:
  -v, --vertical, --down, --below       Split vertically (new pane below)
  -h, --horizontal, --right, --beside   Split horizontally (new pane right, default)
  -d, --dir <path>                      Working directory for the new pane
  -t, --target <sid>                    Target pane to split (defaults to $GOREX_SESSION)
  --json                                Output JSON response

Examples:
  rex split -- nvim +15 /tmp/demo.txt
  rex split --down -- htop
  rex new-tab --dir ~/repos/project
  rex send-keys ':echo "hello"<CR>'
`)
}

func callAppIPC(req ipcRequest) (*ipcResponse, error) {
	conn, err := net.Dial("unix", rex.AppSocketPath())
	if err != nil {
		return nil, fmt.Errorf("could not connect to GoRex window at %s (is GoRex running?): %w", rex.AppSocketPath(), err)
	}
	defer conn.Close()

	b, _ := json.Marshal(req)
	if _, err := conn.Write(append(b, '\n')); err != nil {
		return nil, err
	}

	r := bufio.NewReader(conn)
	line, err := r.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	var res ipcResponse
	if err := json.Unmarshal(line, &res); err != nil {
		return nil, err
	}
	if !res.OK && res.Error != "" {
		return nil, errors.New(res.Error)
	}
	return &res, nil
}

func parseCommandArgs(args []string) (flags []string, cmd []string) {
	for i, arg := range args {
		if arg == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

func cliSplit(args []string) error {
	flags, command := parseCommandArgs(args)

	vertical := false
	dir := ""
	target := os.Getenv("GOREX_SESSION")
	jsonOut := false

	for i := 0; i < len(flags); i++ {
		f := flags[i]
		switch {
		case f == "-v" || f == "--vertical" || f == "--down" || f == "--below":
			vertical = true
		case f == "-h" || f == "--horizontal" || f == "--right" || f == "--beside":
			vertical = false
		case f == "-d" || f == "--dir":
			if i+1 < len(flags) {
				dir = flags[i+1]
				i++
			}
		case f == "-t" || f == "--target":
			if i+1 < len(flags) {
				target = flags[i+1]
				i++
			}
		case f == "--json":
			jsonOut = true
		default:
			// If arguments start without "--" and are not flags, treat remainder as command
			if !strings.HasPrefix(f, "-") && len(command) == 0 {
				command = flags[i:]
				i = len(flags)
			}
		}
	}

	req := ipcRequest{
		Op:        "split",
		Vertical:  vertical,
		TargetSID: target,
		Dir:       dir,
		Command:   command,
	}

	res, err := callAppIPC(req)
	if err != nil {
		return err
	}

	if jsonOut {
		b, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(b))
	} else {
		fmt.Printf("Created pane %s (pid %d)\n", res.SID, res.PID)
	}
	return nil
}

func cliNewTab(args []string) error {
	flags, command := parseCommandArgs(args)

	dir := ""
	jsonOut := false

	for i := 0; i < len(flags); i++ {
		f := flags[i]
		switch {
		case f == "-d" || f == "--dir":
			if i+1 < len(flags) {
				dir = flags[i+1]
				i++
			}
		case f == "--json":
			jsonOut = true
		default:
			if !strings.HasPrefix(f, "-") && len(command) == 0 {
				command = flags[i:]
				i = len(flags)
			}
		}
	}

	req := ipcRequest{
		Op:      "newTab",
		Dir:     dir,
		Command: command,
	}

	res, err := callAppIPC(req)
	if err != nil {
		return err
	}

	if jsonOut {
		b, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(b))
	} else {
		fmt.Printf("Created tab pane %s (pid %d)\n", res.SID, res.PID)
	}
	return nil
}

func cliList(args []string) error {
	jsonOut := false
	for _, a := range args {
		if a == "--json" {
			jsonOut = true
		}
	}

	res, err := callAppIPC(ipcRequest{Op: "list"})
	if err == nil {
		if jsonOut {
			b, _ := json.MarshalIndent(res.Data, "", "  ")
			fmt.Println(string(b))
			return nil
		}

		b, _ := json.Marshal(res.Data)
		var layout LayoutListInfo
		json.Unmarshal(b, &layout)

		for _, t := range layout.Tabs {
			activeMark := " "
			if t.Active {
				activeMark = "*"
			}
			name := t.Name
			if name == "" {
				name = fmt.Sprintf("Tab %d", t.Index+1)
			}
			fmt.Printf("%s [%d] %s (focused: %s)\n", activeMark, t.Index+1, name, t.FocusedSID)
			for _, p := range t.Panes {
				paneMark := "  "
				if p.Focused {
					paneMark = " >"
				}
				fmt.Printf("%s %s  pid:%-6d  %-12s  %dx%d  %s\n", paneMark, p.SID, p.PID, p.Program, p.Cols, p.Rows, p.Dir)
			}
		}
		return nil
	}

	// Fallback to server.sock if GUI app is not running
	client, sErr := rex.Connect()
	if sErr != nil {
		return fmt.Errorf("app IPC failed: %v; server connect failed: %w", err, sErr)
	}
	defer client.Close()

	infos, err := client.List()
	if err != nil {
		return err
	}

	if jsonOut {
		b, _ := json.MarshalIndent(infos, "", "  ")
		fmt.Println(string(b))
		return nil
	}

	fmt.Println("Server sessions (headless / GUI disconnected):")
	for _, in := range infos {
		fmt.Printf("  %s  pid:%-6d  %-12s  %dx%d  %s\n", in.ID, in.PID, in.Program, in.Cols, in.Rows, in.Dir)
	}
	return nil
}

func cliFocus(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: rex focus <sid>")
	}
	sid := args[0]
	_, err := callAppIPC(ipcRequest{Op: "focus", SID: sid})
	if err != nil {
		return err
	}
	fmt.Printf("Focused %s\n", sid)
	return nil
}

func cliClose(args []string) error {
	sid := os.Getenv("GOREX_SESSION")
	if len(args) > 0 {
		sid = args[0]
	}
	if sid == "" {
		return errors.New("no session ID specified and $GOREX_SESSION not set")
	}
	_, err := callAppIPC(ipcRequest{Op: "close", SID: sid})
	if err != nil {
		return err
	}
	fmt.Printf("Closed %s\n", sid)
	return nil
}

func cliSendKeys(args []string) error {
	sid := os.Getenv("GOREX_SESSION")
	text := ""

	if len(args) == 1 {
		if sid == "" {
			return errors.New("usage: rex send-keys [sid] <keys>")
		}
		text = args[0]
	} else if len(args) >= 2 {
		sid = args[0]
		text = strings.Join(args[1:], " ")
	} else {
		return errors.New("usage: rex send-keys [sid] <keys>")
	}

	// Interpret escape sequences like \n, \r, \x1b, <CR>
	text = strings.ReplaceAll(text, "<CR>", "\r")
	text = strings.ReplaceAll(text, "<Esc>", "\x1b")
	text = strings.ReplaceAll(text, "\\n", "\n")
	text = strings.ReplaceAll(text, "\\r", "\r")
	text = strings.ReplaceAll(text, "\\e", "\x1b")

	conn, err := net.Dial("unix", rex.SocketPath())
	if err != nil {
		return fmt.Errorf("connecting to server: %w", err)
	}
	defer conn.Close()

	attachReq := fmt.Sprintf(`{"op":"attach","sid":%q,"cols":80,"rows":24}`+"\n", sid)
	if _, err := conn.Write([]byte(attachReq)); err != nil {
		return err
	}

	r := bufio.NewReader(conn)
	resp, err := r.ReadBytes('\n')
	if err != nil {
		return err
	}
	var okRes struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(resp, &okRes); err != nil || !okRes.OK {
		if okRes.Error != "" {
			return errors.New(okRes.Error)
		}
		return errors.New("attach failed")
	}

	_, err = conn.Write([]byte(text))
	return err
}
