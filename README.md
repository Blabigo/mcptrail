# mcptrail Go SDK

Go SDK for the [MCP Trail](https://mcptrail.com) management API (`/api/v1`) —
govern guardian servers and policies as code. Standard library only.

## Install

```bash
go get github.com/Blabigo/mcptrail
```

## Usage

```go
import (
    "context"
    "github.com/Blabigo/mcptrail"
)

ctx := context.Background()
client := mcptrail.NewClient("https://app.mcptrail.com", os.Getenv("MCPTRAIL_TOKEN"))

// Provision a server (bearer token is returned ONCE)
created, _ := client.CreateServer(ctx, mcptrail.CreateServerInput{
    DisplayName:    "GitHub",
    Slug:           "github",
    UpstreamMcpURL: "https://api.githubcopilot.com/mcp",
})

// Policy as code
_ = client.SetToolPolicy(ctx, "github", "delete_repository", "hitl")
_, _ = client.SetDlpMode(ctx, "github", "block")

// Read audit, resolve approvals
page, _ := client.ListAudit(ctx, mcptrail.AuditQuery{Scope: "github", Limit: 100})
pending, _ := client.ListPendingHitl(ctx)
for _, p := range pending {
    _ = client.ResolveHitl(ctx, p.ID, "approve", "once")
}
```

### Calling a guarded tool (data plane)

```go
proxy := mcptrail.NewProxyClient("https://proxy.mcptrail.com", "github", proxyToken)
result, err := proxy.CallTool(ctx, "get_issue", map[string]any{"number": 42})
```

## Errors

Non-2xx management responses return `*mcptrail.APIError` (Status/Code/Message).
Data-plane JSON-RPC errors return `*mcptrail.RPCError` (Code/Message).

Mirrors [`@mcptrail/sdk`](../sdk-ts) and the [Python SDK](../sdk-py), generated against
[`openapi/mcptrail.yaml`](../../openapi/mcptrail.yaml).
