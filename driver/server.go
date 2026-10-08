// Package driver exposes an explicitly enabled, authenticated local automation API.
package driver

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"nib.dev/nib/events"
)

type Window interface {
	Navigate(string) error
	EvalResult(context.Context, string) (json.RawMessage, error)
	Screenshot(context.Context) ([]byte, error)
	Events() *events.Bus
}

// Options leaves the driver disabled at its call sites when Port == 0.
// Token must be explicitly supplied; it is never printed to logs.
type Options struct {
	Port    int
	Token   string
	Timeout time.Duration
}
type Server struct {
	http *http.Server
	ln   net.Listener
}

// ValidateTimeout accepts zero as the default and rejects negative or excessive waits.
func ValidateTimeout(timeout time.Duration) error {
	if timeout < 0 || timeout > 30*time.Second {
		return errors.New("driver timeout must be between 0 and 30s (0 selects 5s default)")
	}
	return nil
}

func Start(win Window, opts Options) (*Server, error) {
	if opts.Port < 1 || opts.Port > 65535 {
		return nil, errors.New("driver port must be between 1 and 65535")
	}
	if len(opts.Token) < 16 {
		return nil, errors.New("driver token must contain at least 16 bytes")
	}
	if err := ValidateTimeout(opts.Timeout); err != nil {
		return nil, err
	}
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Second
	}
	ln, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", opts.Port))
	if err != nil {
		return nil, err
	}
	s := &Server{ln: ln}
	s.http = &http.Server{Handler: Handler(win, opts), ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: opts.Timeout + time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	go s.http.Serve(ln)
	return s, nil
}
func (s *Server) Close() error { return s.http.Close() }
func (s *Server) Addr() string { return s.ln.Addr().String() }

// Handler is also usable with httptest. At most one native operation runs per driver.
func Handler(win Window, opts Options) http.Handler {
	if opts.Timeout <= 0 {
		opts.Timeout = 5 * time.Second
	}
	gate := make(chan struct{}, 1)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if len(opts.Token) < 16 || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+opts.Token)) != 1 {
			fail(w, 401, "driver/unauthorized", "valid Bearer token required")
			return
		}
		// Browser pages cannot access the control plane, even if a token is leaked into a URL.
		if r.Header.Get("Origin") != "" {
			fail(w, 403, "driver/origin", "browser origins are not permitted")
			return
		}
		if r.URL.Path == "/v1/events" && r.Method == http.MethodGet {
			after, err := strconv.ParseUint(r.URL.Query().Get("after"), 10, 64)
			if r.URL.Query().Get("after") == "" {
				err = nil
			}
			if err != nil {
				fail(w, 400, "driver/args", "invalid after")
				return
			}
			recent := win.Events().Recent()
			filtered := make([]events.Event, 0, len(recent))
			for _, e := range recent {
				if e.Seq > after {
					filtered = append(filtered, e)
				}
			}
			result(w, map[string]any{"events": filtered, "oldest_seq": oldest(recent)})
			return
		}
		if r.URL.Path == "/v1/ready" && r.Method == http.MethodGet {
			// Readiness is evaluated inside the page; socket availability alone is not readiness.
		} else if r.Method != http.MethodPost {
			fail(w, 405, "driver/method", "POST required (GET for ready/events)")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), opts.Timeout)
		defer cancel()
		select {
		case gate <- struct{}{}:
			defer func() { <-gate }()
		default:
			fail(w, 429, "driver/busy", "another operation is running")
			return
		}
		p := &struct {
			Script   string          `json:"script"`
			Target   string          `json:"target"`
			Call     string          `json:"call"`
			Args     json.RawMessage `json:"args"`
			Selector string          `json:"selector"`
			Value    string          `json:"value"`
		}{}
		if r.Method == http.MethodPost {
			dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&p); err != nil {
				fail(w, 400, "driver/args", err.Error())
				return
			}
			if p == nil {
				fail(w, 400, "driver/args", "JSON object required")
				return
			}
			if err := dec.Decode(&struct{}{}); err != io.EOF {
				fail(w, 400, "driver/args", "exactly one JSON object required")
				return
			}
		}
		var js string
		switch r.URL.Path {
		case "/v1/ready":
			js = `({ready:document.readyState==='complete' && !!window.nib, url:location.href})`
		case "/v1/navigate":
			if p.Target == "" {
				fail(w, 400, "driver/args", "target required")
				return
			}
			if err := win.Navigate(p.Target); err != nil {
				fail(w, 500, "driver/native", err.Error())
				return
			}
			result(w, map[string]any{"accepted": true})
			return
		case "/v1/eval":
			js = p.Script
		case "/v1/call":
			if p.Call == "" || strings.HasPrefix(p.Call, "nib.ctrl.") {
				fail(w, 400, "driver/args", "application binding name required")
				return
			}
			args := string(p.Args)
			if args == "" {
				args = "null"
			}
			js = "window.nib.invoke(" + quote(p.Call) + "," + args + ")"
		case "/v1/dom":
			js = `({url:location.href,title:document.title,readyState:document.readyState,html:document.documentElement.outerHTML})`
		case "/v1/click":
			js = "(()=>{const e=document.querySelector(" + quote(p.Selector) + ");if(!e)throw Error('selector not found');e.click();return true})()"
		case "/v1/fill":
			js = "(()=>{const e=document.querySelector(" + quote(p.Selector) + ");if(!e)throw Error('selector not found');if(!(e instanceof HTMLInputElement || e instanceof HTMLTextAreaElement || e instanceof HTMLSelectElement))throw Error('element is not a form control');const d=Object.getOwnPropertyDescriptor(Object.getPrototypeOf(e),'value');d.set.call(e," + quote(p.Value) + ");e.dispatchEvent(new Event('input',{bubbles:true}));e.dispatchEvent(new Event('change',{bubbles:true}));return true})()"
		case "/v1/screenshot":
			png, err := win.Screenshot(ctx)
			if err != nil {
				nativeError(w, err)
				return
			}
			w.Header().Set("Content-Type", "image/png")
			w.Write(png)
			return
		default:
			fail(w, 404, "driver/not_found", "unknown endpoint")
			return
		}
		value, err := win.EvalResult(ctx, js)
		if err != nil {
			nativeError(w, err)
			return
		}
		result(w, value)
	})
}
func quote(s string) string { b, _ := json.Marshal(s); return string(b) }
func oldest(e []events.Event) uint64 {
	if len(e) == 0 {
		return 0
	}
	return e[0].Seq
}
func result(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"result": v})
}
func fail(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func nativeError(w http.ResponseWriter, err error) {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		fail(w, 504, "driver/timeout", err.Error())
	} else {
		fail(w, 422, "driver/operation", err.Error())
	}
}
