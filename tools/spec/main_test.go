package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fixture's test file is kept as .txt, for go vet; it is copied in.
func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	err := filepath.WalkDir("testdata", func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel("testdata", p)
		dst := filepath.Join(dir, strings.TrimSuffix(rel, ".txt"))
		os.MkdirAll(filepath.Dir(dst), 0o755)
		b, _ := os.ReadFile(p)
		return os.WriteFile(dst, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCheck(t *testing.T) {
	root := fixture(t)
	items, refs, probs := load(root)
	if len(items) != 12 {
		t.Fatalf("items = %d, want 12 (code blocks, the glossary and findings hold none)", len(items))
	}
	var got []string
	for _, p := range probs {
		got = append(got, p.Severity+" "+p.ID+" "+p.Msg)
	}
	all := strings.Join(got, "\n")
	for _, want := range []string{
		`error F-2 status "approved"`,
		"error F-2 traces to B-9",
		"error F-7 a test refers to an item",
		"gap F-3 accepted, verified by tests",
		"warning B-2 rejected without a rationale",
		"style F-4 a sentence of 3",
		"warning F-4 vague in a requirement: typically, etc",
		"style F-4 filler: just",
		"warning F-4 avoided term: pane (use block)",
		`warning F-4 source "guess" is not one of`,
		"warning F-1 no source",
		"error B-3 a constraint without **Fails if.**",
		"error B-4 kind wish is not one of need, constraint",
		"error D-1 a decision without **Options.**",
		"error F-3 neither a rule",
		"error F-6 a use case without numbered steps",
		"error F-8 kind speed is not one of function",
		"error F-8 conflicts with F-77",
		"warning F-8 not EARS: conditions first",
		"gap  no functional item names `rex orphan`",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("no %q in:\n%s", want, all)
		}
	}
	for _, bad := range []string{"`rex ls`", "B-2 a need", "F-1 neither", "F-5 neither", "F-6 a use case without **", "F-4 vague in a requirement: can", "F-4 vague in a requirement: typically, etc, can", "F-5 avoided term", "F-5 vague"} {
		if strings.Contains(all, bad) {
			t.Errorf("%q in:\n%s", bad, all)
		}
	}
	if strings.Contains(all, "F-1 ") && strings.Contains(all, "gap F-1") {
		t.Error("F-1 is tested")
	}
	if len(refs) != 1 || refs[0].Test != "TestLs" {
		t.Errorf("refs = %+v", refs)
	}
	for _, it := range items {
		if it.ID == "F-3" && strings.Contains(it.Body, "Not part") {
			t.Error("a heading of a higher level ends an item")
		}
		if it.ID == "F-1" && (it.Status != "accepted" || len(it.Traces) != 1 || it.Verify != "test") {
			t.Errorf("F-1 = %+v", it)
		}
	}
}

func TestCheckFiles(t *testing.T) {
	root := fixture(t)
	t.Chdir(root)
	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w
	code := cmdCheck(root, []string{"-q", "spec/functional/cli.md"})
	os.Stdout = stdout
	w.Close()
	b, _ := io.ReadAll(r)
	out := string(b)
	if code != 1 || !strings.Contains(out, "F-4: vague") || strings.Contains(out, "B-3") {
		t.Errorf("code %d, out:\n%s", code, out)
	}
}

func TestEARS(t *testing.T) {
	for sn, want := range map[string]string{
		"`rex ls` SHALL list the sessions.":                                "",
		"WHEN a command follows `--`, the block SHALL run it.":             "",
		"WHILE a block is zoomed, WHEN it is split, the app SHALL unzoom.": "",
		"IF two labels are given, THEN `rex` SHALL exit with 2.":           "",
		"WHERE the system is Windows, `rex` SHALL use a named pipe.":       "",
		"WHEN two labels are given THEN `rex` SHALL exit with 2.":          "conditions first",
		"WHEN two labels are given, THEN `rex` SHALL exit with 2.":         "THEN only after IF",
		"IF two labels are given, `rex` SHALL exit with 2.":                "IF …, THEN",
		"`rex` SHALL exit with 2 WHEN two labels are given.":               "conditions first",
	} {
		got := earsProblem(sn)
		if want == "" && got != "" || want != "" && !strings.HasPrefix(got, want) {
			t.Errorf("%q: %q, want %q", sn, got, want)
		}
	}
}
