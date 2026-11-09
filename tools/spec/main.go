// Command spec reads, checks and indexes the specification in spec/, and
// links it to the tests that verify it. See spec/README.md.
//
//	spec check [--strict] [--json]   check the spec and its links to the tests
//	spec ls [-l LAYER] [-s STATUS] [-a AREA]
//	spec show ID...                  an item, what it traces to, what traces to it, its tests
//	spec for FILE...                 the items the tests of these files' packages verify
//	spec matrix                      the traceability matrix, business to tests, as markdown
//	spec stats                       items, words and reading time per area
//	spec index                       write .spec/*.jsonl for queries
//	spec query [SQL]                 DuckDB over the spec, the tests, their results and the events
//	spec test -- CMD...              run go test -json, keep the results, print the failures
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// Item is a business need, a decision, a functional or a technical
// requirement.
type Item struct {
	ID           string            `json:"id"`
	Layer        string            `json:"layer"` // business, decision, functional, technical
	Title        string            `json:"title"`
	Status       string            `json:"status"`
	Traces       []string          `json:"traces"`
	Verify       string            `json:"verify,omitempty"`
	Area         string            `json:"area"`
	SupersededBy string            `json:"superseded_by,omitempty"`
	Source       []string          `json:"source"`
	Words        int               `json:"words"`
	Fields       map[string]string `json:"fields,omitempty"`
	File         string            `json:"file"`
	Line         int               `json:"line"`
	Body         string            `json:"body"`
}

// TestRef is a test that says which items it verifies.
type TestRef struct {
	File string   `json:"file"`
	Line int      `json:"line"`
	Test string   `json:"test"`
	IDs  []string `json:"ids"`
}

var (
	layers = map[string]string{"B": "business", "D": "decision", "F": "functional", "T": "technical"}
	// statuses an item may have. Only the user accepts, rejects or defers.
	statuses  = []string{"proposed", "accepted", "rejected", "deferred", "superseded"}
	verifies  = []string{"test", "manual", "event"}
	headingRe = regexp.MustCompile(`^(#{2,4})\s+([A-Z]+)-(\d+)\b[:.]?\s*(.*)$`)
	fieldRe   = regexp.MustCompile(`^[-*]?\s*([a-z][a-z-]*):\s*(.*)$`)
	idRe      = regexp.MustCompile(`\b[BDFT]-\d+\b`)
	refRe     = regexp.MustCompile(`(?://|#|--|/\*)\s*spec:\s*([BDFT]-\d+(?:\s*,\s*[BDFT]-\d+)*)`)
	testFnRe  = []*regexp.Regexp{
		regexp.MustCompile(`func\s+(Test\w+)\s*\(`),
		regexp.MustCompile(`t\.Run\(\s*"([^"]+)"`),
		regexp.MustCompile(`fn\s+(\w+)\s*\(`),
		regexp.MustCompile(`(?:def|function)\s+(\w+)\s*\(`),
		regexp.MustCompile(`(?:it|test|describe)\(\s*["'](.+?)["']`),
	}
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	root, err := findRoot()
	if err != nil {
		die(err)
	}
	cmd, args := os.Args[1], os.Args[2:]
	switch cmd {
	case "check":
		os.Exit(cmdCheck(root, args))
	case "ls":
		cmdLs(root, args)
	case "show":
		cmdShow(root, args)
	case "for":
		cmdFor(root, args)
	case "matrix":
		cmdMatrix(root)
	case "stats":
		cmdStats(root)
	case "index":
		if err := writeIndex(root); err != nil {
			die(err)
		}
	case "query":
		os.Exit(cmdQuery(root, args))
	case "test":
		os.Exit(cmdTest(root, args))
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: spec check|ls|show|for|matrix|index|query|test (see spec/README.md)")
	os.Exit(2)
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "spec:", err)
	os.Exit(1)
}

// findRoot is the directory with spec/, from here up.
func findRoot() (string, error) {
	d, _ := os.Getwd()
	for {
		if st, err := os.Stat(filepath.Join(d, "spec")); err == nil && st.IsDir() {
			return d, nil
		}
		p := filepath.Dir(d)
		if p == d {
			return "", errors.New("no spec/ directory here or above")
		}
		d = p
	}
}

// Problem is something wrong with the spec.
type Problem struct {
	Severity string `json:"severity"` // error, gap, warning
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	ID       string `json:"id,omitempty"`
	Msg      string `json:"msg"`
}

func (p Problem) String() string {
	loc := p.File
	if p.Line > 0 {
		loc += fmt.Sprintf(":%d", p.Line)
	}
	if p.ID != "" {
		loc += " " + p.ID
	}
	return fmt.Sprintf("%-7s %s: %s", p.Severity, loc, p.Msg)
}

