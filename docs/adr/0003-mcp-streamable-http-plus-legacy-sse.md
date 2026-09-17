# MCP Streamable HTTP plus legacy SSE

The current MCP spec serves Streamable HTTP. The official Go SDK (`github.com/modelcontextprotocol/go-sdk` v1.8) still implements the 2024-11-05 SSE transport via `mcp.NewSSEHandler`, so we mount both: `mcp.NewStreamableHTTPHandler` at `/mcp`, and `mcp.NewSSEHandler` at `/mcp/sse` and `/mcp/messages`.

The SDK does not take a separate messages path. GET `/mcp/sse` creates a session and advertises an endpoint of `/mcp/sse?sessionid=...`. POST with `sessionid` is accepted on either `/mcp/sse` or `/mcp/messages` because the handler keys sessions by query string, not path. If `NewSSEHandler` is removed in a future SDK, replace the SSE mounts with HTTP 410 JSON naming `/mcp` as the replacement (this ADR would then be superseded).
