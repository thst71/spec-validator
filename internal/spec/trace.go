package spec

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// coverageMap relates the requirements, the kinds that tests trace to, and the tests.
type coverageMap struct {
	requirements  []item
	byRequirement map[string][]string // requirement ID -> test IDs, in test order
	uncovered     []string
	orphaned      []testCase // tests without a trace to a requirement
}

func coverage(r result, config Config) coverageMap {
	var kinds []string
	for _, test := range config.Tests {
		kinds = append(kinds, test.Traces...)
	}
	c := coverageMap{byRequirement: map[string][]string{}}
	for _, it := range r.items {
		if slices.Contains(kinds, kind(it.id)) {
			c.requirements = append(c.requirements, it)
		}
	}
	for _, test := range r.tests {
		if len(test.traces) == 0 {
			c.orphaned = append(c.orphaned, test)
		}
		for _, id := range test.traces {
			c.byRequirement[id] = append(c.byRequirement[id], test.id)
		}
	}
	for _, it := range c.requirements {
		if len(c.byRequirement[it.id]) == 0 {
			c.uncovered = append(c.uncovered, it.id)
		}
	}
	return c
}

// TraceReport writes the traceability report master: requirements to tests, tests to
// requirements, orphaned tests and uncovered requirements. Problems in the documents leave it
// unwritten; problems in test files are what it reports. With results (Cucumber JSON, may be
// empty for none), every test shows its result.
func TraceReport(root string, config Config, results, date string) ([]string, error) {
	r, err := analyze(root, config)
	if err != nil {
		return nil, err
	}
	if documentProblems := r.problems[:len(r.problems)-len(r.testProblems)]; len(documentProblems) > 0 {
		return documentProblems, nil
	}
	var run *runResults
	if results != "" {
		loaded, err := readResults(results, config)
		if err != nil {
			return nil, err
		}
		run = &loaded
	}
	testsMaster, master := config.reportMasters()
	linked := isFile(filepath.Join(root, filepath.FromSlash(testsMaster)))
	test := func(id, name string) string {
		text := id
		if id == "" {
			text = "scenario " + inline(name)
		} else if linked {
			text = "<<" + id + ">>"
		}
		if run != nil {
			text += " (" + run.statusOf(id) + ")"
		}
		return text
	}
	c := coverage(r, config)

	var b strings.Builder
	header(&b, "Traceability Report", date)
	fmt.Fprintf(&b, "== Summary\n\n[cols=\"3,1\"]\n|===\n| Requirements | %d\n| Covered requirements | %d\n| Uncovered requirements | %d\n| Tests | %d\n| Orphaned tests | %d\n|===\n",
		len(c.requirements), len(c.requirements)-len(c.uncovered), len(c.uncovered), len(r.tests), len(c.orphaned))

	b.WriteString("\n== Requirements to Tests\n\n[cols=\"1,2\",options=\"header\"]\n|===\n| Requirement | Tests\n\n")
	for _, it := range c.requirements {
		var tests []string
		for _, id := range c.byRequirement[it.id] {
			tests = append(tests, test(id, ""))
		}
		fmt.Fprintf(&b, "| <<%s>> | %s\n", it.id, orNone(strings.Join(tests, ", ")))
	}
	b.WriteString("|===\n")

	b.WriteString("\n== Tests to Requirements\n\n[cols=\"1,2\",options=\"header\"]\n|===\n| Test | Requirements\n\n")
	for _, t := range r.tests {
		fmt.Fprintf(&b, "| %s | %s\n", test(t.id, t.name), orNone(references(t.traces)))
	}
	b.WriteString("|===\n")

	b.WriteString("\n== Orphaned Tests\n\n")
	if len(c.orphaned) == 0 {
		b.WriteString("none\n")
	}
	for _, t := range c.orphaned {
		fmt.Fprintf(&b, "* %s, `+%s+`\n", test(t.id, t.name), t.location())
	}

	b.WriteString("\n== Uncovered Requirements\n\n")
	if len(c.uncovered) == 0 {
		b.WriteString("none\n")
	}
	for _, id := range c.uncovered {
		fmt.Fprintf(&b, "* <<%s>>\n", id)
	}

	if len(r.testProblems) > 0 {
		b.WriteString("\n== Problems in Test Files\n\n")
		for _, problem := range r.testProblems {
			fmt.Fprintf(&b, "* %s\n", inline(problem))
		}
	}
	return nil, writeReport(root, master, b.String())
}

func orNone(text string) string {
	if text == "" {
		return "none"
	}
	return text
}
