// Command spec-validator checks a modular AsciiDoc specification: masters and include targets
// exist, every item file is included, anchors are unique and in the folder of their kind,
// references resolve and items trace to their parents.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/thst71/spec-validator/internal/spec"
)

// version is set by the release build.
var version = ""

const (
	exitValid    = 0
	exitProblems = 1
	exitUsage    = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("spec-validator", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "configuration file (default: <project-root>/"+spec.ConfigFile+" if it exists)")
	showVersion := flags.Bool("version", false, "print the version and exit")
	fix := flags.Bool("fix", false, "rewrite shorthand anchors [#ID] to [[ID]], then validate")
	listMasters := flags.Bool("masters", false, "print the configured master files, one per line")
	linkDir := flags.String("link", "", "in the flattened masters below `dir`, turn references to other masters into xrefs")
	testReport := flags.String("test-report", "", "write the test report master from the Cucumber JSON `results`")
	flags.Usage = func() {
		_, _ = fmt.Fprintf(flags.Output(), "Usage: spec-validator [flags] [project-root]\n\nFlags:\n")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitValid
		}
		return exitUsage
	}
	if *showVersion {
		_, _ = fmt.Fprintln(stdout, currentVersion())
		return exitValid
	}
	modes := 0
	for _, set := range []bool{*fix, *listMasters, *linkDir != "", *testReport != ""} {
		if set {
			modes++
		}
	}
	if flags.NArg() > 1 || modes > 1 {
		flags.Usage()
		return exitUsage
	}
	root := "."
	if flags.NArg() == 1 {
		root = flags.Arg(0)
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		_, _ = fmt.Fprintf(stderr, "spec-validator: %s is not a directory\n", root)
		return exitUsage
	}

	config, err := loadConfig(root, *configPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "spec-validator: %v\n", err)
		return exitUsage
	}
	var problems []string
	switch {
	case *listMasters:
		for _, master := range spec.Masters(config) {
			_, _ = fmt.Fprintln(stdout, master)
		}
		return exitValid
	case *linkDir != "":
		problems, err = spec.Link(root, config, *linkDir)
		if err == nil && len(problems) == 0 {
			_, _ = fmt.Fprintln(stdout, "references linked")
			return exitValid
		}
	case *testReport != "":
		problems, err = spec.TestReport(root, config, *testReport, time.Now().Format(time.DateOnly))
		if err == nil && len(problems) == 0 {
			_, _ = fmt.Fprintln(stdout, "test report written")
			return exitValid
		}
	default:
		if *fix {
			var changed []string
			if changed, err = spec.FixAnchors(root, config); err == nil {
				for _, file := range changed {
					_, _ = fmt.Fprintf(stdout, "fixed anchors in %s\n", file)
				}
			}
		}
		if err == nil {
			problems, err = spec.Validate(root, config)
		}
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "spec-validator: %v\n", err)
		return exitUsage
	}
	for _, problem := range problems {
		_, _ = fmt.Fprintln(stderr, problem)
	}
	if len(problems) > 0 {
		_, _ = fmt.Fprintf(stderr, "%d problem(s) found\n", len(problems))
		return exitProblems
	}
	_, _ = fmt.Fprintln(stdout, "specifications are valid")
	return exitValid
}

func loadConfig(root, path string) (spec.Config, error) {
	if path != "" {
		return spec.LoadConfig(path)
	}
	path = filepath.Join(root, spec.ConfigFile)
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return spec.DefaultConfig(), nil
	}
	return spec.LoadConfig(path)
}

func currentVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
