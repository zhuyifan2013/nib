package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"nib.dev/nib/harness"
	"nib.dev/nib/ipc"
)

var evalAfter *string

// probe 是调试/验证子命令：模拟业务进程连上 shell，回显收到的调用。
//
//	nib probe --sock <path> [--navigate "<html>…"]
//
// 页面可经 window.nib.invoke('Probe.Echo', {msg}) 触发回显；结果打到 stdout。
func probe(args []string) {
	fs := flag.NewFlagSet("probe", flag.ExitOnError)
	sock := fs.String("sock", "", "shell 的 unix socket 路径")
	nav := fs.String("navigate", "<html><body><h1>nib probe</h1></body></html>", "起始页面")
	evalAfter = fs.String("evalafter", "", "连接 N 秒后 Eval 一段自省脚本（调试桥接）")
	fs.Parse(args)
	if *sock == "" {
		fs.Usage()
		os.Exit(2)
	}

	win, err := harness.DialRemote(*sock)
	if err != nil {
		log.Fatal(err)
	}
	if *evalAfter != "" {
		go func() {
			time.Sleep(2 * time.Second)
			win.Eval("window.nib.invoke('Probe.Echo',{msg:'state ready='+document.readyState+' nib='+typeof window.nib+' title='+document.title+' url='+location.href}).catch(function(e){window.nib.invoke('Probe.Echo',{msg:'evalerr '+e})})")
		}()
	}
	win.Bind("Probe.Echo", func(args json.RawMessage) (any, *ipc.Error) {
		var a struct {
			Msg string `json:"msg"`
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return nil, ipc.NewError("probe/invalid_args", err.Error())
		}
		fmt.Printf("probe: Echo 被调用 msg=%q\n", a.Msg)
		return "echo: " + a.Msg, nil
	})
	if err := win.Navigate(*nav); err != nil {
		log.Fatal(err)
	}
	log.Printf("probe: 已连接 %s，等待调用…", *sock)
	if err := win.Run(); err != nil {
		log.Fatal(err)
	}
}