// parseSpec reads the items of spec/**/*.md.
func parseSpec(root string) ([]*Item, []Problem) {
	var items []*Item
	var probs []Problem
	dir := filepath.Join(root, "spec")
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		if filepath.Dir(path) == dir && notItems[strings.ToLower(d.Name())] {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		its, ps := parseFile(path, rel)
		items, probs = append(items, its...), append(probs, ps...)
		return nil
	})
	if err != nil {
		probs = append(probs, Problem{Severity: "error", File: "spec", Msg: err.Error()})
	}
	return items, probs
}

func parseFile(path, rel string) ([]*Item, []Problem) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, []Problem{{Severity: "error", File: rel, Msg: err.Error()}}
	}
	area := strings.TrimSuffix(filepath.Base(rel), ".md")
	var items []*Item
	var probs []Problem
	var cur *Item
	var level int
	var body []string
	inFields, inCode := false, false
	flush := func() {
		if cur != nil {
			cur.Body = strings.TrimSpace(strings.Join(body, "\n"))
			items = append(items, cur)
		}
		cur, body = nil, nil
	}
	for i, line := range strings.Split(string(b), "\n") {
		n := i + 1
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCode = !inCode
		}
		if !inCode && strings.HasPrefix(line, "#") {
			hashes := len(line) - len(strings.TrimLeft(line, "#"))
			if m := headingRe.FindStringSubmatch(line); m != nil {
				flush()
				layer, ok := layers[m[2]]
				if !ok {
					probs = append(probs, Problem{"error", rel, n, m[2] + "-" + m[3], "unknown prefix " + m[2] + " (B, D, F or T)"})
					continue
				}
				id := m[2] + "-" + m[3]
				if m[3] != strings.TrimLeft(m[3], "0") && m[3] != "0" {
					probs = append(probs, Problem{"error", rel, n, id, "no leading zeros in IDs"})
				}
				cur = &Item{ID: id, Layer: layer, Title: strings.TrimSpace(m[4]), Area: area,
					File: rel, Line: n, Fields: map[string]string{}, Traces: []string{}}
				level, inFields = len(m[1]), true
				continue
			}
			if cur != nil && hashes <= level {
				flush() // a heading that ends the item
				continue
			}
		}
		if cur == nil {
			continue
		}
		if inFields {
			t := strings.TrimSpace(line)
			if t == "" && len(cur.Fields) == 0 {
				continue
			}
			if m := fieldRe.FindStringSubmatch(t); m != nil && t != "" {
				for _, part := range splitFields(t) {
					if m := fieldRe.FindStringSubmatch(part); m != nil {
						cur.Fields[m[1]] = strings.TrimSpace(m[2])
					}
				}
				continue
			}
			inFields = false
		}
		body = append(body, line)
	}
	flush()
	for _, it := range items {
		it.Status = it.Fields["status"]
		it.Verify = it.Fields["verify"]
		if a := it.Fields["area"]; a != "" {
			it.Area = a
		}
		it.Traces = idRe.FindAllString(it.Fields["traces"], -1)
		if it.Traces == nil {
			it.Traces = []string{}
		}
		it.SupersededBy = strings.Join(idRe.FindAllString(it.Fields["superseded-by"], -1), ",")
		it.Source = []string{}
		for _, src := range strings.Split(it.Fields["source"], ",") {
			if src = strings.TrimSpace(src); src != "" {
				it.Source = append(it.Source, src)
			}
		}
		it.Words = words(it.Title + "\n" + it.Body)
		for _, k := range []string{"status", "verify", "area", "traces", "superseded-by", "source"} {
			delete(it.Fields, k)
		}
		if it.Layer == "functional" && it.Verify == "" {
			it.Verify = "test"
		}
	}
	return items, probs
}

