package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func NewLoggingTransport(base http.RoundTripper) http.RoundTripper {
	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		slog.Debug("http request",
			"method", req.Method,
			"url", req.URL.String(),
			"headers", req.Header,
			"host", req.Host,
			"content_lenght", req.ContentLength)

		start := time.Now()
		resp, err := base.RoundTrip(req)
		duration := time.Since(start)

		if err != nil {
			slog.Error("http request failed",
				"method", req.Method,
				"url", req.URL.String(),
				"duration_ms", duration.Milliseconds(),
				"error", err)
			return resp, err
		}
		if resp != nil {
			slog.Debug("http response",
				"method", req.Method,
				"url", req.URL.String(),
				"status_code", resp.StatusCode,
				"content_length", resp.ContentLength,
				"duration_ms", duration.Milliseconds())
		}
		return resp, err
	})

}
