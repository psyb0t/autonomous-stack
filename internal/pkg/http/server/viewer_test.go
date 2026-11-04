package server

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestViewer(t *testing.T) {
	upstream := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Empty(t, r.Header.Get("Cookie"))
			require.Empty(t, r.Header.Get("Authorization"))
			require.Empty(t, r.Header.Get("X-Forwarded-Host"))
			w.WriteHeader(http.StatusNoContent)
		}),
	)
	t.Cleanup(upstream.Close)
	handler, err := Viewer(upstream.URL, "http://autonomous-stack")
	require.NoError(t, err)

	for _, path := range []string{"/", "/connect/old-link"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(
			response,
			httptest.NewRequest(
				http.MethodGet,
				"http://autonomous-stack"+path,
				nil,
			),
		)
		require.Equal(t, http.StatusSeeOther, response.Code)
		require.Equal(
			t,
			"/vnc.html?autoconnect=true&resize=scale",
			response.Header().Get("Location"),
		)
		require.Empty(t, response.Result().Cookies())
	}

	for _, origin := range []string{"", "http://autonomous-stack"} {
		request := httptest.NewRequest(
			http.MethodGet,
			"http://autonomous-stack/vnc.html",
			nil,
		)
		request.Header.Set("Origin", origin)
		request.Header.Set("Cookie", "old=ignored")
		request.Header.Set("Authorization", "Bearer irrelevant")
		request.Header.Set("X-Forwarded-Host", "foreign.example")

		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		require.Equal(t, http.StatusNoContent, response.Code)
		require.Empty(t, response.Result().Cookies())
	}
}

func TestViewerUpgrade(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.Empty(t, r.Header.Get("Cookie"))

			connection, stream, err := http.NewResponseController(w).Hijack()
			require.NoError(t, err)

			defer func() { require.NoError(t, connection.Close()) }()

			response := &http.Response{
				StatusCode: http.StatusSwitchingProtocols,
				ProtoMajor: 1, ProtoMinor: 1,
				Header: http.Header{
					"Connection": {"Upgrade"}, "Upgrade": {"websocket"},
				},
			}
			require.NoError(t, response.Write(stream))
			require.NoError(t, stream.Flush())

			for range 2 {
				data := make([]byte, 4)
				_, err = io.ReadFull(stream, data)
				require.NoError(t, err)
				require.Equal(t, "ping", string(data))

				_, err = stream.WriteString("pong")
				require.NoError(t, err)
				require.NoError(t, stream.Flush())
			}

			_, err = stream.ReadByte()
			require.ErrorIs(t, err, io.EOF)
		},
	))
	t.Cleanup(upstream.Close)
	manager := managerFixture(t)
	request, err := manager.Request(t.Context(), "task", "reason")
	require.NoError(t, err)
	_, err = manager.Start(t.Context(), request.ID, true)
	require.NoError(t, err)
	handler, err := Viewer(upstream.URL, "http://autonomous-stack")
	require.NoError(t, err)

	proxy := httptest.NewServer(handler)
	t.Cleanup(proxy.Close)
	endpoint, err := url.Parse(proxy.URL)
	require.NoError(t, err)

	dialer := net.Dialer{Timeout: time.Second}
	connection, err := dialer.DialContext(t.Context(), "tcp", endpoint.Host)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, connection.Close()) })
	require.NoError(t, connection.SetDeadline(time.Now().Add(5*time.Second)))

	upgrade, err := http.NewRequestWithContext(
		t.Context(), http.MethodGet,
		"http://autonomous-stack/websockify", nil,
	)
	require.NoError(t, err)
	upgrade.Header.Set("Connection", "Upgrade")
	upgrade.Header.Set("Upgrade", "websocket")
	upgrade.Header.Set("Origin", "http://autonomous-stack")
	require.NoError(t, upgrade.Write(connection))

	reader := bufio.NewReader(connection)
	response, err := http.ReadResponse(
		reader,
		&http.Request{Method: http.MethodGet},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
	require.NoError(t, response.Body.Close())

	_, err = connection.Write([]byte("ping"))
	require.NoError(t, err)

	data := make([]byte, 4)
	_, err = io.ReadFull(reader, data)
	require.NoError(t, err)
	require.Equal(t, "pong", string(data))

	_, err = manager.Finish(t.Context(), request.ID)
	require.NoError(t, err)
	_, err = connection.Write([]byte("ping"))
	require.NoError(t, err)
	_, err = io.ReadFull(reader, data)
	require.NoError(t, err)
	require.Equal(t, "pong", string(data))
}

func TestViewerOriginAndTLS(t *testing.T) {
	upstream := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	)
	t.Cleanup(upstream.Close)

	for _, origin := range []string{
		"http://autonomous-stack", "https://autonomous-stack",
	} {
		handler, err := Viewer(upstream.URL, origin)
		require.NoError(t, err)

		for _, hostile := range []string{
			"http://foreign.example", origin + "@foreign.example",
			"null", origin + "/path",
		} {
			request := httptest.NewRequest(
				http.MethodGet,
				origin+"/vnc.html",
				nil,
			)
			request.Header.Set("Origin", hostile)

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			require.Equal(t, http.StatusForbidden, response.Code)
		}

		request := httptest.NewRequest(http.MethodGet, origin+"/vnc.html", nil)
		request.Host = "foreign.example"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		require.Equal(t, http.StatusForbidden, response.Code)
	}

	_, err := Viewer(upstream.URL, "file:///tmp")
	require.Error(t, err)
	_, err = Viewer("%zz", "http://autonomous-stack")
	require.Error(t, err)
	upstream.Close()
	handler, err := Viewer(upstream.URL, "http://autonomous-stack")
	require.NoError(t, err)

	response := httptest.NewRecorder()
	handler.ServeHTTP(
		response,
		httptest.NewRequest(
			http.MethodGet,
			"http://autonomous-stack/vnc.html",
			nil,
		),
	)
	require.Equal(t, http.StatusBadGateway, response.Code)
}
