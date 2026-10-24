package spec

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	requirement = "[[FR-1.1]]\n=== Requirement\n\nText.\n"
	tech        = "[[TS-1.1]]\n=== Design\n\nImplements <<FR-1.1>>.\n"
	story       = "[[US-1.1]]\n=== Story\n\nTraces to <<TS-1.1>> and <<FR-1.1,the requirement>>.\n"
)

type tree struct{ requirement, tech, story string }

func validTree() tree { return tree{requirement, tech, story} }

// write creates a minimal project: one chapter per kind with an index and one item file.
func (tr tree) write(t *testing.T) string {
	t.Helper()
	return tr.writeTo(t, t.TempDir())
}

// writeTo writes the project into root, which must exist.
func (tr tree) writeTo(t *testing.T, root string) string {
	t.Helper()
	chapters := []struct{ master, folder, content string }{
		{"specification-web.adoc", "specification/requirements", tr.requirement},
		{"tech-specification-web.adoc", "specification/tech-specification", tr.tech},
		{"backlog-web.adoc", "specification/backlog", tr.story},
	}
	for _, c := range chapters {
		writeFile(t, root, c.folder+"/01-chapter/index.adoc", "== Chapter\n\ninclude::item.adoc[]\n")
		writeFile(t, root, c.folder+"/01-chapter/item.adoc", c.content)
		writeFile(t, root, c.master, "= Master\n\ninclude::"+c.folder+"/01-chapter/index.adoc[]\n")
	}
	writeFile(t, root, "specification/release-notes/0.1.0.adoc", "== 0.1.0\n\n<<FR-1.1>>\n")
	writeFile(t, root, "release-notes.adoc", "= Notes\n\ninclude::specification/release-notes/0.1.0.adoc[]\n")
	return root
}

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func validate(t *testing.T, root string) []string {
	t.Helper()
	problems, err := Validate(root, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	return problems
}

func assertContains(t *testing.T, problems []string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !slices.Contains(problems, w) {
			t.Errorf("missing problem %q in %q", w, problems)
		}
	}
}

func assertContainsPart(t *testing.T, problems []string, part string) {
	t.Helper()
	if !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, part) }) {
		t.Errorf("no problem contains %q in %q", part, problems)
	}
}

func TestValidTree(t *testing.T) {
	if problems := validate(t, validTree().write(t)); len(problems) != 0 {
		t.Errorf("want no problems, got %q", problems)
	}
}

func TestMissingMasterAndIncludeTarget(t *testing.T) {
	root := validTree().write(t)
	if err := os.Remove(filepath.Join(root, "release-notes.adoc")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "backlog-web.adoc", "include::specification/backlog/99-gone/index.adoc[]\n")

	assertContains(t, validate(t, root),
		"release-notes.adoc: master file is missing",
		"backlog-web.adoc: include target not found: specification/backlog/99-gone/index.adoc",
		"specification/backlog/01-chapter/index.adoc: not included by backlog-web.adoc",
		"specification/backlog/01-chapter/item.adoc: not included by backlog-web.adoc",
	)
}

func TestNestedIncludesResolveRelativeToTheIncludingFile(t *testing.T) {
	root := validTree().write(t)
	writeFile(t, root, "specification/requirements/01-chapter/orphan.adoc", "[[FR-1.2]]\n=== Orphan\n")
	writeFile(t, root, "specification/requirements/01-chapter/index.adoc",
		"== Chapter\n\ninclude::item.adoc[]\n\ninclude::gone.adoc[]\n")

	problems := validate(t, root)

	assertContains(t, problems,
		"specification/requirements/01-chapter/index.adoc: include target not found: gone.adoc",
		"specification/requirements/01-chapter/orphan.adoc: not included by specification-web.adoc",
	)
	for _, p := range problems {
		if strings.Contains(p, "item.adoc") {
			t.Errorf("unexpected problem %q", p)
		}
	}
}