// splitFields splits "status: accepted · traces: B-1" into its fields.
func splitFields(t string) []string {
	var out []string
	for _, p := range strings.FieldsFunc(t, func(r rune) bool { return r == '·' || r == '|' }) {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

// scanTests finds the "spec: F-1, F-2" comments of the code, outside spec/.
func scanTests(root string) []TestRef {
	var refs []TestRef
	skip := map[string]bool{".git": true, "spec": true, "build": true, ".spec": true, "node_modules": true,
		"target": true, "vendor": true, ".mygo": true, "tools": true}
	// Unreadable files and directories are skipped: they hold no tests
	// anyone runs.
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip what cannot be read
		}
		if d.IsDir() {
			if path != root && (skip[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(path)
		if !slices.Contains([]string{".go", ".rs", ".py", ".ts", ".js", ".sh", ".kt", ".swift"}, ext) {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		rel, _ := filepath.Rel(root, path)
		var lines []string
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			lines = append(lines, sc.Text())
		}
		for i, l := range lines {
			m := refRe.FindStringSubmatch(l)
			if m == nil {
				continue
			}
			ref := TestRef{File: rel, Line: i + 1, IDs: idRe.FindAllString(m[1], -1)}
			// The test it is in front of, or on the line of.
			for j := i; j < len(lines) && j <= i+4 && ref.Test == ""; j++ {
				for _, re := range testFnRe {
					if mm := re.FindStringSubmatch(lines[j]); mm != nil {
						ref.Test = mm[1]
						break
					}
				}
			}
			refs = append(refs, ref)
		}
		return nil
	})
	return refs
}

func check(items []*Item, probs []Problem, refs []TestRef) []Problem {
	byID := map[string]*Item{}
	for _, it := range items {
		if o, ok := byID[it.ID]; ok {
			probs = append(probs, Problem{"error", it.File, it.Line, it.ID, fmt.Sprintf("duplicate ID, also at %s:%d", o.File, o.Line)})
			continue
		}
		byID[it.ID] = it
	}
	tested := map[string][]TestRef{}
	for _, r := range refs {
		for _, id := range r.IDs {
			tested[id] = append(tested[id], r)
			it, ok := byID[id]
			switch {
			case !ok:
				probs = append(probs, Problem{"error", r.File, r.Line, id, "a test refers to an item the spec does not have"})
			case it.Status == "rejected" || it.Status == "superseded":
				probs = append(probs, Problem{"warning", r.File, r.Line, id, "a test verifies a " + it.Status + " item"})
			}
			if r.Test == "" {
				probs = append(probs, Problem{"warning", r.File, r.Line, id, "no test function found right after this spec: comment"})
			}
		}
	}
	tracedBy := map[string][]string{}
	for _, it := range items {
		for _, t := range it.Traces {
			tracedBy[t] = append(tracedBy[t], it.ID)
		}
	}
	for _, it := range items {
		p := func(sev, msg string) { probs = append(probs, Problem{sev, it.File, it.Line, it.ID, msg}) }
		if it.Title == "" {
			p("error", "no title")
		}
		if !slices.Contains(statuses, it.Status) {
			p("error", fmt.Sprintf("status %q is not one of %s", it.Status, strings.Join(statuses, ", ")))
		}
		if it.Body == "" {
			p("warning", "no text")
		}
		for _, t := range it.Traces {
			to, ok := byID[t]
			switch {
			case !ok:
				p("error", "traces to "+t+", which the spec does not have")
			case !tracesOK(it.Layer, to.Layer):
				p("error", fmt.Sprintf("a %s item traces to %s, a %s item", it.Layer, t, to.Layer))
			case it.Status == "accepted" && (to.Status == "rejected" || to.Status == "superseded"):
				p("error", "accepted, but traces to "+t+", which is "+to.Status)
			}
		}
		switch it.Layer {
		case "functional":
			if len(it.Traces) == 0 && it.Status != "rejected" {
				p("error", "a functional item must trace to a business item (traces: B-…)")
			}
			if !slices.Contains(verifies, it.Verify) {
				p("error", fmt.Sprintf("verify %q is not one of %s", it.Verify, strings.Join(verifies, ", ")))
			}
			if it.Status == "accepted" && it.Verify == "test" && len(tested[it.ID]) == 0 {
				p("gap", "accepted, verified by tests, and no test says spec: "+it.ID)
			}
		case "technical":
			if len(it.Traces) == 0 && it.Status != "rejected" {
				p("warning", "a technical item should trace to the functional items it serves")
			}
		case "business":
			if it.Status == "accepted" && len(tracedBy[it.ID]) == 0 {
				p("gap", "accepted, and no functional item traces to it")
			}
		}
		if it.Status == "superseded" {
			if it.SupersededBy == "" {
				p("error", "superseded, but no superseded-by")
			}
			for _, s := range strings.Split(it.SupersededBy, ",") {
				if _, ok := byID[s]; s != "" && !ok {
					p("error", "superseded by "+s+", which the spec does not have")
				}
			}
		}
		if (it.Status == "rejected" || it.Status == "deferred") && it.Fields["rationale"] == "" && !strings.Contains(strings.ToLower(it.Body), "because") {
			p("warning", it.Status+" without a rationale: say why (rationale: …)")
		}
	}
	sort.SliceStable(probs, func(i, j int) bool {
		rank := map[string]int{"error": 0, "gap": 1, "warning": 2, "style": 3}
		if rank[probs[i].Severity] != rank[probs[j].Severity] {
			return rank[probs[i].Severity] < rank[probs[j].Severity]
		}
		if probs[i].File != probs[j].File {
			return probs[i].File < probs[j].File
		}
		return probs[i].Line < probs[j].Line
	})
	return probs
}

// tracesOK tells the layers an item may trace to: down the V, a layer
// traces to the one above it, or further up.
func tracesOK(from, to string) bool {
	switch from {
	case "functional":
		return to == "business" || to == "decision"
	case "technical":
		return to == "functional" || to == "business" || to == "decision"
	case "decision":
		return true
	case "business":
		return to == "business" || to == "decision"
	}
	return false
}

func load(root string) ([]*Item, []TestRef, []Problem) {
	items, probs := parseSpec(root)
	refs := scanTests(root)
	gl, gprobs := parseGlossary(root)
	probs = append(probs, gprobs...)
	probs = append(probs, checkWriting(items, gl)...)
	probs = append(probs, checkShape(items)...)
	return items, refs, check(items, probs, refs)
}

func cmdCheck(root string, args []string) int {
	fl := flag.NewFlagSet("check", flag.ExitOnError)
	strict := fl.Bool("strict", false, "gaps fail the check too")
	asJSON := fl.Bool("json", false, "print the problems as JSON")
	quiet := fl.Bool("q", false, "print only errors (and gaps with --strict)")
	showStyle := fl.Bool("style", false, "print the style notes too: long sentences and items, filler")
	fl.Parse(args)
	items, refs, probs := load(root)
	// With files, check only their problems, and warnings fail too: an
	// author, or an agent's edit hook, sees what they just wrote.
	if files := fl.Args(); len(files) > 0 {
		only := map[string]bool{}
		for _, f := range files {
			if abs, err := filepath.Abs(f); err == nil {
				if rel, err := filepath.Rel(root, abs); err == nil {
					f = rel
				}
			}
			only[filepath.ToSlash(f)] = true
		}
		probs = slices.DeleteFunc(probs, func(p Problem) bool { return !only[filepath.ToSlash(p.File)] })
	}
	fail := 0
	count := map[string]int{}
	for _, p := range probs {
		count[p.Severity]++
		if p.Severity == "error" || p.Severity == "gap" && *strict || p.Severity == "warning" && len(fl.Args()) > 0 {
			fail = 1
		}
	}
	if *asJSON {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"problems": probs, "items": len(items), "tests": len(refs), "ok": fail == 0})
		return fail
	}
	for _, p := range probs {
		if *quiet && !(p.Severity == "error" || p.Severity == "gap" && *strict || p.Severity == "warning" && len(fl.Args()) > 0) {
			continue
		}
		if p.Severity == "style" && !*showStyle {
			continue
		}
		fmt.Println(p)
	}
	byLayer := map[string]int{}
	for _, it := range items {
		byLayer[it.Layer]++
	}
	if !*quiet || fail != 0 {
		fmt.Printf("spec: %d items (%d business, %d decisions, %d functional, %d technical), %d test links; %d errors, %d gaps, %d warnings, %d style notes (--style)\n",
			len(items), byLayer["business"], byLayer["decision"], byLayer["functional"], byLayer["technical"],
			len(refs), count["error"], count["gap"], count["warning"], count["style"])
	}
	return fail
}

