#!/usr/bin/env python3
"""macOS GUI-session test. Uses only a local fixture; --headless hides the window."""
import concurrent.futures
import json
import os
import pathlib
import signal
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

ROOT = pathlib.Path(__file__).resolve().parents[1]
TOKEN = "local-e2e-token-0123456789"


def run(mode, binary, state):
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]
    env = os.environ.copy()
    env["NIB_DRIVER_TOKEN"] = TOKEN
    if mode == "direct":
        env["NIB_TEST_DRIVER_PORT"] = str(port)
        env.pop("NIB_SOCK", None)
        command = [str(binary)]
    else:
        command = [str(binary), "dev", "--nowatch", "--headless", "--driver-port", str(port), "--driver-timeout", "1500ms", "./driver/testdata/app"]
    with open(state / (mode + ".log"), "w") as log:
        process = subprocess.Popen(command, cwd=ROOT, env=env, stdout=log, stderr=log, start_new_session=True)

        def request(path, data=None, token=TOKEN, expected=200):
            req = urllib.request.Request(
                "http://127.0.0.1:%d/v1/%s" % (port, path),
                data=None if data is None else json.dumps(data).encode(),
                headers={"Authorization": "Bearer " + token, "Content-Type": "application/json"},
            )
            try:
                response = urllib.request.urlopen(req, timeout=8)
            except urllib.error.HTTPError as err:
                response = err
            with response:
                raw = response.read()
                assert response.status == expected, (response.status, raw)
                return raw if path == "screenshot" else json.loads(raw)

        try:
            deadline = time.monotonic() + 25
            while time.monotonic() < deadline:
                assert process.poll() is None, "application exited: " + str(state / (mode + ".log"))
                try:
                    ready = request("ready")["result"]
                    if ready["ready"] and request("eval", {"script": "document.title"})["result"] == "Fixture":
                        break
                except (OSError, AssertionError):
                    pass
                time.sleep(.15)
            else:
                raise AssertionError("driver did not become ready")
            request("ready", token="incorrect", expected=401)
            assert request("call", {"call": "Fixture.Echo", "args": {"name": "中文", "n": 42}})["result"] == {"name": "中文", "n": 42}
            failure = request("call", {"call": "Fixture.Fail"}, expected=422)
            assert "fixture/failure" in failure["error"]["message"]
            assert request("eval", {"script": "Promise.resolve({answer:42})"})["result"] == {"answer": 42}
            syntax = request("eval", {"script": "throw new Error('syntax evidence')"}, expected=422)
            assert "syntax evidence" in syntax["error"]["message"]
            request("eval", {"script": "(()=>{const v={};v.self=v;return v})()"}, expected=422)
            request("fill", {"selector": "#input", "value": "中文输入"})
            request("click", {"selector": "#go"})
            assert request("eval", {"script": "document.title"})["result"] == "中文输入"
            request("click", {"selector": "#missing"}, expected=422)
            request("navigate", {"target": "<html><head><title>Navigated</title></head><body>local</body></html>"})
            for _ in range(40):
                if request("eval", {"script": "document.title"})["result"] == "Navigated":
                    break
                time.sleep(.1)
            else:
                raise AssertionError("navigation timed out")
            dom = request("dom", {})["result"]
            assert dom["title"] == "Navigated" and "local" in dom["html"]
            png = request("screenshot", {})
            assert png.startswith(b"\x89PNG\r\n\x1a\n") and len(png) > 1000
            (state / (mode + ".png")).write_bytes(png)
            events = request("events")["result"]["events"]
            assert any(e["kind"] == "ipc.response" for e in events)
            with concurrent.futures.ThreadPoolExecutor() as pool:
                started=time.monotonic()
                pending = pool.submit(request, "eval", {"script": "new Promise(()=>{})"}, expected=504)
                time.sleep(.25)
                request("dom", {}, expected=429)
                pending.result()
                elapsed=time.monotonic()-started
                if mode=="harness":
                    assert 1.0<elapsed<3.5, "custom driver timeout was not propagated: %.2fs"%elapsed
                    print("harness custom timeout: %.2fs"%elapsed)
            assert request("ready")["result"]["ready"]
            if mode == "harness":
                try:
                    request("eval", {"script": "window.nib.invoke('nib.ctrl.close',null)"}, expected=422)
                except (OSError, AssertionError):
                    pass
                assert process.wait(timeout=8) == 0, "dev parent did not handle native close"
            print(mode + ": PASS (auth, binding/error, Promise/cyclic/error eval, navigation, fill/click, DOM, PNG, events, timeout/busy/recovery)")
        finally:
            if process.poll() is None:
                os.killpg(process.pid, signal.SIGTERM)
            try:
                process.wait(timeout=8)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait()
            # The port must no longer accept requests after shutdown.
            with socket.socket() as sock:
                assert sock.connect_ex(("127.0.0.1", port)) != 0, "driver listener survived shutdown"


if __name__ == "__main__":
    state = pathlib.Path(tempfile.mkdtemp(prefix="nib-driver-e2e-"))
    print("Evidence:", state, flush=True)
    subprocess.run(["go", "build", "-o", str(state / "nib"), "./cmd/nib"], cwd=ROOT, check=True)
    subprocess.run(["go", "build", "-o", str(state / "fixture"), "./driver/testdata/app"], cwd=ROOT, check=True)
    run("direct", state / "fixture", state)
    run("harness", state / "nib", state)
