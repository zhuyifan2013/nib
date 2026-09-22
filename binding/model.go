// Package binding 实现 Nib 绑定生成器：解析 Go 服务方法，生成类型安全的 TS 客户端。
//
// 设计见 docs/notes/design-v0.1.md §5.2 与 docs/decisions/binding-generator.md：
//   - 导出的 struct 方法的导出方法即绑定；方法注释含 @binding stream|event 选择类型，
//     缺省为 call（请求-响应）
//   - Go 结构（go/parser + go/types，零新增依赖）→ TS 接口 + IPC 存根
//   - 每个服务一个文件，内容未变不写盘（增量生成）
package binding

// Kind 是绑定方法的 IPC 类型。
type Kind string

const (
	KindCall   Kind = "call"   // 请求-响应
	KindStream Kind = "stream" // 双向流（M4 运行时落地，客户端先生成）
	KindEvent  Kind = "event"  // 广播（M4 运行时落地，客户端先生成）
)

// Param 是一个方法参数。
type Param struct {
	Name string
	Type string // TS 类型
}

// Method 是一个绑定方法。
type Method struct {
	Name   string
	Kind   Kind
	Doc    string // 去掉 @binding 行后的注释（作 JSDoc）
	Params []Param
	Result string // call：TS 返回类型
	Elem   string // stream/event：chan 元素 TS 类型
}

// Service 是一个导出 struct 及其绑定方法。
type Service struct {
	Name    string
	Methods []Method
}

// Package 是一个解析后的 Go 包：全部服务 + 引用到的命名结构体。
type Package struct {
	Services []Service
	Structs  []NamedStruct // 去重后的命名结构体（types.ts）
}

// NamedStruct 是一个需要生成 TS 声明的 Go 命名类型：
// Struct 为真时 Body 是 interface 体，否则 Body 是 type alias 右侧表达式。
type NamedStruct struct {
	Name   string
	Body   string
	Struct bool
}
