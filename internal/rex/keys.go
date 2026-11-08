package rex

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// EncodeKey returns the bytes a terminal sends for a key written in Rex's
// key syntax: modifiers and a key joined by "+", as "ctrl+c", "shift+tab",
// "alt+left" or "enter". appCursor tells that the program asked for the
// application cursor keys (DECCKM), as full-screen programs do, whose
// arrows then send SS3 sequences. The encoding is xterm's, which every
// program reads; printable keys are those of a US layout.
func EncodeKey(spec string, appCursor bool) ([]byte, error) {
	mods, key, err := ParseKey(spec)
	if err != nil {
		return nil, err
	}
	shift, alt, ctrl := mods&ModShift != 0, mods&ModAlt != 0, mods&ModCtrl != 0
	// xterm's modifier parameter: 1 plus shift 1, alt 2, ctrl 4.
	param := 1
	if shift {
		param++
	}
	if alt {
		param += 2
	}
	if ctrl {
		param += 4
	}
	csi := func(final string) []byte {
		if param == 1 {
			if appCursor && len(final) == 1 && strings.Contains("ABCDHF", final) {
				return []byte("\x1bO" + final)
			}
			return []byte("\x1b[" + final)
		}
		return []byte(fmt.Sprintf("\x1b[1;%d%s", param, final))
	}
	tilde := func(n int) []byte {
		if param == 1 {
			return []byte(fmt.Sprintf("\x1b[%d~", n))
		}
		return []byte(fmt.Sprintf("\x1b[%d;%d~", n, param))
	}
	ss3 := func(final string) []byte {
		if param == 1 {
			return []byte("\x1bO" + final)
		}
		return []byte(fmt.Sprintf("\x1b[1;%d%s", param, final))
	}
	withAlt := func(b []byte) []byte {
		if alt {
			return append([]byte{0x1b}, b...)
		}
		return b
	}
	switch key {
	case "enter":
		return withAlt([]byte{'\r'}), nil
	case "tab":
		if shift {
			return withAlt([]byte("\x1b[Z")), nil
		}
		return withAlt([]byte{'\t'}), nil
	case "escape":
		return withAlt([]byte{0x1b}), nil
	case "backspace":
		if ctrl {
			return withAlt([]byte{0x08}), nil
		}
		return withAlt([]byte{0x7f}), nil
	case "space":
		if ctrl {
			return withAlt([]byte{0}), nil
		}
		return withAlt([]byte{' '}), nil
	case "up":
		return csi("A"), nil
	case "down":
		return csi("B"), nil
	case "right":
		return csi("C"), nil
	case "left":
		return csi("D"), nil
	case "home":
		return csi("H"), nil
	case "end":
		return csi("F"), nil
	case "insert":
		return tilde(2), nil
	case "delete":
		return tilde(3), nil
	case "pageup":
		return tilde(5), nil
	case "pagedown":
		return tilde(6), nil
	case "f1":
		return ss3("P"), nil
	case "f2":
		return ss3("Q"), nil
	case "f3":
		return ss3("R"), nil
	case "f4":
		return ss3("S"), nil
	}
	if n, ok := map[string]int{"f5": 15, "f6": 17, "f7": 18, "f8": 19, "f9": 20, "f10": 21, "f11": 23, "f12": 24}[key]; ok {
		return tilde(n), nil
	}
	r, size := utf8.DecodeRuneInString(key)
	if size != len(key) || r == utf8.RuneError {
		return nil, fmt.Errorf("unknown key %q", key)
	}
	if shift {
		r = shifted(r)
	}
	if ctrl {
		c, ok := control(r)
		if !ok {
			return nil, fmt.Errorf("ctrl+%s has no control character", key)
		}
		return withAlt([]byte{c}), nil
	}
	return withAlt([]byte(string(r))), nil
}

// The modifiers of a key spec.
const (
	ModShift = 1 << iota
	ModAlt
	ModCtrl
	ModSuper
)

