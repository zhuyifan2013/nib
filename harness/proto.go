// Package harness 实现 Nib 开发常驻外壳（dev harness）：
//
//	nib dev <pkg>          编排器：构建业务二进制、拉起 shell 进程、监听改动、热替换
//	nib shell --sock <p>   常驻层（内部）：窗口 + WebView + socket 转发，永不退出
//	业务二进制             可替换层：同一份源码，设置 NIB_SOCK 后经远程窗口连到 shell
//
// 架构见 docs/notes/design-v0.1.md §5.3，实现取舍见 docs/decisions/dev-harness-impl.md。
// 业务进程与 shell 之间复用 ipc.Message 信封，走 unix socket 换行分隔 JSON。
package harness

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	"nib.dev/nib/ipc"
)

// 控制调用：业务进程 → shell，Call 以 ctrlPrefix 开头的消息由 shell 直接处理，
// 不转发给前端，也不进入绑定分发。
const (
	ctrlPrefix      = "nib.ctrl."
	CtrlInit        = ctrlPrefix + "init"       // 参数：{navigate, bindings}
	CtrlNavigate    = ctrlPrefix + "navigate"   // 参数："url 或内嵌 HTML"
	CtrlSetTitle    = ctrlPrefix + "set_title"  // 参数："title"
	CtrlResize      = ctrlPrefix + "resize"     // 参数：{width, height}
	CtrlEval        = ctrlPrefix + "eval"       // 参数："js"
	CtrlClose       = ctrlPrefix + "close"      // 参数：null
	CtrlEvents      = ctrlPrefix + "events"     // shell → 业务进程：事件批量/流式转发，参数：[events.Event]
	errAppDown      = "harness/app_unavailable" // 业务进程不在线时的结构化错误码
	errBadTransport = "harness/transport"       // 传输层错误码
)

// IsCtrl 判断 Call 是否是控制调用。
func IsCtrl(call string) bool { return strings.HasPrefix(call, ctrlPrefix) }

// Send 写一条消息（换行分隔 JSON）。写操作无状态，可安全用于任意连接。
func Send(c net.Conn, m *ipc.Message) error {
	b, err := ipc.Marshal(m)
	if err != nil {
		return err
	}
	_, err = c.Write(append(b, '\n'))
	return err
}

// Session 是一条连接上的消息读写会话。
// json.Decoder 常驻并自带缓冲：连续多条消息同抵达时不会丢数据——
// 反例：每条消息新建 Decoder，会把已读进缓冲的后续消息随 Decoder 一起丢弃。
type Session struct {
	c   net.Conn
	dec *json.Decoder
	wmu sync.Mutex // Send 可并发（事件泵 + 转发），写需串行
}

// NewSession 创建会话。
func NewSession(c net.Conn) *Session {
	return &Session{c: c, dec: json.NewDecoder(c)}
}

// Conn 返回底层连接。
func (s *Session) Conn() net.Conn { return s.c }

// Send 写一条消息（并发安全）。
func (s *Session) Send(m *ipc.Message) error {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	return Send(s.c, m)
}

// Recv 读一条消息；连接关闭返回 io.EOF。
func (s *Session) Recv() (*ipc.Message, error) {
	var m ipc.Message
	if err := s.dec.Decode(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// serveConn 持续从会话读消息并交给 handler，直到连接关闭。
func serveConn(s *Session, handler func(*ipc.Message)) error {
	for {
		m, err := s.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("读消息: %w", err)
		}
		handler(m)
	}
}

// initPayload 是 CtrlInit 的参数。
type initPayload struct {
	Navigate string   `json:"navigate"` // 起始 URL 或 "<" 开头的内嵌 HTML
	Bindings []string `json:"bindings"` // 业务进程注册的绑定名
}

// marshalArgs / unmarshalArgs 是 ipc.Message.Args 的小助手。
func marshalArgs(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}

func unmarshalArgs(raw json.RawMessage, v any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, v)
}
