// Package example 是绑定生成器的测试夹具：覆盖全部类型映射与三类绑定。
package example

import "time"

type GreetArgs struct {
	Name      string    `json:"name"`
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type GreetResult struct {
	Message string            `json:"message"`
	Meta    map[string]string `json:"meta"`
	Extra   *GreetArgs        `json:"extra"`
}

type Base struct {
	ID int `json:"id"`
}

type Status string

// GreetService 覆盖 call 类与结构体/切片/map/指针/别名/嵌入/时间映射。
type GreetService struct{}

// Greet 问候。
func (g *GreetService) Greet(args GreetArgs) (GreetResult, error) {
	return GreetResult{}, nil
}

func (g *GreetService) Sum(a int, b float64) (float64, error) { return 0, nil }

func (g *GreetService) Check(flag bool, raw any) (bool, error) { return false, nil }

func (g *GreetService) List(ids []int) ([]Status, error) { return nil, nil }

func (g *GreetService) Ping() (string, error) { return "", nil }

// StatusOf 返回嵌入结构体（展平）+ 匿名结构体。
func (g *GreetService) StatusOf(base Base) (struct {
	Code int `json:"code"`
	Base
}, error) {
	return struct {
		Code int `json:"code"`
		Base
	}{}, nil
}

// hidden 不导出，不参与生成。
func (g *GreetService) hidden() (string, error) { return "", nil }

// TaskService 覆盖 stream 注解。
type TaskService struct{}

// @binding stream
func (t *TaskService) RunPipeline(input string) (<-chan int, error) { return nil, nil }

// BusService 覆盖 event 注解。
type BusService struct{}

// @binding event
func (b *BusService) FileChanged() <-chan GreetArgs { return nil }
