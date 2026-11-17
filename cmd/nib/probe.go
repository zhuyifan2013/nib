package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"nib.dev/nib/cli"
	"nib.dev/nib/harness"
	"nib.dev/nib/ipc"
	"os/signal"
	"syscall"
	"time"
)

func runProbe(sock, nav, evalAfter string, out, logs io.Writer, r *cli.Reporter) error {
	win, err := harness.DialRemote(sock, nil)
	if err != nil {
		return cli.Fail("probe/connect", "Start nib shell and use its --sock path.", err)
	}
	defer win.Close()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			win.Close()
		case <-done:
		}
	}()
	if evalAfter != "" {
		go func() {
			timer := time.NewTimer(2 * time.Second)
			defer timer.Stop()
			select {
			case <-timer.C:
				win.Eval("window.nib.invoke('Probe.Echo',{msg:'state ready='+document.readyState+' nib='+typeof window.nib+' title='+document.title+' url='+location.href}).catch(function(e){window.nib.invoke('Probe.Echo',{msg:'evalerr '+e})})")
			case <-done:
			}
		}()
	}
	win.Bind("Probe.Echo", func(args json.RawMessage) (any, *ipc.Error) {
		var a struct {
			Msg string `json:"msg"`
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return nil, ipc.NewError("probe/invalid_args", err.Error())
		}
		target := out
		if r.JSON {
			target = logs
		}
		fmt.Fprintf(target, "probe: Echo 被调用 msg=%q\n", a.Msg)
		return "echo: " + a.Msg, nil
	})
	if err := win.Navigate(nav); err != nil {
		return err
	}
	r.Lifecycle("running", nil)
	log.Printf("probe: 已连接 %s，等待调用…", sock)
	err = win.Run()
	if ctx.Err() != nil {
		return nil
	}
	return err
}
