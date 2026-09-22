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
	errAppDown      = "harness/app_unavailable" // 业务进程不在线时的结构化错误码
	errBadTransport = "harness/transport"       // 传输层错误码
)

// IsCtrl 判断 Call 是否是控制调用。
func IsCtrl(call string) bool { return strings.HasPrefix(call, ctrlPrefix) }

// Send 写一条消息（换行分隔 JSON）。
func Send(c net.Conn, m *ipc.Message) error {
	b, err := ipc.Marshal(m)
	if err != nil {
		return err
	}
	_, err = c.Write(append(b, '\n'))
	return err
}

// Recv 读一条消息；连接关闭返回 io.EOF。
func Recv(c net.Conn) (*ipc.Message, error) {
	dec := json.NewDecoder(c)
	var m ipc.Message
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// serveConn 持续从 conn 读消息并交给 handler，直到连接关闭。
func serveConn(c net.Conn, handler func(*ipc.Message)) error {
	for {
		m, err := Recv(c)
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
