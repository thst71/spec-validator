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
	id   string
	body string // the text up to the next anchor
	file string
}

type document struct {
	file string // relative to the root, slash-separated
	text string
}

type validator struct {
	root      string
	anchor    *regexp.Regexp
	reference *regexp.Regexp
	problems  []string
}

// Validate returns all problems found below root; an empty list means the specification is
// valid. The error reports files that could not be read.
func Validate(root string, config Config) ([]string, error) {
	resolvedRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	v := &validator{root: resolve(resolvedRoot)}
	if kinds := config.kinds(); len(kinds) > 0 {
		id := `(?:` + strings.Join(quote(kinds), "|") + `)-\d+(?:\.\d+)*`
		v.anchor = regexp.MustCompile(`(?m)^\[#(` + id + `)\]\s*$`)
		v.reference = regexp.MustCompile(`<<(` + id + `)(?:,[^>]*)?>>`)
	}

	anchors := map[string]string{}
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
			return nil, err
		}
		files, err := adocFiles(filepath.Join(v.root, filepath.FromSlash(doc.Folder)))
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			relative := v.relative(file)
			if !included[resolve(file)] {
				v.report("%s: not included by %s", relative, doc.Master)
			}
			text, err := readText(file)
			if err != nil {
				return nil, err
			}
			documents = append(documents, document{file: relative, text: text})
			for _, s := range v.sections(text) {
				if previous, duplicate := anchors[s.id]; duplicate {
					v.report("%s: duplicate anchor %s (also in %s)", relative, s.id, previous)
					continue
				}
				if !slices.Contains(doc.Kinds, kind(s.id)) {
					v.report("%s: anchor %s does not belong in %s", relative, s.id, doc.Folder)
				}
				anchors[s.id] = relative
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
	return v.problems, nil
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
		result = append(result, section{id: text[match[2]:match[3]], body: text[match[1]:end]})
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
