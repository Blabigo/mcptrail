package mcptrail

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type capture struct {
	method string
	path   string
	query  string
	auth   string
	body   map[string]any
}

func stub(t *testing.T, status int, resp string, cap *capture) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cap.method = r.Method
		cap.path = r.URL.EscapedPath()
		cap.query = r.URL.RawQuery
		cap.auth = r.Header.Get("Authorization")
		if b, _ := io.ReadAll(r.Body); len(b) > 0 {
			_ = json.Unmarshal(b, &cap.body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, resp)
	}))
}

func TestListServers(t *testing.T) {
	var c capture
	srv := stub(t, 200, `{"servers":[{"id":"s1","slug":"gh","displayName":"GH"}]}`, &c)
	defer srv.Close()
	servers, err := NewClient(srv.URL, "tok").ListServers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if c.method != "GET" || c.path != "/api/v1/servers" || c.auth != "Bearer tok" {
		t.Fatalf("bad request: %+v", c)
	}
	if len(servers) != 1 || servers[0].Slug != "gh" || servers[0].DisplayName != "GH" {
		t.Fatalf("bad servers: %+v", servers)
	}
}

func TestGetServer(t *testing.T) {
	var c capture
	srv := stub(t, 200, `{"server":{"id":"s1","slug":"gh"}}`, &c)
	defer srv.Close()
	s, err := NewClient(srv.URL, "tok").GetServer(context.Background(), "gh")
	if err != nil {
		t.Fatal(err)
	}
	if c.path != "/api/v1/servers/gh" || s.Slug != "gh" {
		t.Fatalf("bad: %+v %+v", c, s)
	}
}

