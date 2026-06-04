package tstransforms

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/printer"
	"github.com/microsoft/typescript-go/internal/transformers"
)

type RuntimeGuaranteesTransformer struct {
	transformers.Transformer
	compilerOptions          *core.CompilerOptions
	returnTag                string
	returnLabel              string
	boundaryClass            bool
	boundaryExportedVariable bool
}

func NewRuntimeGuaranteesTransformer(opt *transformers.TransformOptions) *transformers.Transformer {
	if !opt.CompilerOptions.RuntimeGuarantees.EmitsRuntimeGuarantees() {
		return nil
	}
	tx := &RuntimeGuaranteesTransformer{compilerOptions: opt.CompilerOptions}
	return tx.NewTransformer(tx.visit, opt.Context)
}

func (tx *RuntimeGuaranteesTransformer) visit(node *ast.Node) *ast.Node {
	switch node.Kind {
	case ast.KindSourceFile:
		visited := tx.Visitor().VisitEachChild(node)
		tx.EmitContext().AddEmitHelper(visited, tx.EmitContext().ReadEmitHelpers()...)
		return visited
	case ast.KindClassDeclaration:
		return tx.visitClassDeclaration(node.AsClassDeclaration())
	case ast.KindClassExpression:
		return tx.visitClassExpression(node.AsClassExpression())
	case ast.KindVariableStatement:
		return tx.visitVariableStatement(node.AsVariableStatement())
	case ast.KindFunctionDeclaration:
		return tx.visitFunctionDeclaration(node.AsFunctionDeclaration())
	case ast.KindFunctionExpression:
		return tx.visitFunctionExpression(node.AsFunctionExpression())
	case ast.KindArrowFunction:
		return tx.visitArrowFunction(node.AsArrowFunction())
	case ast.KindMethodDeclaration:
		return tx.visitMethodDeclaration(node.AsMethodDeclaration())
	case ast.KindConstructor:
		return tx.visitConstructorDeclaration(node.AsConstructorDeclaration())
	case ast.KindGetAccessor:
		return tx.visitGetAccessorDeclaration(node.AsGetAccessorDeclaration())
	case ast.KindSetAccessor:
		return tx.visitSetAccessorDeclaration(node.AsSetAccessorDeclaration())
	case ast.KindReturnStatement:
		return tx.visitReturnStatement(node.AsReturnStatement())
	default:
		return tx.Visitor().VisitEachChild(node)
	}
}

func (tx *RuntimeGuaranteesTransformer) visitClassDeclaration(node *ast.ClassDeclaration) *ast.Node {
	savedBoundaryClass := tx.boundaryClass
	tx.boundaryClass = tx.shouldCheckClass(node.AsNode())
	defer func() { tx.boundaryClass = savedBoundaryClass }()
	return tx.Factory().UpdateClassDeclaration(
		node,
		tx.Visitor().VisitModifiers(node.Modifiers()),
		tx.Visitor().VisitNode(node.Name()),
		tx.Visitor().VisitNodes(node.TypeParameters),
		tx.Visitor().VisitNodes(node.HeritageClauses),
		tx.Visitor().VisitNodes(node.Members),
	)
}

func (tx *RuntimeGuaranteesTransformer) visitClassExpression(node *ast.ClassExpression) *ast.Node {
	savedBoundaryClass := tx.boundaryClass
	tx.boundaryClass = tx.compilerOptions.RuntimeGuarantees == core.RuntimeGuaranteesModeAll ||
		tx.compilerOptions.RuntimeGuarantees == core.RuntimeGuaranteesModeStrict
	defer func() { tx.boundaryClass = savedBoundaryClass }()
	return tx.Factory().UpdateClassExpression(
		node,
		tx.Visitor().VisitModifiers(node.Modifiers()),
		tx.Visitor().VisitNode(node.Name()),
		tx.Visitor().VisitNodes(node.TypeParameters),
		tx.Visitor().VisitNodes(node.HeritageClauses),
		tx.Visitor().VisitNodes(node.Members),
	)
}

