package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestViewerDoesNotRequireHandoff(t *testing.T) {
	upstream := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	)
	t.Cleanup(upstream.Close)
	manager := managerFixture(t)
	handler, err := Viewer(upstream.URL, "http://autonomous-stack")
	require.NoError(t, err)

	check := func() {
		t.Helper()

		response := httptest.NewRecorder()
		handler.ServeHTTP(
			response,
			httptest.NewRequest(
				http.MethodGet,
				"http://autonomous-stack/vnc.html",
				nil,
			),
		)
		require.Equal(t, http.StatusNoContent, response.Code)
		require.Empty(t, response.Result().Cookies())
	}
	check()

	request, err := manager.Request(t.Context(), "task", "reason")
	require.NoError(t, err)
	check()

	_, err = manager.Start(t.Context(), request.ID, true)
	require.NoError(t, err)
	check()

	_, err = manager.Finish(t.Context(), request.ID)
	require.NoError(t, err)
	check()
	require.NoError(t, manager.Close(t.Context()))
	check()
}
