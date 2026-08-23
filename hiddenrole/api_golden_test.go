package hiddenrole

import (
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var updateAPIGolden = flag.Bool("update-api-golden", false, "rewrite testdata/api.golden")

// TestAPI_SurfaceIsPinned ties the kernel's exported surface to Appendix A of
// API.md.
//
// API.md declares itself frozen. Before this test **nothing pinned it** --
// add an exported name, remove one, change its case, and the document would
// not react, until the two slowly stopped agreeing. That is the same wound as
// every other "rule that lives only in a comment" in this project: a rule
// with no test guarding it is only a sentence.
//
// It does not judge whether the API is good, only that **a change cannot
// happen quietly**: changing the exported surface means updating the golden
// file and API.md at the same time, and that step is explicit.
//
// What is pinned is **names plus signatures**. Pinning names alone would let
// a change like "CheckVictory returns a set of Camps instead of one" slip
// through -- not one exported name added or removed, and every implementer
// fails to compile.
//
//	go test ./engine -run TestAPI_SurfaceIsPinned -update-api-golden
func TestAPI_SurfaceIsPinned(t *testing.T) {
	got := strings.Join(exportedNames(t), "\n") + "\n"

	path := filepath.Join("testdata", "api.golden")
	if *updateAPIGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("creating testdata: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("writing the golden file: %v", err)
		}
		t.Logf("updated %s -- remember to sync Appendix A of API.md", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read the golden file (to generate it the first time run "+
			"go test . -run %s -update-api-golden): %v",
			t.Name(), err)
	}
	if got == string(want) {
		return
	}

	added, removed := diffLines(string(want), got)
	t.Errorf("the kernel's exported surface changed.\nadded:   %v\nremoved: %v\n\n"+
		"This is not an error, it is a reminder: the exported surface is what "+
		"API.md declares frozen.\n"+
		"Confirm the change is intended, then do two things together --\n"+
		"  1. go test ./engine -run %s -update-api-golden\n"+
		"  2. update API.md (the body and Appendix A)",
		added, removed, t.Name())
}

// exportedNames parses the non-test source of every public package in the
// module and collects all exported names.
//
// It uses go/ast rather than shelling out to go doc: the test has to run in
// any environment, and should not depend on a toolchain's output format.
// boundary_test.go already uses the same approach (parsing event.go for every
// event type declaration).
func exportedNames(t *testing.T) []string {
	t.Helper()

	var names []string
	fset := token.NewFileSet()

	// Every **public** package in the module counts, not just the root one.
	//
	// enginetest is the public sub-package rules packages use for random
	// games (the same position as net/http/httptest). It used to be called
	// internal/gamefuzz -- `internal/` cannot be imported from outside the
	// module, and the engine had to become its own library, so it had to go
	// public. Being public, it is guarded by the freeze: otherwise it would
	// become a back door around that discipline.
	for _, dir := range []string{".", "enginetest"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("reading package directory %s: %v", dir, err)
		}
		prefix := ""
		if dir != "." {
			prefix = dir + "."
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
			if err != nil {
				t.Fatalf("parsing %s: %v", name, err)
			}
			for _, n := range exportedIn(f) {
				kind, rest, _ := strings.Cut(n, " ")
				names = append(names, kind+" "+prefix+rest)
			}
		}
	}

	sort.Strings(names)
	return names
}