func (tx *RuntimeGuaranteesTransformer) visitVariableStatement(node *ast.VariableStatement) *ast.Node {
	savedBoundaryExportedVariable := tx.boundaryExportedVariable
	tx.boundaryExportedVariable = tx.compilerOptions.RuntimeGuarantees == core.RuntimeGuaranteesModeBoundary &&
		ast.HasSyntacticModifier(node.AsNode(), ast.ModifierFlagsExportDefault)
	defer func() { tx.boundaryExportedVariable = savedBoundaryExportedVariable }()
	return tx.Factory().UpdateVariableStatement(
		node,
		tx.Visitor().VisitModifiers(node.Modifiers()),
		tx.Visitor().VisitNode(node.DeclarationList),
	)
}

func (tx *RuntimeGuaranteesTransformer) visitFunctionDeclaration(node *ast.FunctionDeclaration) *ast.Node {
	if ast.NodeIsMissing(node.Body) {
		return tx.Visitor().VisitEachChild(node.AsNode())
	}
	check := tx.shouldCheckDeclaration(node.AsNode())
	body := tx.visitFunctionBody(node.AsNode(), node.Parameters, node.Type, node.Body, check)
	return tx.Factory().UpdateFunctionDeclaration(
		node,
		tx.Visitor().VisitModifiers(node.Modifiers()),
		node.AsteriskToken,
		tx.Visitor().VisitNode(node.Name()),
		tx.Visitor().VisitNodes(node.TypeParameters),
		tx.Visitor().VisitNodes(node.Parameters),
		tx.Visitor().VisitNode(node.Type),
		tx.Visitor().VisitNode(node.FullSignature),
		body,
	)
}

func (tx *RuntimeGuaranteesTransformer) visitFunctionExpression(node *ast.FunctionExpression) *ast.Node {
	check := tx.compilerOptions.RuntimeGuarantees == core.RuntimeGuaranteesModeAll ||
		tx.compilerOptions.RuntimeGuarantees == core.RuntimeGuaranteesModeStrict ||
		tx.boundaryExportedVariable
	body := tx.visitFunctionBody(node.AsNode(), node.Parameters, node.Type, node.Body, check)
	return tx.Factory().UpdateFunctionExpression(
		node,
		tx.Visitor().VisitModifiers(node.Modifiers()),
		node.AsteriskToken,
		tx.Visitor().VisitNode(node.Name()),
		tx.Visitor().VisitNodes(node.TypeParameters),
		tx.Visitor().VisitNodes(node.Parameters),
		tx.Visitor().VisitNode(node.Type),
		tx.Visitor().VisitNode(node.FullSignature),
		body,
	)
}

func (tx *RuntimeGuaranteesTransformer) visitArrowFunction(node *ast.ArrowFunction) *ast.Node {
	check := tx.compilerOptions.RuntimeGuarantees == core.RuntimeGuaranteesModeAll ||
		tx.compilerOptions.RuntimeGuarantees == core.RuntimeGuaranteesModeStrict ||
		tx.boundaryExportedVariable
	body := tx.visitConciseBody(node.AsNode(), node.Parameters, node.Type, node.Body, check)
	return tx.Factory().UpdateArrowFunction(
		node,
		tx.Visitor().VisitModifiers(node.Modifiers()),
		tx.Visitor().VisitNodes(node.TypeParameters),
		tx.Visitor().VisitNodes(node.Parameters),
		tx.Visitor().VisitNode(node.Type),
		tx.Visitor().VisitNode(node.FullSignature),
		node.EqualsGreaterThanToken,
		body,
	)
}

func (tx *RuntimeGuaranteesTransformer) visitMethodDeclaration(node *ast.MethodDeclaration) *ast.Node {
	if ast.NodeIsMissing(node.Body) {
		return tx.Visitor().VisitEachChild(node.AsNode())
	}
	check := tx.shouldCheckClassMember(node.AsNode())
	body := tx.visitFunctionBody(node.AsNode(), node.Parameters, node.Type, node.Body, check)
	return tx.Factory().UpdateMethodDeclaration(
		node,
		tx.Visitor().VisitModifiers(node.Modifiers()),
		node.AsteriskToken,
		tx.Visitor().VisitNode(node.Name()),
		node.PostfixToken,
		tx.Visitor().VisitNodes(node.TypeParameters),
		tx.Visitor().VisitNodes(node.Parameters),
		tx.Visitor().VisitNode(node.Type),
		tx.Visitor().VisitNode(node.FullSignature),
		body,
	)
}

