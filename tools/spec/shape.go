package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Every layer has a template (spec/README.md, "Templates"), so that every
// author, person or agent, writes an item the same way. A slot is a
// paragraph that starts with its label in bold: "**Needs.** …".
var (
	slotRe     = regexp.MustCompile(`(?m)^\*\*([A-Z][a-z]+(?: [a-z]+)*)\.\*\*`)
	stepRe     = regexp.MustCompile(`(?m)^\d+\.\s`)
	tableRe    = regexp.MustCompile(`(?m)^\|.*\|\s*$`)
	needSlots  = []string{"Who", "Needs", "Why", "Fails if"}
	limitSlots = []string{"Constraint", "Why", "Fails if"}
	adrSlots   = []string{"Context", "Options", "Choice", "Reason"}
	caseSlots  = []string{"Actor", "Goal"}
	kinds      = []string{"need", "constraint"}
	// A functional item is a function, or a quality the product must
	// have (Volere's non-functional requirements).
	fkinds = []string{"function", "performance", "security", "usability", "operational"}
	// EARS: a requirement starts with its conditions, each closed by a
	// comma: WHILE (a state), WHEN (an event), WHERE (a feature), IF …,
	// THEN (an unwanted case). Without one, it holds always.
	earsKey  = regexp.MustCompile(`\b(WHILE|WHEN|WHERE|IF|THEN)\b`)
	earsLead = regexp.MustCompile(`^(?:WHILE|WHEN|WHERE|IF)\b.*,\s*(?:THEN\s+)?[^,]*\bSHALL\b`)
	maxRules = 7
)

func conflicts(it *Item) []string { return idRe.FindAllString(it.Fields["conflicts"], -1) }

// earsProblem tells how a requirement sentence misses EARS, or "".
func earsProblem(sn string) string {
	keys := earsKey.FindAllString(sn, -1)
	if len(keys) == 0 {
		return ""
	}
	if !earsLead.MatchString(sn) {
		return "conditions first, each closed by a comma: WHEN …, the X SHALL …"
	}
	if slices.Contains(keys, "THEN") && !slices.Contains(keys, "IF") {
		return "THEN only after IF: WHEN …, the X SHALL … (or IF …, THEN …)"
	}
	if slices.Contains(keys, "IF") && !slices.Contains(keys, "THEN") {
		return "IF …, THEN the X SHALL …"
	}
	return ""
}

func slots(body string) []string {
	var s []string
	for _, m := range slotRe.FindAllStringSubmatch(body, -1) {
		s = append(s, m[1])
	}
	return s
}

func missing(have, want []string) []string {
	var m []string
	for _, w := range want {
		if !slices.Contains(have, w) {
			m = append(m, w)
		}
	}
	return m
}

// checkShape reports items that do not follow their layer's template.
// Rejected and superseded items are history and keep the shape they had.
func checkShape(items []*Item) []Problem {
	var probs []Problem
	ids := map[string]*Item{}
	for _, it := range items {
		ids[it.ID] = it
	}
	for _, it := range items {
		for _, c := range conflicts(it) {
			switch o := ids[c]; {
			case o == nil:
				probs = append(probs, Problem{"error", it.File, it.Line, it.ID, "conflicts with " + c + ", which the spec does not have"})
			case it.Status == "accepted" && o.Status == "accepted":
				probs = append(probs, Problem{"gap", it.File, it.Line, it.ID, "accepted, and conflicts with " + c + ", accepted too: resolve it"})
			}
		}
		if it.Status == "rejected" || it.Status == "superseded" || it.Body == "" {
			continue
		}
		bad := func(msg string) { probs = append(probs, Problem{"error", it.File, it.Line, it.ID, msg}) }
		have := slots(it.Body)
		need := func(want []string, form string) {
			if m := missing(have, want); len(m) > 0 {
				bad(form + " without **" + strings.Join(m, ".**, **") + ".** (spec/README.md, Templates)")
			}
		}
		switch it.Layer {
		case "business":
			kind := it.Fields["kind"]
			switch kind {
			case "", "need":
				need(needSlots, "a need")
			case "constraint":
				need(limitSlots, "a constraint")
			default:
				bad("kind " + kind + " is not one of " + strings.Join(kinds, ", "))
			}
		case "decision":
			need(adrSlots, "a decision")
		case "functional":
			if k := it.Fields["kind"]; k != "" && !slices.Contains(fkinds, k) {
				bad("kind " + k + " is not one of " + strings.Join(fkinds, ", "))
			}
			rules := 0
			for _, sn := range sentences(prose(it.Body)) {
				if !requirementRe.MatchString(sn) {
					continue
				}
				rules++
				if e := earsProblem(sn); e != "" {
					probs = append(probs, Problem{"warning", it.File, it.Line, it.ID, "not EARS: " + e + ": " + clip(sn, 50)})
				}
			}
			if rules > maxRules {
				probs = append(probs, Problem{"style", it.File, it.Line, it.ID, fmt.Sprintf("%d rules: one use case, or several?", rules)})
			}
			if slices.Contains(have, "Goal") || slices.Contains(have, "Actor") {
				need(caseSlots, "a use case")
				if !stepRe.MatchString(it.Body) {
					bad("a use case without numbered steps")
				}
				continue
			}
			rule := false
			for _, sn := range sentences(prose(it.Body)) {
				if requirementRe.MatchString(sn) {
					rule = true
				}
			}
			if !rule && !tableRe.MatchString(it.Body) {
				bad("neither a rule (SHALL, WHEN … THEN), a use case (**Actor.**, **Goal.**, steps) nor a table")
			}
		}
	}
	return probs
}

// The reference (spec/reference/*.md) is made from the code: each heading
// that is a code span names something the code has, as "## `rex split`",
// with its other names on an "Also:" line. Each needs a functional item
// that names it: code with no stated intent is a gap.
var (
	refHeadRe = regexp.MustCompile("^##\\s+`([^`]+)`\\s*$")
	refAlsoRe = regexp.MustCompile("`([^`]+)`")
)

type refName struct {
	names []string
	file  string
	line  int
}

func parseReference(root string) []refName {
	var out []refName
	files, _ := filepath.Glob(filepath.Join(root, "spec", "reference", "*.md"))
	for _, path := range files {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(root, path)
		for n, line := range strings.Split(string(b), "\n") {
			if m := refHeadRe.FindStringSubmatch(line); m != nil {
				out = append(out, refName{names: []string{m[1]}, file: rel, line: n + 1})
			} else if strings.HasPrefix(line, "Also:") && len(out) > 0 {
				for _, m := range refAlsoRe.FindAllStringSubmatch(line, -1) {
					out[len(out)-1].names = append(out[len(out)-1].names, m[1])
				}
			}
		}
	}
	return out
}

func checkReference(items []*Item, refs []refName) []Problem {
	var text strings.Builder
	for _, it := range items {
		if it.Layer == "functional" && it.Status != "rejected" && it.Status != "superseded" {
			text.WriteString(it.Title + "\n" + it.Body + "\n")
		}
	}
	all := text.String()
	var probs []Problem
	for _, r := range refs {
		named := false
		for _, n := range r.names {
			if regexp.MustCompile("`" + regexp.QuoteMeta(n) + "[ `]").MatchString(all) {
				named = true
				break
			}
		}
		if !named {
			probs = append(probs, Problem{"gap", r.file, r.line, "", "no functional item names `" + r.names[0] + "`: what is it for?"})
		}
	}
	return probs
}