func TestUnresolvedReferenceAndDuplicateAnchor(t *testing.T) {
	tr := validTree()
	tr.requirement += "\n[[FR-1.1]]\n=== Again\n"
	tr.tech = "[[TS-1.1]]\n=== Design\n\nImplements <<FR-9.9>>.\n"

	problems := validate(t, tr.write(t))

	assertContainsPart(t, problems, "duplicate anchor FR-1.1")
	assertContainsPart(t, problems, "unresolved reference <<FR-9.9>>")
}

func TestTraceabilityIsEnforced(t *testing.T) {
	tr := validTree()
	tr.tech = "[[TS-1.1]]\n=== Design\n\nNo requirement.\n"
	tr.story = "[[US-1.1]]\n=== Story\n\nOnly <<FR-1.1>>.\n"

	assertContains(t, validate(t, tr.write(t)),
		"specification/tech-specification/01-chapter/item.adoc: TS-1.1 references no requirement",
		"specification/backlog/01-chapter/item.adoc: US-1.1 references no technical specification",
	)
}

func TestAnchorInTheWrongFolder(t *testing.T) {
	tr := validTree()
	tr.requirement += "\n[[US-7.1]]\n=== Misplaced <<TS-1.1>> <<FR-1.1>>\n"

	assertContainsPart(t, validate(t, tr.write(t)), "US-7.1 does not belong in specification/requirements")
}

func TestWindowsLineEndings(t *testing.T) {
	tr := validTree()
	tr.tech = strings.ReplaceAll(tech, "\n", "\r\n")

	if problems := validate(t, tr.write(t)); len(problems) != 0 {
		t.Errorf("want no problems, got %q", problems)
	}
}

func TestCustomLayout(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "docs/req.adoc", "= Requirements\n\ninclude::req/REQ-1.adoc[]\n")
	writeFile(t, root, "docs/req/REQ-1.adoc", "[[REQ-1]]\n=== One\n")
	writeFile(t, root, "docs/design.adoc", "= Design\n\ninclude::design/DES-1.adoc[]\n")
	writeFile(t, root, "docs/design/DES-1.adoc", "[[DES-1]]\n=== One\n\nNo parent.\n")
	config := Config{
		Documents: []Document{
			{Master: "docs/req.adoc", Folder: "docs/req", Kinds: []string{"REQ"}},
			{Master: "docs/design.adoc", Folder: "docs/design", Kinds: []string{"DES"}},
		},
		Traceability: []Trace{{Kind: "DES", Requires: []string{"REQ"}, Name: "requirement"}},
	}

	problems, err := Validate(root, config)
	if err != nil {
		t.Fatal(err)
	}

	if want := []string{"docs/design/DES-1.adoc: DES-1 references no requirement"}; !slices.Equal(problems, want) {
		t.Errorf("want %q, got %q", want, problems)
	}
}

func TestLoadConfig(t *testing.T) {
	root := t.TempDir()
	cases := map[string]string{
		"unknown field": `{"documents": [{"master": "a.adoc", "folder": "a", "kind": ["A"]}]}`,
		"no documents":  `{"documents": []}`,
		"bad kind":      `{"documents": [{"master": "a.adoc", "folder": "a", "kinds": ["a-1"]}]}`,
		"unknown trace": `{"documents": [{"master": "a.adoc", "folder": "a", "kinds": ["A"]}], "traceability": [{"kind": "B", "requires": ["A"], "name": "a"}]}`,
		"duplicate":     `{"documents": [{"master": "a.adoc", "folder": "a", "kinds": ["A"]}, {"master": "b.adoc", "folder": "b", "kinds": ["A"]}]}`,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			writeFile(t, root, "config.json", content)
			if _, err := LoadConfig(filepath.Join(root, "config.json")); err == nil {
				t.Error("want an error")
			}
		})
	}

	writeFile(t, root, "config.json", `{"documents": [{"master": "a.adoc", "folder": "a", "kinds": ["A"]}]}`)
	if _, err := LoadConfig(filepath.Join(root, "config.json")); err != nil {
		t.Errorf("want a valid config, got %v", err)
	}
	if err := DefaultConfig().check(); err != nil {
		t.Errorf("default config is invalid: %v", err)
	}
}

