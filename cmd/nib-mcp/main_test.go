package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestDriverErrorRedactsEscapedToken(t *testing.T) {
	token := `1234567890123456"secret\more`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		inner, _ := json.Marshal(map[string]string{"message": token, token: "detail"})
		json.NewEncoder(w).Encode(map[string]string{"message": "operation failed: " + string(inner)})
	}))
	defer server.Close()
	b := &bridge{base: server.URL, token: token, client: server.Client()}
	_, _, err := b.call(context.Background(), "GET", "/v1/ready", nil, false)
	if err == nil || strings.Contains(err.Error(), "1234567890123456") || !strings.Contains(err.Error(), "driver/error") {
		t.Fatalf("unredacted error: %v", err)
	}
}

func TestBridgeDriverRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token-123456" {
			t.Error("missing authorization")
		}
		switch r.URL.Path {
		case "/v1/click":
			body, _ := io.ReadAll(r.Body)
			if r.Method != http.MethodPost || !strings.Contains(string(body), `"selector":"#button"`) {
				t.Errorf("invalid click request %s %s", r.Method, body)
			}
			w.Write([]byte(`{"result":true}`))
		case "/v1/screenshot":
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("png"))
		default:
			http.Error(w, "failure", http.StatusUnauthorized)
		}
	}))
	defer server.Close()
	b := &bridge{base: server.URL, token: "test-token-123456", client: server.Client()}
	result, _, err := b.call(context.Background(), "POST", "/v1/click", selector{Selector: "#button"}, false)
	if err != nil || result.Content[0].(*mcp.TextContent).Text != `{"result":true}` {
		t.Fatalf("result=%v err=%v", result, err)
	}
	result, _, err = b.call(context.Background(), "POST", "/v1/screenshot", empty{}, true)
	if err != nil || string(result.Content[0].(*mcp.ImageContent).Data) != "png" {
		t.Fatalf("result=%v err=%v", result, err)
	}
	_, _, err = b.call(context.Background(), "GET", "/v1/ready", nil, false)
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected driver error, got %v", err)
	}
}

func TestMCPBindingSchemaAndCall(t *testing.T) {
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/v1/call" || !strings.Contains(string(body), `"number":9007199254740993`) {
			t.Errorf("unexpected driver request %s %s", r.URL.Path, body)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"result":"hello"}`)
	}))
	defer httpServer.Close()
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	s := server(&bridge{base: httpServer.URL, token: "test-token-123456", client: httpServer.Client()})
	serverSession, err := s.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "call", Arguments: json.RawMessage(`{"call":"Greeting.Hello","args":{"nested":{"number":9007199254740993}}}`)})
	if err != nil || result.IsError {
		t.Fatalf("result=%v error=%v", result, err)
	}
	if !strings.Contains(result.Content[0].(*mcp.TextContent).Text, "hello") {
		t.Fatal(result)
	}
}
