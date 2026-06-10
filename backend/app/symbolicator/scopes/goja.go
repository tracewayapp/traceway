package scopes

import (
	"regexp"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/file"
	"github.com/dop251/goja/parser"
)

func parseGoja(bundle []byte) ([]Transition, error) {
	src := string(bundle)
	prog, err := parser.ParseFile(nil, "", src, 0, parser.WithDisableSourceMaps)
	if err != nil {
		neutralized := neutralizeModuleSyntax(src)
		if neutralized == src {
			return nil, err
		}
		prog, err = parser.ParseFile(nil, "", neutralized, 0, parser.WithDisableSourceMaps)
		if err != nil {
			return nil, err
		}
		src = neutralized
	}

	c := &scopeCollector{}
	c.walk(prog)
	return scopesFromRaw(src, c.raw), nil
}

type scopeCollector struct {
	raw []rawScope
}

func off(idx file.Idx) uint32 {
	if idx <= 1 {
		return 0
	}
	return uint32(idx - 1)
}

func (c *scopeCollector) recordFunction(lit *ast.FunctionLiteral) {
	namePos := lit.Idx0()
	if lit.Name != nil {
		namePos = lit.Name.Idx
	}
	c.raw = append(c.raw, rawScope{start: off(lit.Idx0()), end: off(lit.Idx1()), namePos: off(namePos)})
}

func (c *scopeCollector) recordArrow(lit *ast.ArrowFunctionLiteral) {
	c.raw = append(c.raw, rawScope{start: off(lit.Idx0()), end: off(lit.Idx1()), namePos: off(lit.Idx0())})
}

func (c *scopeCollector) recordClass(lit *ast.ClassLiteral) {
	namePos := lit.Idx0()
	if lit.Name != nil {
		namePos = lit.Name.Idx
	}
	c.raw = append(c.raw, rawScope{start: off(lit.Idx0()), end: off(lit.Idx1()), namePos: off(namePos)})
}

func (c *scopeCollector) walk(n ast.Node) {
	if n == nil {
		return
	}
	switch t := n.(type) {
	case *ast.FunctionLiteral:
		c.recordFunction(t)
	case *ast.ArrowFunctionLiteral:
		c.recordArrow(t)
	case *ast.ClassLiteral:
		c.recordClass(t)
	}
	c.walkChildren(n)
}

func (c *scopeCollector) walkExprs(exprs []ast.Expression) {
	for _, e := range exprs {
		c.walkExpr(e)
	}
}

func (c *scopeCollector) walkStmts(stmts []ast.Statement) {
	for _, s := range stmts {
		c.walkStmt(s)
	}
}

func (c *scopeCollector) walkExpr(e ast.Expression) {
	if e != nil {
		c.walk(e)
	}
}

func (c *scopeCollector) walkStmt(s ast.Statement) {
	if s != nil {
		c.walk(s)
	}
}

func (c *scopeCollector) walkBindings(bindings []*ast.Binding) {
	for _, b := range bindings {
		if b != nil {
			c.walk(b)
		}
	}
}

