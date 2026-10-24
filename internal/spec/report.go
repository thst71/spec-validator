package spec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Cucumber JSON, as written by Gherkin runners (cucumber-js, playwright-bdd, pytest-bdd, …).
type cucumberFeature struct {
	URI      string            `json:"uri"`
	Name     string            `json:"name"`
	Elements []cucumberElement `json:"elements"`
}

type cucumberElement struct {
	Type    string         `json:"type"` // scenario or background
	Name    string         `json:"name"`
	Line    int            `json:"line"`
	Keyword string         `json:"keyword"`
	Tags    []cucumberTag  `json:"tags"`
	Steps   []cucumberStep `json:"steps"`
}

type cucumberTag struct {
	Name string `json:"name"`
}

type cucumberStep struct {
	Keyword string `json:"keyword"`
	Name    string `json:"name"`
	Result  struct {
		Status       string `json:"status"`
		ErrorMessage string `json:"error_message"`
	} `json:"result"`
}

// statusOrder ranks the results from worst to best; "not run" is a scenario without results.
var statusOrder = []string{"failed", "ambiguous", "undefined", "pending", "skipped", "passed", "not run"}

// status is the first step result that is not passed, else passed.
func (e cucumberElement) status() string {
	for _, step := range e.Steps {
		if step.Result.Status != "passed" {
			return step.Result.Status
		}
	}
	return "passed"
}

// worst is the worst of the given results; without results a scenario was not run.
func worst(statuses []string) string {
	if len(statuses) == 0 {
		return "not run"
	}
	result := statuses[0]
	for _, status := range statuses[1:] {
		if rank(status) < rank(result) {
			result = status
		}
	}
	return result
}

func rank(status string) int {
	if i := slices.Index(statusOrder, status); i >= 0 {
		return i
	}
	return 0 // an unknown status counts as a failure
}

// runResults are the results of a run per test ID, plus the results that match no test.
type runResults struct {
	byID      map[string][]cucumberElement // the elements of a scenario, backgrounds folded in
	unmatched []unmatchedResult
}

type unmatchedResult struct {
	uri     string
	element cucumberElement
}

func readResults(path string, config Config) (runResults, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return runResults{}, err
	}
	var features []cucumberFeature
	if err := json.Unmarshal(data, &features); err != nil {
		return runResults{}, fmt.Errorf("%s: %w", path, err)
	}
	// cucumber-js and playwright-bdd write tags with @, pytest-bdd without
	idTag := regexp.MustCompile(`^@?(` + idPattern(config.testKinds()) + `)$`)
	r := runResults{byID: map[string][]cucumberElement{}}
	for _, feature := range features {
		var background []cucumberStep
		for _, element := range feature.Elements {
			if element.Type == "background" {
				background = element.Steps
				continue
			}
			element.Steps = append(slices.Clone(background), element.Steps...)
			background = nil
			id := ""
			for _, t := range element.Tags {
				if match := idTag.FindStringSubmatch(t.Name); match != nil {
					id = match[1]
				}
			}
			if id == "" {
				r.unmatched = append(r.unmatched, unmatchedResult{feature.URI, element})
				continue
			}
			r.byID[id] = append(r.byID[id], element)
		}
	}
	return r, nil
}

// statusOf is the result of a test in a run.
func (r runResults) statusOf(id string) string {
	var statuses []string
	for _, element := range r.byID[id] {
		statuses = append(statuses, element.status())
	}
	return worst(statuses)
}

// TestReport validates the specification and writes the test report master from the
// Cucumber JSON in results. It returns the problems of an invalid specification, which leave
// the report unwritten; the error reports unreadable files.
func TestReport(root string, config Config, results, date string) ([]string, error) {
	r, err := analyze(root, config)
	if err != nil || len(r.problems) > 0 {
		return r.problems, err
	}
	run, err := readResults(results, config)
	if err != nil {
		return nil, err
	}
	master, _ := config.reportMasters()

	var b strings.Builder
	header(&b, "Test Report", date)

	counts := map[string]int{}
	for _, test := range r.tests {
		counts[run.statusOf(test.id)]++
	}
	b.WriteString("== Summary\n\n[cols=\"3,1\",options=\"header\"]\n|===\n| Result | Scenarios\n\n")
	for _, status := range statusOrder {
		if counts[status] > 0 {
			fmt.Fprintf(&b, "| %s | %d\n", status, counts[status])
		}
	}
	fmt.Fprintf(&b, "| total | %d\n|===\n", len(r.tests))

	file := ""
	for _, test := range r.tests {
		if test.file != file {
			file = test.file
			fmt.Fprintf(&b, "\n== %s\n\nFile: `+%s+`\n", headingText(test.feature, file), file)
		}
		fmt.Fprintf(&b, "\n[[%s]]\n=== %s %s\n\n", test.id, test.id, inline(test.name))
		fmt.Fprintf(&b, "Traces to:: %s +\nResult:: %s\n", references(test.traces), run.statusOf(test.id))
		elements := run.byID[test.id]
		for i, element := range elements {
			if len(elements) > 1 {
				fmt.Fprintf(&b, "\n.Example %d: %s\n", i+1, element.status())
			} else {
				b.WriteString("\n")
			}
			steps(&b, element.Steps)
		}
	}

	if len(run.unmatched) > 0 {
		b.WriteString("\n== Results without scenario\n\nThese results carry no known test ID.\n\n")
		for _, u := range run.unmatched {
			fmt.Fprintf(&b, "* %s (`+%s+`, line %d): %s\n", inline(u.element.Name), u.uri, u.element.Line, u.element.status())
		}
	}
	return nil, writeReport(root, master, b.String())
}

func header(b *strings.Builder, title, date string) {
	fmt.Fprintf(b, "= %s\n:toc: left\n:toclevels: 2\n\nVersion {revnumber}, written %s.\n\n", title, date)
}

func headingText(name, fallback string) string {
	if name == "" {
		return "`+" + fallback + "+`"
	}
	return inline(name)
}

func steps(b *strings.Builder, steps []cucumberStep) {
	for _, step := range steps {
		fmt.Fprintf(b, "* *%s* %s: %s\n", inline(strings.TrimSpace(step.Keyword)), inline(step.Name), step.Result.Status)
		if message := step.Result.ErrorMessage; message != "" {
			b.WriteString("+\n")
			literal(b, message)
		}
	}
}

// literal writes text as a literal block whose delimiter no line of text can close.
func literal(b *strings.Builder, text string) {
	delimiter := "...."
	for _, line := range strings.Split(text, "\n") {
		if strings.Trim(line, ".") == "" && len(line) >= len(delimiter) {
			delimiter = strings.Repeat(".", len(line)+1)
		}
	}
	fmt.Fprintf(b, "%s\n%s\n%s\n", delimiter, strings.TrimRight(text, "\n"), delimiter)
}

// inline passes text through with its special characters escaped and nothing else, so that
// step texts and names cannot create markup, attributes or references.
func inline(text string) string {
	if text == "" {
		return ""
	}
	return "pass:c[" + strings.ReplaceAll(text, "]", `\]`) + "]"
}

func references(ids []string) string {
	refs := make([]string, len(ids))
	for i, id := range ids {
		refs[i] = "<<" + id + ">>"
	}
	return strings.Join(refs, ", ")
}

func writeReport(root, master, text string) error {
	path := filepath.Join(root, filepath.FromSlash(master))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o644)
}
