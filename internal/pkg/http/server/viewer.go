package server

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/autonomous-stack/internal/pkg/config"
	"github.com/psyb0t/ctxerrors"
)

const connectPrefix = "/connect/"

func Viewer(
	target, viewerURL string,
) (http.Handler, error) {
	if err := config.ValidateViewerURL(viewerURL); err != nil {
		return nil, ctxerrors.Wrap(err, "validate tailnet viewer origin")
	}

	origin, err := url.Parse(viewerURL)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "parse tailnet viewer origin")
	}

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

	return viewerHandler(origin, proxy), nil
}

func viewerHandler(
	origin *url.URL, proxy *httputil.ReverseProxy,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")

		if !sameOrigin(r, origin) {
			aichteeteapee.WriteJSON(
				w,
				http.StatusForbidden,
				aichteeteapee.ErrorResponseForbidden,
			)

			return
		}

		if r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, connectPrefix) {
			http.Redirect(
				w,
				r,
				"/vnc.html?autoconnect=true&resize=scale",
				http.StatusSeeOther,
			)

			return
		}

		proxy.ServeHTTP(w, r)
	})
}

func sameOrigin(r *http.Request, expected *url.URL) bool {
	if !strings.EqualFold(r.Host, expected.Host) {
		return false
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	parsed, err := url.Parse(origin)

	return err == nil && parsed.Scheme == expected.Scheme &&
		strings.EqualFold(parsed.Host, expected.Host) && parsed.User == nil &&
		parsed.Path == "" && parsed.RawQuery == "" && parsed.Fragment == ""
}
