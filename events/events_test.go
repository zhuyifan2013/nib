package events

import (
	"testing"
	"time"
)

func TestEmitSeqAndOrder(t *testing.T) {
	b := NewBus(0)
	for i := 0; i < 5; i++ {
		b.Emit(IPCCall, map[string]any{"n": i})
	}
	recent := b.Recent()
	if len(recent) != 5 {
		t.Fatalf("Recent() = %d, want 5", len(recent))
	}
	for i, ev := range recent {
		if ev.Seq != uint64(i+1) {
			t.Fatalf("Seq = %d, want %d", ev.Seq, i+1)
		}
		if ev.Kind != IPCCall || ev.Data["n"] != i {
			t.Fatalf("事件内容错位: %+v", ev)
		}
		if ev.Time == "" {
			t.Fatal("Time 为空")
		}
	}
}

func TestRingOverwrite(t *testing.T) {
	b := NewBus(3)
	for i := 0; i < 7; i++ {
		b.Emit(WindowNavigated, map[string]any{"i": i})
	}
	recent := b.Recent()
	if len(recent) != 3 {
		t.Fatalf("Recent() = %d, want 3（环形覆盖）", len(recent))
	}
	for i, ev := range recent {
		if ev.Data["i"] != 4+i {
			t.Fatalf("保留的事件错位: got %v want %d", ev.Data["i"], 4+i)
		}
	}
}

func TestSubscribeReplay(t *testing.T) {
	b := NewBus(0)
	b.Emit(IPCCall, map[string]any{"a": 1})
	sub := b.Subscribe(true)
	defer sub.Unsubscribe()
	select {
	case ev := <-sub.C:
		if ev.Data["a"] != 1 {
			t.Fatalf("重放事件错位: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("重放超时")
	}
	// 实时事件
	b.Emit(IPCResponse, map[string]any{"b": 2})
	select {
	case ev := <-sub.C:
		if ev.Data["b"] != 2 {
			t.Fatalf("实时事件错位: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("实时投递超时")
	}
}

func TestSubscribeNoReplay(t *testing.T) {
	b := NewBus(0)
	b.Emit(IPCCall, nil)
	sub := b.Subscribe(false)
	defer sub.Unsubscribe()
	select {
	case <-sub.C:
		t.Fatal("no-replay 订阅不应收到历史事件")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestDropSlowConsumer(t *testing.T) {
	b := NewBus(0)
	sub := b.Subscribe(false)
	defer sub.Unsubscribe()
	for i := 0; i < 200; i++ { // channel 容量 64，超出应被丢弃而非阻塞
		b.Emit(IPCCall, nil)
	}
}
