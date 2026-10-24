package spec

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestMastersWithReports(t *testing.T) {
	root := validTree().write(t)
	configured := []string{"specification-web.adoc", "tech-specification-web.adoc", "backlog-web.adoc", "release-notes.adoc"}

	if got := Masters(root, bdtConfig()); !slices.Equal(got, configured) {
		t.Errorf("without reports: %q", got)
	}

	writeFile(t, root, DefaultTraceReport, "= T\n")
	writeFile(t, root, DefaultTestReport, "= T\n")
	want := slices.Concat(configured, []string{DefaultTestReport, DefaultTraceReport})
	if got := Masters(root, bdtConfig()); !slices.Equal(got, want) {
		t.Errorf("with reports: want %q, got %q", want, got)
	}
}

func TestLinkWithReports(t *testing.T) {
	tr := validTree()
	tr.story = story + "\nAccepted by <<AT-1.1>>.\n"
	root := tr.write(t)
	writeFile(t, root, "specification/acceptance/greeting.feature", greeting)
	if _, err := TestReport(root, bdtConfig(), writeResults(t, root), "2026-10-24"); err != nil {
		t.Fatal(err)
	}
	if _, err := TraceReport(root, bdtConfig(), "", "2026-10-24"); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	for _, master := range Masters(root, bdtConfig()) {
		data, err := os.ReadFile(filepath.Join(root, master))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, out, master, string(data))
	}
	// The flattened backlog master holds the story; flattening is not part of this test.
	writeFile(t, out, "backlog-web.adoc", tr.story)

	problems, err := Link(root, bdtConfig(), out)
	if err != nil || problems != nil {
		t.Fatal(problems, err)
	}

	for file, want := range map[string]string{
		"backlog-web.adoc": "Accepted by xref:test-report.adoc#AT-1.1[AT-1.1].",
		DefaultTestReport:  "Traces to:: xref:specification-web.adoc#FR-1.1[FR-1.1]",
		DefaultTraceReport: "| xref:specification-web.adoc#FR-1.1[FR-1.1] | xref:test-report.adoc#AT-1.1[AT-1.1]",
	} {
		data, err := os.ReadFile(filepath.Join(out, file))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), want) {
			t.Errorf("%s: missing %q in\n%s", file, want, data)
		}
	}
}

func writeResults(t *testing.T, root string) string {
	t.Helper()
	writeFile(t, root, "results.json", `[{"uri": "greeting.feature", "elements": [
	  {"type": "scenario", "name": "Greet", "line": 5, "tags": [{"name": "@AT-1.1"}],
	   "steps": [{"keyword": "Then ", "name": "hello", "result": {"status": "passed"}}]}]}]`)
	return filepath.Join(root, "results.json")
}