func cmdLs(root string, args []string) {
	fl := flag.NewFlagSet("ls", flag.ExitOnError)
	layer := fl.String("l", "", "layer: business, decision, functional, technical (or B, D, F, T)")
	status := fl.String("s", "", "status")
	area := fl.String("a", "", "area")
	fl.Parse(args)
	if l, ok := layers[strings.ToUpper(*layer)]; ok {
		*layer = l
	}
	items, _ := parseSpec(root)
	for _, it := range items {
		if *layer != "" && it.Layer != *layer || *status != "" && it.Status != *status || *area != "" && it.Area != *area {
			continue
		}
		fmt.Printf("%-6s %-10s %-14s %s\n", it.ID, it.Status, it.Area, it.Title)
	}
}

func cmdShow(root string, ids []string) {
	items, refs, _ := load(root)
	byID := map[string]*Item{}
	for _, it := range items {
		byID[it.ID] = it
	}
	for _, id := range ids {
		it, ok := byID[strings.ToUpper(id)]
		if !ok {
			fmt.Fprintf(os.Stderr, "spec: no %s\n", id)
			continue
		}
		fmt.Printf("%s %s\n  %s · %s · %s:%d", it.ID, it.Title, it.Status, it.Layer, it.File, it.Line)
		if it.Verify != "" {
			fmt.Printf(" · verify: %s", it.Verify)
		}
		fmt.Printf("\n\n%s\n\n", it.Body)
		for _, t := range it.Traces {
			if to := byID[t]; to != nil {
				fmt.Printf("  traces to   %s %s (%s)\n", t, to.Title, to.Status)
			}
		}
		for _, c := range conflicts(it) {
			if to := byID[c]; to != nil {
				fmt.Printf("  conflicts   %s %s (%s)\n", c, to.Title, to.Status)
			}
		}
		for _, o := range items {
			if slices.Contains(conflicts(o), it.ID) {
				fmt.Printf("  conflicts   %s %s (%s)\n", o.ID, o.Title, o.Status)
			}
			if slices.Contains(o.Traces, it.ID) {
				fmt.Printf("  traced by   %s %s (%s)\n", o.ID, o.Title, o.Status)
			}
		}
		for _, r := range refs {
			if slices.Contains(r.IDs, it.ID) {
				fmt.Printf("  tested by   %s (%s:%d)\n", r.Test, r.File, r.Line)
			}
		}
		fmt.Println()
	}
}

