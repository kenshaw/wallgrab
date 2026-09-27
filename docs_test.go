package main_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// These tests read the repository, not the command. They hold the layout that
// D26 set up, and they need no network.

// TestClaudeImportsAgents holds D26. AGENTS.md holds the rules, because Codex
// and the other agents read it, and CLAUDE.md imports it, so that Claude Code
// reads the same rules. A rule written in CLAUDE.md reaches Claude Code alone.
// A symbolic link is refused for the reason in TestSkillsAreCopies.
func TestClaudeImportsAgents(t *testing.T) {
	t.Parallel()
	fi, err := os.Lstat("CLAUDE.md")
	switch {
	case err != nil:
		t.Fatal(err)
	case !fi.Mode().IsRegular():
		t.Fatalf("CLAUDE.md is not an ordinary file, got mode %v. Make it a file that holds @AGENTS.md. See D26", fi.Mode())
	}
	if s := strings.TrimSuffix(read(t, "CLAUDE.md"), "\n"); s != "@AGENTS.md" {
		t.Errorf("CLAUDE.md holds %q. It holds only @AGENTS.md, and the rules go in AGENTS.md. See D26", s)
	}
	if _, err := os.Stat("AGENTS.md"); err != nil {
		t.Errorf("CLAUDE.md imports AGENTS.md, which is missing: %v", err)
	}
}

// TestTheRootHoldsFourDocuments holds D26. A document that appears in the
// root is one that nobody filed.
func TestTheRootHoldsFourDocuments(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		"README.md": true, "AGENTS.md": true, "CLAUDE.md": true, "CONTRIBUTING.md": true,
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if !allowed[e.Name()] {
			t.Errorf("%s is in the repository root. Only README, AGENTS, CLAUDE and CONTRIBUTING belong there, "+
				"and everything else goes in docs/. See D26", e.Name())
		}
	}
	for name := range allowed {
		if _, err := os.Stat(name); err != nil {
			t.Errorf("expected %s in the repository root", name)
		}
	}
}

// decision is one decision heading in docs/PLAN.md, such as:
//
//	### D10. The CA bundle is downloaded at run time. Decided. Amends D8.
type decision struct {
	num     string
	title   string
	status  string
	heading string
}

// decisionHeading matches a heading that names a decision.
var decisionHeading = regexp.MustCompile(`(?m)^### (D(\d+)\. .*)$`)

// headingParts splits a heading into its title and its status. The status
// starts with Decided or Proposed, and each later part names another
// decision.
var headingParts = regexp.MustCompile(`^D\d+\. (.+?)\. ((?:Decided|Proposed)(?:\. (?:Amends|Amended by|Supersedes|Superseded by) D\d+)*)\.$`)

// decisions reads every decision heading in docs/PLAN.md, in order. The
// numbers start at 1 and have no gap, because the log is append only.
func decisions(t *testing.T) []decision {
	t.Helper()
	var out []decision
	for _, m := range decisionHeading.FindAllStringSubmatch(read(t, planFile), -1) {
		parts := headingParts.FindStringSubmatch(m[1])
		if parts == nil {
			t.Errorf("%s: %q is not \"### D<n>. <Title>. <Status>.\". The status starts with Decided or Proposed,"+
				" then \"Amends D<n>\" or \"Amended by D<n>\" where they apply", planFile, m[0])
			continue
		}
		if want := strconv.Itoa(len(out) + 1); m[2] != want {
			t.Errorf("%s: D%s comes where D%s belongs. The numbers start at 1 and have no gap", planFile, m[2], want)
		}
		out = append(out, decision{num: m[2], title: parts[1], status: parts[2], heading: m[1]})
	}
	if len(out) == 0 {
		t.Fatalf("%s holds no decision heading, so this guards nothing", planFile)
	}
	return out
}

// planFile holds the decisions. dbmeta D111 keeps the decisions of a small
// project in one file.
var planFile = filepath.Join("docs", "PLAN.md")

