// Nib M1 demo：最小可运行应用，验证 窗口 → 桥接 → Go 绑定 全链路，
// 并自检三条 IPC 错误路径均返回结构化 ipc.Error（见 docs/tests/ipc-error-path.md）。
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
.case{margin:8px 0}
.pass{color:#1a7f37}.fail{color:#cf222e}
</style></head>
<body>
<h1>Nib Demo</h1>
<p><button id="btn">window.nib.invoke('GreetService.Greet')</button>
<button id="selftest">重跑错误路径自检</button></p>
<div id="out">等待调用…</div>
<script>
function show(obj){ document.getElementById('out').textContent = typeof obj === 'string' ? obj : JSON.stringify(obj, null, 2); }

document.getElementById('btn').onclick = async function(){
  try {
    const r = await window.nib.invoke('GreetService.Greet', {name: 'AI'});
    show('✅ 结果: ' + r);
  } catch(e) { show('❌ 错误: ' + JSON.stringify(e, null, 2)); }
};

// 三条错误路径自检：断言 reject 收到含 code/message 字段的结构化对象。
var cases = [
  { name: '调用不存在绑定',      run: function(){ return window.nib.invoke('NoSuch.Method', {}); }, want: 'ipc/not_found' },
  { name: '参数解码失败',        run: function(){ return window.nib.invoke('GreetService.Greet', "not-an-object"); }, want: 'greet/invalid_args' },
  { name: 'Go handler 返回错误', run: function(){ return window.nib.invoke('GreetService.Boom', {}); }, want: 'demo/boom' },
];

async function selftest(){
  var lines = [], allPass = true;
  for (var i = 0; i < cases.length; i++){
    var c = cases[i];
    try {
      await c.run();
      lines.push('FAIL  ' + c.name + ' — 期望 reject，实际 resolve'); allPass = false;
    } catch(e){
      var structured = (e !== null && typeof e === 'object' && typeof e.code === 'string' && typeof e.message === 'string');
      var ok = structured && e.code === c.want;
      if (!ok) allPass = false;
      lines.push((ok ? 'PASS  ' : 'FAIL  ') + c.name + '  code=' + (e && e.code) + '  message=' + (e && e.message));
    }
  }
  var text = lines.join('\n');
  document.getElementById('out').textContent = text;
  document.title = allPass ? 'Nib ✅ 错误路径自检通过' : 'Nib ❌ 错误路径自检失败';
  // 回传结果，便于脚本化断言（stdout 日志）。
  window.nib.invoke('Demo.Report', {pass: allPass, lines: lines}).catch(function(){});
}
document.getElementById('selftest').onclick = selftest;
selftest();
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

	// 自检结果回传：前端把错误路径断言结果送回 Go 端 stdout，支持脚本化验证。
	rt.Bind("Demo.Report", func(args json.RawMessage) (any, *ipc.Error) {
		log.Printf("error-path selftest: %s", string(args))
		return "ok", nil
	})

	// 故意失败的绑定，用于验证 handler 错误透传为结构化 ipc.Error。
	rt.Bind("GreetService.Boom", func(args json.RawMessage) (any, *ipc.Error) {
		return nil, ipc.NewError("demo/boom", "intentional failure for error-path test")
	})

	if err := rt.Run(page); err != nil {
		log.Fatal(err)
	}
}
