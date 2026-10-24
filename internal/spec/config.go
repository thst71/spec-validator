// Package spec validates a modular AsciiDoc specification: includes, anchors, references and
// traceability.
package spec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
)

// ConfigFile is looked up in the project root when no configuration is given explicitly.
const ConfigFile = ".spec-validator.json"

// Document is one master file and the folder holding the files it includes.
type Document struct {
	Master string   `json:"master"`
	Folder string   `json:"folder"`
	Kinds  []string `json:"kinds"` // anchor kinds defined in Folder, e.g. "FR"
}

// Trace requires every item of Kind to reference at least one item of the Requires kinds.
type Trace struct {
	Kind     string   `json:"kind"`
	Requires []string `json:"requires"`
	Name     string   `json:"name"` // what is missing, for the message: "requirement"
}

// TestKind is a kind of test whose IDs live in test files instead of AsciiDoc anchors, e.g.
// acceptance scenarios (AT) in Gherkin feature files.
type TestKind struct {
	Kind   string   `json:"kind"`
	Folder string   `json:"folder"`
	Format string   `json:"format"` // only "gherkin"; manual tests may add another format
	Traces []string `json:"traces"` // the anchor kinds a test may trace to, e.g. FR, NFR
}

// Reports names the generated report masters; empty fields take the defaults.
type Reports struct {
	Tests        string `json:"tests"`
	Traceability string `json:"traceability"`
}

// Config describes the layout of a project's specification.
type Config struct {
	Documents    []Document `json:"documents"`
	Traceability []Trace    `json:"traceability"`
	Tests        []TestKind `json:"tests"`
	Reports      Reports    `json:"reports"`
}

// Default names of the report masters in the project root.
const (
	DefaultTestReport  = "test-report.adoc"
	DefaultTraceReport = "traceability-report.adoc"
)

var kindPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]*$`)

// DefaultConfig is the layout of the spec-system skill of agent-skills.
func DefaultConfig() Config {
	return Config{
		Documents: []Document{
			{Master: "specification-web.adoc", Folder: "specification/requirements", Kinds: []string{"FR", "NFR"}},
			{Master: "tech-specification-web.adoc", Folder: "specification/tech-specification", Kinds: []string{"TS"}},
			{Master: "backlog-web.adoc", Folder: "specification/backlog", Kinds: []string{"US"}},
			{Master: "release-notes.adoc", Folder: "specification/release-notes", Kinds: []string{}},
		},
		Traceability: []Trace{
			{Kind: "TS", Requires: []string{"FR", "NFR"}, Name: "requirement"},
			{Kind: "US", Requires: []string{"FR", "NFR"}, Name: "requirement"},
			{Kind: "US", Requires: []string{"TS"}, Name: "technical specification"},
		},
	}
}

// LoadConfig reads a configuration file. Unknown fields are an error, so that a typo does not
// silently fall back to a default.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var config Config
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	if err := config.check(); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return config, nil
}

func (c Config) check() error {
	if len(c.Documents) == 0 {
		return errors.New("no documents configured")
	}
	known := map[string]bool{}
	for _, document := range c.Documents {
		if document.Master == "" || document.Folder == "" {
			return errors.New("every document needs a master and a folder")
		}
		for _, kind := range document.Kinds {
			if !kindPattern.MatchString(kind) {
				return fmt.Errorf("invalid anchor kind %q", kind)
			}
			if known[kind] {
				return fmt.Errorf("anchor kind %q is defined by two documents", kind)
			}
			known[kind] = true
		}
	}
	for _, trace := range c.Traceability {
		if !known[trace.Kind] {
			return fmt.Errorf("traceability for unknown kind %q", trace.Kind)
		}
		if trace.Name == "" || len(trace.Requires) == 0 {
			return fmt.Errorf("traceability for %q needs requires and name", trace.Kind)
		}
		for _, kind := range trace.Requires {
			if !known[kind] {
				return fmt.Errorf("traceability for %q requires unknown kind %q", trace.Kind, kind)
			}
		}
	}
	tests := map[string]bool{}
	for _, test := range c.Tests {
		switch {
		case !kindPattern.MatchString(test.Kind):
			return fmt.Errorf("invalid test kind %q", test.Kind)
		case known[test.Kind]:
			return fmt.Errorf("test kind %q is an anchor kind of a document", test.Kind)
		case tests[test.Kind]:
			return fmt.Errorf("test kind %q is defined twice", test.Kind)
		case test.Folder == "":
			return fmt.Errorf("test kind %q needs a folder", test.Kind)
		case test.Format != "gherkin":
			return fmt.Errorf("test kind %q: unsupported format %q (supported: gherkin)", test.Kind, test.Format)
		case len(test.Traces) == 0:
			return fmt.Errorf("test kind %q needs traces", test.Kind)
		}
		for _, kind := range test.Traces {
			if !known[kind] {
				return fmt.Errorf("test kind %q traces to unknown kind %q", test.Kind, kind)
			}
		}
		tests[test.Kind] = true
	}
	return nil
}

// reportMasters returns the paths of the test report and the traceability report.
func (c Config) reportMasters() (tests, traceability string) {
	tests, traceability = c.Reports.Tests, c.Reports.Traceability
	if tests == "" {
		tests = DefaultTestReport
	}
	if traceability == "" {
		traceability = DefaultTraceReport
	}
	return tests, traceability
}

func (c Config) testKinds() []string {
	kinds := make([]string, len(c.Tests))
	for i, test := range c.Tests {
		kinds[i] = test.Kind
	}
	return kinds
}

func (c Config) kinds() []string {
	var kinds []string
	for _, document := range c.Documents {
		kinds = append(kinds, document.Kinds...)
	}
	return kinds
}
