// nib-mcp exposes an explicitly enabled local Nib driver over MCP stdio.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type bridge struct {
	base   string
	token  string
	client *http.Client
}
type empty struct{}
type target struct {
	Target string `json:"target" jsonschema:"URL or HTML to navigate to"`
}
type script struct {
	Script string `json:"script" jsonschema:"JavaScript expression to evaluate"`
}
type binding struct {
	Call string `json:"call" jsonschema:"Application binding name"`
	Args any    `json:"args,omitempty" jsonschema:"Arguments for the binding"`
}
type selector struct {
	Selector string `json:"selector" jsonschema:"CSS selector"`
}
type fill struct {
	Selector string `json:"selector" jsonschema:"CSS selector"`
	Value    string `json:"value" jsonschema:"Value to enter"`
}
type eventArgs struct {
	After uint64 `json:"after,omitempty" jsonschema:"Return events after this sequence"`
}

func (b *bridge) call(ctx context.Context, method, path string, input any, image bool) (*mcp.CallToolResult, any, error) {
	var body io.Reader
	if input != nil {
		raw, err := json.Marshal(input)
		if err != nil {
			return nil, nil, err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, b.base+path, body)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+b.token)
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := b.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("driver request failed: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (33<<20)+1))
	if err != nil {
		return nil, nil, err
	}
	if len(raw) > 33<<20 {
		return nil, nil, errors.New("driver response exceeds 33 MiB")
	}
	if resp.StatusCode != http.StatusOK {
		var failure struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = json.Unmarshal(raw, &failure)
		code := failure.Error.Code
		switch code {
		case "driver/unauthorized", "driver/origin", "driver/args", "driver/method", "driver/busy", "driver/native", "driver/not_found", "driver/timeout", "driver/operation":
		default:
			code = "driver/error"
		}
		return nil, nil, fmt.Errorf("driver returned HTTP %d (%s); see driver diagnostics for details", resp.StatusCode, code)
	}
	if image {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.ImageContent{Data: raw, MIMEType: "image/png"}}}, nil, nil
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}}, nil, nil
}

func server(b *bridge) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "nib", Version: "0.1.0"}, nil)
	mcp.AddTool(s, &mcp.Tool{Name: "ready", Description: "Check the current page and bridge readiness"}, func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, any, error) {
		return b.call(ctx, "GET", "/v1/ready", nil, false)
	})
	mcp.AddTool(s, &mcp.Tool{Name: "events", Description: "Read diagnostic events after a sequence number"}, func(ctx context.Context, _ *mcp.CallToolRequest, a eventArgs) (*mcp.CallToolResult, any, error) {
		return b.call(ctx, "GET", "/v1/events?after="+url.QueryEscape(strconv.FormatUint(a.After, 10)), nil, false)
	})
	mcp.AddTool(s, &mcp.Tool{Name: "navigate", Description: "Navigate the application window"}, func(ctx context.Context, _ *mcp.CallToolRequest, a target) (*mcp.CallToolResult, any, error) {
		return b.call(ctx, "POST", "/v1/navigate", a, false)
	})
	mcp.AddTool(s, &mcp.Tool{Name: "eval", Description: "Evaluate JavaScript in the application window"}, func(ctx context.Context, _ *mcp.CallToolRequest, a script) (*mcp.CallToolResult, any, error) {
		return b.call(ctx, "POST", "/v1/eval", a, false)
	})
	mcp.AddTool(s, &mcp.Tool{Name: "call", Description: "Call an application binding through the page bridge"}, func(ctx context.Context, req *mcp.CallToolRequest, _ binding) (*mcp.CallToolResult, any, error) {
		var args struct {
			Call string          `json:"call"`
			Args json.RawMessage `json:"args,omitempty"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, nil, err
		}
		return b.call(ctx, "POST", "/v1/call", args, false)
	})
	mcp.AddTool(s, &mcp.Tool{Name: "dom", Description: "Read the current DOM snapshot"}, func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, any, error) {
		return b.call(ctx, "POST", "/v1/dom", empty{}, false)
	})
	mcp.AddTool(s, &mcp.Tool{Name: "click", Description: "Click an element by CSS selector"}, func(ctx context.Context, _ *mcp.CallToolRequest, a selector) (*mcp.CallToolResult, any, error) {
		return b.call(ctx, "POST", "/v1/click", a, false)
	})
	mcp.AddTool(s, &mcp.Tool{Name: "fill", Description: "Fill a form control by CSS selector"}, func(ctx context.Context, _ *mcp.CallToolRequest, a fill) (*mcp.CallToolResult, any, error) {
		return b.call(ctx, "POST", "/v1/fill", a, false)
	})
	mcp.AddTool(s, &mcp.Tool{Name: "screenshot", Description: "Capture a PNG of the application window"}, func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, any, error) {
		return b.call(ctx, "POST", "/v1/screenshot", empty{}, true)
	})
	return s
}
func run() error {
	port := flag.Int("driver-port", 0, "local Nib driver port")
	flag.Parse()
	if *port < 1 || *port > 65535 || flag.NArg() != 0 {
		return errors.New("provide --driver-port in the range 1..65535")
	}
	token := os.Getenv("NIB_DRIVER_TOKEN")
	if len(token) < 16 {
		return errors.New("NIB_DRIVER_TOKEN must contain at least 16 bytes")
	}
	b := &bridge{base: fmt.Sprintf("http://127.0.0.1:%d", *port), token: token, client: &http.Client{Transport: &http.Transport{}, Timeout: 35 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}
	return server(b).Run(context.Background(), &mcp.StdioTransport{})
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