// ParseKey parses "ctrl+shift+a" into modifiers and the key's name, in
// lower case, with "[KeyA]" names of physical keys taken as their US
// layout's.
func ParseKey(spec string) (mods int, key string, err error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return 0, "", fmt.Errorf("empty key")
	}
	// The key is after the last "+", but "ctrl++" is ctrl and plus.
	rest := ""
	switch {
	case spec == "+":
		key = "+"
	case strings.HasSuffix(spec, "++"):
		key, rest = "+", spec[:len(spec)-2]
	default:
		i := strings.LastIndex(spec, "+")
		key = spec[i+1:]
		if i > 0 {
			rest = spec[:i]
		}
	}
	if key == "" {
		return 0, "", fmt.Errorf("no key in %q", spec)
	}
	var modNames []string
	if rest != "" {
		modNames = strings.Split(rest, "+")
	}
	for _, m := range modNames {
		switch strings.ToLower(strings.TrimSpace(m)) {
		case "shift":
			mods |= ModShift
		case "alt", "opt", "option", "meta":
			mods |= ModAlt
		case "ctrl", "control":
			mods |= ModCtrl
		case "cmd", "command", "super", "win":
			mods |= ModSuper
		case "":
		default:
			return 0, "", fmt.Errorf("unknown modifier %q in %q", m, spec)
		}
	}
	if strings.HasPrefix(key, "[") && strings.HasSuffix(key, "]") {
		code := key[1 : len(key)-1]
		switch {
		case strings.HasPrefix(code, "Key") && len(code) == 4:
			key = strings.ToLower(code[3:])
		case strings.HasPrefix(code, "Digit") && len(code) == 6:
			key = code[5:]
		case strings.HasPrefix(code, "Arrow"):
			key = strings.ToLower(code[5:])
		default:
			if k, ok := codeNames[code]; ok {
				key = k
			} else {
				key = strings.ToLower(code)
			}
		}
	}
	if len(key) > 1 {
		key = strings.ToLower(key)
		if k, ok := keyAliases[key]; ok {
			key = k
		}
	}
	return mods, key, nil
}

var keyAliases = map[string]string{
	"return": "enter", "cr": "enter", "esc": "escape", "bs": "backspace", "del": "delete",
	"pgup": "pageup", "pgdn": "pagedown", "page_up": "pageup", "page_down": "pagedown",
	"ins": "insert", "plus": "+", "minus": "-", "equal": "=", "comma": ",", "period": ".",
	"slash": "/", "backslash": "\\", "semicolon": ";", "quote": "'", "backquote": "`",
	"bracketleft": "[", "bracketright": "]",
}

var codeNames = map[string]string{
	"Minus": "-", "Equal": "=", "BracketLeft": "[", "BracketRight": "]", "Backslash": "\\",
	"Semicolon": ";", "Quote": "'", "Backquote": "`", "Comma": ",", "Period": ".", "Slash": "/",
	"Space": "space", "Enter": "enter", "Tab": "tab", "Escape": "escape", "Backspace": "backspace",
}

// shifted is what a key types with shift, on a US layout.
func shifted(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 'a' + 'A'
	}
	const from, to = "1234567890-=[]\\;',./`", "!@#$%^&*()_+{}|:\"<>?~"
	if i := strings.IndexRune(from, r); i >= 0 {
		return rune(to[i])
	}
	return r
}

// control returns the control character ctrl and a key type.
func control(r rune) (byte, bool) {
	switch {
	case r >= 'a' && r <= 'z':
		return byte(r-'a') + 1, true
	case r >= 'A' && r <= 'Z':
		return byte(r-'A') + 1, true
	}
	switch r {
	case '@', '2', ' ':
		return 0, true
	case '[', '3':
		return 0x1b, true
	case '\\', '4':
		return 0x1c, true
	case ']', '5':
		return 0x1d, true
	case '^', '6':
		return 0x1e, true
	case '_', '-', '7', '/':
		return 0x1f, true
	case '?', '8':
		return 0x7f, true
	}
	return 0, false
}