// exportedIn returns one file's exported names **and signatures**. A method
// is recorded as Receiver.Method.
//
// Signatures are recorded, not just names, because "the name is unchanged and
// the parameters or results are not" is just as breaking a change -- turning
// VictoryChecker.CheckVictory's result from one Camp into a set adds and
// removes no exported name at all. Pinning names alone would let that
// **slip through quietly**, and not letting changes happen quietly is this
// test's entire point.
//
// Struct fields are not recorded here: field changes are guarded by the
// snapshot golden and by their own tests. Interface method sets are recorded
// -- an interface is a contract, and adding a method makes every implementer
// fail to compile.
func exportedIn(f *ast.File) []string {
	var out []string
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if !d.Name.IsExported() {
				continue
			}
			if d.Recv == nil || len(d.Recv.List) == 0 {
				out = append(out, "func "+d.Name.Name+signatureOf(d.Type))
				continue
			}
			recv := receiverName(d.Recv.List[0].Type)
			if !ast.IsExported(recv) {
				continue // a method on an unexported type cannot leave the package
			}
			out = append(out, "method "+recv+"."+d.Name.Name+signatureOf(d.Type))

		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if !s.Name.IsExported() {
						continue
					}
					out = append(out, "type "+s.Name.Name+typeShapeOf(s.Type))
					// Interface method sets are pinned too: adding a method
					// makes every implementer fail to compile, which is the
					// hardest kind of breaking change there is.
					if iface, ok := s.Type.(*ast.InterfaceType); ok {
						for _, m := range iface.Methods.List {
							fn, ok := m.Type.(*ast.FuncType)
							if !ok || len(m.Names) == 0 {
								continue
							}
							out = append(out,
								"iface "+s.Name.Name+"."+m.Names[0].Name+signatureOf(fn))
						}
					}
				case *ast.ValueSpec:
					kind := "const "
					if d.Tok == token.VAR {
						kind = "var "
					}
					for _, n := range s.Names {
						if n.IsExported() {
							out = append(out, kind+n.Name)
						}
					}
				}
			}
		}
	}
	return out
}

// signatureOf renders a function type as one line, keeping the types and
// dropping the parameter names.
//
// Renaming a parameter is not a breaking change; changing its **type** is.
func signatureOf(fn *ast.FuncType) string {
	return "(" + fieldTypes(fn.Params) + ")" + resultTypes(fn.Results)
}

// typeShapeOf gives the shape of a type declaration.
//
// It only means anything for types that are themselves function types
// (ResolverFunc, VictoryFunc and the other adapters) -- change their
// signature and no exported name moves. Everything else returns the empty
// string: struct fields and interface methods are each recorded their own
// way.
func typeShapeOf(expr ast.Expr) string {
	if fn, ok := expr.(*ast.FuncType); ok {
		return " func" + signatureOf(fn)
	}
	return ""
}

// fieldTypes lists the types of parameters or results.
func fieldTypes(list *ast.FieldList) string {
	if list == nil || len(list.List) == 0 {
		return ""
	}
	var out []string
	for _, f := range list.List {
		t := exprString(f.Type)
		n := len(f.Names)
		if n == 0 {
			n = 1
		}
		for i := 0; i < n; i++ {
			out = append(out, t)
		}
	}
	return strings.Join(out, ", ")
}

// resultTypes renders results: none writes nothing, one takes no parentheses.
func resultTypes(list *ast.FieldList) string {
	s := fieldTypes(list)
	switch {
	case s == "":
		return ""
	case list != nil && len(list.List) == 1 && len(list.List[0].Names) <= 1:
		return " " + s
	default:
		return " (" + s + ")"
	}
}

// exprString renders a type expression the way it looks in source.
//
// Written by hand rather than using go/printer: what is wanted here is a
// **stable** single line, not pretty formatting, and the golden should not
// change with the formatter's version.
func exprString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + exprString(t.X)
	case *ast.SelectorExpr:
		return exprString(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + exprString(t.Elt)
		}
		return "[" + exprString(t.Len) + "]" + exprString(t.Elt)
	case *ast.Ellipsis:
		return "..." + exprString(t.Elt)
	case *ast.MapType:
		return "map[" + exprString(t.Key) + "]" + exprString(t.Value)
	case *ast.FuncType:
		return "func" + signatureOf(t)
	case *ast.InterfaceType:
		if t.Methods == nil || len(t.Methods.List) == 0 {
			return "interface{}"
		}
		return "interface{...}"
	case *ast.StructType:
		return "struct{...}"
	case *ast.ChanType:
		return "chan " + exprString(t.Value)
	case *ast.BasicLit:
		return t.Value
	default:
		return "?"
	}
}

// receiverName gives the receiver's type name, with the pointer stripped.
func receiverName(expr ast.Expr) string {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// diffLines gives what was added and removed between two listings, for the
// failure message.
func diffLines(want, got string) (added, removed []string) {
	inWant := map[string]bool{}
	for _, l := range strings.Split(strings.TrimSpace(want), "\n") {
		inWant[l] = true
	}
	inGot := map[string]bool{}
	for _, l := range strings.Split(strings.TrimSpace(got), "\n") {
		inGot[l] = true
		if !inWant[l] {
			added = append(added, l)
		}
	}
	for l := range inWant {
		if !inGot[l] {
			removed = append(removed, l)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}