// cmdFor lists the items verified by the tests of the packages (the
// directories) of the given files: what a change to them may touch.
func cmdFor(root string, files []string) {
	items, refs, _ := load(root)
	byID := map[string]*Item{}
	for _, it := range items {
		byID[it.ID] = it
	}
	seen := map[string]bool{}
	for _, f := range files {
		abs, _ := filepath.Abs(f)
		rel, _ := filepath.Rel(root, abs)
		dir := filepath.Dir(rel)
		for _, r := range refs {
			if filepath.Dir(r.File) != dir && r.File != rel {
				continue
			}
			for _, id := range r.IDs {
				if !seen[id] {
					seen[id] = true
					if it := byID[id]; it != nil {
						fmt.Printf("%-6s %-10s %s\n", it.ID, it.Status, it.Title)
					}
				}
			}
		}
	}
}

func cmdMatrix(root string) {
	items, refs, _ := load(root)
	byID := map[string]*Item{}
	for _, it := range items {
		byID[it.ID] = it
	}
	tests := map[string][]string{}
	for _, r := range refs {
		for _, id := range r.IDs {
			tests[id] = append(tests[id], r.Test)
		}
	}
	// What is in force or on the way: accepted and proposed items.
	live := func(it *Item) bool { return it.Status == "accepted" || it.Status == "proposed" }
	mark := func(it *Item) string {
		if it.Status == "proposed" {
			return " *(proposed)*"
		}
		return ""
	}
	fmt.Println("| Business | Functional | Verified by | Technical |")
	fmt.Println("|---|---|---|---|")
	for _, b := range items {
		if b.Layer != "business" || !live(b) {
			continue
		}
		var fs []*Item
		for _, f := range items {
			if f.Layer == "functional" && slices.Contains(f.Traces, b.ID) && live(f) {
				fs = append(fs, f)
			}
		}
		if len(fs) == 0 {
			fmt.Printf("| %s %s%s | — | — | — |\n", b.ID, b.Title, mark(b))
		}
		for _, f := range fs {
			v := f.Verify
			if f.Verify == "test" {
				if len(tests[f.ID]) == 0 {
					v = "**no test**"
				} else {
					v = strings.Join(tests[f.ID], ", ")
				}
			}
			var ts []string
			for _, t := range items {
				if t.Layer == "technical" && slices.Contains(t.Traces, f.ID) {
					ts = append(ts, t.ID)
				}
			}
			fmt.Printf("| %s %s%s | %s %s%s | %s | %s |\n", b.ID, b.Title, mark(b), f.ID, f.Title, mark(f), v, strings.Join(ts, ", "))
		}
	}
}

func indexDir(root string) string { return filepath.Join(root, ".spec") }