func (tx *RuntimeGuaranteesTransformer) visitConstructorDeclaration(node *ast.ConstructorDeclaration) *ast.Node {
	if ast.NodeIsMissing(node.Body) {
		return tx.Visitor().VisitEachChild(node.AsNode())
	}
	check := tx.shouldCheckClassMember(node.AsNode())
	body := tx.visitFunctionBody(node.AsNode(), node.Parameters, nil, node.Body, check)
	return tx.Factory().UpdateConstructorDeclaration(
		node,
		tx.Visitor().VisitModifiers(node.Modifiers()),
		tx.Visitor().VisitNodes(node.TypeParameters),
		tx.Visitor().VisitNodes(node.Parameters),
		tx.Visitor().VisitNode(node.Type),
		tx.Visitor().VisitNode(node.FullSignature),
		body,
	)
}

func (tx *RuntimeGuaranteesTransformer) visitGetAccessorDeclaration(node *ast.GetAccessorDeclaration) *ast.Node {
	check := tx.shouldCheckClassMember(node.AsNode())
	body := tx.visitFunctionBody(node.AsNode(), node.Parameters, node.Type, node.Body, check)
	return tx.Factory().UpdateGetAccessorDeclaration(
		node,
		tx.Visitor().VisitModifiers(node.Modifiers()),
		tx.Visitor().VisitNode(node.Name()),
		tx.Visitor().VisitNodes(node.TypeParameters),
		tx.Visitor().VisitNodes(node.Parameters),
		tx.Visitor().VisitNode(node.Type),
		tx.Visitor().VisitNode(node.FullSignature),
		body,
	)
}

func (tx *RuntimeGuaranteesTransformer) visitSetAccessorDeclaration(node *ast.SetAccessorDeclaration) *ast.Node {
	check := tx.shouldCheckClassMember(node.AsNode())
	body := tx.visitFunctionBody(node.AsNode(), node.Parameters, nil, node.Body, check)
	return tx.Factory().UpdateSetAccessorDeclaration(
		node,
		tx.Visitor().VisitModifiers(node.Modifiers()),
		tx.Visitor().VisitNode(node.Name()),
		tx.Visitor().VisitNodes(node.TypeParameters),
		tx.Visitor().VisitNodes(node.Parameters),
		tx.Visitor().VisitNode(node.Type),
		tx.Visitor().VisitNode(node.FullSignature),
		body,
	)
}

func (tx *RuntimeGuaranteesTransformer) visitReturnStatement(node *ast.ReturnStatement) *ast.Node {
	expression := tx.Visitor().VisitNode(node.Expression)
	if tx.returnTag != "" && expression != nil {
		expression = tx.newGuaranteeCall(expression, tx.returnTag, tx.returnLabel+" return", false)
	}
	return tx.Factory().UpdateReturnStatement(node, expression)
}

func (tx *RuntimeGuaranteesTransformer) visitFunctionBody(node *ast.Node, parameters *ast.ParameterList, typeNode *ast.Node, body *ast.Node, check bool) *ast.Node {
	if body == nil {
		return tx.Visitor().VisitNode(body)
	}
	returnTag := ""
	if check {
		returnTag = runtimeGuaranteeTag(typeNode)
	}
	savedReturnTag, savedReturnLabel := tx.returnTag, tx.returnLabel
	tx.returnTag = returnTag
	tx.returnLabel = functionLabel(node)
	visitedBody := tx.Visitor().VisitNode(body)
	tx.returnTag, tx.returnLabel = savedReturnTag, savedReturnLabel
	if !check || visitedBody == nil {
		return visitedBody
	}
	return tx.prependChecks(visitedBody, tx.parameterCheckStatements(parameters))
}

func (tx *RuntimeGuaranteesTransformer) visitConciseBody(node *ast.Node, parameters *ast.ParameterList, typeNode *ast.Node, body *ast.Node, check bool) *ast.Node {
	if body == nil {
		return tx.Visitor().VisitNode(body)
	}
	returnTag := ""
	if check {
		returnTag = runtimeGuaranteeTag(typeNode)
	}
	if body.Kind == ast.KindBlock {
		return tx.visitFunctionBody(node, parameters, typeNode, body, check)
	}
	visitedBody := tx.Visitor().VisitNode(body)
	if !check {
		return visitedBody
	}
	if returnTag != "" && visitedBody != nil {
		visitedBody = tx.newGuaranteeCall(visitedBody, returnTag, functionLabel(node)+" return", false)
	}
	checks := tx.parameterCheckStatements(parameters)
	if len(checks) == 0 {
		return visitedBody
	}
	statements := append(checks, tx.Factory().NewReturnStatement(visitedBody))
	return tx.Factory().NewBlock(tx.Factory().NewNodeList(statements), true)
}

