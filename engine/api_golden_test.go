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

// exportedNames 解析本包的非测试源码，收集全部导出名。
//
// 走 go/ast 而不是 shell 出去跑 go doc：测试要能在任何环境下跑，
// 也不该依赖工具链的输出格式。同一个办法 boundary_test.go 已经用过一次
// （它解析 event.go 取全部事件类型声明）。
func exportedNames(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("读包目录: %v", err)
	}

	var names []string
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("解析 %s: %v", name, err)
		}
		names = append(names, exportedIn(f)...)
	}

	sort.Strings(names)
	return names
}

// exportedIn 一个文件里的导出名。方法记成 Receiver.Method，
// 结构体字段不记——字段的增删由快照 golden 与各自的测试守着，
// 这里盯的是「有哪些名字能被外面叫到」。
func exportedIn(f *ast.File) []string {
	var out []string
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if !d.Name.IsExported() {
				continue
			}
			if d.Recv == nil || len(d.Recv.List) == 0 {
				out = append(out, "func "+d.Name.Name)
				continue
			}
			recv := receiverName(d.Recv.List[0].Type)
			if !ast.IsExported(recv) {
				continue // 未导出类型的方法出不了包
			}
			out = append(out, "method "+recv+"."+d.Name.Name)

		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if s.Name.IsExported() {
						out = append(out, "type "+s.Name.Name)
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
