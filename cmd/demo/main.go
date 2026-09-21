// Nib M1 demo：最小可运行应用，验证 窗口 → 桥接 → Go 绑定 全链路。
package main

import (
	"encoding/json"
	"log"

	"nib.dev/nib/core"
	"nib.dev/nib/ipc"
)

const page = `<!doctype html><html><head><meta charset="utf-8"><title>Nib</title>
<style>
body{font-family:-apple-system,system-ui;margin:48px;color:#222}
button{font-size:16px;padding:10px 20px;cursor:pointer}
#out{margin-top:16px;font-family:ui-monospace,monospace;white-space:pre-wrap}
</style></head>
<body>
<h1>Nib Demo</h1>
<p><button id="btn">window.nib.invoke('GreetService.Greet')</button></p>
<div id="out">等待调用…</div>
<script>
document.getElementById('btn').onclick = async function(){
  const out = document.getElementById('out');
  try {
    const r = await window.nib.invoke('GreetService.Greet', {name: 'AI'});
    out.textContent = '✅ 结果: ' + r;
  } catch(e) {
    out.textContent = '❌ 错误: ' + JSON.stringify(e, null, 2);
  }
};
</script>
</body></html>`

func main() {
	rt := core.New(core.Options{Title: "Nib Demo", Width: 900, Height: 600, DevTools: true})

	rt.Bind("GreetService.Greet", func(args json.RawMessage) (any, *ipc.Error) {
		var a struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return nil, ipc.NewError("greet/invalid_args", err.Error())
		}
		return "Hello, " + a.Name + "! (from Go)", nil
	})

	if err := rt.Run(page); err != nil {
		log.Fatal(err)
	}
}
