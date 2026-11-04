package server

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/psyb0t/autonomous-stack/internal/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestViewer(t *testing.T) {
	upstream := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Empty(t, r.Header.Get("Cookie"))
			require.Empty(t, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusNoContent)
		}),
	)
	t.Cleanup(upstream.Close)
	manager := managerFixture(t)
	handler, err := Viewer(manager, upstream.URL)
	require.NoError(t, err)
	request, err := manager.Request(t.Context(), "task", "reason")
	require.NoError(t, err)
	active, err := manager.Start(t.Context(), request.ID, true)
	require.NoError(t, err)
	link, err := url.Parse(active.URL)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, link.String(), nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	require.Equal(t, http.StatusSeeOther, response.Code)
	cookies := response.Result().Cookies()
	require.Len(t, cookies, 1)
	require.True(t, cookies[0].Secure)
	require.True(t, cookies[0].HttpOnly)
	require.Equal(t, config.CookieName, cookies[0].Name)

	for _, tc := range []struct {
		name, origin string
		cookie       bool
		want         int
	}{
		{name: "missing", want: http.StatusUnauthorized},
		{name: "valid", cookie: true, want: http.StatusNoContent},
		{
			name:   "same-origin",
			cookie: true,
			origin: "https://fixture.trycloudflare.com",
			want:   http.StatusNoContent,
		},
		{
			name:   "foreign-origin",
			cookie: true,
			origin: "https://evil.example",
			want:   http.StatusForbidden,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodGet,
				"https://fixture.trycloudflare.com/vnc.html",
				nil,
			)
			if tc.cookie {
				req.AddCookie(cookies[0])
			}

			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			require.Equal(t, tc.want, response.Code)
		})
	}

	_, err = manager.Finish(t.Context(), request.ID)
	require.NoError(t, err)

	response = httptest.NewRecorder()
	handler.ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, link.String(), nil),
	)
	require.Equal(t, http.StatusUnauthorized, response.Code)
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

			data := make([]byte, 4)
			_, err = io.ReadFull(stream, data)
			require.NoError(t, err)
			require.Equal(t, "ping", string(data))

			_, err = stream.WriteString("pong")
			require.NoError(t, err)
			require.NoError(t, stream.Flush())
		},
	))
	t.Cleanup(upstream.Close)
	manager := managerFixture(t)
	request, err := manager.Request(t.Context(), "task", "reason")
	require.NoError(t, err)
	active, err := manager.Start(t.Context(), request.ID, true)
	require.NoError(t, err)
	link, err := url.Parse(active.URL)
	require.NoError(t, err)
	handler, err := Viewer(manager, upstream.URL)
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

	credential := strings.TrimPrefix(link.Path, connectPrefix)
	upgrade, err := http.NewRequestWithContext(
		t.Context(), http.MethodGet,
		"http://fixture.trycloudflare.com/websockify", nil,
	)
	require.NoError(t, err)
	upgrade.Header.Set("Connection", "Upgrade")
	upgrade.Header.Set("Upgrade", "websocket")
	upgrade.Header.Set("Origin", "https://fixture.trycloudflare.com")
	upgrade.AddCookie(&http.Cookie{
		Name: config.CookieName, Value: credential,
		Secure: true, HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
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
}
