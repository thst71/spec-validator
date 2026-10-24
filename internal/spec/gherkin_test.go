package spec

import (
	"slices"
	"testing"
)

func tagNames(tags []tag) []string {
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.name
	}
	return names
}

func TestParseFeature(t *testing.T) {
	text := `# a comment
@FR-1.1 @wip
Feature: Greeting
  Some description.

  Background:
    Given a running app

  @AT-1.1
  Scenario: Greet a user
    When I open the page
    Then I see "Hello"

  @NFR-1
  Rule: Fast greetings

    @AT-1.2 @slow # trailing comment
    Example: Greet quickly
      Then the greeting appears within 1 s

    @AT-1.3
    Scenario Outline: Greet <name>
      Given the name "<name>"
      """
      Scenario: not a scenario
      """
      Then I see "Hello <name>"

      @FR-1.2
      Examples:
        | name  |
        | Alice |

  Rule: Other rule
    @AT-1.4
    Scenario Template: Templated
      Then nothing
`
	f, problems := parseFeature(text)

	if len(problems) > 0 {
		t.Fatalf("unexpected problems %q", problems)
	}
	if f.name != "Greeting" || f.line != 3 {
		t.Errorf("feature %q at %d", f.name, f.line)
	}
	want := []struct {
		name               string
		line               int
		own, inherited, ex []string
	}{
		{"Greet a user", 10, []string{"@AT-1.1"}, []string{"@FR-1.1", "@wip"}, nil},
		{"Greet quickly", 18, []string{"@AT-1.2", "@slow"}, []string{"@FR-1.1", "@wip", "@NFR-1"}, nil},
		{"Greet <name>", 22, []string{"@AT-1.3"}, []string{"@FR-1.1", "@wip", "@NFR-1"}, []string{"@FR-1.2"}},
		{"Templated", 36, []string{"@AT-1.4"}, []string{"@FR-1.1", "@wip"}, nil},
	}
	if len(f.scenarios) != len(want) {
		t.Fatalf("want %d scenarios, got %d: %+v", len(want), len(f.scenarios), f.scenarios)
	}
	for i, w := range want {
		s := f.scenarios[i]
		if s.name != w.name || s.line != w.line {
			t.Errorf("scenario %d: %q at %d, want %q at %d", i, s.name, s.line, w.name, w.line)
		}
		if got := tagNames(s.tags); !slices.Equal(got, w.own) {
			t.Errorf("scenario %d own tags %q, want %q", i, got, w.own)
		}
		if got := tagNames(s.inherited); !slices.Equal(got, w.inherited) {
			t.Errorf("scenario %d inherited tags %q, want %q", i, got, w.inherited)
		}
		if got := tagNames(s.examples); !slices.Equal(got, w.ex) {
			t.Errorf("scenario %d examples tags %q, want %q", i, got, w.ex)
		}
	}
}

func TestParseFeatureLanguage(t *testing.T) {
	_, problems := parseFeature("# language: de\nFunktionalität: Gruß\n")
	if want := []string{"1: language de is not supported; write feature files in English"}; !slices.Equal(problems, want) {
		t.Errorf("want %q, got %q", want, problems)
	}
	if _, problems := parseFeature("# language: en\nFeature: Greeting\n"); len(problems) > 0 {
		t.Errorf("en must be accepted, got %q", problems)
	}
}

func TestParseFeatureBacktickDocString(t *testing.T) {
	f, _ := parseFeature("Feature: F\n  @AT-1\n  Scenario: S\n    Given\n    ```\n    @AT-2\n    Scenario: inside\n    ```\n")
	if len(f.scenarios) != 1 || f.scenarios[0].name != "S" {
		t.Errorf("doc string content must be skipped, got %+v", f.scenarios)
	}
}
