// nibgen 是 Nib 绑定生成器 CLI：扫描 Go 包中的服务方法，生成类型安全的 TS 客户端。
//
//	usage: nibgen [-out dir] <pkgdir>
//
// 示例：
//
//	nibgen -out ./frontend/src/bindings ./backend
//	go run nib.dev/nib/cmd/nibgen -out ./bindings ./cmd/demo
package main

import (
	"flag"
	"fmt"
	"os"

	"nib.dev/nib/binding"
)

func main() {
	out := flag.String("out", "./bindings", "TS 客户端输出目录")
	flag.Parse()
	dir := flag.Arg(0)
	if dir == "" {
		dir = "."
	}

	pkg, err := binding.Load(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "nibgen: "+err.Error())
		os.Exit(1)
	}
	if len(pkg.Services) == 0 {
		fmt.Fprintln(os.Stderr, "nibgen: 未在 "+dir+" 发现绑定服务（导出 struct 的导出方法）")
		os.Exit(1)
	}

	rep, err := binding.Generate(pkg, *out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "nibgen: "+err.Error())
		os.Exit(1)
	}

	fmt.Printf("nibgen: %d 个服务 -> %s\n", len(pkg.Services), *out)
	for _, f := range rep.Written {
		fmt.Println("  write  " + f)
	}
	for _, f := range rep.Unchanged {
		fmt.Println("  same   " + f)
	}
	for _, f := range rep.Removed {
		fmt.Println("  remove " + f)
	}
}
