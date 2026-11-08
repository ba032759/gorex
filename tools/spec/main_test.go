package main

import (
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
	if len(items) != 6 {
		t.Fatalf("items = %d, want 6 (code blocks, the glossary and findings hold none)", len(items))
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
		"warning F-4 a sentence of 32 words",
		"warning F-4 vague words: should, typically",
		"warning F-4 say SHALL, not can",
		"warning F-4 avoided term: pane (use block)",
		`warning F-4 source "guess" is not one of`,
		"warning F-1 no source",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("no %q in:\n%s", want, all)
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
