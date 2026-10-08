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

	"nib.dev/nib/driver"
	"nib.dev/nib/harness"
)

func main() {
	dev := flag.NewFlagSet("dev", flag.ExitOnError)
	noWatch := dev.Bool("nowatch", false, "不监听文件改动（只构建运行一次）")
	interval := dev.Duration("interval", 250*time.Millisecond, "文件轮询间隔")
	devEvents := dev.String("events", "", "事件流 NDJSON 导出路径（\"-\" 为 stdout）")
	devHeadless := dev.Bool("headless", false, "隐藏窗口（仍需 macOS GUI session）")
	devPort := dev.Int("driver-port", 0, "本机 HTTP 驱动端口；NIB_DRIVER_TOKEN 至少 16 字节")
	devTimeout := dev.Duration("driver-timeout", 0, "驱动请求超时（0=5s 默认，最大30s）")
	shell := flag.NewFlagSet("shell", flag.ExitOnError)
	sock := shell.String("sock", "", "unix socket 路径")
	shellEvents := shell.String("events", "", "事件流 NDJSON 导出路径（\"-\" 为 stdout）")

	shellHeadless := shell.Bool("headless", false, "隐藏窗口（仍需 macOS GUI session）")
	shellPort := shell.Int("driver-port", 0, "本机 HTTP 驱动端口；NIB_DRIVER_TOKEN 至少 16 字节")
	shellTimeout := shell.Duration("driver-timeout", 0, "内部：驱动请求超时（0=5s 默认，最大30s）")
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
			Headless: *devHeadless,
			Driver:   driver.Options{Port: *devPort, Timeout: *devTimeout, Token: os.Getenv("NIB_DRIVER_TOKEN")},
			NoWatch:  *noWatch,
			Interval: *interval,
			Events:   *devEvents,
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
		if err := harness.RunShellWithOptions(*sock, *shellEvents, harness.ShellOptions{Headless: *shellHeadless, Driver: driver.Options{Port: *shellPort, Timeout: *shellTimeout, Token: os.Getenv("NIB_DRIVER_TOKEN")}}); err != nil {
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
  nib dev [-nowatch] [-interval 250ms] [-events <path|->] [--headless --driver-port <port> --driver-timeout <duration>] <pkg>
                                               开发常驻外壳（改业务代码即热替换）
  nib probe --sock <path>                      模拟业务进程（调试 shell/桥接）
  nib shell --sock <path> [-events <path|->] [--headless --driver-port <port> --driver-timeout <duration>]  内部：常驻窗口层（dev 自动拉起）`)
}
