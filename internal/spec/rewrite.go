package spec

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// FixAnchors rewrites every shorthand anchor [#ID] below the configured folders to [[ID]]. It
// returns the changed files, relative to root and slash-separated.
func FixAnchors(root string, config Config) ([]string, error) {
	p := newPatterns(config)
	if p.shorthand == nil {
		return nil, nil
	}
	var changed []string
	for _, doc := range config.Documents {
		files, err := adocFiles(filepath.Join(root, filepath.FromSlash(doc.Folder)))
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err != nil {
				return nil, err
			}
			fixed := p.shorthand.ReplaceAll(data, []byte("[[$1]]$2"))
			if string(fixed) == string(data) {
				continue
			}
			info, err := os.Stat(file)
			if err != nil {
				return nil, err
			}
			if err := os.WriteFile(file, fixed, info.Mode().Perm()); err != nil {
				return nil, err
			}
			relative, err := filepath.Rel(root, file)
			if err != nil {
				return nil, err
			}
			changed = append(changed, filepath.ToSlash(relative))
		}
	}
	return changed, nil
}

// Link prepares flattened masters for rendering to separate HTML files: in each master below
// out (at the master's path relative to root, as written by asciidoctor-reducer), every
// reference to an anchor of another master becomes an inter-document xref. root must hold a
// valid specification; otherwise the problems are returned and nothing is written.
func Link(root string, config Config, out string) ([]string, error) {
	r, err := analyze(root, config)
	if err != nil || len(r.problems) > 0 {
		return r.problems, err
	}
	p := newPatterns(config)
	for _, doc := range config.Documents {
		file := filepath.Join(out, filepath.FromSlash(doc.Master))
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("flattened master: %w", err)
		}
		text := string(data)
		if p.reference != nil {
			text = p.reference.ReplaceAllStringFunc(text, func(reference string) string {
				match := p.reference.FindStringSubmatch(reference)
				id, label := match[1], strings.TrimSpace(match[2])
				target, ok := r.masterOf[id]
				if !ok || target == doc.Master {
					return reference
				}
				if label == "" {
					label = id
				}
				return "xref:" + relativeMaster(doc.Master, target) + "#" + id + "[" + strings.ReplaceAll(label, "]", `\]`) + "]"
			})
		}
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// relativeMaster is the slash-separated path from the directory of master from to master to.
func relativeMaster(from, to string) string {
	fromParts := strings.Split(path.Dir(from), "/")
	if path.Dir(from) == "." {
		fromParts = nil
	}
	toParts := strings.Split(to, "/")
	common := 0
	for common < len(fromParts) && common < len(toParts)-1 && fromParts[common] == toParts[common] {
		common++
	}
	return strings.Repeat("../", len(fromParts)-common) + strings.Join(toParts[common:], "/")
}

// Masters lists the configured master files in configuration order.
func Masters(config Config) []string {
	masters := make([]string, len(config.Documents))
	for i, doc := range config.Documents {
		masters[i] = doc.Master
	}
	return masters
}