func (c *scopeCollector) walkChildren(n ast.Node) {
	switch t := n.(type) {
	case *ast.Program:
		c.walkStmts(t.Body)

	case *ast.BlockStatement:
		c.walkStmts(t.List)
	case *ast.ExpressionStatement:
		c.walkExpr(t.Expression)
	case *ast.IfStatement:
		c.walkExpr(t.Test)
		c.walkStmt(t.Consequent)
		c.walkStmt(t.Alternate)
	case *ast.ForStatement:
		if t.Initializer != nil {
			c.walk(t.Initializer.(ast.Node))
		}
		c.walkExpr(t.Test)
		c.walkExpr(t.Update)
		c.walkStmt(t.Body)
	case *ast.ForInStatement:
		c.walkForInto(t.Into)
		c.walkExpr(t.Source)
		c.walkStmt(t.Body)
	case *ast.ForOfStatement:
		c.walkForInto(t.Into)
		c.walkExpr(t.Source)
		c.walkStmt(t.Body)
	case *ast.WhileStatement:
		c.walkExpr(t.Test)
		c.walkStmt(t.Body)
	case *ast.DoWhileStatement:
		c.walkStmt(t.Body)
		c.walkExpr(t.Test)
	case *ast.SwitchStatement:
		c.walkExpr(t.Discriminant)
		for _, cs := range t.Body {
			if cs != nil {
				c.walk(cs)
			}
		}
	case *ast.CaseStatement:
		c.walkExpr(t.Test)
		c.walkStmts(t.Consequent)
	case *ast.TryStatement:
		if t.Body != nil {
			c.walk(t.Body)
		}
		if t.Catch != nil {
			c.walk(t.Catch)
		}
		if t.Finally != nil {
			c.walk(t.Finally)
		}
	case *ast.CatchStatement:
		if t.Parameter != nil {
			c.walk(t.Parameter)
		}
		if t.Body != nil {
			c.walk(t.Body)
		}
	case *ast.ThrowStatement:
		c.walkExpr(t.Argument)
	case *ast.ReturnStatement:
		c.walkExpr(t.Argument)
	case *ast.LabelledStatement:
		c.walkStmt(t.Statement)
	case *ast.WithStatement:
		c.walkExpr(t.Object)
		c.walkStmt(t.Body)
	case *ast.VariableStatement:
		c.walkBindings(t.List)
	case *ast.LexicalDeclaration:
		c.walkBindings(t.List)
	case *ast.VariableDeclaration:
		c.walkBindings(t.List)
	case *ast.FunctionDeclaration:
		if t.Function != nil {
			c.walk(t.Function)
		}
	case *ast.ClassDeclaration:
		if t.Class != nil {
			c.walk(t.Class)
		}

	case *ast.ForLoopInitializerExpression:
		c.walkExpr(t.Expression)
	case *ast.ForLoopInitializerVarDeclList:
		c.walkBindings(t.List)
	case *ast.ForLoopInitializerLexicalDecl:
		c.walk(&t.LexicalDeclaration)
	case *ast.ForIntoVar:
		if t.Binding != nil {
			c.walk(t.Binding)
		}
	case *ast.ForIntoExpression:
		c.walkExpr(t.Expression)
	case *ast.ForDeclaration:
		if t.Target != nil {
			c.walk(t.Target)
		}

	case *ast.FunctionLiteral:
		if t.ParameterList != nil {
			c.walkBindings(t.ParameterList.List)
			c.walkExpr(t.ParameterList.Rest)
		}
		if t.Body != nil {
			c.walk(t.Body)
		}
	case *ast.ArrowFunctionLiteral:
		if t.ParameterList != nil {
			c.walkBindings(t.ParameterList.List)
			c.walkExpr(t.ParameterList.Rest)
		}
		if t.Body != nil {
			c.walk(t.Body)
		}
	case *ast.ExpressionBody:
		c.walkExpr(t.Expression)
	case *ast.ClassLiteral:
		c.walkExpr(t.SuperClass)
		for _, el := range t.Body {
			if el != nil {
				c.walk(el)
			}
		}
	case *ast.MethodDefinition:
		c.walkExpr(t.Key)
		if t.Body != nil {
			c.walk(t.Body)
		}
	case *ast.FieldDefinition:
		c.walkExpr(t.Key)
		c.walkExpr(t.Initializer)
	case *ast.ClassStaticBlock:
		if t.Block != nil {
			c.walk(t.Block)
		}
	case *ast.Binding:
		if t.Target != nil {
			c.walk(t.Target)
		}
		c.walkExpr(t.Initializer)
	case *ast.AssignExpression:
		c.walkExpr(t.Left)
		c.walkExpr(t.Right)
	case *ast.BinaryExpression:
		c.walkExpr(t.Left)
		c.walkExpr(t.Right)
	case *ast.ConditionalExpression:
		c.walkExpr(t.Test)
		c.walkExpr(t.Consequent)
		c.walkExpr(t.Alternate)
	case *ast.UnaryExpression:
		c.walkExpr(t.Operand)
	case *ast.CallExpression:
		c.walkExpr(t.Callee)
		c.walkExprs(t.ArgumentList)
	case *ast.NewExpression:
		c.walkExpr(t.Callee)
		c.walkExprs(t.ArgumentList)
	case *ast.DotExpression:
		c.walkExpr(t.Left)
	case *ast.PrivateDotExpression:
		c.walkExpr(t.Left)
	case *ast.BracketExpression:
		c.walkExpr(t.Left)
		c.walkExpr(t.Member)
	case *ast.SequenceExpression:
		c.walkExprs(t.Sequence)
	case *ast.ArrayLiteral:
		c.walkExprs(t.Value)
	case *ast.ArrayPattern:
		c.walkExprs(t.Elements)
		c.walkExpr(t.Rest)
	case *ast.ObjectLiteral:
		for _, p := range t.Value {
			if p != nil {
				c.walk(p)
			}
		}
	case *ast.ObjectPattern:
		for _, p := range t.Properties {
			if p != nil {
				c.walk(p)
			}
		}
		c.walkExpr(t.Rest)
	case *ast.PropertyKeyed:
		c.walkExpr(t.Key)
		c.walkExpr(t.Value)
	case *ast.PropertyShort:
		c.walkExpr(t.Initializer)
	case *ast.SpreadElement:
		c.walkExpr(t.Expression)
	case *ast.TemplateLiteral:
		c.walkExpr(t.Tag)
		c.walkExprs(t.Expressions)
	case *ast.YieldExpression:
		c.walkExpr(t.Argument)
	case *ast.AwaitExpression:
		c.walkExpr(t.Argument)
	case *ast.OptionalChain:
		c.walkExpr(t.Expression)
	case *ast.Optional:
		c.walkExpr(t.Expression)
	}
}

