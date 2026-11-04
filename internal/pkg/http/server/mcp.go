package server

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/psyb0t/aichteeteapee/serbewr/middleware"
	"github.com/psyb0t/autonomous-stack/internal/pkg/handoff"
	"github.com/psyb0t/ctxerrors"
)

type requestInput struct {
	Key    string `json:"key"`
	Reason string `json:"reason"`
}
type startInput struct {
	RequestID string `json:"request_id"`
	UserReady bool   `json:"user_ready"`
}
type finishInput struct {
	RequestID string `json:"request_id"`
}
type handlers struct{ manager *handoff.Manager }

const maximumMCPBody = 65536

func MCP(manager *handoff.Manager, token string) http.Handler {
	srv := mcp.NewServer(
		&mcp.Implementation{Name: "autonomous-stack", Version: "dev"}, nil,
	)
	h := handlers{manager: manager}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "request_help",
		Description: "Ask the Telegram owner for help. No tunnel is opened.",
	}, h.request)
	mcp.AddTool(srv, &mcp.Tool{
		Name: "start_handoff",
		Description: `Open the viewer only after the owner replies ready.
Pass the request ID and user_ready=true. The owner receives the link.`,
	}, h.start)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "finish_handoff",
		Description: "Close the viewer when the owner finishes or cancels.",
	}, h.finish)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "handoff_status",
		Description: "Read the current help request and viewer status.",
	}, h.status)

	handler := mcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true},
	)

	return middleware.BearerAuth(middleware.WithBearerAuthTokens(token))(
		http.MaxBytesHandler(handler, maximumMCPBody),
	)
}

func (h handlers) request(
	ctx context.Context, _ *mcp.CallToolRequest, in requestInput,
) (*mcp.CallToolResult, handoff.State, error) {
	state, err := h.manager.Request(ctx, in.Key, in.Reason)

	return nil, state, ctxerrors.Wrap(err, "request human help")
}

func (h handlers) start(
	ctx context.Context, _ *mcp.CallToolRequest, in startInput,
) (*mcp.CallToolResult, handoff.State, error) {
	result, err := h.manager.Start(ctx, in.RequestID, in.UserReady)

	// The viewer credential goes only to the owner, not into Claude's history.
	return nil, result.State, ctxerrors.Wrap(err, "start browser handoff")
}

func (h handlers) finish(
	ctx context.Context, _ *mcp.CallToolRequest, in finishInput,
) (*mcp.CallToolResult, handoff.State, error) {
	state, err := h.manager.Finish(ctx, in.RequestID)

	return nil, state, ctxerrors.Wrap(err, "finish browser handoff")
}

func (h handlers) status(
	_ context.Context, _ *mcp.CallToolRequest, _ struct{},
) (*mcp.CallToolResult, handoff.State, error) {
	return nil, h.manager.Status(), nil
}