func writeIndex(root string) error {
	items, refs, probs := load(root)
	dir := indexDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// write writes the rows of a file, failing on the first that fails.
	write := func(name string, rows func(enc func(any))) error {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		e := json.NewEncoder(f)
		var werr error
		rows(func(v any) {
			if werr == nil {
				werr = e.Encode(v)
			}
		})
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		return werr
	}
	if err := write("items.jsonl", func(e func(any)) {
		for _, it := range items {
			e(it)
		}
	}); err != nil {
		return err
	}
	if err := write("links.jsonl", func(e func(any)) {
		for _, it := range items {
			for _, t := range it.Traces {
				e(map[string]string{"from": it.ID, "to": t, "kind": "traces"})
			}
		}
		for _, r := range refs {
			for _, id := range r.IDs {
				e(map[string]string{"from": r.File + "#" + r.Test, "to": id, "kind": "tests"})
			}
		}
	}); err != nil {
		return err
	}
	if err := write("tests.jsonl", func(e func(any)) {
		for _, r := range refs {
			for _, id := range r.IDs {
				e(map[string]any{"id": id, "test": r.Test, "file": r.File, "line": r.Line})
			}
		}
	}); err != nil {
		return err
	}
	return write("problems.jsonl", func(e func(any)) {
		for _, p := range probs {
			e(p)
		}
	})
}

// eventsGlob is where GoRex's wide events are.
func eventsGlob() string {
	if d := os.Getenv("GOREX_DIR"); d != "" {
		return filepath.Join(d, "events", "*.jsonl")
	}
	c, _ := os.UserConfigDir()
	return filepath.Join(c, "GoRex", "events", "*.jsonl")
}

// views are the tables of "spec query".
func views(root string) string {
	dir := indexDir(root)
	j := func(name string) string { return "'" + filepath.Join(dir, name) + "'" }
	var b strings.Builder
	fmt.Fprintf(&b, "create view items as select * from read_json_auto(%s);\n", j("items.jsonl"))
	fmt.Fprintf(&b, "create view links as select * from read_json_auto(%s);\n", j("links.jsonl"))
	fmt.Fprintf(&b, "create view tests as select * from read_json_auto(%s);\n", j("tests.jsonl"))
	if fi, err := os.Stat(filepath.Join(dir, "problems.jsonl")); err == nil && fi.Size() > 0 {
		fmt.Fprintf(&b, "create view problems as select * from read_json_auto(%s);\n", j("problems.jsonl"))
	}
	if fi, err := os.Stat(filepath.Join(dir, "test-results.jsonl")); err == nil && fi.Size() > 0 {
		fmt.Fprintf(&b, "create view test_results as select * from read_json_auto(%s, union_by_name=true) where Test is not null;\n", j("test-results.jsonl"))
	}
	if m, _ := filepath.Glob(eventsGlob()); len(m) > 0 {
		fmt.Fprintf(&b, "create view events as select * from read_json_auto('%s', union_by_name=true, sample_size=-1);\n", eventsGlob())
	}
	return b.String()
}

func cmdQuery(root string, args []string) int {
	if err := writeIndex(root); err != nil {
		die(err)
	}
	if _, err := exec.LookPath("duckdb"); err != nil {
		die(errors.New("duckdb is not installed"))
	}
	init := filepath.Join(indexDir(root), "views.sql")
	if err := os.WriteFile(init, []byte(views(root)), 0o644); err != nil {
		die(err)
	}
	var c *exec.Cmd
	if len(args) == 0 {
		c = exec.Command("duckdb", "-init", init)
	} else {
		c = exec.Command("duckdb", "-c", views(root)+strings.Join(args, " "))
	}
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		die(err)
	}
	return 0
}

// cmdTest runs a go test -json command, keeps its output for queries as
// test_results, and prints what failed with its output, or a line that
// all passed.
func cmdTest(root string, args []string) int {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		args = []string{"go", "test", "-json", "./..."}
	}
	os.MkdirAll(indexDir(root), 0o755)
	out, err := os.Create(filepath.Join(indexDir(root), "test-results.jsonl"))
	if err != nil {
		die(err)
	}
	defer out.Close()
	c := exec.Command(args[0], args[1:]...)
	c.Stderr = os.Stderr
	pipe, _ := c.StdoutPipe()
	if err := c.Start(); err != nil {
		die(err)
	}
	type key struct{ pkg, test string }
	output := map[key][]string{}
	var failed []key
	pass, fail, skip := 0, 0, 0
	sc := bufio.NewScanner(pipe)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Bytes()
		out.Write(append(line, '\n'))
		var ev struct{ Action, Package, Test, Output string }
		if json.Unmarshal(line, &ev) != nil {
			fmt.Println(string(line)) // a build error, as text
			continue
		}
		k := key{ev.Package, ev.Test}
		switch ev.Action {
		case "output":
			output[k] = append(output[k], ev.Output)
		case "pass":
			if ev.Test != "" {
				pass++
			}
			delete(output, k)
		case "skip":
			skip++
			delete(output, k)
		case "fail":
			if ev.Test != "" {
				fail++
			}
			failed = append(failed, k)
		}
	}
	err = c.Wait()
	for _, k := range failed {
		if k.test == "" && fail > 0 {
			continue // the package failed through its tests
		}
		fmt.Printf("--- FAIL %s %s\n%s", k.pkg, k.test, strings.Join(output[k], ""))
	}
	fmt.Printf("tests: %d passed, %d failed, %d skipped\n", pass, fail, skip)
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		return 1
	}
	return 0
}

