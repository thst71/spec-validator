package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// traceTree has FR-1.1 (covered twice), FR-1.2 (covered), FR-1.3 (uncovered), an orphaned
// scenario and a scenario covering two requirements.
func traceTree(t *testing.T) string {
	t.Helper()
	tr := validTree()
	tr.requirement = requirement + "\n[[FR-1.2]]\n=== Second\n\n[[FR-1.3]]\n=== Third\n"
	root := tr.write(t)
	writeFile(t, root, "specification/acceptance/a.feature", `Feature: A
  @AT-1 @FR-1.1
  Scenario: One
    Then x

  @AT-2 @FR-1.1 @FR-1.2
  Scenario: Two
    Then x

  @AT-3 @FR-9.9
  Scenario: Orphan
    Then x
`)
	return root
}

func TestCoverage(t *testing.T) {
	r, err := analyze(traceTree(t), bdtConfig())
	if err != nil {
		t.Fatal(err)
	}
	c := coverage(r, bdtConfig())

	if got := strings.Join(c.byRequirement["FR-1.1"], ","); got != "AT-1,AT-2" {
		t.Errorf("FR-1.1 covered by %s", got)
	}
	if got := strings.Join(c.byRequirement["FR-1.2"], ","); got != "AT-2" {
		t.Errorf("FR-1.2 covered by %s", got)
	}
	if got := strings.Join(c.uncovered, ","); got != "FR-1.3" {
		t.Errorf("uncovered %s", got)
	}
	if len(c.orphaned) != 1 || c.orphaned[0].id != "AT-3" {
		t.Errorf("orphaned %+v", c.orphaned)
	}
	if got := strings.Join(requirementIDs(c.requirements), ","); got != "FR-1.1,FR-1.2,FR-1.3" {
		t.Errorf("requirements %s", got)
	}
}

func requirementIDs(items []item) []string {
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.id
	}
	return ids
}

func TestTraceReport(t *testing.T) {
	root := traceTree(t)

	problems, err := TraceReport(root, bdtConfig(), "", "2026-10-24")
	if err != nil || len(problems) > 0 {
		t.Fatalf("got %q, %v", problems, err)
	}

	assertGolden(t, filepath.Join(root, DefaultTraceReport), "testdata/traceability-report.adoc")
}

func TestTraceReportWithResultsAndTestReport(t *testing.T) {
	root := traceTree(t)
	writeFile(t, root, DefaultTestReport, "= Test Report\n")
	writeFile(t, root, "results.json", `[{"uri": "a.feature", "elements": [
	  {"type": "scenario", "name": "One", "tags": [{"name": "@AT-1"}], "steps": [{"result": {"status": "passed"}}]},
	  {"type": "scenario", "name": "Two", "tags": [{"name": "@AT-2"}], "steps": [{"result": {"status": "failed"}}]}]}]`)

	if _, err := TraceReport(root, bdtConfig(), filepath.Join(root, "results.json"), "2026-10-24"); err != nil {
		t.Fatal(err)
	}

	report, err := os.ReadFile(filepath.Join(root, DefaultTraceReport))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"| <<FR-1.1>> | <<AT-1>> (passed), <<AT-2>> (failed)",
		"| <<AT-3>> (not run) | none",
	} {
		if !strings.Contains(string(report), want) {
			t.Errorf("missing %q in\n%s", want, report)
		}
	}
}

func TestTraceReportRefusesInvalidDocuments(t *testing.T) {
	root := traceTree(t)
	writeFile(t, root, "specification/backlog/01-chapter/item.adoc", "[[US-1.1]]\n=== Story\n")

	problems, err := TraceReport(root, bdtConfig(), "", "2026-10-24")
	if err != nil || len(problems) == 0 {
		t.Fatalf("want problems, got %q, %v", problems, err)
	}
}

func assertGolden(t *testing.T, gotFile, wantFile string) {
	t.Helper()
	got, err := os.ReadFile(gotFile)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(wantFile)
	if err != nil || string(got) != string(want) {
		_ = os.WriteFile(wantFile+".got", got, 0o644)
		t.Errorf("%s differs from %s; see %s.got", gotFile, wantFile, wantFile)
	}
}
