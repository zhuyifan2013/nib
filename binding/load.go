package binding

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var bindingRe = regexp.MustCompile(`^@binding\s+(\w+)`)

// Load 解析 dir 下的 Go 包，提取全部绑定服务。
// 方法必须定义在包内（方法即绑定的来源，跨包方法不参与生成）。
func Load(dir string) (*Package, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	if len(pkgs) != 1 {
		names := make([]string, 0, len(pkgs))
		for n := range pkgs {
			names = append(names, n)
		}
		return nil, fmt.Errorf("%s 下应恰有一个包（排除 _test），实际：%s", dir, strings.Join(names, ", "))
	}
	var astPkg *ast.Package
	for _, p := range pkgs {
		astPkg = p
	}

	// 类型检查：类型映射依赖 go/types 的结果；importer.Default() 覆盖已编译的依赖包。
	var files []*ast.File
	for _, f := range astPkg.Files {
		files = append(files, f)
	}
	info := &types.Info{
		Defs: map[*ast.Ident]types.Object{},
	}
	tpkg, err := (&types.Config{Importer: importer.Default()}).Check(astPkg.Name, fset, files, info)
	if err != nil {
		return nil, fmt.Errorf("类型检查 %s 失败：%w", astPkg.Name, err)
	}

	ctx := newGenCtx(tpkg)
	svcMap := map[string]*Service{}
	var problems []string

	// 第一遍：收集 @binding 注解（按位置关联到方法）。
	kinds := map[token.Pos]Kind{}
	for _, f := range astPkg.Files {
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || fd.Doc == nil {
				continue
			}
			for _, cm := range fd.Doc.List {
				m := bindingRe.FindStringSubmatch(strings.TrimSpace(strings.TrimPrefix(cm.Text, "//")))
				if m == nil {
					continue
				}
				switch Kind(m[1]) {
				case KindStream, KindEvent:
					kinds[fd.Pos()] = Kind(m[1])
				default:
					problems = append(problems, fmt.Sprintf("%s：未知的 @binding 类型 %q（支持 stream|event）",
						fset.Position(fd.Pos()), m[1]))
				}
			}
		}
	}

	// 第二遍：提取导出 struct 的导出方法。
	for _, f := range astPkg.Files {
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || !fd.Name.IsExported() {
				continue
			}
			svcName := recvName(fd)
			if svcName == "" || !ast.IsExported(svcName) {
				continue
			}
			obj := info.Defs[fd.Name]
			sig, ok := obj.Type().(*types.Signature)
			if !ok {
				continue
			}
			m, err := buildMethod(fd, sig, kinds[fd.Pos()], ctx, fset)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s.%s：%v", svcName, fd.Name.Name, err))
				continue
			}
			svc := svcMap[svcName]
			if svc == nil {
				svc = &Service{Name: svcName}
				svcMap[svcName] = svc
			}
			svc.Methods = append(svc.Methods, *m)
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return nil, fmt.Errorf("绑定解析发现问题：\n  %s", strings.Join(problems, "\n  "))
	}

	names := make([]string, 0, len(svcMap))
	for n := range svcMap {
		names = append(names, n)
	}
	sort.Strings(names)
	pkg := &Package{}
	for _, n := range names {
		svc := svcMap[n]
		sort.SliceStable(svc.Methods, func(i, j int) bool { return svc.Methods[i].Name < svc.Methods[j].Name })
		if len(svc.Methods) > 0 {
			pkg.Services = append(pkg.Services, *svc)
		}
	}
	pkg.Structs = ctx.namedStructs()
	return pkg, nil
}

// recvName 取方法接收者的类型名（解指针）。
func recvName(fd *ast.FuncDecl) string {
	if len(fd.Recv.List) != 1 {
		return ""
	}
	t := fd.Recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	id, ok := t.(*ast.Ident)
	if !ok {
		return ""
	}
	return id.Name
}

// buildMethod 把一个 FuncDecl 的签名转成 Method；kind 缺省为 call。
func buildMethod(fd *ast.FuncDecl, sig *types.Signature, kind Kind, ctx *genCtx, fset *token.FileSet) (*Method, error) {
	if sig.Variadic() {
		return nil, fmt.Errorf("变长参数不支持")
	}
	if kind == "" {
		kind = KindCall
	}
	m := &Method{Name: fd.Name.Name, Kind: kind, Doc: docComment(fd)}

	switch kind {
	case KindCall:
		if sig.Results().Len() != 2 || !implementsError(sig.Results().At(1).Type()) {
			return nil, fmt.Errorf("call 方法须为 (T, error) 双返回值，实际 %d 个返回值", sig.Results().Len())
		}
		r, err := tsType(sig.Results().At(0).Type(), ctx)
		if err != nil {
			return nil, fmt.Errorf("返回值: %w", err)
		}
		m.Result = r
	case KindStream:
		if sig.Results().Len() != 2 || !implementsError(sig.Results().At(1).Type()) {
			return nil, fmt.Errorf("stream 方法须为 (<-chan T, error) 双返回值")
		}
		elem, err := chanElem(sig.Results().At(0).Type(), ctx)
		if err != nil {
			return nil, fmt.Errorf("返回值: %w", err)
		}
		m.Elem = elem
	case KindEvent:
		if sig.Results().Len() != 1 {
			return nil, fmt.Errorf("event 方法须为单一 <-chan T 返回值")
		}
		elem, err := chanElem(sig.Results().At(0).Type(), ctx)
		if err != nil {
			return nil, fmt.Errorf("返回值: %w", err)
		}
		m.Elem = elem
	}

	params := sig.Params()
	for i := 0; i < params.Len(); i++ {
		p := params.At(i)
		ts, err := tsType(p.Type(), ctx)
		if err != nil {
			return nil, fmt.Errorf("参数 %s: %w", p.Name(), err)
		}
		name := p.Name()
		if name == "" || name == "_" {
			name = fmt.Sprintf("p%d", i)
		}
		m.Params = append(m.Params, Param{Name: name, Type: ts})
	}
	return m, nil
}

// chanElem 解出 chan 的元素类型（stream/event 的返回值）。
func chanElem(t types.Type, ctx *genCtx) (string, error) {
	ch, ok := types.Unalias(t).(*types.Chan)
	if !ok {
		return "", fmt.Errorf("期望 chan 返回值，得到 %s", t.String())
	}
	return tsType(ch.Elem(), ctx)
}

func implementsError(t types.Type) bool {
	errObj := types.Universe.Lookup("error")
	return types.Implements(t, errObj.Type().Underlying().(*types.Interface))
}

// docComment 取方法注释（去掉 @binding 指令行与空行），作 JSDoc 用。
func docComment(fd *ast.FuncDecl) string {
	if fd.Doc == nil {
		return ""
	}
	var lines []string
	for _, cm := range fd.Doc.List {
		text := strings.TrimSpace(strings.TrimPrefix(cm.Text, "//"))
		if bindingRe.MatchString(text) || text == "" {
			continue
		}
		lines = append(lines, text)
	}
	return strings.Join(lines, "\n")
}

// cleanDir 供 CLI 与测试共用：确保输出目录存在。
func cleanDir(dir string) error {
	return os.MkdirAll(filepath.Join(dir, "services"), 0o755)
}
