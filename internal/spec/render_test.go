package spec

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// renderImage is the Asciidoctor image of the release-actions render step.
const renderImage = "asciidoctor/docker-asciidoctor:1.108.0@sha256:66041d57c25cec8b54cbf8b13141a83c3cb4768d41ccb3683cab28271c6775df"

// TestRenderChain runs the render chain of release-actions on a project with both reports:
// asciidoctor-reducer flattens each master, Link turns references across masters into xrefs,
// and asciidoctor must render every master without a warning.
func TestRenderChain(t *testing.T) {
	if testing.Short() {
		t.Skip("renders in Docker")
	}
	// The image is a Linux image; Docker on a Windows runner may run Windows containers only.
	if osType, err := exec.Command("docker", "info", "--format", "{{.OSType}}").Output(); err != nil {
		t.Skip("Docker is not available")
	} else if strings.TrimSpace(string(osType)) != "linux" {
		t.Skipf("Docker runs %s containers, the render image needs linux", strings.TrimSpace(string(osType)))
	}
	// Docker may share only the home directory (e.g. Colima), not the system's temp folders.
	// The work folder has a unique name, and the cleanup removes exactly that folder.
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(home, ".cache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	work, err := os.MkdirTemp(cache, "spec-validator-render-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(work) })
	project, out := filepath.Join(work, "project"), filepath.Join(work, "out")
	for _, dir := range []string{project, out} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	tr := validTree()
	tr.story = story + "\nAccepted by <<AT-1.1>>.\n"
	tr.writeTo(t, project)
	writeFile(t, project, "specification/acceptance/greeting.feature", greetingFeature)
	results, err := os.ReadFile("testdata/results.json")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, project, "results.json", string(results))
	config := bdtConfig()
	if problems, err := TestReport(project, config, filepath.Join(project, "results.json"), "2026-10-24"); err != nil || problems != nil {
		t.Fatal(problems, err)
	}
	if problems, err := TraceReport(project, config, filepath.Join(project, "results.json"), "2026-10-24"); err != nil || problems != nil {
		t.Fatal(problems, err)
	}

	docker := func(args ...string) {
		t.Helper()
		run := []string{"run", "--rm", "-e", "HOME=/tmp", "-v", project + ":/src:ro", "-v", out + ":/out", "-w", "/src"}
		if runtime.GOOS == "linux" {
			run = append(run, "-u", strconv.Itoa(os.Getuid())+":"+strconv.Itoa(os.Getgid()))
		}
		output, err := exec.Command("docker", append(append(run, renderImage), args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("docker %q: %v\n%s", args, err, output)
		}
	}
	masters := Masters(project, config)
	for _, master := range masters {
		docker("asciidoctor-reducer", "-o", "/out/flat/"+master, master)
	}
	if problems, err := Link(project, config, filepath.Join(out, "flat")); err != nil || problems != nil {
		t.Fatal(problems, err)
	}
	for _, master := range masters {
		html := master[:len(master)-len(".adoc")] + ".html"
		docker("asciidoctor", "-v", "--failure-level", "INFO", "-a", "revnumber=0.2.0", "-B", "/src",
			"-o", "/out/html/"+html, "/out/flat/"+master)
		if !isFile(filepath.Join(out, "html", html)) {
			t.Errorf("%s was not rendered", html)
		}
	}
	if len(masters) != 6 {
		t.Errorf("want 4 masters and 2 reports, got %q", masters)
	}
}
