// Package ipc 定义 Nib 前端(WebView)与后端(Go)之间的通信协议。
//
// 设计目标（见 doc/nib-framework-design-v0.1.md §5.4/§5.5）：
//   - 单一错误格式贯穿 JS ↔ Go 边界，携带完整因果链
//   - 消息信封统一覆盖请求-响应(call)、双向流(stream)、广播(event)
package ipc

import "encoding/json"

// Layer 标识错误因果链中的一层。
type Layer string

const (
	LayerJS  Layer = "js"  // 前端调用栈
	LayerIPC Layer = "ipc" // IPC 记录（调用名、参数摘要）
	LayerGo  Layer = "go"  // Go 端栈帧
)

// Frame 是因果链中的一帧。
type Frame struct {
	Layer Layer          `json:"layer"`
	Frame string         `json:"frame,omitempty"` // 代码位置，如 fs_service.go:88 os.Open
	Call  string         `json:"call,omitempty"`  // IPC 调用名，如 fs.read
	Extra map[string]any `json:"extra,omitempty"`
}

// Error 是贯穿边界的统一错误格式。
type Error struct {
	Code      string  `json:"code"`              // 如 fs.read/permission_denied
	Message   string  `json:"message"`
	Chain     []Frame `json:"chain,omitempty"`   // JS → IPC → Go 因果链
	Retryable bool    `json:"retryable"`
	Doc       string  `json:"doc,omitempty"`     // 文档链接
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// NewError 构造一个带 Go 层栈帧的错误。
func NewError(code, message string) *Error {
	return &Error{Code: code, Message: message, Retryable: false}
}

// Direction 标识消息方向/类型。
type Direction string

const (
	DirRequest  Direction = "request"  // 前端 → 后端：调用
	DirResponse Direction = "response" // 后端 → 前端：响应
	DirEvent    Direction = "event"    // 后端 → 前端：广播
	DirStream   Direction = "stream"   // 双向流帧
)

// SeqKind 标识流式序列帧的种类。
type SeqKind string

const (
	SeqNext SeqKind = "next"  // 流中的一条数据
	SeqDone SeqKind = "done"  // 流正常结束
	SeqErr  SeqKind = "error" // 流异常终止（Error 字段给出原因）
)

// Message 是 IPC 消息的顶层信封。
type Message struct {
	ID  string    `json:"id"`
	Dir Direction `json:"dir"`

	// call：服务名.方法名，如 "GreetService.Greet"
	Call string          `json:"call,omitempty"`
	Args json.RawMessage `json:"args,omitempty"`

	// 响应内容（三选一）
	Result json.RawMessage `json:"result,omitempty"`
	Error  *Error          `json:"error,omitempty"`

	// 流式帧
	Stream SeqKind `json:"stream,omitempty"`
	Done   bool    `json:"done,omitempty"`
}

// Marshal / Unmarshal 辅助。
func Marshal(m *Message) ([]byte, error)  { return json.Marshal(m) }
func Unmarshal(data []byte) (*Message, error) {
	var m Message
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Handler 是绑定方法的签名：接收原始参数，返回可序列化结果或 *Error。
// 定义在 ipc 包以避免 core ↔ webview 的循环依赖。
type Handler func(args json.RawMessage) (any, *Error)