// The files of spec/ that hold no items.
var notItems = map[string]bool{"readme.md": true, "glossary.md": true, "findings.md": true, "questions.md": true}

var (
	sources = []string{"owner", "rex-docs", "repo", "code", "tests"}
	// A requirement is a sentence with SHALL. In one, these words blur
	// what must hold; in a description they are fine.
	requirementRe = regexp.MustCompile(`\bSHALL\b`)
	vagueReq      = regexp.MustCompile(`(?i)\b(should|may|might|can|could|will|would|typically|usually|normally|as needed|appropriate|if possible)\b`)
	// Vague anywhere.
	vagueAll = regexp.MustCompile(`(?i)\betc\b\.?|\band/or\b`)
	fillerRe = regexp.MustCompile(`(?i)\b(note that|in order to|it is important (that|to)|basically|simply|just)\b`)
	fenceRe  = regexp.MustCompile("(?s)```.*?```")
	codeRe   = regexp.MustCompile("`[^`]*`")
	linkRe   = regexp.MustCompile(`\]\([^)]*\)|https?://\S+`)
	listRe   = regexp.MustCompile(`^\s*(?:[-*]|\d+[a-z]?\.)\s+`)
	sentRe   = regexp.MustCompile(`[.!?;]\s+`)
)

// Lengths past which check --style asks whether a sentence or an item is
// still easy to read. Not limits.
const (
	longSentence = 30
	longItem     = 200
)

