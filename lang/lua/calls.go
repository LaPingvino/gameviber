package lua

import (
	"reflect"
	"sort"
	"strings"

	lua "github.com/yuin/gopher-lua"
	"github.com/yuin/gopher-lua/ast"
	"github.com/yuin/gopher-lua/parse"
)

// Call is a call in a Lua file to a plain name the file doesn't declare and
// Lua doesn't define: a GameViber function or a script of the project, for
// `gameviber check` to look up (as gml.Calls for GML, js.Calls for JavaScript).
// gopher-lua keeps lines only, no columns.
type Call struct {
	Fn   string
	Line int
}

// Calls lists such calls in an event or script. Left out: names the file
// declares anywhere (local, function, parameters, for variables, and names it
// assigns — an assigned name becomes an instance variable, which may hold a
// function), Lua's own globals (string, math, tostring …), and calls on a
// value (other:instance_destroy(), math.floor()). Syntax errors: as Check.
func Calls(file, src string) ([]Call, error) {
	chunk, err := parse.Parse(strings.NewReader(src), file)
	if err != nil {
		return nil, err
	}
	declared := map[string]bool{}
	type call struct {
		name string
		line int
	}
	var calls []call
	walk(reflect.ValueOf(chunk), func(n any) {
		switch x := n.(type) {
		case *ast.LocalAssignStmt:
			for _, name := range x.Names {
				declared[name] = true
			}
		case *ast.AssignStmt:
			for _, l := range x.Lhs {
				if id, ok := l.(*ast.IdentExpr); ok {
					declared[id.Value] = true
				}
			}
		case *ast.FuncDefStmt:
			if x.Name != nil {
				if id, ok := x.Name.Func.(*ast.IdentExpr); ok {
					declared[id.Value] = true
				}
			}
		case *ast.ParList:
			for _, name := range x.Names {
				declared[name] = true
			}
		case *ast.NumberForStmt:
			declared[x.Name] = true
		case *ast.GenericForStmt:
			for _, name := range x.Names {
				declared[name] = true
			}
		case *ast.FuncCallExpr:
			if id, ok := x.Func.(*ast.IdentExpr); ok && x.Receiver == nil {
				calls = append(calls, call{id.Value, x.Line()})
			}
		}
	})
	L := lua.NewState()
	defer L.Close()
	var out []Call
	seen := map[call]bool{}
	for _, c := range calls {
		if declared[c.name] || L.GetGlobal(c.name) != lua.LNil || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, Call{Fn: c.name, Line: c.line})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Line < out[j].Line })
	return out, nil
}

// walk visits every node under v (gopher-lua's ast has no walker).
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
		visit(v.Interface())
		walk(v.Elem(), visit)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				walk(v.Field(i), visit)
			}
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			walk(v.Index(i), visit)
		}
	}
}
