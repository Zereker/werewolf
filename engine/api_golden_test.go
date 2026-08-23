package engine

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

var updateAPIGolden = flag.Bool("update-api-golden", false, "重写 testdata/api.golden")

// TestAPI_SurfaceIsPinned 内核的导出面与 docs/API.md 的附录 A 绑在一起。
//
// docs/API.md 声称自己是「冻结的对象」。而在这个测试之前，**没有任何东西
// 钉住它**——加一个导出名、删一个、改个大小写，文档不会有任何反应，
// 两边慢慢就对不上了。那与这个项目其他「规矩只写在注释里」的伤口是同一类
// 问题：一条规矩没有测试守着，就只是一句话。
//
// 它不判断 API 好不好，只保证**变更不会悄悄发生**：改了导出面就必须同时
// 更新基准文件与 API.md，这一步是显式的。
//
// 钉住的是**名字加签名**。只钉名字的话，「把 CheckVictory 的返回值从一个
// Camp 改成一组」这种改动会溜过去——导出名一个都不增不减，而所有实现者
// 都会编译不过。
//
//	go test ./engine -run TestAPI_SurfaceIsPinned -update-api-golden
func TestAPI_SurfaceIsPinned(t *testing.T) {
	got := strings.Join(exportedNames(t), "\n") + "\n"

	path := filepath.Join("testdata", "api.golden")
	if *updateAPIGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("建 testdata: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("写基准文件: %v", err)
		}
		t.Logf("已更新 %s——记得同步 docs/API.md 附录 A", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读基准文件失败（首次生成跑 go test ./engine -run %s -update-api-golden）: %v",
			t.Name(), err)
	}
	if got == string(want) {
		return
	}

	added, removed := diffLines(string(want), got)
	t.Errorf("内核的导出面变了。\n新增：%v\n删除：%v\n\n"+
		"这不是错误，是提醒：导出面是 docs/API.md 声称冻结的东西。\n"+
		"确认这次变更是有意的，然后一起做两件事——\n"+
		"  1. go test ./engine -run %s -update-api-golden\n"+
		"  2. 更新 docs/API.md（正文与附录 A）",
		added, removed, t.Name())
}

// exportedNames 解析模块里每个公开包的非测试源码，收集全部导出名。
//
// 走 go/ast 而不是 shell 出去跑 go doc：测试要能在任何环境下跑，
// 也不该依赖工具链的输出格式。同一个办法 boundary_test.go 已经用过一次
// （它解析 event.go 取全部事件类型声明）。
func exportedNames(t *testing.T) []string {
	t.Helper()

	var names []string
	fset := token.NewFileSet()

	// 模块里每一个**公开**的包都算数，不只是 engine 自己。
	//
	// enginetest 是给规则包做随机对局用的公开子包（位置同
	// net/http/httptest）。它此前叫 internal/gamefuzz——`internal/` 出了
	// module 就 import 不了，而引擎要独立成库，所以它必须公开。
	// 公开了就该被冻结守着：不然它会成为一个绕开纪律的后门。
	for _, dir := range []string{".", "enginetest"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("读包目录 %s: %v", dir, err)
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
				t.Fatalf("解析 %s: %v", name, err)
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

// exportedIn 一个文件里的导出名**与签名**。方法记成 Receiver.Method。
//
// 记签名而不只记名字，是因为「名字没变、参数或返回值变了」同样是一次
// 破坏性变更——把 VictoryChecker.CheckVictory 的返回值从一个 Camp 改成
// 一组，导出名一个都不增不减。只钉名字的话那种改动会**悄悄溜过去**，
// 而这个测试的全部意义就是不让变更悄悄发生。
//
// 结构体字段不在这里记：字段的增删由快照 golden 与各自的测试守着。
// 接口的方法集要记——接口是契约，加一个方法就是让所有实现者编译不过。
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
				continue // 未导出类型的方法出不了包
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
					// 接口的方法集也钉住：加一个方法就是让所有实现者
					// 编译不过，那是最硬的一种破坏性变更。
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

// signatureOf 把一个函数类型打成一行，只保留类型、不保留参数名。
//
// 参数改名不是破坏性变更，参数**类型**改了才是。
func signatureOf(fn *ast.FuncType) string {
	return "(" + fieldTypes(fn.Params) + ")" + resultTypes(fn.Results)
}

// typeShapeOf 类型声明的形状。
//
// 只对「本身就是一个函数类型」的那些有意义（ResolverFunc、VictoryFunc
// 这类适配器）——它们的签名改了，导出名不会动。其余类型返回空串：
// 结构体字段与接口方法各有各的记法。
func typeShapeOf(expr ast.Expr) string {
	if fn, ok := expr.(*ast.FuncType); ok {
		return " func" + signatureOf(fn)
	}
	return ""
}

// fieldTypes 参数或返回值的类型列表。
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

// resultTypes 返回值，零个不写，一个不加括号。
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

// exprString 把一个类型表达式打成源码的样子。
//
// 手写而不是用 go/printer：这里要的是**稳定**的一行，不是好看的排版，
// 而且不想让 golden 因为格式化器的版本而变。
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

// receiverName 取接收者的类型名，剥掉指针。
func receiverName(expr ast.Expr) string {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// diffLines 两份清单的增删，供失败信息用。
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
