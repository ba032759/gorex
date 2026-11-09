package rex

import (
	"strings"
	"testing"
)

// A line the terminal wrapped comes as its rows, unless unwrapped.
//
// spec: F-164
func TestCaptureRows(t *testing.T) {
	for _, c := range []struct{ text, want string }{
		{"abcdefgh\nab", "abcd\nefgh\nab"},
		{"abcd\n", "abcd\n"},
		{"ab漢字x", "ab漢\n字x"}, // a wide character that does not fit goes on
		{"ééééé", "éééé\né"},
	} {
		if got := splitRows(c.text, 4); got != c.want {
			t.Errorf("splitRows(%q) = %q, want %q", c.text, got, c.want)
		}
	}

	cl, _ := startServer(t)
	s := act(t, cl, "session.new", Args{Create: &CreateOptions{Dir: "/tmp", Cols: 40, Rows: 10,
		Command: []string{"sh", "-c", "printf '%050d\\n' 0; sleep 30"}}})
	b := s["block_id"].(string)
	text := waitText(t, cl, b, "0000000000")
	if !strings.Contains(text, strings.Repeat("0", 40)+"\n"+strings.Repeat("0", 10)+"\n") {
		t.Errorf("capture %q", text)
	}
	out := act(t, cl, "capture", Args{Block: b, Trim: true, Unwrap: true})
	if !strings.Contains(out["text"].(string), strings.Repeat("0", 50)+"\n") {
		t.Errorf("capture --unwrap %q", out["text"])
	}
}

// spec: F-165
func TestScrollbackSize(t *testing.T) {
	if scrollback != 64<<20 {
		t.Errorf("scrollback %d, not about 64 MiB", scrollback)
	}
}
