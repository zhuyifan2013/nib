// nib 是 Nib 工具链 CLI。
//
//	nib dev [-nowatch] <pkg>   开发常驻外壳：窗口常驻，改 Go 业务代码 <2s 热替换
//	nib shell --sock <path>    内部命令：常驻窗口层（由 dev 子进程拉起，勿手动调用）
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"nib.dev/nib/harness"
)

func main() {
	dev := flag.NewFlagSet("dev", flag.ExitOnError)
	noWatch := dev.Bool("nowatch", false, "不监听文件改动（只构建运行一次）")
	interval := dev.Duration("interval", 250*time.Millisecond, "文件轮询间隔")
	shell := flag.NewFlagSet("shell", flag.ExitOnError)
	sock := shell.String("sock", "", "unix socket 路径")

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "dev":
		dev.Parse(os.Args[2:])
		pkg := dev.Arg(0)
		if pkg == "" {
			usage()
			os.Exit(2)
		}
		err := harness.Dev(harness.DevOptions{
			Pkg:      pkg,
			NoWatch:  *noWatch,
			Interval: *interval,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "nib dev: "+err.Error())
			os.Exit(1)
		}
	case "probe":
		probe(os.Args[2:])
	case "shell":
		shell.Parse(os.Args[2:])
		if *sock == "" {
			usage()
			os.Exit(2)
		}
		if err := harness.RunShell(*sock); err != nil {
			fmt.Fprintln(os.Stderr, "nib shell: "+err.Error())
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  nib dev [-nowatch] [-interval 250ms] <pkg>   开发常驻外壳（改业务代码即热替换）
  nib probe --sock <path>                      模拟业务进程（调试 shell/桥接）
  nib shell --sock <path>                      内部：常驻窗口层（dev 自动拉起）`)
}
