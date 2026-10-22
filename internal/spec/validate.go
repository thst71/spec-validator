package spec

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

var includePattern = regexp.MustCompile(`(?m)^include::([^\[]+)\[`)

type section struct {
	id        string
	shorthand bool   // written [#ID] instead of [[ID]]
	body      string // the text up to the next anchor
	file      string
}

type document struct {
	file string // relative to the root, slash-separated
	text string
}

// patterns are the expressions built from the configured anchor kinds; nil without kinds.
type patterns struct {
	anchor    *regexp.Regexp // [[ID]] or the shorthand [#ID], alone on a line
	shorthand *regexp.Regexp // [#ID] alone on a line, keeping the line ending
	reference *regexp.Regexp // <<ID>> or <<ID,text>>
}

func newPatterns(config Config) patterns {
	kinds := config.kinds()
	if len(kinds) == 0 {
		return patterns{}
	}
	id := `(?:` + strings.Join(quote(kinds), "|") + `)-\d+(?:\.\d+)*`
	return patterns{
		anchor:    regexp.MustCompile(`(?m)^(?:\[\[(` + id + `)\]\]|\[#(` + id + `)\])[ \t]*$`),
		shorthand: regexp.MustCompile(`(?m)^\[#(` + id + `)\]([ \t]*\r?)$`),
		reference: regexp.MustCompile(`<<(` + id + `)(?:,([^>]*))?>>`),
	}
}

type validator struct {
	patterns
	root     string
	problems []string
}

// result is a validation pass: the problems and, for the links, the master defining each ID.
type result struct {
	problems []string
	masterOf map[string]string
}

// Validate returns all problems found below root; an empty list means the specification is
// valid. The error reports files that could not be read.
func Validate(root string, config Config) ([]string, error) {
	r, err := analyze(root, config)
	if err != nil {
		return nil, err
	}
	return r.problems, nil
}

func analyze(root string, config Config) (result, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return result{}, err
	}
	v := &validator{patterns: newPatterns(config), root: resolve(absolute)}

	anchors := map[string]string{}
	masterOf := map[string]string{}
	var sections []section
	var documents []document
	for _, doc := range config.Documents {
		master := filepath.Join(v.root, filepath.FromSlash(doc.Master))
		if !isFile(master) {
			v.report("%s: master file is missing", doc.Master)
			continue
		}
		included := map[string]bool{}
		if err := v.includes(master, included); err != nil {
			return result{}, err
		}
		files, err := adocFiles(filepath.Join(v.root, filepath.FromSlash(doc.Folder)))
		if err != nil {
			return result{}, err
		}
		for _, file := range files {
			relative := v.relative(file)
			if !included[resolve(file)] {
				v.report("%s: not included by %s", relative, doc.Master)
			}
			text, err := readText(file)
			if err != nil {
				return result{}, err
			}
			documents = append(documents, document{file: relative, text: text})
			for _, s := range v.sections(text) {
				if s.shorthand {
					v.report("%s: anchor [#%s] must be written [[%s]]; Asciidoctor reads a dot after # as a role", relative, s.id, s.id)
				}
				if previous, duplicate := anchors[s.id]; duplicate {
					v.report("%s: duplicate anchor %s (also in %s)", relative, s.id, previous)
					continue
				}
				if !slices.Contains(doc.Kinds, kind(s.id)) {
					v.report("%s: anchor %s does not belong in %s", relative, s.id, doc.Folder)
				}
				anchors[s.id] = relative
				masterOf[s.id] = doc.Master
				s.file = relative
				sections = append(sections, s)
			}
		}
	}

	for _, doc := range documents {
		for _, id := range v.references(doc.text) {
			if _, ok := anchors[id]; !ok {
				v.report("%s: unresolved reference <<%s>>", doc.file, id)
			}
		}
	}

	for _, s := range sections {
		referenced := map[string]bool{}
		for _, id := range v.references(s.body) {
			referenced[kind(id)] = true
		}
		for _, trace := range config.Traceability {
			if trace.Kind != kind(s.id) {
				continue
			}
			if !slices.ContainsFunc(trace.Requires, func(k string) bool { return referenced[k] }) {
				v.report("%s: %s references no %s", s.file, s.id, trace.Name)
			}
		}
	}
	return result{problems: v.problems, masterOf: masterOf}, nil
}

func (v *validator) report(format string, args ...any) {
	v.problems = append(v.problems, fmt.Sprintf(format, args...))
}

// includes adds every file that document includes, directly or nested, to seen. Targets are
// resolved relative to the including file, as Asciidoctor does.
func (v *validator) includes(document string, seen map[string]bool) error {
	text, err := readText(document)
	if err != nil {
		return err
	}
	for _, match := range includePattern.FindAllStringSubmatch(text, -1) {
		target := strings.TrimSpace(match[1])
		path := resolve(filepath.Join(filepath.Dir(document), filepath.FromSlash(target)))
		if !isFile(path) {
			v.report("%s: include target not found: %s", v.relative(document), target)
		} else if !seen[path] {
			seen[path] = true
			if err := v.includes(path, seen); err != nil {
				return err
			}
		}
	}
	return nil
}

// sections splits text at its anchors, in document order.
func (v *validator) sections(text string) []section {
	if v.anchor == nil {
		return nil
	}
	matches := v.anchor.FindAllStringSubmatchIndex(text, -1)
	result := make([]section, 0, len(matches))
	for i, match := range matches {
		end := len(text)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		s := section{body: text[match[1]:end]}
		if match[2] >= 0 {
			s.id = text[match[2]:match[3]]
		} else {
			s.id, s.shorthand = text[match[4]:match[5]], true
		}
		result = append(result, s)
	}
	return result
}

// references returns the distinct IDs referenced in text, sorted.
func (v *validator) references(text string) []string {
	if v.reference == nil {
		return nil
	}
	unique := map[string]bool{}
	for _, match := range v.reference.FindAllStringSubmatch(text, -1) {
		unique[match[1]] = true
	}
	ids := make([]string, 0, len(unique))
	for id := range unique {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (v *validator) relative(path string) string {
	relative, err := filepath.Rel(v.root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}

// adocFiles lists the .adoc files below folder in lexical order; a missing folder has none.
func adocFiles(folder string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(folder, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && path == folder {
				return filepath.SkipDir
			}
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".adoc") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func readText(path string) (string, error) {
	data, err := os.ReadFile(path)
	return strings.ReplaceAll(string(data), "\r\n", "\n"), err
}

func resolve(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return filepath.Clean(path)
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func kind(id string) string {
	return id[:strings.IndexByte(id, '-')]
}

func quote(kinds []string) []string {
	quoted := make([]string, len(kinds))
	for i, k := range kinds {
		quoted[i] = regexp.QuoteMeta(k)
	}
	return quoted
}
