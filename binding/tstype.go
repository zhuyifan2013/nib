package binding

import (
	"fmt"
	"go/types"
	"reflect"
	"sort"
	"strings"
)

// genCtx 携带一次生成过程中的命名类型注册表（去重，按名排序输出）。
// 结构体生成 TS interface；具名基础类型/切片/map 生成 type alias（保名不换型）。
type genCtx struct {
	pkg     *types.Package
	structs map[string]*namedDecl // name -> 待生成的命名类型
}

type namedDecl struct {
	obj      *types.TypeName
	isStruct bool
}

func newGenCtx(pkg *types.Package) *genCtx {
	return &genCtx{pkg: pkg, structs: map[string]*namedDecl{}}
}

func (c *genCtx) namedStructs() []NamedStruct {
	names := make([]string, 0, len(c.structs))
	for n := range c.structs {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]NamedStruct, 0, len(names))
	for _, n := range names {
		d := c.structs[n]
		if d.isStruct {
			body, err := structBody(d.obj.Type().Underlying().(*types.Struct), c)
			if err != nil {
				continue // 注册时已验证，不会失败
			}
			out = append(out, NamedStruct{Name: n, Body: body, Struct: true})
		} else {
			body, err := tsType(d.obj.Type().Underlying(), c)
			if err != nil {
				continue
			}
			out = append(out, NamedStruct{Name: n, Body: body})
		}
	}
	return out
}

// tsType 把 Go 类型映射为 TS 类型。
func tsType(t types.Type, c *genCtx) (string, error) {
	switch v := types.Unalias(t).(type) {
	case *types.Pointer:
		inner, err := tsType(v.Elem(), c)
		if err != nil {
			return "", err
		}
		return inner + " | null", nil
	case *types.Basic:
		return basicTS(v)
	case *types.Slice:
		elem, err := tsType(v.Elem(), c)
		if err != nil {
			return "", err
		}
		return elem + "[]", nil
	case *types.Array:
		elem, err := tsType(v.Elem(), c)
		if err != nil {
			return "", err
		}
		return elem + "[]", nil
	case *types.Map:
		key, ok := v.Key().Underlying().(*types.Basic)
		if !ok || key.Kind() != types.String {
			return "", fmt.Errorf("仅支持 string 键的 map，得到 %s", v.Key())
		}
		elem, err := tsType(v.Elem(), c)
		if err != nil {
			return "", err
		}
		return "Record<string, " + elem + ">", nil
	case *types.Struct:
		body, err := structBody(v, c)
		if err != nil {
			return "", err
		}
		return "{ " + body + " }", nil
	case *types.Interface:
		if v.Empty() {
			return "unknown", nil
		}
		return "", fmt.Errorf("非空 interface 不支持：%s", v.String())
	case *types.Named:
		return namedTS(v, c)
	default:
		return "", fmt.Errorf("不支持的类型：%s", t.String())
	}
}

func basicTS(b *types.Basic) (string, error) {
	switch b.Kind() {
	case types.Bool, types.UntypedBool:
		return "boolean", nil
	case types.String, types.UntypedString:
		return "string", nil
	case types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
		types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64,
		types.Float32, types.Float64, types.UntypedInt, types.UntypedFloat:
		return "number", nil
	default:
		return "", fmt.Errorf("不支持的基本类型：%s", b.Name())
	}
}

func namedTS(n *types.Named, c *genCtx) (string, error) {
	obj := n.Obj()
	// 跨包特例：time.Time / json.RawMessage。
	if obj.Pkg() != nil && obj.Pkg() != c.pkg {
		if obj.Pkg().Path() == "time" && obj.Name() == "Time" {
			return "string", nil
		}
		if obj.Pkg().Path() == "encoding/json" && obj.Name() == "RawMessage" {
			return "unknown", nil
		}
	}
	// 本包命名类型 → 生成具名声明（interface 或 type alias），保持类型身份。
	if obj.Pkg() == c.pkg {
		_, isStruct := n.Underlying().(*types.Struct)
		if existing, ok := c.structs[obj.Name()]; ok {
			_ = existing
			return obj.Name(), nil
		}
		c.structs[obj.Name()] = &namedDecl{obj: obj, isStruct: isStruct}
		return obj.Name(), nil
	}
	return "", fmt.Errorf("外部包命名类型暂不支持：%s（请用基础类型、time.Time 或本包类型）", n.String())
}

// structBody 生成 TS interface 体（两空格缩进字段，分号结尾）。
// 内联（匿名嵌入）的导出结构体字段会被展平；omitempty 字段标记可选。
func structBody(s *types.Struct, c *genCtx) (string, error) {
	var fields []string
	for i := 0; i < s.NumFields(); i++ {
		f := s.Field(i)
		if !f.Exported() {
			continue
		}
		tag := reflect.StructTag(s.Tag(i)).Get("json")
		name, opts := parseJSONTag(tag)
		if name == "-" {
			continue
		}
		if f.Embedded() {
			// 展平匿名嵌入的结构体（指针解引用）。
			t := f.Type()
			if p, ok := t.(*types.Pointer); ok {
				t = p.Elem()
			}
			named, ok := t.(*types.Named)
			if !ok {
				return "", fmt.Errorf("嵌入字段 %s 不是具名结构体", f.Name())
			}
			inner, ok := named.Underlying().(*types.Struct)
			if !ok {
				return "", fmt.Errorf("嵌入字段 %s 不是结构体", f.Name())
			}
			flat, err := structBody(inner, c)
			if err != nil {
				return "", err
			}
			if flat != "" {
				fields = append(fields, flat)
			}
			continue
		}
		if name == "" {
			name = f.Name()
		}
		ts, err := tsType(f.Type(), c)
		if err != nil {
			return "", fmt.Errorf("字段 %s: %w", f.Name(), err)
		}
		if opts["omitempty"] {
			name += "?"
		}
		fields = append(fields, name+": "+ts)
	}
	return strings.Join(fields, "; "), nil
}

func parseJSONTag(tag string) (string, map[string]bool) {
	opts := map[string]bool{}
	if tag == "" {
		return "", opts
	}
	parts := strings.Split(tag, ",")
	for _, o := range parts[1:] {
		opts[o] = true
	}
	return parts[0], opts
}
