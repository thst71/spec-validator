package spec

import (
	"path/filepath"
	"slices"
	"testing"
)

func bdtConfig() Config {
	c := DefaultConfig()
	c.Tests = []TestKind{{Kind: "AT", Folder: "specification/acceptance", Format: "gherkin", Traces: []string{"FR", "NFR"}}}
	return c
}

func validateBDT(t *testing.T, root string) []string {
	t.Helper()
	problems, err := Validate(root, bdtConfig())
	if err != nil {
		t.Fatal(err)
	}
	return problems
}

const greeting = "@FR-1.1\nFeature: Greeting\n\n  @AT-1.1\n  Scenario: Greet\n    Then hello\n"

func TestFeatureFilesValid(t *testing.T) {
	root := validTree().write(t)
	writeFile(t, root, "specification/acceptance/01-greeting/greeting.feature", greeting)

	if problems := validateBDT(t, root); len(problems) > 0 {
		t.Errorf("want no problems, got %q", problems)
	}
}

func TestWithoutTestsFeatureFilesAreIgnored(t *testing.T) {
	root := validTree().write(t)
	writeFile(t, root, "specification/acceptance/x.feature", "Feature: F\n  Scenario: no id\n")

	if problems := validate(t, root); len(problems) > 0 {
		t.Errorf("want no problems without tests, got %q", problems)
	}
}

func TestScenarioIDs(t *testing.T) {
	root := validTree().write(t)
	writeFile(t, root, "specification/acceptance/a.feature", "@FR-1.1\nFeature: A\n\n  Scenario: Missing\n    Then x\n\n  @AT-1 @AT-2\n  Scenario: Two\n    Then x\n\n  @AT-3\n  Scenario: First\n    Then x\n")
	writeFile(t, root, "specification/acceptance/b.feature", "@FR-1.1\nFeature: B\n  @AT-3\n  Scenario: Again\n    Then x\n")

	assertContains(t, validateBDT(t, root),
		`specification/acceptance/a.feature:4: scenario "Missing" has no AT tag`,
		`specification/acceptance/a.feature:8: scenario "Two" has 2 AT tags`,
		`specification/acceptance/b.feature:4: duplicate AT-3 (also in specification/acceptance/a.feature:12)`,
	)
}

func TestScenarioTraces(t *testing.T) {
	root := validTree().write(t)
	writeFile(t, root, "specification/acceptance/a.feature", "Feature: A\n\n  @AT-1 @FR-9.9\n  Scenario: Unknown\n    Then x\n\n  @AT-2 @US-1.1 @FR-1.1\n  Scenario: Story\n    Then x\n\n  @AT-3 @wip\n  Scenario: Nothing\n    Then x\n")

	problems := validateBDT(t, root)

	assertContains(t, problems,
		"specification/acceptance/a.feature:4: AT-1 traces to unknown FR-9.9",
		"specification/acceptance/a.feature:4: AT-1 references no requirement",
		"specification/acceptance/a.feature:8: AT-2 traces to US-1.1; AT traces to FR, NFR only",
		"specification/acceptance/a.feature:12: AT-3 references no requirement",
	)
	if slices.Contains(problems, "specification/acceptance/a.feature:8: AT-2 references no requirement") {
		t.Errorf("AT-2 traces to FR-1.1, got %q", problems)
	}
}

func TestTestIDOnFeatureOrExamples(t *testing.T) {
	root := validTree().write(t)
	writeFile(t, root, "specification/acceptance/a.feature", "@AT-9 @FR-1.1\nFeature: A\n  @AT-1\n  Scenario Outline: O\n    Then <x>\n    @AT-2\n    Examples:\n      | x |\n      | 1 |\n")

	assertContains(t, validateBDT(t, root),
		"specification/acceptance/a.feature:2: AT tag @AT-9 on Feature; put the ID on the scenario",
		"specification/acceptance/a.feature:7: AT tag @AT-2 on Examples; put the ID on the scenario",
	)
}

func TestReferenceToScenario(t *testing.T) {
	tr := validTree()
	tr.story = story + "\nAccepted by <<AT-1.1>> and <<AT-7>>.\n"
	root := tr.write(t)
	writeFile(t, root, "specification/acceptance/greeting.feature", greeting)

	problems := validateBDT(t, root)

	if want := []string{"specification/backlog/01-chapter/item.adoc: unresolved reference <<AT-7>>"}; !slices.Equal(problems, want) {
		t.Errorf("want %q, got %q", want, problems)
	}
}

func TestLanguageOfFeatureFile(t *testing.T) {
	root := validTree().write(t)
	writeFile(t, root, "specification/acceptance/a.feature", "# language: de\nFunktionalität: A\n")

	assertContains(t, validateBDT(t, root),
		"specification/acceptance/a.feature:1: language de is not supported; write feature files in English")
}

func TestLoadConfigTests(t *testing.T) {
	root := t.TempDir()
	docs := `"documents": [{"master": "a.adoc", "folder": "a", "kinds": ["FR"]}]`
	cases := map[string]string{
		"format":        `{` + docs + `, "tests": [{"kind": "AT", "folder": "t", "format": "asciidoc", "traces": ["FR"]}]}`,
		"no folder":     `{` + docs + `, "tests": [{"kind": "AT", "format": "gherkin", "traces": ["FR"]}]}`,
		"document kind": `{` + docs + `, "tests": [{"kind": "FR", "folder": "t", "format": "gherkin", "traces": ["FR"]}]}`,
		"bad kind":      `{` + docs + `, "tests": [{"kind": "at", "folder": "t", "format": "gherkin", "traces": ["FR"]}]}`,
		"no traces":     `{` + docs + `, "tests": [{"kind": "AT", "folder": "t", "format": "gherkin", "traces": []}]}`,
		"unknown trace": `{` + docs + `, "tests": [{"kind": "AT", "folder": "t", "format": "gherkin", "traces": ["XX"]}]}`,
		"twice":         `{` + docs + `, "tests": [{"kind": "AT", "folder": "t", "format": "gherkin", "traces": ["FR"]}, {"kind": "AT", "folder": "u", "format": "gherkin", "traces": ["FR"]}]}`,
		"report field":  `{` + docs + `, "reports": {"test": "x.adoc"}}`,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			writeFile(t, root, "config.json", content)
			if _, err := LoadConfig(filepath.Join(root, "config.json")); err == nil {
				t.Error("want an error")
			}
		})
	}

	writeFile(t, root, "config.json", `{`+docs+`, "tests": [{"kind": "AT", "folder": "t", "format": "gherkin", "traces": ["FR"]}], "reports": {"tests": "docs/tests.adoc"}}`)
	config, err := LoadConfig(filepath.Join(root, "config.json"))
	if err != nil {
		t.Fatalf("want a valid config, got %v", err)
	}
	if tests, trace := config.reportMasters(); tests != "docs/tests.adoc" || trace != "traceability-report.adoc" {
		t.Errorf("report masters %q, %q", tests, trace)
	}
	if err := bdtConfig().check(); err != nil {
		t.Errorf("BDT config is invalid: %v", err)
	}
}
