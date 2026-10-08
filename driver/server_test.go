package driver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"nib.dev/nib/events"
	"strings"
	"testing"
	"time"
)

type fake struct {
	bus  *events.Bus
	js   string
	wait bool
}

func (f *fake) Navigate(string) error { return nil }
func (f *fake) Events() *events.Bus   { return f.bus }
func (f *fake) EvalResult(ctx context.Context, s string) (json.RawMessage, error) {
	f.js = s
	if f.wait {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return json.RawMessage(`{"ok":true}`), nil
}
func (f *fake) Screenshot(context.Context) ([]byte, error) { return []byte("png"), nil }

const token = "test-token-at-least-16"

func request(h http.Handler, method, path, body, auth, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", auth)
	r.Header.Set("Origin", origin)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestSecurityAndLimits(t *testing.T) {
	f := &fake{bus: events.NewBus(2)}
	h := Handler(f, Options{Token: token})
	for _, c := range []struct {
		path, body, auth, origin string
		status                   int
	}{
		{"/v1/eval", `{"script":"1"}`, "", "", 401},
		{"/v1/eval", `{"script":"1"}`, "Bearer " + token, "http://evil", 403},
		{"/v1/dom", `null`, "Bearer " + token, "", 400},
		{"/v1/eval", `{"script":"1","extra":true}`, "Bearer " + token, "", 400},
		{"/v1/eval", `{"script":"1"} {}`, "Bearer " + token, "", 400},
		{"/v1/eval", `{"script":"` + strings.Repeat("x", 1<<20) + `"}`, "Bearer " + token, "", 400},
		{"/v1/call", `{"call":"nib.ctrl.close"}`, "Bearer " + token, "", 400},
		{"/v1/unknown", `{}`, "Bearer " + token, "", 404},
	} {
		w := request(h, "POST", c.path, c.body, c.auth, c.origin)
		if w.Code != c.status {
			t.Errorf("%s: got %d want %d", c.path, w.Code, c.status)
		}
	}
	if f.js != "" {
		t.Fatal("invalid request executed JS")
	}
}
func TestTimeoutAndQuoting(t *testing.T) {
	f := &fake{bus: events.NewBus(2), wait: true}
	h := Handler(f, Options{Token: token, Timeout: 10 * time.Millisecond})
	w := request(h, "POST", "/v1/eval", `{"script":"new Promise(()=>{})"}`, "Bearer "+token, "")
	if w.Code != 504 {
		t.Fatal(w.Code, w.Body.String())
	}
	f.wait = false
	w = request(h, "POST", "/v1/fill", `{"selector":"#x');throw Error('oops","value":"中文\"\\\n"}`, "Bearer "+token, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if !strings.Contains(f.js, quote("#x');throw Error('oops")) || !strings.Contains(f.js, quote("中文\"\\\n")) {
		t.Fatal("unescaped JS", f.js)
	}
}
func TestEventsAndScreenshot(t *testing.T) {
	f := &fake{bus: events.NewBus(2)}
	for i := 0; i < 3; i++ {
		f.bus.Emit(events.IPCCall, nil)
	}
	h := Handler(f, Options{Token: token})
	w := request(h, "GET", "/v1/events?after=2", "", "Bearer "+token, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"oldest_seq":2`) || strings.Contains(w.Body.String(), `"seq":2`) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request(h, "POST", "/v1/screenshot", `{}`, "Bearer "+token, "")
	if w.Header().Get("Content-Type") != "image/png" || w.Body.String() != "png" {
		t.Fatal(w.Body.String())
	}
}
func TestStartValidation(t *testing.T) {
	f := &fake{bus: events.NewBus(2)}
	for _, o := range []Options{{Port: 0, Token: token}, {Port: 65536, Token: token}, {Port: 8123, Token: "short"}, {Port: 8123, Token: token, Timeout: 31 * time.Second}, {Port: 8123, Token: token, Timeout: -time.Nanosecond}} {
		if s, err := Start(f, o); err == nil {
			s.Close()
			t.Fatal("invalid options accepted")
		}
	}
}
