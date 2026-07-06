// Package mcptrail is the Go SDK for the MCP Trail management API (/api/v1).
// It mirrors the TypeScript @mcptrail/sdk and Python mcptrail packages: Bearer
// auth, the /api/v1 base path, and APIError for non-2xx responses. Generated
// against openapi/mcptrail.yaml.
package mcptrail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the MCP Trail cloud origin. Override via NewClient for self-hosting.
const DefaultBaseURL = "https://app.mcptrail.com"

// Client is a Bearer-authenticated client for the management API.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// NewClient targets baseURL with the given management API key (or mct_ CLI token).
func NewClient(baseURL, token string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// APIError carries the `{ error: { code, message } }` body of a non-2xx response.
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("api error %d (%s): %s", e.Status, e.Code, e.Message)
}

type Server struct {
	ID           string `json:"id"`
	Slug         string `json:"slug"`
	DisplayName  string `json:"displayName"`
	UpstreamKind string `json:"upstreamKind"`
	ProxyEnabled bool   `json:"proxyEnabled"`
	ToolAuthMode string `json:"toolAuthMode"`
	DlpMode      string `json:"dlpMode"`
	CreatedAt    *int64 `json:"createdAt"`
	// WebmcpOrigin is the bridged website origin for a webmcp_bridge server; empty otherwise.
	WebmcpOrigin           string `json:"webmcpOrigin,omitempty"`
	BudgetCreditsRemaining int64  `json:"budgetCreditsRemaining"`
	BudgetCreditCap        int64  `json:"budgetCreditCap"`
}

type ToolPolicy struct {
	ToolName    string  `json:"toolName"`
	Description string  `json:"description"`
	Policy      *string `json:"policy"`
	LastSeenAt  *int64  `json:"lastSeenAt"`
}

type CreateServerInput struct {
	DisplayName    string `json:"displayName"`
	Slug           string `json:"slug"`
	UpstreamMcpURL string `json:"upstreamMcpUrl"`
}

type CreateServerResult struct {
	Server struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
	} `json:"server"`
	BearerToken          string `json:"bearerToken"`
	BearerTokenExpiresAt int64  `json:"bearerTokenExpiresAt"`
}

// CreateWebmcpServerInput provisions a browser-bridged (WebMCP) server for a website origin.
type CreateWebmcpServerInput struct {
	DisplayName string `json:"displayName"`
	Slug        string `json:"slug"`
	Origin      string `json:"origin"`
}

type WebmcpPairing struct {
	Token      string `json:"token"`
	ConnectURL string `json:"connectUrl"`
	Origin     string `json:"origin,omitempty"`
}

type CreateWebmcpServerResult struct {
	Server struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
	} `json:"server"`
	ProxyURL             string        `json:"proxyUrl"`
	BearerToken          string        `json:"bearerToken"`
	BearerTokenExpiresAt int64         `json:"bearerTokenExpiresAt"`
	Pairing              WebmcpPairing `json:"pairing"`
}

// RunToken is a short-lived member token for running a server's tools through the proxy.
type RunToken struct {
	BearerToken string `json:"bearerToken"`
	ProxyURL    string `json:"proxyUrl"`
	ExpiresAt   int64  `json:"expiresAt"`
}

// ServerPatch updates gateway + auth-mode settings; nil fields are left unchanged.
type ServerPatch struct {
	ProxyEnabled          *bool   `json:"proxyEnabled,omitempty"`
	ToolAuthMode          *string `json:"toolAuthMode,omitempty"`
	ResourceAuthMode      *string `json:"resourceAuthMode,omitempty"`
	PromptAuthMode        *string `json:"promptAuthMode,omitempty"`
	ResourceBlockFileURIs *bool   `json:"resourceBlockFileUris,omitempty"`
}

type UpdateResult struct {
	OK      bool     `json:"ok"`
	Applied []string `json:"applied"`
}

type AuditQuery struct {
	Scope     string
	Limit     int
	MaxAgeSec int
	Cursor    string
}

type AuditPage struct {
	Rows       []json.RawMessage `json:"rows"`
	NextCursor *string           `json:"nextCursor"`
	HasMore    bool              `json:"hasMore"`
}

type PendingHitl struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	ToolName  string `json:"toolName"`
	Status    string `json:"status"`
	CreatedAt *int64 `json:"createdAt"`
	ExpiresAt *int64 `json:"expiresAt"`
}

