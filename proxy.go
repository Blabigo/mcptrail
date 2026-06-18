package mcptrail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultProxyBaseURL is the MCP Trail cloud proxy origin.
const DefaultProxyBaseURL = "https://proxy.mcptrail.com"

const (
	proxyProtocolVersion = "2024-11-05"
	// Streamable HTTP requires Accept to list both JSON and SSE.
	proxyAccept = "application/json, text/event-stream"
)

// RPCError is a JSON-RPC error returned by the proxy/upstream (e.g. a policy denial).
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

// ProxyClient calls a guarded MCP server through the proxy (POST /v1/proxy/{slug})
// over MCP Streamable HTTP: initialize -> mcp-session-id capture ->
// notifications/initialized, then tools/list / tools/call, reusing the session.
type ProxyClient struct {
	url       string
	token     string
	http      *http.Client
	sessionID string
	rpcID     int
}

// NewProxyClient targets {baseURL}/v1/proxy/{slug} with a per-server bearer token.
func NewProxyClient(baseURL, slug, token string) *ProxyClient {
	if baseURL == "" {
		baseURL = DefaultProxyBaseURL
	}
	return &ProxyClient{
		url:   strings.TrimRight(baseURL, "/") + "/v1/proxy/" + url.PathEscape(slug),
		token: token,
		http:  &http.Client{Timeout: 60 * time.Second},
		rpcID: 1,
	}
}

func (c *ProxyClient) SessionID() string { return c.sessionID }

type proxyResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *RPCError       `json:"error"`
}

// Initialize opens an MCP session (idempotent once a session id is held).
func (c *ProxyClient) Initialize(ctx context.Context) error {
	if c.sessionID != "" {
		return nil
	}
	initBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      c.nextID(),
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": proxyProtocolVersion,
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "mcptrail-go", "version": "0.1.0"},
		},
	}
	if _, err := c.post(ctx, initBody, false, true); err != nil {
		return err
	}
	if c.sessionID == "" {
		return fmt.Errorf("initialize: proxy did not return an mcp-session-id header")
	}
	_, err := c.post(ctx, map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}, true, false)
	return err
}

// ListTools returns the raw `tools` array from tools/list.
func (c *ProxyClient) ListTools(ctx context.Context) (json.RawMessage, error) {
	if err := c.Initialize(ctx); err != nil {
		return nil, err
	}
	result, err := c.rpc(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Tools json.RawMessage `json:"tools"`
	}
	_ = json.Unmarshal(result, &wrap)
	return wrap.Tools, nil
}

// CallTool invokes a tool and returns the raw JSON-RPC result.
func (c *ProxyClient) CallTool(ctx context.Context, name string, args map[string]any) (json.RawMessage, error) {
	if err := c.Initialize(ctx); err != nil {
		return nil, err
	}
	if args == nil {
		args = map[string]any{}
	}
	return c.rpc(ctx, "tools/call", map[string]any{"name": name, "arguments": args})
}

func (c *ProxyClient) nextID() int {
	id := c.rpcID
	c.rpcID++
	return id
}

func (c *ProxyClient) rpc(ctx context.Context, method string, params any) (json.RawMessage, error) {
	body := map[string]any{"jsonrpc": "2.0", "id": c.nextID(), "method": method, "params": params}
	parsed, err := c.post(ctx, body, true, false)
	if err != nil {
		return nil, err
	}
	if parsed.Error != nil {
		return nil, parsed.Error
	}
	return parsed.Result, nil
}

func (c *ProxyClient) post(ctx context.Context, body any, withSession, captureSession bool) (proxyResponse, error) {
	if c.token == "" {
		return proxyResponse{}, fmt.Errorf("no proxy token: pass a token to NewProxyClient or set MCPTRAIL_PROXY_TOKEN")
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return proxyResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(raw))
	if err != nil {
		return proxyResponse{}, err
	}
	req.Header.Set("Accept", proxyAccept)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	if withSession {
		req.Header.Set("mcp-session-id", c.sessionID)
		req.Header.Set("mcp-protocol-version", proxyProtocolVersion)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return proxyResponse{}, err
	}
	defer resp.Body.Close()
	if captureSession {
		if sid := strings.TrimSpace(resp.Header.Get("mcp-session-id")); sid != "" {
			c.sessionID = sid
		}
	}
	data, _ := io.ReadAll(resp.Body)
	return parseProxyResponse(data), nil
}

// parseProxyResponse reads a JSON-RPC body that may be plain JSON or an SSE stream.
func parseProxyResponse(data []byte) proxyResponse {
	t := strings.TrimSpace(string(data))
	var out proxyResponse
	if t == "" {
		return out
	}
	if strings.HasPrefix(t, "{") {
		_ = json.Unmarshal([]byte(t), &out)
		return out
	}
	for _, line := range strings.Split(t, "\n") {
		s := strings.TrimSpace(line)
		if !strings.HasPrefix(s, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(s, "data:"))
		if strings.HasPrefix(payload, "{") {
			if json.Unmarshal([]byte(payload), &out) == nil {
				return out
			}
		}
	}
	return out
}
