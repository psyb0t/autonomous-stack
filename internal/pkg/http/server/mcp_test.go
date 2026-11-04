package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/psyb0t/autonomous-stack/internal/pkg/config"
	"github.com/psyb0t/autonomous-stack/internal/pkg/handoff"
	"github.com/stretchr/testify/require"
)

type notifier struct{}

func (notifier) Send(context.Context, string) error { return nil }

func managerFixture(t *testing.T) *handoff.Manager {
	t.Helper()
	manager, err := handoff.New(
		config.Config{
			StatePath:  filepath.Join(t.TempDir(), "state.json"),
			RequestTTL: time.Hour,
			ViewerURL:  "http://autonomous-stack",
		},
		notifier{},
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, manager.Close(t.Context())) })

	return manager
}

func call(
	t *testing.T,
	handler http.Handler,
	auth, body string,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	if auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)

	return response
}

func TestMCP(t *testing.T) {
	manager := managerFixture(t)
	handler := MCP(manager, "EXAMPLE-DO-NOT-USE")

	const initialize = `{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-11-25",
    "capabilities": {},
    "clientInfo": {
      "name": "test",
      "version": "1"
    }
  }
}`
	for _, auth := range []string{"", "wrong"} {
		require.Equal(
			t,
			http.StatusUnauthorized,
			call(t, handler, auth, initialize).Code,
		)
	}

	require.Equal(
		t,
		http.StatusOK,
		call(t, handler, "EXAMPLE-DO-NOT-USE", initialize).Code,
	)
	response := call(
		t,
		handler,
		"EXAMPLE-DO-NOT-USE",
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	)
	require.Equal(t, http.StatusOK, response.Code)

	var listed struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &listed))
	require.Len(t, listed.Result.Tools, 4)
	response = call(
		t,
		handler,
		"EXAMPLE-DO-NOT-USE",
		`{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "request_help",
    "arguments": {
      "key": "task",
      "reason": "help"
    }
  }
}`,
	)
	require.Equal(t, http.StatusOK, response.Code)
	require.False(
		t,
		bytes.Contains(response.Body.Bytes(), []byte(`"isError":true`)),
	)
	require.Equal(t, handoff.PhasePending, manager.Status().Phase)
	response = call(
		t,
		handler,
		"EXAMPLE-DO-NOT-USE",
		`{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "start_handoff",
    "arguments": {
      "request_id": "wrong",
      "user_ready": false
    }
  }
}`,
	)
	require.Contains(t, response.Body.String(), `"isError":true`)

	for _, tc := range []struct {
		name      string
		arguments map[string]any
	}{
		{name: "start_handoff", arguments: map[string]any{
			"user_ready": true,
		}},
		{name: "handoff_status", arguments: map[string]any{}},
		{name: "finish_handoff", arguments: map[string]any{}},
	} {
		payload, err := json.Marshal(map[string]any{
			"jsonrpc": "2.0", "id": 5, "method": "tools/call",
			"params": map[string]any{
				"name": tc.name, "arguments": tc.arguments,
			},
		})
		require.NoError(t, err)
		response := call(t, handler, "EXAMPLE-DO-NOT-USE", string(payload))
		require.Equal(t, http.StatusOK, response.Code)
		require.NotContains(t, response.Body.String(), `"isError":true`)
		require.NotContains(t, response.Body.String(), "/connect/")
	}

	require.Equal(t, handoff.PhaseCompleted, manager.Status().Phase)
}
