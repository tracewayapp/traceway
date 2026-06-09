package symbolicator

import (
	"regexp"
	"slices"
	"unicode/utf16"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/file"
	"github.com/dop251/goja/parser"
)

func parseFunctionScopes(bundle []byte) (*functionScopes, error) {
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

	offsets := make([]uint32, 0, len(c.raw)*3)
	for _, s := range c.raw {
		offsets = append(offsets, s.start, s.end, s.namePos)
	}
	pos := convertOffsets(src, offsets)

	scopes := make([]genScope, len(c.raw))
	for i, s := range c.raw {
		start, end, name := pos[s.start], pos[s.end], pos[s.namePos]
		scopes[i] = genScope{
			startLine: start.line, startCol: start.col,
			endLine: end.line, endCol: end.col,
			nameLine: name.line, nameCol: name.col,
		}
	}
	return &functionScopes{transitions: buildTransitions(scopes)}, nil
}

type genScope struct {
	startLine, startCol uint32
	endLine, endCol     uint32
	nameLine, nameCol   uint32
}

type functionScopes struct {
	transitions []scopeTransition // sorted by generated position
}

// scopeTransition marks that, from this generated position onward, the
// innermost enclosing function's name token is at (nameLine, nameCol). has is
// false when no function encloses the range (global scope).
type scopeTransition struct {
	line, col         uint32
	nameLine, nameCol uint32
	has               bool
}

type scopeEvent struct {
	line, col uint32
	start     bool
	scope     int
}

// buildTransitions flattens the well-nested function scopes into a sorted list
// of transitions in a single sweep, so the resolver can find the enclosing
// function of any token with a linear merge instead of scanning every scope per
// token.
func buildTransitions(scopes []genScope) []scopeTransition {
	events := make([]scopeEvent, 0, len(scopes)*2)
	for i := range scopes {
		s := scopes[i]
		if !less(s.startLine, s.startCol, s.endLine, s.endCol) {
			continue
		}
		events = append(events,
			scopeEvent{line: s.startLine, col: s.startCol, start: true, scope: i},
			scopeEvent{line: s.endLine, col: s.endCol, start: false, scope: i},
		)
	}
	slices.SortFunc(events, func(a, b scopeEvent) int {
		if a.line != b.line {
			return int(a.line) - int(b.line)
		}
		if a.col != b.col {
			return int(a.col) - int(b.col)
		}
		if a.start == b.start {
			return 0
		}
		if !a.start { // a scope ending here closes before another opens
			return -1
		}
		return 1
	})

	var transitions []scopeTransition
	stack := make([]int, 0, 16)
	var lastNameLine, lastNameCol uint32
	lastHas, haveLast := false, false

	for i := 0; i < len(events); {
		line, col := events[i].line, events[i].col
		for i < len(events) && events[i].line == line && events[i].col == col {
			if events[i].start {
				stack = append(stack, events[i].scope)
			} else {
				stack = removeFromStack(stack, events[i].scope)
			}
			i++
		}

		var nameLine, nameCol uint32
		has := false
		if len(stack) > 0 {
			top := scopes[stack[len(stack)-1]]
			nameLine, nameCol, has = top.nameLine, top.nameCol, true
		}
		if !haveLast || has != lastHas || nameLine != lastNameLine || nameCol != lastNameCol {
			transitions = append(transitions, scopeTransition{line: line, col: col, nameLine: nameLine, nameCol: nameCol, has: has})
			lastNameLine, lastNameCol, lastHas, haveLast = nameLine, nameCol, has, true
		}
	}
	return transitions
}

func removeFromStack(stack []int, scope int) []int {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i] == scope {
			return append(stack[:i], stack[i+1:]...)
		}
	}
	return stack
}

func less(aLine, aCol, bLine, bCol uint32) bool {
	return aLine < bLine || (aLine == bLine && aCol < bCol)
}

type genPos struct {
	line, col uint32
}

func convertOffsets(src string, offsets []uint32) map[uint32]genPos {
	sorted := append([]uint32(nil), offsets...)
	slices.Sort(sorted)

	out := make(map[uint32]genPos, len(sorted))
	var line, col uint32
	oi := 0
	for i, r := range src {
		for oi < len(sorted) && sorted[oi] == uint32(i) {
			out[sorted[oi]] = genPos{line, col}
			oi++
		}
		if oi >= len(sorted) {
			return out
		}
		if r == '\n' {
			line++
			col = 0
		} else {
			col += uint32(utf16.RuneLen(r))
		}
	}
	for oi < len(sorted) {
		out[sorted[oi]] = genPos{line, col}
		oi++
	}
	return out
}

type rawScope struct {
	start, end, namePos uint32
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
