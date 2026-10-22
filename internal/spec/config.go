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

// Config describes the layout of a project's specification.
type Config struct {
	Documents    []Document `json:"documents"`
	Traceability []Trace    `json:"traceability"`
}

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
	return nil
}

func (c Config) kinds() []string {
	var kinds []string
	for _, document := range c.Documents {
		kinds = append(kinds, document.Kinds...)
	}
	return kinds
}