func TestShorthandAnchor(t *testing.T) {
	tr := validTree()
	tr.requirement = "[#FR-1.1]\n=== Requirement\n"

	problems := validate(t, tr.write(t))

	want := []string{"specification/requirements/01-chapter/item.adoc: anchor [#FR-1.1] must be written [[FR-1.1]]; Asciidoctor reads a dot after # as a role"}
	if !slices.Equal(problems, want) {
		t.Errorf("want %q, got %q", want, problems)
	}
}

func TestFixAnchors(t *testing.T) {
	tr := validTree()
	tr.requirement = "[#FR-1.1]\r\n=== Requirement\r\n"
	tr.tech = "[#TS-1.1]  \n=== Design\n\nImplements <<FR-1.1>>.\n"
	root := tr.write(t)

	changed, err := FixAnchors(root, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"specification/requirements/01-chapter/item.adoc", "specification/tech-specification/01-chapter/item.adoc"}
	if !slices.Equal(changed, want) {
		t.Errorf("want %q, got %q", want, changed)
	}
	data, err := os.ReadFile(filepath.Join(root, "specification/requirements/01-chapter/item.adoc"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "[[FR-1.1]]\r\n=== Requirement\r\n" {
		t.Errorf("unexpected content %q", data)
	}
	if problems := validate(t, root); len(problems) != 0 {
		t.Errorf("want no problems after the fix, got %q", problems)
	}
}

func TestLink(t *testing.T) {
	root := validTree().write(t)
	out := t.TempDir()
	writeFile(t, out, "specification-web.adoc", "[[FR-1.1]]\n=== Requirement\n")
	writeFile(t, out, "tech-specification-web.adoc", tech)
	writeFile(t, out, "backlog-web.adoc", story+"Again <<US-1.1>>.\n")
	writeFile(t, out, "release-notes.adoc", "== 0.1.0\n\n<<FR-1.1>>\n")

	problems, err := Link(root, DefaultConfig(), out)
	if err != nil || problems != nil {
		t.Fatal(problems, err)
	}

	data, err := os.ReadFile(filepath.Join(out, "backlog-web.adoc"))
	if err != nil {
		t.Fatal(err)
	}
	want := "[[US-1.1]]\n=== Story\n\nTraces to xref:tech-specification-web.adoc#TS-1.1[TS-1.1] and xref:specification-web.adoc#FR-1.1[the requirement].\nAgain <<US-1.1>>.\n"
	if string(data) != want {
		t.Errorf("want %q, got %q", want, data)
	}
}

func TestLinkRefusesAnInvalidSpecification(t *testing.T) {
	tr := validTree()
	tr.tech = "[[TS-1.1]]\n=== Design\n"
	out := t.TempDir()

	problems, err := Link(tr.write(t), DefaultConfig(), out)

	if err != nil || len(problems) == 0 {
		t.Errorf("want problems, got %q, %v", problems, err)
	}
}

func TestRelativeMaster(t *testing.T) {
	cases := []struct{ from, to, want string }{
		{"a.adoc", "b.adoc", "b.adoc"},
		{"docs/a.adoc", "b.adoc", "../b.adoc"},
		{"docs/a.adoc", "docs/b.adoc", "b.adoc"},
		{"a.adoc", "docs/x/b.adoc", "docs/x/b.adoc"},
		{"docs/x/a.adoc", "docs/y/b.adoc", "../y/b.adoc"},
	}
	for _, c := range cases {
		if got := relativeMaster(c.from, c.to); got != c.want {
			t.Errorf("relativeMaster(%q, %q) = %q, want %q", c.from, c.to, got, c.want)
		}
	}
}

// TestOwnSpecification keeps this repository's specification valid.
func TestOwnSpecification(t *testing.T) {
	if problems := validate(t, filepath.Join("..", "..")); len(problems) != 0 {
		t.Errorf("want no problems, got %q", problems)
	}
}