// prose is the text of markdown without code, links and tables: what is
// written in words.
func prose(md string) string {
	md = fenceRe.ReplaceAllString(md, " ")
	md = codeRe.ReplaceAllString(md, "CODE")
	md = linkRe.ReplaceAllString(md, "]")
	var out []string
	for _, l := range strings.Split(md, "\n") {
		if t := strings.TrimSpace(l); strings.HasPrefix(t, "|") || strings.HasPrefix(t, "#") {
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// words counts what a reader reads: prose and tables, not code blocks.
func words(md string) int {
	md = fenceRe.ReplaceAllString(md, " ")
	n := 0
	for _, l := range strings.Split(md, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "|") && strings.Trim(t, "|-: ") == "" {
			continue // a table's rule
		}
		n += len(strings.Fields(strings.ReplaceAll(l, "|", " ")))
	}
	return n
}

// sentences splits prose into its sentences: list items, numbered steps
// and paragraphs end them too.
func sentences(text string) []string {
	var units []string
	var cur []string
	end := func() {
		if len(cur) > 0 {
			units = append(units, strings.Join(cur, " "))
			cur = nil
		}
	}
	for _, l := range strings.Split(text, "\n") {
		switch {
		case strings.TrimSpace(l) == "":
			end()
		case listRe.MatchString(l):
			end()
			cur = append(cur, listRe.ReplaceAllString(l, ""))
		default:
			cur = append(cur, strings.TrimSpace(l))
		}
	}
	end()
	var out []string
	for _, u := range units {
		for _, s := range sentRe.Split(u, -1) {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

// Term is a glossary entry.
type Term struct {
	Term  string
	Avoid []string
}

// parseGlossary reads spec/glossary.md: a heading per term, and an
// optional "avoid: a, b" line of words not to use for it.
func parseGlossary(root string) ([]Term, []Problem) {
	b, err := os.ReadFile(filepath.Join(root, "spec", "glossary.md"))
	if err != nil {
		return nil, nil
	}
	var terms []Term
	var probs []Problem
	seen := map[string]int{}
	for i, l := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(t, "### "):
			name := strings.ToLower(strings.Trim(strings.TrimPrefix(t, "### "), " `"))
			if n, ok := seen[name]; ok {
				probs = append(probs, Problem{"error", "spec/glossary.md", i + 1, "", fmt.Sprintf("%q is defined twice, also at line %d", name, n)})
			}
			seen[name] = i + 1
			terms = append(terms, Term{Term: name})
		case strings.HasPrefix(t, "avoid:") && len(terms) > 0:
			for _, w := range strings.Split(strings.TrimPrefix(t, "avoid:"), ",") {
				if w = strings.ToLower(strings.TrimSpace(w)); w != "" {
					terms[len(terms)-1].Avoid = append(terms[len(terms)-1].Avoid, w)
				}
			}
		}
	}
	// A word avoided for one term must not be another term.
	for _, t := range terms {
		for _, w := range t.Avoid {
			if _, ok := seen[w]; ok {
				probs = append(probs, Problem{"error", "spec/glossary.md", seen[t.Term], "", fmt.Sprintf("%q is a term, and avoided for %q", w, t.Term)})
			}
		}
	}
	return terms, probs
}

// checkWriting warns where the writing can mislead: vague words in a
// requirement, terms the glossary avoids, unknown sources. Length is a
// style note, shown with check --style: readability matters, the numbers
// only hint at it.
func checkWriting(items []*Item, gl []Term) []Problem {
	var probs []Problem
	type avoided struct {
		re   *regexp.Regexp
		term string
	}
	var avoid []avoided
	// Longer terms that contain an avoided word are fine: "tab bar" is a
	// term though "tab" is avoided.
	var compound []*regexp.Regexp
	for _, t := range gl {
		for _, w := range t.Avoid {
			avoid = append(avoid, avoided{regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(w) + `s?\b`), w + " (use " + t.Term + ")"})
		}
		if strings.Contains(t.Term, " ") {
			compound = append(compound, regexp.MustCompile(`(?i)\b`+regexp.QuoteMeta(t.Term)+`s?\b`))
		}
	}
	for _, it := range items {
		if it.Status == "rejected" || it.Status == "superseded" {
			continue
		}
		warn := func(msg string) { probs = append(probs, Problem{"warning", it.File, it.Line, it.ID, msg}) }
		style := func(msg string) { probs = append(probs, Problem{"style", it.File, it.Line, it.ID, msg}) }
		text := prose(it.Title + "\n" + it.Body)
		var vague, filler []string
		for _, sn := range sentences(prose(it.Body)) {
			if requirementRe.MatchString(sn) {
				vague = append(vague, vagueReq.FindAllString(sn, -1)...)
			}
			vague = append(vague, vagueAll.FindAllString(sn, -1)...)
			filler = append(filler, fillerRe.FindAllString(sn, -1)...)
			if n := len(strings.Fields(sn)); n > longSentence {
				style(fmt.Sprintf("a sentence of %d words: %q", n, clip(sn, 60)))
			}
		}
		if len(vague) > 0 {
			warn("vague in a requirement: " + strings.Join(uniq(vague), ", ") + " (say SHALL, or say exactly)")
		}
		if len(filler) > 0 {
			style("filler: " + strings.Join(uniq(filler), ", "))
		}
		if it.Words > longItem {
			style(fmt.Sprintf("%d words: does it say one thing?", it.Words))
		}
		masked := text
		for _, re := range compound {
			masked = re.ReplaceAllString(masked, "TERM")
		}
		for _, a := range avoid {
			if a.re.MatchString(masked) {
				warn("avoided term: " + a.term)
			}
		}
		if len(it.Source) == 0 {
			warn("no source: (" + strings.Join(sources, ", ") + ")")
		}
		for _, src := range it.Source {
			if !slices.Contains(sources, src) {
				warn(fmt.Sprintf("source %q is not one of %s", src, strings.Join(sources, ", ")))
			}
		}
	}
	return probs
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func uniq(ws []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, w := range ws {
		w = strings.ToLower(w)
		if !seen[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	return out
}

// cmdStats prints the size of each area, and the time it takes to read:
// what a reviewer faces.
func cmdStats(root string) {
	items, refs, _ := load(root)
	type area struct {
		layer, name string
		n, words    int
		tested      int
	}
	tested := map[string]bool{}
	for _, r := range refs {
		for _, id := range r.IDs {
			tested[id] = true
		}
	}
	byArea := map[string]*area{}
	var order []string
	total := 0
	for _, it := range items {
		k := it.Layer + "/" + it.Area
		a, ok := byArea[k]
		if !ok {
			a = &area{layer: it.Layer, name: it.Area}
			byArea[k] = a
			order = append(order, k)
		}
		a.n++
		a.words += it.Words
		total += it.Words
		if tested[it.ID] {
			a.tested++
		}
	}
	sort.Strings(order)
	fmt.Printf("%-12s %-18s %6s %7s %8s %7s\n", "layer", "area", "items", "words", "minutes", "tested")
	for _, k := range order {
		a := byArea[k]
		fmt.Printf("%-12s %-18s %6d %7d %8.0f %7d\n", a.layer, a.name, a.n, a.words, float64(a.words)/200, a.tested)
	}
	fmt.Printf("%-31s %6d %7d %8.0f\n", "total", len(items), total, float64(total)/200)
}
