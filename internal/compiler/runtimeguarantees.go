package compiler

import (
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/diagnostics"
	"github.com/microsoft/typescript-go/internal/scanner"
)

func getRuntimeGuaranteeDiagnostics(sourceFile *ast.SourceFile, options *core.CompilerOptions, suggestions bool) []*ast.Diagnostic {
	mode := runtimeGuaranteeModeForFile(sourceFile, options)
	if sourceFile == nil || !mode.ReportsRuntimeGuaranteeRisks() {
		return nil
	}

	strict := mode.EnforcesRuntimeGuaranteeRisks()
	var diags []*ast.Diagnostic
	add := func(node *ast.Node, risk string) {
		if node == nil {
			return
		}
		message := diagnostics.Runtime_guarantees_cannot_model_0_Add_a_runtime_contract_handler_or_unsafe_boundary_before_enabling_strict_mode
		if suggestions {
			message = diagnostics.Runtime_guarantees_observed_0_Add_a_runtime_contract_handler_or_unsafe_boundary_before_enabling_strict_mode
		}
		diag := ast.NewDiagnostic(sourceFile, node.Loc, message, risk)
		if suggestions {
			diag.SetCategory(diagnostics.CategorySuggestion)
		}
		diags = append(diags, diag)
	}

	var walk ast.Visitor
	walk = func(node *ast.Node) bool {
		if runtimeGuaranteeAnnotationsForNode(sourceFile, node).unsafe {
			return false
		}

		checkRuntimeGuaranteeDeclaredClause(sourceFile, node, add)

		switch node.Kind {
		case ast.KindFunctionDeclaration:
			checkRuntimeGuaranteeFunctionLike(node, node.AsFunctionDeclaration().Parameters, node.AsFunctionDeclaration().Type, strict, add)
		case ast.KindFunctionExpression:
			checkRuntimeGuaranteeFunctionLike(node, node.AsFunctionExpression().Parameters, node.AsFunctionExpression().Type, strict, add)
		case ast.KindArrowFunction:
			checkRuntimeGuaranteeFunctionLike(node, node.AsArrowFunction().Parameters, node.AsArrowFunction().Type, strict, add)
		case ast.KindMethodDeclaration:
			checkRuntimeGuaranteeFunctionLike(node, node.AsMethodDeclaration().Parameters, node.AsMethodDeclaration().Type, strict, add)
		case ast.KindConstructor:
			checkRuntimeGuaranteeFunctionLike(node, node.AsConstructorDeclaration().Parameters, nil, strict, add)
		case ast.KindGetAccessor:
			checkRuntimeGuaranteeFunctionLike(node, node.AsGetAccessorDeclaration().Parameters, node.AsGetAccessorDeclaration().Type, strict, add)
		case ast.KindSetAccessor:
			checkRuntimeGuaranteeFunctionLike(node, node.AsSetAccessorDeclaration().Parameters, nil, strict, add)
		case ast.KindCallExpression:
			checkRuntimeGuaranteeCall(sourceFile, node, add)
		case ast.KindThrowStatement:
			if !runtimeGuaranteeAnnotationsForContext(sourceFile, node).throws {
				add(node, "an explicit throw statement")
			}
		case ast.KindAsExpression, ast.KindTypeAssertionExpression:
			add(node, "an unchecked type assertion")
		case ast.KindNonNullExpression:
			add(node, "a non-null assertion")
		case ast.KindPropertyAccessExpression:
			checkRuntimeGuaranteePropertyAccess(sourceFile, node, add)
		case ast.KindElementAccessExpression:
			if strict {
				add(node, "unchecked element access")
			}
		case ast.KindAwaitExpression:
			if strict && !runtimeGuaranteeAnnotationsForContext(sourceFile, node).throws {
				add(node, "an awaited operation whose rejection path is not modeled")
			}
		case ast.KindForStatement, ast.KindForInStatement, ast.KindForOfStatement, ast.KindWhileStatement, ast.KindDoStatement:
			if strict && !runtimeGuaranteeAnnotationsForNode(sourceFile, node).bounded && !runtimeGuaranteeAnnotationsForContext(sourceFile, node).total {
				add(node, "an unbounded loop or iteration")
			}
		}

		node.ForEachChild(walk)
		return false
	}
	sourceFile.AsNode().ForEachChild(walk)
	return diags
}

func GetRuntimeGuaranteeDiagnostics(sourceFile *ast.SourceFile, options *core.CompilerOptions, suggestions bool) []*ast.Diagnostic {
	return getRuntimeGuaranteeDiagnostics(sourceFile, options, suggestions)
}

func runtimeGuaranteeModeForFile(sourceFile *ast.SourceFile, options *core.CompilerOptions) core.RuntimeGuaranteesMode {
	if sourceFile != nil && hasRuntimeGuaranteedDirective(sourceFile) {
		return core.RuntimeGuaranteesModeStrict
	}
	return options.RuntimeGuarantees
}

