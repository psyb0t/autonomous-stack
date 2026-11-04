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
	RequestID string `json:"request_id,omitempty"`
	UserReady bool   `json:"user_ready"`
}
type finishInput struct {
	RequestID string `json:"request_id,omitempty"`
}
type handlers struct{ manager *handoff.Manager }

const maximumMCPBody = 65536

func MCP(manager *handoff.Manager, token string) http.Handler {
	srv := mcp.NewServer(
		&mcp.Implementation{Name: "autonomous-stack", Version: "dev"}, nil,
	)
	h := handlers{manager: manager}
	mcp.AddTool(srv, &mcp.Tool{
		Name: "request_help",
		Description: `Ask the owner for help and pause automation.
Tailnet viewer access stays available.`,
	}, h.request)
	mcp.AddTool(srv, &mcp.Tool{
		Name: "start_handoff",
		Description: `Pause after the owner replies ready. Pass user_ready=true.
Omit the request ID to use the current request.
Viewer access is always available on the tailnet.`,
	}, h.start)
	mcp.AddTool(srv, &mcp.Tool{
		Name: "finish_handoff",
		Description: `Finish current assistance so automation can resume.
Does not close browser access.`,
	}, h.finish)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "handoff_status",
		Description: "Read the current help request and coordination status.",
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
