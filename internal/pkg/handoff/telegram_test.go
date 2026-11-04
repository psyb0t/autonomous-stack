package handoff

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTelegram(t *testing.T) {
	for _, body := range []string{`{"ok":true}`, `{"ok":false}`, `invalid`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					require.Equal(
						t,
						"/botEXAMPLE-DO-NOT-USE/sendMessage",
						r.URL.Path,
					)

					var payload struct {
						ChatID int64  `json:"chat_id"`
						Text   string `json:"text"`
					}
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					require.Equal(t, int64(42), payload.ChatID)
					require.Equal(t, "hello", payload.Text)

					_, err := w.Write([]byte(body))
					require.NoError(t, err)
				}),
			)
			defer server.Close()

			client := NewTelegram(server.URL, "EXAMPLE-DO-NOT-USE", 42)

			err := client.Send(t.Context(), "hello")
			if body == `{"ok":true}` {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.NotContains(t, err.Error(), client.Token)
			}

			server.Close()

			err = client.Send(t.Context(), "hello")
			require.Error(t, err)
			require.NotContains(t, err.Error(), client.Token)
		})
	}
}