func hasRuntimeGuaranteedDirective(sourceFile *ast.SourceFile) bool {
	if sourceFile == nil {
		return false
	}
	for _, statement := range sourceFile.Statements.Nodes {
		if !ast.IsPrologueDirective(statement) {
			return false
		}
		expression := statement.Expression()
		if expression != nil && expression.Text() == "use runtime guaranteed" {
			return true
		}
	}
	return false
}

func checkRuntimeGuaranteeFunctionLike(node *ast.Node, parameters *ast.ParameterList, typeNode *ast.Node, strict bool, add func(*ast.Node, string)) {
	if parameters != nil {
		for _, param := range parameters.Nodes {
			if param == nil || param.Kind != ast.KindParameter || ast.IsThisParameter(param) {
				continue
			}
			p := param.AsParameterDeclaration()
			if p.DotDotDotToken != nil {
				add(param, "a rest parameter contract")
			}
			if p.Name() != nil && !ast.IsIdentifier(p.Name()) {
				add(p.Name(), "a destructured parameter contract")
			}
			if p.Type != nil && runtimeGuaranteeTag(p.Type) == "" {
				add(p.Type, "the parameter type "+runtimeGuaranteeTypeDescription(p.Type))
			}
			if strict && p.Type == nil {
				add(param, "an unannotated parameter")
			}
		}
	}
	if typeNode != nil && runtimeGuaranteeTag(typeNode) == "" {
		add(typeNode, "the return type "+runtimeGuaranteeTypeDescription(typeNode))
	} else if strict && typeNode == nil && node.Kind != ast.KindConstructor && node.Kind != ast.KindSetAccessor {
		add(node, "an unannotated return type")
	}
}

var knownRuntimeGuaranteeEffects = map[string]bool{
	"clock":   true,
	"dom":     true,
	"env":     true,
	"fs":      true,
	"io":      true,
	"module":  true,
	"network": true,
	"process": true,
	"random":  true,
	"storage": true,
	"time":    true,
}

func checkRuntimeGuaranteeDeclaredClause(sourceFile *ast.SourceFile, node *ast.Node, add func(*ast.Node, string)) {
	if sourceFile == nil || node == nil {
		return
	}
	clause, ok := sourceFile.RuntimeGuarantees[node]
	if !ok {
		return
	}
	for _, effect := range clause.Effects {
		if !knownRuntimeGuaranteeEffects[effect] {
			add(node, "an unknown runtime effect '"+effect+"'")
		}
	}
	if clause.HasValidates && len(clause.Validates) == 0 {
		add(node, "a validation clause with no validated shape")
	}
}

func checkRuntimeGuaranteeCall(sourceFile *ast.SourceFile, node *ast.Node, add func(*ast.Node, string)) {
	call := node.AsCallExpression()
	annotations := runtimeGuaranteeAnnotationsForContext(sourceFile, node).merge(runtimeGuaranteeAnnotationsForNode(sourceFile, node))
	if ast.IsImportCall(node) {
		if !annotations.hasEffect("module") {
			add(node, "a dynamic import effect")
		}
		return
	}
	switch call.Expression.Kind {
	case ast.KindIdentifier:
		switch call.Expression.Text() {
		case "fetch":
			if !annotations.hasEffect("network") {
				add(node, "a network effect from fetch")
			}
		case "eval":
			add(node, "dynamic code execution through eval")
		case "setTimeout", "setInterval":
			if !annotations.hasEffect("time") {
				add(node, "a time/scheduler effect")
			}
		case "require":
			if !annotations.hasEffect("module") {
				add(node, "a CommonJS require boundary")
			}
		}
	case ast.KindPropertyAccessExpression:
		access := call.Expression.AsPropertyAccessExpression()
		if access.Name() == nil {
			return
		}
		switch runtimeGuaranteeExpressionName(access.Expression) + "." + access.Name().Text() {
		case "JSON.parse":
			if !annotations.throws || !annotations.validates {
				add(node, "JSON.parse, which can throw and returns unvalidated input")
			}
		case "process.env":
			if !annotations.hasEffect("env") {
				add(node, "process.env input")
			}
		}
	}
}

func checkRuntimeGuaranteePropertyAccess(sourceFile *ast.SourceFile, node *ast.Node, add func(*ast.Node, string)) {
	access := node.AsPropertyAccessExpression()
	if access.Name() == nil {
		return
	}
	if runtimeGuaranteeExpressionName(access.Expression) == "process" && access.Name().Text() == "env" {
		if !runtimeGuaranteeAnnotationsForContext(sourceFile, node).hasEffect("env") {
			add(node, "process.env input")
		}
	}
}

type runtimeGuaranteeAnnotations struct {
	unsafe    bool
	throws    bool
	validates bool
	total     bool
	bounded   bool
	effects   map[string]bool
}

func (annotations runtimeGuaranteeAnnotations) hasEffect(effect string) bool {
	return annotations.effects != nil && annotations.effects[effect]
}

