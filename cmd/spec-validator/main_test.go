package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func execute(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

// project writes a one-document project; content is its single item file.
func project(t *testing.T, content string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "req.adoc", "include::req/item.adoc[]\n")
	writeFile(t, root, "req/item.adoc", content)
	writeFile(t, root, ".spec-validator.json", `{"documents": [{"master": "req.adoc", "folder": "req", "kinds": ["R"]}]}`)
	return root
}

func TestValidProject(t *testing.T) {
	code, stdout, _ := execute(project(t, "[#R-1]\n=== One\n"))

	if code != exitValid || !strings.Contains(stdout, "specifications are valid") {
		t.Errorf("got %d, %q", code, stdout)
	}
}

func TestProblems(t *testing.T) {
	code, _, stderr := execute(project(t, "[#R-1]\n=== One <<R-2>>\n"))

	if code != exitProblems {
		t.Errorf("want exit %d, got %d", exitProblems, code)
	}
	if !strings.Contains(stderr, "req/item.adoc: unresolved reference <<R-2>>") || !strings.Contains(stderr, "1 problem(s) found") {
		t.Errorf("unexpected output %q", stderr)
	}
}

func TestExplicitConfig(t *testing.T) {
	root := project(t, "[#R-1]\n=== One\n")
	writeFile(t, root, "other.json", `{"documents": [{"master": "missing.adoc", "folder": "req", "kinds": ["R"]}]}`)

	code, _, stderr := execute("-config", filepath.Join(root, "other.json"), root)

	if code != exitProblems || !strings.Contains(stderr, "missing.adoc: master file is missing") {
		t.Errorf("got %d, %q", code, stderr)
	}
}

func TestUsageErrors(t *testing.T) {
	root := project(t, "[#R-1]\n=== One\n")
	writeFile(t, root, "broken.json", `{"documents": `)
	cases := map[string][]string{
		"two roots":      {root, root},
		"no directory":   {filepath.Join(root, "req.adoc")},
		"unknown flag":   {"-strict", root},
		"broken config":  {"-config", filepath.Join(root, "broken.json"), root},
		"missing config": {"-config", filepath.Join(root, "none.json"), root},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			if code, _, _ := execute(args...); code != exitUsage {
				t.Errorf("want exit %d, got %d", exitUsage, code)
			}
		})
	}
}

func TestVersion(t *testing.T) {
	code, stdout, _ := execute("-version")

	if code != exitValid || strings.TrimSpace(stdout) == "" {
		t.Errorf("got %d, %q", code, stdout)
	}
}
