// Package events 是 Nib 运行时的结构化事件流（M2 诊断层底座，设计见 docs/notes/design-v0.1.md 支柱二）。
//
// 事件流回答的问题是"应用刚才做了什么"，AI agent 的观察环节（t_observe）建立在此之上：
//   - 每个事件带全局递增 Seq 与时间戳，NDJSON 可直接管道处理
//   - Bus 带环形缓冲：晚到的订阅者可重放（harness 模式下业务进程重连后补齐窗口侧事件）
//   - 慢消费者的事件被丢弃而非阻塞发送方（观测数据不得反噬运行时）
package events

import (
	"sync"
	"time"
)

// Kind 是事件种类（点分字符串，值稳定；新增种类是兼容变更，改名是破坏变更）。
type Kind string

const (
	WindowCreated   Kind = "window.created"   // 窗口+WebView 创建完成 {title,width,height,devtools}
	WindowNavigated Kind = "window.navigated" // Navigate 调用 {target}
	WindowClosed    Kind = "window.closed"    // 窗口关闭 {}

	IPCCall     Kind = "ipc.call"     // 前端发起调用 {id,call,args_bytes}
	IPCResponse Kind = "ipc.response" // 调用成功 {id,result_bytes}
	IPCError    Kind = "ipc.error"    // 调用失败 {id,code,message}

	RuntimePanic Kind = "runtime.panic" // handler panic 被兜底 {call,stack}

	AppConnected    Kind = "harness.app_connected"    // 业务进程上线 {bindings}
	AppDisconnected Kind = "harness.app_disconnected" // 业务进程离线（热替换中？）{}
)

// Event 是事件流中的一条记录。
type Event struct {
	Seq  uint64         `json:"seq"`
	Time string         `json:"time"` // RFC3339Nano，UTC
	Kind Kind           `json:"kind"`
	Data map[string]any `json:"data,omitempty"`
}

const defaultBuffer = 256

// Bus 是进程内事件总线。
type Bus struct {
	mu   sync.Mutex
	seq  uint64
	buf  []Event // 环形缓冲（定长，覆盖写）
	pos  int     // 下一个写入位置
	full bool    // 缓冲是否已写满一圈

	nextSub uint64
	subs    map[uint64]chan Event
}

// NewBus 创建总线；buffer <= 0 时用默认值（256）。
func NewBus(buffer int) *Bus {
	if buffer <= 0 {
		buffer = defaultBuffer
	}
	return &Bus{buf: make([]Event, buffer), subs: map[uint64]chan Event{}}
}

// Emit 追加一条事件并分发给所有订阅者；订阅者 channel 满时丢弃（见包注释）。
func (b *Bus) Emit(kind Kind, data map[string]any) Event {
	b.mu.Lock()
	b.seq++
	ev := Event{Seq: b.seq, Time: time.Now().UTC().Format(time.RFC3339Nano), Kind: kind, Data: data}
	b.buf[b.pos] = ev
	b.pos++
	if b.pos == len(b.buf) {
		b.pos = 0
		b.full = true
	}
	// Keep the lock during nonblocking delivery so Unsubscribe cannot close a channel concurrently.
	for _, ch := range b.subs {
		select {
		case ch <- ev:
		default:
		}
	}
	b.mu.Unlock()
	return ev
}

// Inject 原样写入一条外部事件（保留其 Seq/Time），用于跨进程转发汇入。
// 注意：Seq 是各进程独立的编号空间，跨进程关联请以 Time 为准。
func (b *Bus) Inject(ev Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf[b.pos] = ev
	b.pos++
	if b.pos == len(b.buf) {
		b.pos = 0
		b.full = true
	}
}

// Recent 返回缓冲中的事件快照（按 Seq 升序）。
func (b *Bus) Recent() []Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := b.pos
	start := 0
	if b.full {
		n = len(b.buf)
		start = b.pos // 写满一圈后，最旧事件在下一个写入位置
	}
	out := make([]Event, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, b.buf[(start+i)%len(b.buf)])
	}
	return out
}

// Subscribe 订阅事件流；replay 为 true 时先收到缓冲中的历史事件。
// 订阅者 channel 容量 64，消费不及时会被丢弃，需要全量请用 Recent() 轮询。
func (b *Bus) Subscribe(replay bool) *Subscription {
	ch := make(chan Event, 64)
	b.mu.Lock()
	id := b.nextSub
	b.nextSub++
	b.subs[id] = ch
	b.mu.Unlock()

	s := &Subscription{C: ch, ch: ch, bus: b, id: id}
	if replay {
		for _, ev := range b.Recent() {
			select {
			case ch <- ev:
			default:
			}
		}
	}
	return s
}

// Subscription 是一个事件订阅。
type Subscription struct {
	C    <-chan Event
	ch   chan Event // C 的写端（Unsubscribe 时关闭）
	bus  *Bus
	id   uint64
	once sync.Once
}

// Unsubscribe 退订；之后 C 会被关闭。
func (s *Subscription) Unsubscribe() {
	s.once.Do(func() {
		s.bus.mu.Lock()
		delete(s.bus.subs, s.id)
		close(s.ch)
		s.bus.mu.Unlock()
	})
}
