package spec

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// tag is a Gherkin tag with the line it stands on.
type tag struct {
	name string // with the leading @
	line int
}

// gherkinScenario is a Scenario or Scenario Outline of a feature file.
type gherkinScenario struct {
	name      string
	line      int   // line of the keyword
	tags      []tag // its own tags
	inherited []tag // tags of its Feature and Rule
	examples  []tag // tags of its Examples blocks
}

// keywordTags are tags of a Feature, Rule or Examples line, kept for the checks.
type keywordTags struct {
	keyword string
	line    int
	tags    []tag
}

type featureFile struct {
	name      string
	line      int
	scenarios []gherkinScenario
	keywords  []keywordTags // Feature, Rule and Examples with their tags
}

// parseFeature reads the structure of a Gherkin feature file in English: feature, rules,
// scenarios and their tags. Steps, tables and descriptions are skipped; doc strings are skipped
// to their end, so that keyword lines inside them do not count. Problems carry the line number.
func parseFeature(text string) (featureFile, []string) {
	var (
		f           featureFile
		problems    []string
		pending     []tag
		featureTags []tag
		ruleTags    []tag
		docString   string // the delimiter of an open doc string
	)
	for i, raw := range strings.Split(text, "\n") {
		number := i + 1
		line := strings.TrimSpace(raw)
		if docString != "" {
			if strings.HasPrefix(line, docString) {
				docString = ""
			}
			continue
		}
		switch {
		case strings.HasPrefix(line, `"""`), strings.HasPrefix(line, "```"):
			docString = line[:3]
			continue
		case line == "":
			continue
		case strings.HasPrefix(line, "#"):
			if language, ok := strings.CutPrefix(strings.TrimSpace(line[1:]), "language:"); ok {
				if language = strings.TrimSpace(language); language != "en" {
					problems = append(problems, fmt.Sprintf("%d: language %s is not supported; write feature files in English", number, language))
				}
			}
			continue
		case strings.HasPrefix(line, "@"):
			pending = append(pending, parseTags(line, number)...)
			continue
		}

		keyword, name, ok := gherkinKeyword(line)
		if !ok {
			continue // a step, table row or description
		}
		tags := pending
		pending = nil
		switch keyword {
		case "Feature":
			f.name, f.line, featureTags = name, number, tags
			f.keywords = append(f.keywords, keywordTags{keyword, number, tags})
		case "Rule":
			ruleTags = tags
			f.keywords = append(f.keywords, keywordTags{keyword, number, tags})
		case "Examples":
			if n := len(f.scenarios); n > 0 {
				f.scenarios[n-1].examples = append(f.scenarios[n-1].examples, tags...)
			}
			f.keywords = append(f.keywords, keywordTags{keyword, number, tags})
		case "Scenario":
			inherited := append(append([]tag{}, featureTags...), ruleTags...)
			f.scenarios = append(f.scenarios, gherkinScenario{name: name, line: number, tags: tags, inherited: inherited})
		}
	}
	return f, problems
}

// gherkinKeyword recognises the structural keywords; Scenario stands for every form of it.
func gherkinKeyword(line string) (keyword, name string, ok bool) {
	forms := []struct{ prefix, keyword string }{
		{"Feature:", "Feature"},
		{"Rule:", "Rule"},
		{"Background:", "Background"},
		{"Scenario Outline:", "Scenario"},
		{"Scenario Template:", "Scenario"},
		{"Scenario:", "Scenario"},
		{"Example:", "Scenario"},
		{"Examples:", "Examples"},
		{"Scenarios:", "Examples"},
	}
	for _, form := range forms {
		if rest, found := strings.CutPrefix(line, form.prefix); found {
			return form.keyword, strings.TrimSpace(rest), true
		}
	}
	return "", "", false
}

// parseTags reads the tags of a tag line; a # outside a tag starts a comment.
func parseTags(line string, number int) []tag {
	var tags []tag
	for _, field := range strings.Fields(line) {
		if strings.HasPrefix(field, "#") {
			break
		}
		if strings.HasPrefix(field, "@") && len(field) > 1 {
			tags = append(tags, tag{name: field, line: number})
		}
	}
	return tags
}

// featureFiles lists the .feature files below folder in lexical order; a missing folder has
// none.
func featureFiles(folder string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(folder, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && path == folder {
				return filepath.SkipDir
			}
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".feature") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
