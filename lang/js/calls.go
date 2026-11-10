package js

import (
	"reflect"
	"sort"

	"github.com/dop251/goja"
	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/parser"
)

// Call is a call in a JavaScript file to a plain name the file doesn't
// declare and JavaScript doesn't define: a GameViber function or a script of
// the project, for `gameviber check` to look up (as gml.Calls for GML).
type Call struct {
	Fn        string
	Line, Col int // 1-based, in the file as written
}

// Calls lists such calls in an event or script (wrapped as Check and the
// runner wrap it, positions mapped back to the file). Calls through a name
// the file declares anywhere (a function, var / let / const, a parameter) or
// JavaScript's own globals (Math, parseInt …) are left out; calls on objects
// (other.instance_destroy(), Math.floor()) too. Syntax errors: as Check.
func Calls(file, src string) ([]Call, error) {
	prefix := "(function(" + scriptParams + "){"
	prog, err := parser.ParseFile(nil, file, prefix+src+"\n})", 0)
	if err != nil {
		return nil, err
	}
	declared := map[string]bool{}
	var calls []*ast.CallExpression
	seen := map[*ast.CallExpression]bool{}
	walk(reflect.ValueOf(prog), func(n any) {
		switch x := n.(type) {
		case *ast.Binding:
			if id, ok := x.Target.(*ast.Identifier); ok {
				declared[id.Name.String()] = true
			}
		case *ast.FunctionLiteral:
			if x.Name != nil {
				declared[x.Name.Name.String()] = true
			}
		case *ast.CallExpression:
			if !seen[x] { // the tree reaches some nodes twice
				seen[x] = true
				calls = append(calls, x)
			}
		}
	})
	globals := goja.New().GlobalObject()
	var out []Call
	for _, c := range calls {
		id, ok := c.Callee.(*ast.Identifier)
		if !ok {
			continue
		}
		name := id.Name.String()
		if declared[name] || globals.Get(name) != nil {
			continue
		}
		p := prog.File.Position(int(id.Idx) - prog.File.Base())
		col := p.Column
		if p.Line == 1 {
			col -= len(prefix)
		}
		out = append(out, Call{Fn: name, Line: p.Line, Col: col})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Line < out[j].Line || out[i].Line == out[j].Line && out[i].Col < out[j].Col
	})
	return out, nil
}

var nodeType = reflect.TypeOf((*ast.Node)(nil)).Elem()

// walk visits every AST node under v (goja's ast has no walker): pointers to
// structs, the interfaces they sit in, slices of them.
func walk(v reflect.Value, visit func(any)) {
	switch v.Kind() {
	case reflect.Interface:
		if !v.IsNil() {
			walk(v.Elem(), visit)
		}
	case reflect.Pointer:
		if v.IsNil() || v.Elem().Kind() != reflect.Struct {
			return
		}
		if v.Type().Implements(nodeType) || v.Type().Elem().Name() == "Binding" {
			visit(v.Interface())
		}
		walk(v.Elem(), visit)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if f := v.Type().Field(i); f.IsExported() {
				walk(v.Field(i), visit)
			}
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			walk(v.Index(i), visit)
		}
	}
}
