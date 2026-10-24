package spec

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// testCase is a test found in a test file: an acceptance scenario of a feature file.
type testCase struct {
	kind    string
	id      string // empty when the scenario has no ID
	name    string
	feature string // name of the feature
	file    string // relative to the root, slash-separated
	line    int
	traces  []string // IDs of the items it traces to, resolved and of an allowed kind
}

func (t testCase) location() string {
	return fmt.Sprintf("%s:%d", t.file, t.line)
}

// testFiles reads the test files of every test kind and checks their IDs and traces against
// the anchors of the documents. It returns the tests in file and line order and the problems.
func (v *validator) testFiles(config Config, anchors map[string]string) ([]testCase, []string, error) {
	var (
		tests    []testCase
		problems []string
	)
	report := func(file string, line int, format string, args ...any) {
		problems = append(problems, fmt.Sprintf("%s:%d: ", file, line)+fmt.Sprintf(format, args...))
	}
	traceTag := regexp.MustCompile(`^@(` + idPattern(config.kinds()) + `)$`)
	first := map[string]string{} // test ID -> location of its first definition
	for _, testKind := range config.Tests {
		idTag := regexp.MustCompile(`^@(` + idPattern([]string{testKind.Kind}) + `)$`)
		files, err := featureFiles(filepath.Join(v.root, filepath.FromSlash(testKind.Folder)))
		if err != nil {
			return nil, nil, err
		}
		for _, path := range files {
			text, err := readText(path)
			if err != nil {
				return nil, nil, err
			}
			file := v.relative(path)
			feature, parseProblems := parseFeature(text)
			for _, problem := range parseProblems {
				problems = append(problems, file+":"+problem)
			}
			for _, keyword := range feature.keywords {
				for _, t := range keyword.tags {
					if idTag.MatchString(t.name) {
						report(file, keyword.line, "%s tag %s on %s; put the ID on the scenario", testKind.Kind, t.name, keyword.keyword)
					}
				}
			}
			for _, s := range feature.scenarios {
				test := testCase{kind: testKind.Kind, name: s.name, feature: feature.name, file: file, line: s.line}
				var ids []string
				for _, t := range s.tags {
					if match := idTag.FindStringSubmatch(t.name); match != nil {
						ids = append(ids, match[1])
					}
				}
				label := fmt.Sprintf("scenario %q", s.name)
				switch len(ids) {
				case 0:
					report(file, s.line, "%s has no %s tag", label, testKind.Kind)
				case 1:
					test.id, label = ids[0], ids[0]
					if previous, duplicate := first[test.id]; duplicate {
						report(file, s.line, "duplicate %s (also in %s)", test.id, previous)
						test.id = ""
					} else {
						first[test.id] = test.location()
					}
				default:
					report(file, s.line, "%s has %d %s tags", label, len(ids), testKind.Kind)
				}

				for _, t := range slices.Concat(s.tags, s.inherited, s.examples) {
					match := traceTag.FindStringSubmatch(t.name)
					if match == nil {
						continue // an ID of a test kind or another tag, e.g. @wip
					}
					id := match[1]
					switch {
					case !slices.Contains(testKind.Traces, kind(id)):
						report(file, s.line, "%s traces to %s; %s traces to %s only", label, id, testKind.Kind, strings.Join(testKind.Traces, ", "))
					case anchors[id] == "":
						report(file, s.line, "%s traces to unknown %s", label, id)
					case !slices.Contains(test.traces, id):
						test.traces = append(test.traces, id)
					}
				}
				if len(test.traces) == 0 {
					report(file, s.line, "%s references no requirement", label)
				}
				tests = append(tests, test)
			}
		}
	}
	return tests, problems, nil
}
