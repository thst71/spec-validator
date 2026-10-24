package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const greetingFeature = `@FR-1.1
Feature: Greeting
  Background:
    Given a running app

  @AT-1.1
  Scenario: Greet
    Then I see "Hello"

  @AT-1.2
  Scenario Outline: Greet <name>
    Then I see <name>
    Examples:
      | name  |
      | Alice |
      | Bob   |

  @AT-1.3
  Scenario: Not run
    Then nothing
`

// bdtTree is a valid tree with the greeting feature file and the fixture results.
func bdtTree(t *testing.T) (root, results string) {
	t.Helper()
	root = validTree().write(t)
	writeFile(t, root, "specification/acceptance/greeting.feature", greetingFeature)
	data, err := os.ReadFile("testdata/results.json")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "results.json", string(data))
	return root, filepath.Join(root, "results.json")
}

func TestElementStatus(t *testing.T) {
	cases := []struct {
		steps []string
		want  string
	}{
		{[]string{"passed", "passed"}, "passed"},
		{[]string{"passed", "failed", "skipped"}, "failed"},
		{[]string{"undefined", "skipped"}, "undefined"},
		{[]string{"pending"}, "pending"},
		{[]string{"skipped"}, "skipped"},
		{nil, "passed"},
	}
	for _, c := range cases {
		var e cucumberElement
		for _, s := range c.steps {
			step := cucumberStep{}
			step.Result.Status = s
			e.Steps = append(e.Steps, step)
		}
		if got := e.status(); got != c.want {
			t.Errorf("steps %q: want %s, got %s", c.steps, c.want, got)
		}
	}
}

func TestWorstStatus(t *testing.T) {
	if got := worst([]string{"passed", "skipped", "failed", "pending"}); got != "failed" {
		t.Errorf("want failed, got %s", got)
	}
	if got := worst(nil); got != "not run" {
		t.Errorf("want not run, got %s", got)
	}
}

func TestEscapeInline(t *testing.T) {
	if got := inline(`a *b* <<x>> ] {attr}`); got != `pass:c[a *b* <<x>> \] {attr}]` {
		t.Errorf("got %s", got)
	}
	if got := inline(""); got != "" {
		t.Errorf("empty text: got %q", got)
	}
}

func TestTestReport(t *testing.T) {
	root, results := bdtTree(t)

	problems, err := TestReport(root, bdtConfig(), results, "2026-10-24")
	if err != nil || len(problems) > 0 {
		t.Fatalf("got %q, %v", problems, err)
	}

	got, err := os.ReadFile(filepath.Join(root, DefaultTestReport))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/test-report.adoc")
	if err != nil {
		t.Fatal(err)
	}
	// Git may check the golden file out with CRLF (Windows)
	if string(got) != strings.ReplaceAll(string(want), "\r\n", "\n") {
		_ = os.WriteFile("testdata/test-report.adoc.got", got, 0o644)
		t.Errorf("report differs from testdata/test-report.adoc; see testdata/test-report.adoc.got")
	}
}

func TestTestReportRefusesAnInvalidSpecification(t *testing.T) {
	root, results := bdtTree(t)
	writeFile(t, root, "specification/acceptance/broken.feature", "Feature: B\n  Scenario: no id\n")

	problems, err := TestReport(root, bdtConfig(), results, "2026-10-24")
	if err != nil || len(problems) == 0 {
		t.Fatalf("want problems, got %q, %v", problems, err)
	}
	if _, err := os.Stat(filepath.Join(root, DefaultTestReport)); err == nil {
		t.Error("report written for an invalid specification")
	}
}

func TestTestReportUnreadableResults(t *testing.T) {
	root, _ := bdtTree(t)
	writeFile(t, root, "bad.json", "{")

	if _, err := TestReport(root, bdtConfig(), filepath.Join(root, "bad.json"), "2026-10-24"); err == nil || !strings.Contains(err.Error(), "bad.json") {
		t.Errorf("want an error naming the file, got %v", err)
	}
}

// pytest-bdd writes tags without the leading @ and URIs relative to its features folder.
func TestResultsWithoutAtSign(t *testing.T) {
	root, _ := bdtTree(t)
	writeFile(t, root, "pytest-bdd.json", `[{"uri": "greeting.feature", "name": "Greeting", "tags": [{"name": "FR-1.1", "line": 1}], "elements": [
	  {"type": "scenario", "name": "Greet", "line": 7, "tags": [{"name": "AT-1.1", "line": 6}],
	   "steps": [{"keyword": "Then", "name": "I see \"Hello\"", "result": {"status": "passed"}}]}]}]`)

	run, err := readResults(filepath.Join(root, "pytest-bdd.json"), bdtConfig())
	if err != nil {
		t.Fatal(err)
	}
	if got := run.statusOf("AT-1.1"); got != "passed" {
		t.Errorf("want passed, got %s", got)
	}
	if len(run.unmatched) != 0 {
		t.Errorf("want no unmatched results, got %+v", run.unmatched)
	}
}