func TestCreateServer(t *testing.T) {
	var c capture
	srv := stub(t, 201, `{"server":{"id":"s1","slug":"gh"},"bearerToken":"mg_live_x","bearerTokenExpiresAt":9}`, &c)
	defer srv.Close()
	res, err := NewClient(srv.URL, "tok").CreateServer(context.Background(), CreateServerInput{
		DisplayName: "GH", Slug: "gh", UpstreamMcpURL: "https://e.com/mcp",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.method != "POST" || c.body["slug"] != "gh" || c.body["upstreamMcpUrl"] != "https://e.com/mcp" {
		t.Fatalf("bad body: %+v", c.body)
	}
	if res.BearerToken != "mg_live_x" {
		t.Fatalf("bad token: %q", res.BearerToken)
	}
}

func TestCreateWebmcpServer(t *testing.T) {
	var c capture
	srv := stub(t, 201, `{"server":{"id":"s1","slug":"shop"},"proxyUrl":"https://p/v1/proxy/shop","bearerToken":"mg_live_x","bearerTokenExpiresAt":9,"pairing":{"token":"mg_wmcp_x","connectUrl":"wss://p/v1/bridge/connect/shop"}}`, &c)
	defer srv.Close()
	res, err := NewClient(srv.URL, "tok").CreateWebmcpServer(context.Background(), CreateWebmcpServerInput{
		DisplayName: "Shop", Slug: "shop", Origin: "https://shop.example",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.method != "POST" || c.path != "/api/v1/servers" || c.body["kind"] != "webmcp" || c.body["origin"] != "https://shop.example" {
		t.Fatalf("bad request: %s %s %+v", c.method, c.path, c.body)
	}
	if res.Pairing.Token != "mg_wmcp_x" || res.ProxyURL != "https://p/v1/proxy/shop" {
		t.Fatalf("bad result: %+v", res)
	}
}

func TestMintRunToken(t *testing.T) {
	var c capture
	srv := stub(t, 201, `{"bearerToken":"mg_mbr_x","proxyUrl":"https://p/v1/proxy/shop","expiresAt":9}`, &c)
	defer srv.Close()
	res, err := NewClient(srv.URL, "tok").MintRunToken(context.Background(), "shop")
	if err != nil {
		t.Fatal(err)
	}
	if c.method != "POST" || c.path != "/api/v1/servers/shop/run-token" {
		t.Fatalf("bad request: %s %s", c.method, c.path)
	}
	if res.BearerToken != "mg_mbr_x" {
		t.Fatalf("bad token: %q", res.BearerToken)
	}
}

func TestMintWebmcpPairing(t *testing.T) {
	var c capture
	srv := stub(t, 201, `{"token":"mg_wmcp_x","connectUrl":"wss://p/v1/bridge/connect/shop","origin":"https://shop.example"}`, &c)
	defer srv.Close()
	res, err := NewClient(srv.URL, "tok").MintWebmcpPairing(context.Background(), "shop")
	if err != nil {
		t.Fatal(err)
	}
	if c.method != "POST" || c.path != "/api/v1/servers/shop/webmcp-pairing" {
		t.Fatalf("bad request: %s %s", c.method, c.path)
	}
	if res.ConnectURL != "wss://p/v1/bridge/connect/shop" {
		t.Fatalf("bad connectUrl: %q", res.ConnectURL)
	}
}

func TestUpdateServerSendsOnlyProvidedFields(t *testing.T) {
	var c capture
	srv := stub(t, 200, `{"ok":true,"applied":["proxyEnabled"]}`, &c)
	defer srv.Close()
	enabled := false
	res, err := NewClient(srv.URL, "tok").UpdateServer(context.Background(), "gh", ServerPatch{ProxyEnabled: &enabled})
	if err != nil {
		t.Fatal(err)
	}
	if c.method != "PATCH" || c.path != "/api/v1/servers/gh" {
		t.Fatalf("bad request: %+v", c)
	}
	if _, hasTool := c.body["toolAuthMode"]; hasTool {
		t.Fatalf("nil patch field must be omitted: %+v", c.body)
	}
	if c.body["proxyEnabled"] != false {
		t.Fatalf("bad body: %+v", c.body)
	}
	if len(res.Applied) != 1 || res.Applied[0] != "proxyEnabled" {
		t.Fatalf("bad applied: %+v", res.Applied)
	}
}

func TestListTools(t *testing.T) {
	var c capture
	srv := stub(t, 200, `{"tools":[{"toolName":"a","policy":"hitl"}]}`, &c)
	defer srv.Close()
	tools, err := NewClient(srv.URL, "tok").ListTools(context.Background(), "gh")
	if err != nil {
		t.Fatal(err)
	}
	if c.path != "/api/v1/servers/gh/tools" || len(tools) != 1 || tools[0].ToolName != "a" {
		t.Fatalf("bad: %+v %+v", c, tools)
	}
}

func TestSetToolPolicyEncodesName(t *testing.T) {
	var c capture
	srv := stub(t, 200, `{"ok":true}`, &c)
	defer srv.Close()
	if err := NewClient(srv.URL, "tok").SetToolPolicy(context.Background(), "gh", "delete db", "hitl"); err != nil {
		t.Fatal(err)
	}
	if c.method != "PUT" || c.path != "/api/v1/servers/gh/tools/delete%20db/policy" || c.body["policy"] != "hitl" {
		t.Fatalf("bad: %+v", c)
	}
}

func TestDlpModeGetSet(t *testing.T) {
	var cg capture
	gsrv := stub(t, 200, `{"dlpMode":"redacted"}`, &cg)
	defer gsrv.Close()
	mode, err := NewClient(gsrv.URL, "tok").GetDlpMode(context.Background(), "gh")
	if err != nil || mode != "redacted" {
		t.Fatalf("get: %v %q", err, mode)
	}

	var cs capture
	ssrv := stub(t, 200, `{"ok":true,"effectiveMode":"monitor"}`, &cs)
	defer ssrv.Close()
	eff, err := NewClient(ssrv.URL, "tok").SetDlpMode(context.Background(), "gh", "block")
	if err != nil || eff != "monitor" {
		t.Fatalf("set: %v %q", err, eff)
	}
	if cs.body["mode"] != "block" {
		t.Fatalf("bad body: %+v", cs.body)
	}
}

func TestListAuditQuery(t *testing.T) {
	var c capture
	srv := stub(t, 200, `{"rows":[],"nextCursor":null,"hasMore":false}`, &c)
	defer srv.Close()
	_, err := NewClient(srv.URL, "tok").ListAudit(context.Background(), AuditQuery{Scope: "gh", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if c.path != "/api/v1/audit" || c.query != "limit=50&scope=gh" {
		t.Fatalf("bad query: path=%s query=%s", c.path, c.query)
	}
}

func TestHitlListAndResolve(t *testing.T) {
	var cl capture
	lsrv := stub(t, 200, `{"pending":[{"id":"p1","slug":"gh","toolName":"x","status":"pending"}]}`, &cl)
	defer lsrv.Close()
	pending, err := NewClient(lsrv.URL, "tok").ListPendingHitl(context.Background())
	if err != nil || len(pending) != 1 || pending[0].ID != "p1" {
		t.Fatalf("list: %v %+v", err, pending)
	}

	var cr capture
	rsrv := stub(t, 200, `{"ok":true,"message":"approved"}`, &cr)
	defer rsrv.Close()
	if err := NewClient(rsrv.URL, "tok").ResolveHitl(context.Background(), "p1", "approve", "session"); err != nil {
		t.Fatal(err)
	}
	if cr.method != "POST" || cr.path != "/api/v1/hitl/p1" || cr.body["decision"] != "approve" || cr.body["mode"] != "session" {
		t.Fatalf("bad resolve: %+v", cr)
	}
}

func TestBundlesListAndCreate(t *testing.T) {
	var cl capture
	lsrv := stub(t, 200, `{"bundles":[{"id":"b1","slug":"bundle","members":[]}]}`, &cl)
	defer lsrv.Close()
	bundles, err := NewClient(lsrv.URL, "tok").ListBundles(context.Background())
	if err != nil || len(bundles) != 1 || bundles[0].Slug != "bundle" {
		t.Fatalf("list: %v %+v", err, bundles)
	}

	var cc capture
	csrv := stub(t, 201, `{"bundle":{"id":"b1","slug":"bundle"},"bearerToken":"mg_live_b","bearerTokenExpiresAt":9}`, &cc)
	defer csrv.Close()
	res, err := NewClient(csrv.URL, "tok").CreateBundle(context.Background(), CreateBundleInput{
		DisplayName: "B", Slug: "bundle", MemberSlugs: []string{"gh", "slack"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cc.method != "POST" || res.Bundle.Slug != "bundle" {
		t.Fatalf("bad create: %+v %+v", cc, res)
	}
}

func TestDlpRulesCRUD(t *testing.T) {
	var cl capture
	lsrv := stub(t, 200, `{"rules":[{"id":"r1","kind":"regex"}]}`, &cl)
	defer lsrv.Close()
	rules, err := NewClient(lsrv.URL, "tok").ListDlpRules(context.Background())
	if err != nil || len(rules) != 1 || rules[0].ID != "r1" {
		t.Fatalf("list: %v %+v", err, rules)
	}

	var cc capture
	csrv := stub(t, 201, `{"id":"r1"}`, &cc)
	defer csrv.Close()
	id, err := NewClient(csrv.URL, "tok").CreateDlpRule(context.Background(), CreateDlpRuleInput{
		Name: "x", Kind: "regex", Pattern: `\d+`, Action: "block",
	})
	if err != nil || id != "r1" {
		t.Fatalf("create: %v %q", err, id)
	}
	if cc.body["kind"] != "regex" {
		t.Fatalf("bad body: %+v", cc.body)
	}

	var cd capture
	dsrv := stub(t, 200, `{"ok":true}`, &cd)
	defer dsrv.Close()
	if err := NewClient(dsrv.URL, "tok").DeleteDlpRule(context.Background(), "r1"); err != nil {
		t.Fatal(err)
	}
	if cd.method != "DELETE" || cd.path != "/api/v1/dlp/rules/r1" {
		t.Fatalf("bad delete: %+v", cd)
	}
}

func TestAPIError(t *testing.T) {
	var c capture
	srv := stub(t, 403, `{"error":{"code":"insufficient_scope","message":"nope"}}`, &c)
	defer srv.Close()
	_, err := NewClient(srv.URL, "tok").ListServers(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 403 || apiErr.Code != "insufficient_scope" {
		t.Fatalf("expected APIError 403, got %v", err)
	}
}

func TestMissingToken(t *testing.T) {
	if _, err := NewClient("https://api.test", "").ListServers(context.Background()); err == nil {
		t.Fatal("expected error with empty token")
	}
}