func (annotations runtimeGuaranteeAnnotations) merge(other runtimeGuaranteeAnnotations) runtimeGuaranteeAnnotations {
	if other.unsafe {
		annotations.unsafe = true
	}
	if other.throws {
		annotations.throws = true
	}
	if other.validates {
		annotations.validates = true
	}
	if other.total {
		annotations.total = true
	}
	if other.bounded {
		annotations.bounded = true
	}
	for effect := range other.effects {
		if annotations.effects == nil {
			annotations.effects = make(map[string]bool)
		}
		annotations.effects[effect] = true
	}
	return annotations
}

func runtimeGuaranteeAnnotationsForContext(sourceFile *ast.SourceFile, node *ast.Node) runtimeGuaranteeAnnotations {
	annotations := runtimeGuaranteeAnnotationsForNode(sourceFile, node)
	if fn := ast.FindAncestor(node, ast.IsFunctionLike); fn != nil {
		annotations = annotations.merge(runtimeGuaranteeAnnotationsForNode(sourceFile, fn))
	}
	return annotations
}

func runtimeGuaranteeAnnotationsForNode(sourceFile *ast.SourceFile, node *ast.Node) runtimeGuaranteeAnnotations {
	var result runtimeGuaranteeAnnotations
	if sourceFile == nil || node == nil {
		return result
	}
	text := sourceFile.Text()
	for commentRange := range scanner.GetLeadingCommentRanges(&ast.NodeFactory{}, text, node.Pos()) {
		comment := text[commentRange.Pos():commentRange.End()]
		result = result.merge(parseRuntimeGuaranteeAnnotationComment(comment))
	}
	if clause, ok := sourceFile.RuntimeGuarantees[node]; ok {
		result = result.merge(runtimeGuaranteeAnnotationsForClause(clause))
	}
	return result
}

func runtimeGuaranteeAnnotationsForClause(clause ast.RuntimeGuaranteeClause) runtimeGuaranteeAnnotations {
	var result runtimeGuaranteeAnnotations
	if len(clause.Throws) > 0 {
		result.throws = true
	}
	if len(clause.Validates) > 0 {
		result.validates = true
	}
	if clause.Total {
		result.total = true
	}
	if clause.Bounded {
		result.bounded = true
	}
	for _, effect := range clause.Effects {
		if result.effects == nil {
			result.effects = make(map[string]bool)
		}
		result.effects[strings.ToLower(effect)] = true
	}
	return result
}

func parseRuntimeGuaranteeAnnotationComment(comment string) runtimeGuaranteeAnnotations {
	var result runtimeGuaranteeAnnotations
	normalized := strings.NewReplacer("*", " ", "/", " ", ",", " ", "[", " ", "]", " ", ":", " ").Replace(comment)
	words := strings.Fields(normalized)
	for i, word := range words {
		switch word {
		case "@runtime-unsafe", "@runtimeUnsafe":
			result.unsafe = true
		case "@runtime-throws", "@runtimeThrows":
			result.throws = true
		case "@runtime-validates", "@runtimeValidates":
			result.validates = true
		case "@runtime-total", "@runtimeTotal":
			result.total = true
		case "@runtime-bounded", "@runtimeBounded":
			result.bounded = true
		case "@runtime-effects", "@runtimeEffects", "@runtime-resource", "@runtimeResource":
			for _, effect := range words[i+1:] {
				if strings.HasPrefix(effect, "@") {
					break
				}
				if result.effects == nil {
					result.effects = make(map[string]bool)
				}
				result.effects[strings.ToLower(effect)] = true
			}
		}
	}
	return result
}

func runtimeGuaranteeExpressionName(node *ast.Node) string {
	if node == nil {
		return ""
	}
	switch node.Kind {
	case ast.KindIdentifier:
		return node.Text()
	case ast.KindPropertyAccessExpression:
		access := node.AsPropertyAccessExpression()
		if access.Name() == nil {
			return runtimeGuaranteeExpressionName(access.Expression)
		}
		base := runtimeGuaranteeExpressionName(access.Expression)
		if base == "" {
			return access.Name().Text()
		}
		return base + "." + access.Name().Text()
	}
	return ""
}

func runtimeGuaranteeTag(typeNode *ast.Node) string {
	if typeNode == nil {
		return ""
	}
	switch typeNode.Kind {
	case ast.KindStringKeyword:
		return "string"
	case ast.KindNumberKeyword:
		return "number"
	case ast.KindBooleanKeyword:
		return "boolean"
	case ast.KindBigIntKeyword:
		return "bigint"
	case ast.KindSymbolKeyword:
		return "symbol"
	case ast.KindUndefinedKeyword:
		return "undefined"
	case ast.KindObjectKeyword:
		return "object"
	case ast.KindTypeReference:
		if name := typeNode.Name(); name != nil && ast.IsIdentifier(name) && name.Text() == "Function" {
			return "function"
		}
	}
	return ""
}

func runtimeGuaranteeTypeDescription(typeNode *ast.Node) string {
	if typeNode == nil {
		return "unknown"
	}
	return typeNode.Kind.String()
}
