package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"slices"
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
  --fix             Write source updates for risks that can be safely modeled.
  --fix-dry-run     Print source updates that would be written, without changing files.
  --ignore pattern  Skip a file or directory by name, relative path, or glob. Can be repeated.

`)
}

type runtimeGuaranteeFixMode int

const (
	runtimeGuaranteeFixModeNone runtimeGuaranteeFixMode = iota
	runtimeGuaranteeFixModeApply
	runtimeGuaranteeFixModeDryRun
)

func runGuaranteeScan(args []string) int {
	flags := flag.NewFlagSet("guarantee scan", flag.ContinueOnError)
	strict := flags.Bool("strict", false, "report risks as strict-mode errors")
	fix := flags.Bool("fix", false, "write source updates for risks that can be safely modeled")
	fixDryRun := flags.Bool("fix-dry-run", false, "print source updates that would be written, without changing files")
	var ignorePatterns runtimeGuaranteeIgnorePatterns
	flags.Var(&ignorePatterns, "ignore", "skip a file or directory by name, relative path, or glob; can be repeated")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *fix && *fixDryRun {
		fmt.Fprintln(os.Stderr, "--fix and --fix-dry-run cannot be used together.")
		return 2
	}
	fixMode := runtimeGuaranteeFixModeNone
	if *fix {
		fixMode = runtimeGuaranteeFixModeApply
	} else if *fixDryRun {
		fixMode = runtimeGuaranteeFixModeDryRun
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
	fixes := 0
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
		sourceFile := parseRuntimeGuaranteeSourceFile(fileName, root, scriptKind, string(text))
		files++
		if fixMode != runtimeGuaranteeFixModeNone {
			fileFixes := compiler.GetRuntimeGuaranteeAutoFixes(sourceFile, options)
			if len(fileFixes) > 0 {
				if fixMode == runtimeGuaranteeFixModeDryRun {
					printRuntimeGuaranteeFixes(rel, path, sourceFile, fileFixes)
				} else {
					updated, edits, err := applyRuntimeGuaranteeFixes(sourceFile, string(text), fileFixes)
					if err != nil {
						return err
					}
					if edits > 0 && updated != string(text) {
						if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
							return err
						}
						fixes += edits
						sourceFile = parseRuntimeGuaranteeSourceFile(fileName, root, scriptKind, updated)
					}
				}
			}
		}
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

	fmt.Fprintf(os.Stdout, "\nRuntime guarantee scan: %d risk(s) across %d file(s).", risks, files)
	if fixMode == runtimeGuaranteeFixModeApply {
		fmt.Fprintf(os.Stdout, " Applied %d fix(es).", fixes)
	}
	fmt.Fprintln(os.Stdout)
	if *strict && risks > 0 {
		return 1
	}
	return 0
}

func parseRuntimeGuaranteeSourceFile(fileName string, root string, scriptKind core.ScriptKind, text string) *ast.SourceFile {
	return parser.ParseSourceFile(ast.SourceFileParseOptions{
		FileName: fileName,
		Path:     tspath.ToPath(fileName, tspath.NormalizePath(root), osvfs.FS().UseCaseSensitiveFileNames()),
	}, text, scriptKind)
}

type runtimeGuaranteeTextEdit struct {
	pos  int
	text string
}

func printRuntimeGuaranteeFixes(rel string, path string, sourceFile *ast.SourceFile, fixes []compiler.RuntimeGuaranteeAutoFix) {
	if rel == "" {
		rel = path
	}
	for _, fix := range fixes {
		pos, ok := runtimeGuaranteeFixInsertPosition(sourceFile, fix)
		if !ok {
			continue
		}
		line, character := scanner.GetECMALineAndUTF16CharacterOfPosition(sourceFile, pos)
		fmt.Fprintf(os.Stdout, "%s:%d:%d - fix-dry-run: %s\n", filepath.ToSlash(rel), line+1, character+1, runtimeGuaranteeFixDescription(fix))
	}
}

func applyRuntimeGuaranteeFixes(sourceFile *ast.SourceFile, text string, fixes []compiler.RuntimeGuaranteeAutoFix) (string, int, error) {
	edits := make([]runtimeGuaranteeTextEdit, 0, len(fixes))
	for _, fix := range fixes {
		pos, ok := runtimeGuaranteeFixInsertPosition(sourceFile, fix)
		if !ok {
			continue
		}
		editText, ok := runtimeGuaranteeFixText(sourceFile, text, pos, fix)
		if !ok {
			continue
		}
		edits = append(edits, runtimeGuaranteeTextEdit{pos: pos, text: editText})
	}
	slices.SortStableFunc(edits, func(a, b runtimeGuaranteeTextEdit) int {
		if a.pos != b.pos {
			return b.pos - a.pos
		}
		return strings.Compare(a.text, b.text)
	})
	for _, edit := range edits {
		if edit.pos < 0 || edit.pos > len(text) {
			return "", 0, fmt.Errorf("runtime guarantee fix position %d is outside source text", edit.pos)
		}
		text = text[:edit.pos] + edit.text + text[edit.pos:]
	}
	return text, len(edits), nil
}

func runtimeGuaranteeFixInsertPosition(sourceFile *ast.SourceFile, fix compiler.RuntimeGuaranteeAutoFix) (int, bool) {
	if fix.Node == nil {
		return 0, false
	}
	if fix.CommentAnnotation != "" {
		if sourceFile == nil {
			return fix.Node.Pos(), true
		}
		return scanner.GetTokenPosOfNode(fix.Node, sourceFile, false), true
	}
	if sourceFile != nil {
		if clause, ok := sourceFile.RuntimeGuarantees[fix.Node]; ok && !clause.IsEmpty() {
			return clause.Loc.End(), true
		}
	}
	switch fix.Node.Kind {
	case ast.KindFunctionDeclaration:
		node := fix.Node.AsFunctionDeclaration()
		if node.Type != nil {
			return node.Type.End(), true
		}
		if node.Parameters != nil {
			return runtimeGuaranteePositionAfterParameters(sourceFile, node.Parameters), true
		}
	case ast.KindFunctionExpression:
		node := fix.Node.AsFunctionExpression()
		if node.Type != nil {
			return node.Type.End(), true
		}
		if node.Parameters != nil {
			return runtimeGuaranteePositionAfterParameters(sourceFile, node.Parameters), true
		}
	case ast.KindArrowFunction:
		node := fix.Node.AsArrowFunction()
		if node.Type != nil {
			return node.Type.End(), true
		}
		if node.Parameters != nil {
			return runtimeGuaranteePositionAfterParameters(sourceFile, node.Parameters), true
		}
	case ast.KindMethodDeclaration:
		node := fix.Node.AsMethodDeclaration()
		if node.Type != nil {
			return node.Type.End(), true
		}
		if node.Parameters != nil {
			return runtimeGuaranteePositionAfterParameters(sourceFile, node.Parameters), true
		}
	case ast.KindConstructor:
		node := fix.Node.AsConstructorDeclaration()
		if node.Parameters != nil {
			return runtimeGuaranteePositionAfterParameters(sourceFile, node.Parameters), true
		}
	case ast.KindGetAccessor:
		node := fix.Node.AsGetAccessorDeclaration()
		if node.Type != nil {
			return node.Type.End(), true
		}
		if node.Parameters != nil {
			return runtimeGuaranteePositionAfterParameters(sourceFile, node.Parameters), true
		}
	case ast.KindSetAccessor:
		node := fix.Node.AsSetAccessorDeclaration()
		if node.Parameters != nil {
			return runtimeGuaranteePositionAfterParameters(sourceFile, node.Parameters), true
		}
	}
	return 0, false
}

func runtimeGuaranteePositionAfterParameters(sourceFile *ast.SourceFile, parameters *ast.ParameterList) int {
	pos := parameters.End()
	if sourceFile == nil {
		return pos
	}
	text := sourceFile.Text()
	if pos < 0 || pos >= len(text) {
		return pos
	}
	if close := runtimeGuaranteeFindNextToken(text, pos, ')'); close >= 0 {
		return close + 1
	}
	return pos
}

func runtimeGuaranteeFindNextToken(text string, pos int, token byte) int {
	for pos < len(text) {
		switch text[pos] {
		case ' ', '\t', '\n', '\r', '\f', '\v':
			pos++
		case '/':
			if pos+1 >= len(text) {
				return -1
			}
			switch text[pos+1] {
			case '/':
				pos += 2
				for pos < len(text) && text[pos] != '\n' && text[pos] != '\r' {
					pos++
				}
			case '*':
				pos += 2
				for pos+1 < len(text) && !(text[pos] == '*' && text[pos+1] == '/') {
					pos++
				}
				if pos+1 >= len(text) {
					return -1
				}
				pos += 2
			default:
				return -1
			}
		default:
			if text[pos] == token {
				return pos
			}
			return -1
		}
	}
	return -1
}

func runtimeGuaranteeFixText(sourceFile *ast.SourceFile, text string, pos int, fix compiler.RuntimeGuaranteeAutoFix) (string, bool) {
	if fix.CommentAnnotation != "" {
		indent := runtimeGuaranteeLineIndent(text, pos)
		return "/** " + fix.CommentAnnotation + " */\n" + indent, true
	}
	clauseText := runtimeGuaranteeClauseText(fix.Clause)
	if clauseText == "" {
		return "", false
	}
	prefix := " "
	if sourceFile != nil {
		if clause, ok := sourceFile.RuntimeGuarantees[fix.Node]; ok && !clause.IsEmpty() {
			prefix = " "
		}
	}
	return prefix + clauseText, true
}

func runtimeGuaranteeClauseText(clause ast.RuntimeGuaranteeClause) string {
	var parts []string
	if clause.HasThrows {
		parts = append(parts, "throws ["+strings.Join(clause.Throws, ", ")+"]")
	}
	if clause.HasEffects {
		parts = append(parts, "effects ["+strings.Join(clause.Effects, ", ")+"]")
	}
	if clause.HasValidates {
		parts = append(parts, "validates ["+strings.Join(clause.Validates, ", ")+"]")
	}
	if clause.Total {
		parts = append(parts, "total")
	}
	if clause.Bounded {
		parts = append(parts, "bounded")
	}
	return strings.Join(parts, " ")
}

func runtimeGuaranteeFixDescription(fix compiler.RuntimeGuaranteeAutoFix) string {
	if fix.CommentAnnotation != "" {
		return "add " + fix.CommentAnnotation + " annotation"
	}
	clause := runtimeGuaranteeClauseText(fix.Clause)
	if clause == "" {
		return fix.Reason
	}
	if fix.Reason == "" {
		return "add " + clause
	}
	return "add " + clause + " (" + fix.Reason + ")"
}

func runtimeGuaranteeLineIndent(text string, pos int) string {
	lineStart := strings.LastIndexByte(text[:pos], '\n') + 1
	var end int
	for end = lineStart; end < pos; end++ {
		if text[end] != ' ' && text[end] != '\t' {
			break
		}
	}
	return text[lineStart:end]
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