type BundleMember struct {
	Slug         string `json:"slug"`
	DisplayName  string `json:"displayName"`
	Alias        string `json:"alias"`
	Position     int    `json:"position"`
	Enabled      bool   `json:"enabled"`
	UpstreamKind string `json:"upstreamKind"`
}

type Bundle struct {
	ID          string         `json:"id"`
	Slug        string         `json:"slug"`
	DisplayName string         `json:"displayName"`
	Members     []BundleMember `json:"members"`
}

type CreateBundleInput struct {
	DisplayName string   `json:"displayName"`
	Slug        string   `json:"slug"`
	MemberSlugs []string `json:"memberSlugs"`
}

type CreateBundleResult struct {
	Bundle struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
	} `json:"bundle"`
	BearerToken          string `json:"bearerToken"`
	BearerTokenExpiresAt int64  `json:"bearerTokenExpiresAt"`
}

type DlpRule struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Enabled        bool     `json:"enabled"`
	Kind           string   `json:"kind"`
	Pattern        string   `json:"pattern"`
	Severity       string   `json:"severity"`
	Action         string   `json:"action"`
	ScopeServerIDs []string `json:"scopeServerIds"`
	ScopeToolNames []string `json:"scopeToolNames"`
	CreatedAt      *int64   `json:"createdAt"`
	UpdatedAt      *int64   `json:"updatedAt"`
}

type CreateDlpRuleInput struct {
	Name           string   `json:"name"`
	Kind           string   `json:"kind"`
	Pattern        string   `json:"pattern"`
	Action         string   `json:"action"`
	Severity       string   `json:"severity,omitempty"`
	ScopeServerIDs []string `json:"scopeServerIds,omitempty"`
	ScopeToolNames []string `json:"scopeToolNames,omitempty"`
}

// do performs an authenticated request against /api/v1 + path, decoding a 2xx
// JSON body into out (when non-nil) and mapping any non-2xx into *APIError.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	if c.token == "" {
		return fmt.Errorf("no API token: pass a token to NewClient or set MCPTRAIL_TOKEN")
	}
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+"/api/v1"+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &APIError{Status: resp.StatusCode, Code: "error", Message: string(data)}
		var parsed struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(data, &parsed) == nil && parsed.Error.Code != "" {
			apiErr.Code = parsed.Error.Code
			apiErr.Message = parsed.Error.Message
		}
		return apiErr
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

func (c *Client) ListServers(ctx context.Context) ([]Server, error) {
	var r struct {
		Servers []Server `json:"servers"`
	}
	return r.Servers, c.do(ctx, http.MethodGet, "/servers", nil, &r)
}

func (c *Client) GetServer(ctx context.Context, slug string) (*Server, error) {
	var r struct {
		Server Server `json:"server"`
	}
	if err := c.do(ctx, http.MethodGet, "/servers/"+url.PathEscape(slug), nil, &r); err != nil {
		return nil, err
	}
	return &r.Server, nil
}