func (c *scopeCollector) walkForInto(into ast.ForInto) {
	if into != nil {
		c.walk(into)
	}
}

var moduleStmtPatterns = []*regexp.Regexp{
	regexp.MustCompile(`export\s*\{[^}]*\}\s*(?:from\s*("[^"]*"|'[^']*'))?\s*;?`),
	regexp.MustCompile(`export\s+default\s`),
	regexp.MustCompile(`export\s+(const|let|var|function|class|async)\b`),
	regexp.MustCompile(`import\s*(?:[\w$*{},\s]+?from\s*)?("[^"]*"|'[^']*')\s*;?`),
}

func neutralizeModuleSyntax(src string) string {
	buf := []byte(src)
	changed := false
	for _, re := range moduleStmtPatterns {
		for _, m := range re.FindAllIndex(buf, -1) {
			if !atStatementPosition(buf, m[0]) {
				continue
			}
			keep := 0
			if re == moduleStmtPatterns[2] {
				sub := re.FindSubmatchIndex(buf[m[0]:m[1]])
				if len(sub) >= 4 && sub[2] >= 0 {
					keep = m[1] - m[0] - sub[2]
				}
			}
			for i := m[0]; i < m[1]-keep; i++ {
				if buf[i] != '\n' && buf[i] != '\r' {
					buf[i] = ' '
				}
			}
			changed = true
		}
	}
	if !changed {
		return src
	}
	return string(buf)
}

func atStatementPosition(buf []byte, offset int) bool {
	i := offset - 1
	for i >= 0 && (buf[i] == ' ' || buf[i] == '\t') {
		i--
	}
	if i < 0 {
		return true
	}
	switch buf[i] {
	case ';', '}', '\n', '\r':
		return true
	}
	return false
}
