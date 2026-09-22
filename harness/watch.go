package harness

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GoFileSignature 返回 dir 下（递归）全部 .go 文件的 路径 -> "size:mtime" 签名图。
// 用于轮询式变更检测：零依赖，桌面项目文件数少，250ms 轮询开销可忽略。
func GoFileSignature(dir string) (map[string]string, error) {
	sig := map[string]string{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && path != dir {
				return filepath.SkipDir // 跳过 .nib/.git 等隐藏目录
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			sig[rel] = info.Name() + ":" + itoa(info.Size()) + ":" + info.ModTime().String()
		}
		return nil
	})
	return sig, err
}

func itoa(i int64) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	p := len(buf)
	for i > 0 {
		p--
		buf[p] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[p:])
}

// WatchChanges 每 interval 比对一次签名，变化时向 changes 通道发信号（带 300ms 去抖）。
// 调用 stop() 结束；初始签名作为基准，启动不触发事件。
func WatchChanges(dir string, interval time.Duration) (changes <-chan struct{}, stop func()) {
	out := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		last, err := GoFileSignature(dir)
		if err != nil {
			last = map[string]string{}
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		var debounce *time.Timer
		for {
			select {
			case <-done:
				if debounce != nil {
					debounce.Stop()
				}
				return
			case <-ticker.C:
				cur, err := GoFileSignature(dir)
				if err != nil {
					continue
				}
				if mapsEqual(last, cur) {
					continue
				}
				last = cur
				if debounce != nil {
					debounce.Stop()
				}
				debounce = time.AfterFunc(300*time.Millisecond, func() {
					select {
					case out <- struct{}{}:
					default:
					}
				})
			}
		}
	}()
	return out, func() { close(done) }
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}
