package main

import (
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
)

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
	for _, it := range items {
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
