package server

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/autonomous-stack/internal/pkg/config"
	"github.com/psyb0t/autonomous-stack/internal/pkg/handoff"
	"github.com/psyb0t/ctxerrors"
)

const connectPrefix = "/connect/"

func Viewer(manager *handoff.Manager, target string) (http.Handler, error) {
	endpoint, err := url.Parse(target)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "parse browser viewer target")
	}

	proxy := &httputil.ReverseProxy{}
	proxy.Rewrite = func(request *httputil.ProxyRequest) {
		request.SetURL(endpoint)
		request.Out.Host = endpoint.Host
		request.Out.Header.Del("Cookie")
		request.Out.Header.Del("Authorization")
		request.Out.Header.Del("Forwarded")
		request.Out.Header.Del("X-Forwarded-Host")
		request.Out.Header.Del("X-Forwarded-Proto")
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		aichteeteapee.WriteJSON(
			w,
			http.StatusBadGateway,
			aichteeteapee.ErrorResponseInternalServerError,
		)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")

		if after, ok := strings.CutPrefix(r.URL.Path, connectPrefix); ok {
			if r.Method != http.MethodGet || !manager.Authorize(after) {
				unauthorized(w)

				return
			}

			connect(w, r, manager)

			return
		}

		cookie, err := r.Cookie(config.CookieName)
		if err != nil || !manager.Authorize(cookie.Value) {
			unauthorized(w)

			return
		}

		if !sameOrigin(r) {
			aichteeteapee.WriteJSON(
				w,
				http.StatusForbidden,
				aichteeteapee.ErrorResponseForbidden,
			)

			return
		}

		proxy.ServeHTTP(w, r)
	}), nil
}

func unauthorized(w http.ResponseWriter) {
	aichteeteapee.WriteJSON(
		w,
		http.StatusUnauthorized,
		aichteeteapee.ErrorResponseUnauthorized,
	)
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	parsed, err := url.Parse(origin)

	return err == nil && parsed.Scheme == "https" &&
		strings.EqualFold(parsed.Host, r.Host)
}

func connect(w http.ResponseWriter, r *http.Request, manager *handoff.Manager) {
	expires := manager.Status().ExpiresAt
	http.SetCookie(w, &http.Cookie{
		Name:     config.CookieName,
		Value:    strings.TrimPrefix(r.URL.Path, connectPrefix),
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Expires:  expires, MaxAge: int(time.Until(expires).Seconds()),
	})
	http.Redirect(
		w,
		r,
		"/vnc.html?autoconnect=true&resize=scale",
		http.StatusSeeOther,
	)
}
