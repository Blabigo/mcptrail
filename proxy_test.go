package mcptrail

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type proxyCall struct {
	path      string
	auth      string
	accept    string
	sessionID string
	protoVer  string
	method    string
	params    json.RawMessage
}

func fakeProxyServer(t *testing.T, sse, toolError bool) (*httptest.Server, *[]proxyCall) {
	t.Helper()
	var calls []proxyCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Method string          `json:"method"`
			ID     int             `json:"id"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.Unmarshal(body, &req)
		calls = append(calls, proxyCall{
			path:      r.URL.EscapedPath(),
			auth:      r.Header.Get("Authorization"),
			accept:    r.Header.Get("Accept"),
			sessionID: r.Header.Get("mcp-session-id"),
			protoVer:  r.Header.Get("mcp-protocol-version"),
			method:    req.Method,
			params:    req.Params,
		})
		switch req.Method {
		case "initialize":
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("mcp-session-id", "sess-1")
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{}}`)
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"result":{"tools":[{"name":"ping"}]}}`)
		case "tools/call":
			if toolError {
				_, _ = io.WriteString(w, `{"error":{"code":-32000,"message":"denied by policy"}}`)
				return
			}
			if sse {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, "event: message\ndata: {\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"pong\"}]}}\n\n")
				return
			}
			_, _ = io.WriteString(w, `{"result":{"content":[{"type":"text","text":"pong"}]}}`)
		default:
			_, _ = io.WriteString(w, `{}`)
		}
	}))
	return srv, &calls
}

func findProxyCall(calls []proxyCall, method string) *proxyCall {
	for i := range calls {
		if calls[i].method == method {
			return &calls[i]
		}
	}
	return nil
}

func TestProxyInitializeHandshake(t *testing.T) {
	srv, calls := fakeProxyServer(t, false, false)
	defer srv.Close()
	c := NewProxyClient(srv.URL, "demo server", "mg_live_x")
	if err := c.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	init := findProxyCall(*calls, "initialize")
	if init == nil || init.path != "/v1/proxy/demo%20server" || init.auth != "Bearer mg_live_x" {
		t.Fatalf("bad initialize: %+v", init)
	}
	if !strings.Contains(init.accept, "text/event-stream") || !strings.Contains(init.accept, "application/json") {
		t.Fatalf("Accept must list JSON + SSE: %q", init.accept)
	}
	note := findProxyCall(*calls, "notifications/initialized")
	if note == nil || note.sessionID != "sess-1" {
		t.Fatalf("initialized note missing session: %+v", note)
	}
	if c.SessionID() != "sess-1" {
		t.Fatalf("session not captured: %q", c.SessionID())
	}
}

func TestProxyCallToolReusesSession(t *testing.T) {
	srv, calls := fakeProxyServer(t, false, false)
	defer srv.Close()
	c := NewProxyClient(srv.URL, "demo", "mg_live_x")

	result, err := c.CallTool(context.Background(), "ping", map[string]any{"hello": "world"})
	if err != nil {
		t.Fatal(err)
	}
	call := findProxyCall(*calls, "tools/call")
	if call.sessionID != "sess-1" || call.protoVer != "2024-11-05" {
		t.Fatalf("missing session/proto headers: %+v", call)
	}
	var p struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	_ = json.Unmarshal(call.params, &p)
	if p.Name != "ping" || p.Arguments["hello"] != "world" {
		t.Fatalf("bad params: %+v", p)
	}
	if !strings.Contains(string(result), "pong") {
		t.Fatalf("bad result: %s", result)
	}

	if _, err := c.CallTool(context.Background(), "ping", nil); err != nil {
		t.Fatal(err)
	}
	inits := 0
	for _, cl := range *calls {
		if cl.method == "initialize" {
			inits++
		}
	}
	if inits != 1 {
		t.Fatalf("expected one initialize, got %d", inits)
	}
}

func TestProxyCallToolParsesSSE(t *testing.T) {
	srv, _ := fakeProxyServer(t, true, false)
	defer srv.Close()
	result, err := NewProxyClient(srv.URL, "demo", "tok").CallTool(context.Background(), "ping", nil)
	if err != nil || !strings.Contains(string(result), "pong") {
		t.Fatalf("SSE not parsed: %v %s", err, result)
	}
}

func TestProxyCallToolRPCError(t *testing.T) {
	srv, _ := fakeProxyServer(t, false, true)
	defer srv.Close()
	_, err := NewProxyClient(srv.URL, "demo", "tok").CallTool(context.Background(), "ping", nil)
	var rpcErr *RPCError
	if !errors.As(err, &rpcErr) || rpcErr.Code != -32000 || !strings.Contains(rpcErr.Message, "denied by policy") {
		t.Fatalf("expected RPCError, got %v", err)
	}
}

func TestProxyMissingToken(t *testing.T) {
	if err := NewProxyClient("https://proxy.test", "demo", "").Initialize(context.Background()); err == nil {
		t.Fatal("expected error with empty token")
	}
}