func (c *Client) CreateServer(ctx context.Context, in CreateServerInput) (*CreateServerResult, error) {
	var r CreateServerResult
	if err := c.do(ctx, http.MethodPost, "/servers", in, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// CreateWebmcpServer provisions a browser-bridged (WebMCP) server; returns proxy URL,
// bearer token, and a pairing token/URL for the Webmcp Trail extension.
func (c *Client) CreateWebmcpServer(ctx context.Context, in CreateWebmcpServerInput) (*CreateWebmcpServerResult, error) {
	body := map[string]string{"kind": "webmcp", "displayName": in.DisplayName, "slug": in.Slug, "origin": in.Origin}
	var r CreateWebmcpServerResult
	if err := c.do(ctx, http.MethodPost, "/servers", body, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// MintRunToken mints a short-lived member token for running a server's tools through the proxy.
func (c *Client) MintRunToken(ctx context.Context, slug string) (*RunToken, error) {
	var r RunToken
	if err := c.do(ctx, http.MethodPost, "/servers/"+url.PathEscape(slug)+"/run-token", nil, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// MintWebmcpPairing mints a fresh WebMCP pairing token + connect URL for a browser-bridged server.
func (c *Client) MintWebmcpPairing(ctx context.Context, slug string) (*WebmcpPairing, error) {
	var r WebmcpPairing
	if err := c.do(ctx, http.MethodPost, "/servers/"+url.PathEscape(slug)+"/webmcp-pairing", nil, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func (c *Client) UpdateServer(ctx context.Context, slug string, patch ServerPatch) (*UpdateResult, error) {
	var r UpdateResult
	if err := c.do(ctx, http.MethodPatch, "/servers/"+url.PathEscape(slug), patch, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func (c *Client) ListTools(ctx context.Context, slug string) ([]ToolPolicy, error) {
	var r struct {
		Tools []ToolPolicy `json:"tools"`
	}
	return r.Tools, c.do(ctx, http.MethodGet, "/servers/"+url.PathEscape(slug)+"/tools", nil, &r)
}

func (c *Client) SetToolPolicy(ctx context.Context, slug, toolName, policy string) error {
	path := "/servers/" + url.PathEscape(slug) + "/tools/" + url.PathEscape(toolName) + "/policy"
	return c.do(ctx, http.MethodPut, path, map[string]string{"policy": policy}, nil)
}

func (c *Client) GetDlpMode(ctx context.Context, slug string) (string, error) {
	var r struct {
		DlpMode string `json:"dlpMode"`
	}
	return r.DlpMode, c.do(ctx, http.MethodGet, "/servers/"+url.PathEscape(slug)+"/dlp", nil, &r)
}

// SetDlpMode returns the effective mode after any plan clamping.
func (c *Client) SetDlpMode(ctx context.Context, slug, mode string) (string, error) {
	var r struct {
		EffectiveMode string `json:"effectiveMode"`
	}
	if err := c.do(ctx, http.MethodPut, "/servers/"+url.PathEscape(slug)+"/dlp", map[string]string{"mode": mode}, &r); err != nil {
		return "", err
	}
	return r.EffectiveMode, nil
}

func (c *Client) ListAudit(ctx context.Context, q AuditQuery) (*AuditPage, error) {
	v := url.Values{}
	if q.Scope != "" {
		v.Set("scope", q.Scope)
	}
	if q.Limit > 0 {
		v.Set("limit", strconv.Itoa(q.Limit))
	}
	if q.MaxAgeSec > 0 {
		v.Set("maxAgeSec", strconv.Itoa(q.MaxAgeSec))
	}
	if q.Cursor != "" {
		v.Set("cursor", q.Cursor)
	}
	path := "/audit"
	if enc := v.Encode(); enc != "" {
		path += "?" + enc
	}
	var r AuditPage
	if err := c.do(ctx, http.MethodGet, path, nil, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func (c *Client) ListPendingHitl(ctx context.Context) ([]PendingHitl, error) {
	var r struct {
		Pending []PendingHitl `json:"pending"`
	}
	return r.Pending, c.do(ctx, http.MethodGet, "/hitl", nil, &r)
}

// ResolveHitl approves or denies a pending action (decision: "approve"|"deny";
// mode "once"|"session", defaulting to "once").
func (c *Client) ResolveHitl(ctx context.Context, approvalID, decision, mode string) error {
	if mode == "" {
		mode = "once"
	}
	body := map[string]string{"decision": decision, "mode": mode}
	return c.do(ctx, http.MethodPost, "/hitl/"+url.PathEscape(approvalID), body, nil)
}

func (c *Client) ListBundles(ctx context.Context) ([]Bundle, error) {
	var r struct {
		Bundles []Bundle `json:"bundles"`
	}
	return r.Bundles, c.do(ctx, http.MethodGet, "/bundles", nil, &r)
}

func (c *Client) CreateBundle(ctx context.Context, in CreateBundleInput) (*CreateBundleResult, error) {
	var r CreateBundleResult
	if err := c.do(ctx, http.MethodPost, "/bundles", in, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func (c *Client) ListDlpRules(ctx context.Context) ([]DlpRule, error) {
	var r struct {
		Rules []DlpRule `json:"rules"`
	}
	return r.Rules, c.do(ctx, http.MethodGet, "/dlp/rules", nil, &r)
}

// CreateDlpRule creates a custom DLP rule and returns its id.
func (c *Client) CreateDlpRule(ctx context.Context, in CreateDlpRuleInput) (string, error) {
	var r struct {
		ID string `json:"id"`
	}
	if err := c.do(ctx, http.MethodPost, "/dlp/rules", in, &r); err != nil {
		return "", err
	}
	return r.ID, nil
}

func (c *Client) DeleteDlpRule(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/dlp/rules/"+url.PathEscape(id), nil, nil)
}
