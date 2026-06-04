package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/diagnosticwriter"
	"github.com/microsoft/typescript-go/internal/locale"
	"github.com/microsoft/typescript-go/internal/parser"
	"github.com/microsoft/typescript-go/internal/scanner"
	"github.com/microsoft/typescript-go/internal/tspath"
	"github.com/microsoft/typescript-go/internal/vfs/osvfs"
)

func runGuarantee(args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printGuaranteeHelp()
		return 0
	}
	switch args[0] {
	case "scan":
		return runGuaranteeScan(args[1:])
	case "init":
		return runGuaranteeInit(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown guarantee command %q.\n\n", args[0])
		printGuaranteeHelp()
		return 2
	}
}

func printGuaranteeHelp() {
	fmt.Fprint(os.Stdout, `tsgo guarantee

Runtime guarantee onboarding commands.

Commands:
  scan [path]       Scan TypeScript/JavaScript files and report runtime-guarantee risks.
  init [path]       Print a starter migration config for a project.

Scan options:
  --strict          Report risks as strict-mode errors.
  --ignore pattern  Skip a file or directory by name, relative path, or glob. Can be repeated.

`)
}

func runGuaranteeScan(args []string) int {
	flags := flag.NewFlagSet("guarantee scan", flag.ContinueOnError)
	strict := flags.Bool("strict", false, "report risks as strict-mode errors")
	var ignorePatterns runtimeGuaranteeIgnorePatterns
	flags.Var(&ignorePatterns, "ignore", "skip a file or directory by name, relative path, or glob; can be repeated")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	root := "."
	if flags.NArg() > 0 {
		root = flags.Arg(0)
	}
	root, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	options := &core.CompilerOptions{RuntimeGuarantees: core.RuntimeGuaranteesModeObserve}
	if *strict {
		options.RuntimeGuarantees = core.RuntimeGuaranteesModeStrict
	}

	files := 0
	risks := 0
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			rel = ""
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "dist", "build", "coverage", ".next", ".turbo":
				return filepath.SkipDir
			}
			if ignorePatterns.match(rel, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if ignorePatterns.match(rel, d.Name()) {
			return nil
		}
		scriptKind := core.GetScriptKindFromFileName(path)
		if scriptKind == core.ScriptKindUnknown || scriptKind == core.ScriptKindJSON {
			return nil
		}
		text, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fileName := tspath.NormalizePath(path)
		sourceFile := parser.ParseSourceFile(ast.SourceFileParseOptions{
			FileName: fileName,
			Path:     tspath.ToPath(fileName, tspath.NormalizePath(root), osvfs.FS().UseCaseSensitiveFileNames()),
		}, string(text), scriptKind)
		files++
		diags := compiler.GetRuntimeGuaranteeDiagnostics(sourceFile, options, !*strict)
		for _, diag := range diags {
			risks++
			line, character := scanner.GetECMALineAndUTF16CharacterOfPosition(sourceFile, diag.Pos())
			if rel == "" {
				rel = path
			}
			message := diagnosticwriter.FlattenDiagnosticMessage(diagnosticwriter.WrapASTDiagnostic(diag), "\n", locale.Default)
			fmt.Fprintf(os.Stdout, "%s:%d:%d - %s\n", filepath.ToSlash(rel), line+1, character+1, message)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fmt.Fprintf(os.Stdout, "\nRuntime guarantee scan: %d risk(s) across %d file(s).\n", risks, files)
	if *strict && risks > 0 {
		return 1
	}
	return 0
}

type runtimeGuaranteeIgnorePatterns []string

func (patterns *runtimeGuaranteeIgnorePatterns) String() string {
	return strings.Join(*patterns, ",")
}

func (patterns *runtimeGuaranteeIgnorePatterns) Set(value string) error {
	for _, pattern := range strings.Split(value, ",") {
		pattern = strings.TrimSpace(filepath.ToSlash(pattern))
		pattern = strings.TrimPrefix(pattern, "./")
		pattern = strings.Trim(pattern, "/")
		if pattern != "" {
			*patterns = append(*patterns, pattern)
		}
	}
	return nil
}

func (patterns runtimeGuaranteeIgnorePatterns) match(rel string, base string) bool {
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "./")
	rel = strings.Trim(rel, "/")
	for _, pattern := range patterns {
		if runtimeGuaranteeIgnorePatternMatches(pattern, rel, base) {
			return true
		}
	}
	return false
}

func runtimeGuaranteeIgnorePatternMatches(pattern string, rel string, base string) bool {
	if pattern == base || pattern == rel {
		return true
	}
	if !strings.Contains(pattern, "/") {
		if strings.HasPrefix(rel, pattern+"/") {
			return true
		}
		matched, _ := pathpkg.Match(pattern, base)
		return matched
	}
	trimmed := strings.TrimSuffix(pattern, "/**")
	if trimmed != pattern && (rel == trimmed || strings.HasPrefix(rel, trimmed+"/")) {
		return true
	}
	if strings.HasSuffix(pattern, "/") && strings.HasPrefix(rel, strings.TrimSuffix(pattern, "/")+"/") {
		return true
	}
	if strings.HasPrefix(rel, pattern+"/") {
		return true
	}
	matched, _ := pathpkg.Match(pattern, rel)
	return matched
}

func runGuaranteeInit(args []string) int {
	flags := flag.NewFlagSet("guarantee init", flag.ContinueOnError)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	root := "."
	if flags.NArg() > 0 {
		root = flags.Arg(0)
	}
	root = strings.TrimRight(root, string(filepath.Separator))
	fmt.Fprintf(os.Stdout, `Suggested migration config for %s:

{
  "compilerOptions": {
    "runtimeGuarantees": "observe"
  }
}

Then run:
  tsgo guarantee scan %s

Use --ignore for generated assets, for example:
  tsgo guarantee scan --ignore public --ignore "*.generated.js" %s

Use "runtimeGuarantees": "strict" or "build" once reported risks are modeled with @runtime-* annotations or isolated with @runtime-unsafe.
`, root, root, root)
	return 0
}
