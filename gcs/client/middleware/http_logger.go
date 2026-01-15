package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
)

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func NewLoggingTransport(base http.RoundTripper) http.RoundTripper {
	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		reqDump, _ := httputil.DumpRequest(req, false)
		slog.Debug("REQUEST", "dump", string(reqDump))

		resp, err := base.RoundTrip(req)

		if err != nil {
			slog.Error("ERROR", "dump", err)
		}
		if resp != nil {
			respDump, _ := httputil.DumpResponse(resp, false)
			slog.Debug("RESPONSE", "dump", string(respDump))
		}
		return resp, err
	})

}