func (tx *RuntimeGuaranteesTransformer) prependChecks(body *ast.Node, checks []*ast.Node) *ast.Node {
	if len(checks) == 0 || body == nil || body.Kind != ast.KindBlock {
		return body
	}
	block := body.AsBlock()
	statements := block.Statements.Nodes
	insert := 0
	for insert < len(statements) && ast.IsPrologueDirective(statements[insert]) {
		insert++
	}
	newStatements := make([]*ast.Node, 0, len(statements)+len(checks))
	newStatements = append(newStatements, statements[:insert]...)
	newStatements = append(newStatements, checks...)
	newStatements = append(newStatements, statements[insert:]...)
	return tx.Factory().UpdateBlock(block, tx.Factory().NewNodeList(newStatements), block.MultiLine)
}

func (tx *RuntimeGuaranteesTransformer) parameterCheckStatements(parameters *ast.ParameterList) []*ast.Node {
	if parameters == nil {
		return nil
	}
	var checks []*ast.Node
	for _, param := range parameters.Nodes {
		if param == nil || param.Kind != ast.KindParameter || ast.IsThisParameter(param) {
			continue
		}
		p := param.AsParameterDeclaration()
		if p.DotDotDotToken != nil || p.Name() == nil || !ast.IsIdentifier(p.Name()) {
			continue
		}
		tag := runtimeGuaranteeTag(p.Type)
		if tag == "" {
			continue
		}
		checks = append(checks, tx.Factory().NewExpressionStatement(tx.newGuaranteeCall(
			tx.Factory().NewIdentifier(p.Name().Text()),
			tag,
			p.Name().Text(),
			p.QuestionToken != nil,
		)))
	}
	return checks
}

func (tx *RuntimeGuaranteesTransformer) newGuaranteeCall(value *ast.Node, tag string, label string, optional bool) *ast.Node {
	tx.EmitContext().RequestEmitHelper(printer.TsgGuaranteeHelper)
	return tx.Factory().NewCallExpression(
		tx.Factory().NewUnscopedHelperName("__tsg"),
		nil,
		nil,
		tx.Factory().NewNodeList([]*ast.Node{
			value,
			tx.Factory().NewStringLiteral(tag, ast.TokenFlagsNone),
			tx.Factory().NewStringLiteral(label, ast.TokenFlagsNone),
			tx.Factory().NewKeywordExpression(core.IfElse(optional, ast.KindTrueKeyword, ast.KindFalseKeyword)),
		}),
		ast.NodeFlagsNone,
	)
}

func (tx *RuntimeGuaranteesTransformer) shouldCheckClass(node *ast.Node) bool {
	return tx.compilerOptions.RuntimeGuarantees == core.RuntimeGuaranteesModeAll ||
		tx.compilerOptions.RuntimeGuarantees == core.RuntimeGuaranteesModeStrict ||
		ast.HasSyntacticModifier(node, ast.ModifierFlagsExportDefault)
}

func (tx *RuntimeGuaranteesTransformer) shouldCheckDeclaration(node *ast.Node) bool {
	return tx.compilerOptions.RuntimeGuarantees == core.RuntimeGuaranteesModeAll ||
		tx.compilerOptions.RuntimeGuarantees == core.RuntimeGuaranteesModeStrict ||
		ast.HasSyntacticModifier(node, ast.ModifierFlagsExportDefault)
}

func (tx *RuntimeGuaranteesTransformer) shouldCheckClassMember(node *ast.Node) bool {
	if tx.compilerOptions.RuntimeGuarantees == core.RuntimeGuaranteesModeAll ||
		tx.compilerOptions.RuntimeGuarantees == core.RuntimeGuaranteesModeStrict {
		return true
	}
	return tx.boundaryClass && !ast.HasSyntacticModifier(node, ast.ModifierFlagsNonPublicAccessibilityModifier)
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

func functionLabel(node *ast.Node) string {
	if node == nil {
		return "function"
	}
	if name := node.Name(); name != nil {
		return name.Text()
	}
	return "function"
}