// TestTheDecisionIndexIsComplete checks the table at the top of docs/PLAN.md
// against the headings. A reader finds a decision by its number in that
// table, so a missing row or a stale status hides it.
func TestTheDecisionIndexIsComplete(t *testing.T) {
	t.Parallel()
	rows := make(map[string]string)
	for _, m := range regexp.MustCompile(`(?m)^\| \[D(\d+)\]\(.*$`).FindAllStringSubmatch(read(t, planFile), -1) {
		if _, ok := rows[m[1]]; ok {
			t.Errorf("%s has two rows for D%s", planFile, m[1])
		}
		rows[m[1]] = m[0]
	}
	written := make(map[string]bool)
	for _, d := range decisions(t) {
		written[d.num] = true
		want := fmt.Sprintf("| [D%s](#%s) | %s | %s |", d.num, anchor(d.heading), d.title, d.status)
		switch got, ok := rows[d.num]; {
		case !ok:
			t.Errorf("D%s has no row in the index in %s. Add:\n%s", d.num, planFile, want)
		case got != want:
			t.Errorf("D%s: the row in the index in %s is\n%s\nand the heading says\n%s", d.num, planFile, got, want)
		}
	}
	for num := range rows {
		if !written[num] {
			t.Errorf("the index in %s has a row for D%s, and no heading holds it", planFile, num)
		}
	}
}

// anchor returns the anchor that GitHub gives a heading. It keeps the letters,
// the digits, the hyphens and the underscores in lower case, makes each space
// a hyphen, and drops every other character.
func anchor(heading string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(heading) {
		switch {
		case r == ' ':
			b.WriteRune('-')
		case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// TestAnAmendmentPointsBothWays checks that a decision that changes an
// earlier one says so, and that the earlier one says it back. A reader who
// finds the older decision must see at once that it no longer holds alone.
func TestAnAmendmentPointsBothWays(t *testing.T) {
	t.Parallel()
	all := decisions(t)
	status := make(map[string]string, len(all))
	for _, d := range all {
		status[d.num] = d.status
	}
	inverse := map[string]string{
		"Amends":        "Amended by",
		"Amended by":    "Amends",
		"Supersedes":    "Superseded by",
		"Superseded by": "Supersedes",
	}
	naming := regexp.MustCompile(`\b(Amends|Amended by|Supersedes|Superseded by) D(\d+)\b`)
	for _, d := range all {
		for _, m := range naming.FindAllStringSubmatch(d.status, -1) {
			other, ok := status[m[2]]
			switch {
			case !ok:
				t.Errorf("D%s says %q, and D%s is not a decision", d.num, m[0], m[2])
				continue
			case m[2] == d.num:
				t.Errorf("D%s names itself: %q", d.num, m[0])
				continue
			}
			want := inverse[m[1]] + " D" + d.num
			if !regexp.MustCompile(`\b` + want + `\b`).MatchString(other) {
				t.Errorf("D%s says %q, and the status of D%s is %q. Add %q to the heading of D%s and to its row",
					d.num, m[0], m[2], other, want, m[2])
			}
		}
	}
}

// TestEveryProposedDecisionHasAQuestion checks that each decision that Ken
// has not made is named in "Open questions for Ken". A proposed decision that
// no question names is one that nobody asks him about.
func TestEveryProposedDecisionHasAQuestion(t *testing.T) {
	t.Parallel()
	_, questions, ok := strings.Cut(read(t, planFile), "\n## Open questions for Ken\n")
	if !ok {
		t.Fatalf("%s has no section called \"Open questions for Ken\"", planFile)
	}
	for _, d := range decisions(t) {
		if !strings.HasPrefix(d.status, "Proposed") {
			continue
		}
		if !regexp.MustCompile(`\bD` + d.num + `\b`).MatchString(questions) {
			t.Errorf("D%s is Proposed, and no open question in %s names it. Ask Ken with a question that names D%s",
				d.num, planFile, d.num)
		}
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
