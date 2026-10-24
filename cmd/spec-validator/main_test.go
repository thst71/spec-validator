package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func execute(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

// project writes a one-document project; content is its single item file.
func project(t *testing.T, content string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "req.adoc", "include::req/item.adoc[]\n")
	writeFile(t, root, "req/item.adoc", content)
	writeFile(t, root, ".spec-validator.json", `{"documents": [{"master": "req.adoc", "folder": "req", "kinds": ["R"]}]}`)
	return root
}

func TestValidProject(t *testing.T) {
	code, stdout, _ := execute(project(t, "[[R-1]]\n=== One\n"))

	if code != exitValid || !strings.Contains(stdout, "specifications are valid") {
		t.Errorf("got %d, %q", code, stdout)
	}
}

func TestProblems(t *testing.T) {
	code, _, stderr := execute(project(t, "[[R-1]]\n=== One <<R-2>>\n"))

	if code != exitProblems {
		t.Errorf("want exit %d, got %d", exitProblems, code)
	}
	if !strings.Contains(stderr, "req/item.adoc: unresolved reference <<R-2>>") || !strings.Contains(stderr, "1 problem(s) found") {
		t.Errorf("unexpected output %q", stderr)
	}
}

func TestExplicitConfig(t *testing.T) {
	root := project(t, "[[R-1]]\n=== One\n")
	writeFile(t, root, "other.json", `{"documents": [{"master": "missing.adoc", "folder": "req", "kinds": ["R"]}]}`)

	code, _, stderr := execute("-config", filepath.Join(root, "other.json"), root)

	if code != exitProblems || !strings.Contains(stderr, "missing.adoc: master file is missing") {
		t.Errorf("got %d, %q", code, stderr)
	}
}

func TestUsageErrors(t *testing.T) {
	root := project(t, "[[R-1]]\n=== One\n")
	writeFile(t, root, "broken.json", `{"documents": `)
	cases := map[string][]string{
		"two roots":      {root, root},
		"no directory":   {filepath.Join(root, "req.adoc")},
		"unknown flag":   {"-strict", root},
		"broken config":  {"-config", filepath.Join(root, "broken.json"), root},
		"missing config": {"-config", filepath.Join(root, "none.json"), root},
		"two modes":      {"-fix", "-masters", root},
		"missing link":   {"-link", filepath.Join(root, "none"), root},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			if code, _, _ := execute(args...); code != exitUsage {
				t.Errorf("want exit %d, got %d", exitUsage, code)
			}
		})
	}
}

func TestVersion(t *testing.T) {
	code, stdout, _ := execute("-version")

	if code != exitValid || strings.TrimSpace(stdout) == "" {
		t.Errorf("got %d, %q", code, stdout)
	}
}

func TestFix(t *testing.T) {
	root := project(t, "[#R-1]\n=== One\n")

	code, stdout, stderr := execute("-fix", root)

	if code != exitValid || !strings.Contains(stdout, "fixed anchors in req/item.adoc") {
		t.Errorf("got %d, %q, %q", code, stdout, stderr)
	}
}

func TestMasters(t *testing.T) {
	code, stdout, _ := execute("-masters", project(t, "[[R-1]]\n=== One\n"))

	if code != exitValid || stdout != "req.adoc\n" {
		t.Errorf("got %d, %q", code, stdout)
	}
}

func TestLink(t *testing.T) {
	root := project(t, "[[R-1]]\n=== One\n")
	out := t.TempDir()
	writeFile(t, out, "req.adoc", "[[R-1]]\n=== One <<R-1>>\n")

	code, stdout, stderr := execute("-link", out, root)

	if code != exitValid || !strings.Contains(stdout, "references linked") {
		t.Errorf("got %d, %q, %q", code, stdout, stderr)
	}
}

func TestLinkWithProblems(t *testing.T) {
	root := project(t, "[[R-1]]\n=== One <<R-2>>\n")

	if code, _, _ := execute("-link", t.TempDir(), root); code != exitProblems {
		t.Errorf("want exit %d, got %d", exitProblems, code)
	}
}

// bdtProject writes a project with one requirement and acceptance tests in feature files.
func bdtProject(t *testing.T, feature string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "req.adoc", "include::req/item.adoc[]\n")
	writeFile(t, root, "req/item.adoc", "[[FR-1]]\n=== One\n")
	writeFile(t, root, "acceptance/one.feature", feature)
	writeFile(t, root, ".spec-validator.json", `{"documents": [{"master": "req.adoc", "folder": "req", "kinds": ["FR"]}],
 "tests": [{"kind": "AT", "folder": "acceptance", "format": "gherkin", "traces": ["FR"]}]}`)
	return root
}

func TestFeatureFiles(t *testing.T) {
	code, stdout, _ := execute(bdtProject(t, "@FR-1\nFeature: One\n  @AT-1\n  Scenario: S\n    Then ok\n"))
	if code != exitValid || !strings.Contains(stdout, "specifications are valid") {
		t.Errorf("valid project: got %d, %q", code, stdout)
	}

	code, _, stderr := execute(bdtProject(t, "Feature: One\n  Scenario: S\n    Then ok\n"))
	for _, want := range []string{
		`acceptance/one.feature:2: scenario "S" has no AT tag`,
		`acceptance/one.feature:2: scenario "S" references no requirement`,
		"2 problem(s) found",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("missing %q in %q", want, stderr)
		}
	}
	if code != exitProblems {
		t.Errorf("want exit %d, got %d", exitProblems, code)
	}
}

const results = `[{"uri": "acceptance/one.feature", "name": "One", "elements": [
 {"type": "scenario", "name": "S", "line": 3, "tags": [{"name": "@AT-1"}],
  "steps": [{"keyword": "Then ", "name": "ok", "result": {"status": "failed"}}]}]}]`

func TestTestReport(t *testing.T) {
	root := bdtProject(t, "@FR-1\nFeature: One\n  @AT-1\n  Scenario: S\n    Then ok\n")
	writeFile(t, root, "results.json", results)

	code, stdout, stderr := execute("-test-report", filepath.Join(root, "results.json"), root)

	if code != exitValid || !strings.Contains(stdout, "test report written") {
		t.Fatalf("got %d, %q, %q", code, stdout, stderr)
	}
	report, err := os.ReadFile(filepath.Join(root, "test-report.adoc"))
	if err != nil || !strings.Contains(string(report), "Result:: failed") {
		t.Errorf("report %q, %v", report, err)
	}
}

func TestTestReportErrors(t *testing.T) {
	root := bdtProject(t, "@FR-1\nFeature: One\n  @AT-1\n  Scenario: S\n    Then ok\n")
	if code, _, _ := execute("-test-report", filepath.Join(root, "missing.json"), root); code != exitUsage {
		t.Errorf("missing results: want exit %d, got %d", exitUsage, code)
	}

	invalid := bdtProject(t, "Feature: One\n  Scenario: S\n    Then ok\n")
	writeFile(t, invalid, "results.json", results)
	if code, _, _ := execute("-test-report", filepath.Join(invalid, "results.json"), invalid); code != exitProblems {
		t.Errorf("invalid specification: want exit %d, got %d", exitProblems, code)
	}
	if code, _, _ := execute("-test-report", "r.json", "-masters", root); code != exitUsage {
		t.Errorf("two modes: want exit %d, got %d", exitUsage, code)
	}
}
