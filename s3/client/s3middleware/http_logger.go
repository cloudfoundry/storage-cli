package s3middleware

import (
	"log/slog"
	"net/http"
	"time"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func NewS3LoggingTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}

	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		start := time.Now()
		resp, err := base.RoundTrip(req)
		duration := time.Since(start)

		attrs := []any{
			"method", req.Method,
			"url", req.URL.String(),
			"host", req.Host,
			"request_content_length", req.ContentLength,
			"duration_ms", duration.Milliseconds(),
		}

		if resp != nil {
			for k, v := range parseResponseFields(resp) {
				attrs = append(attrs, k, v)
			}
		}

		if err != nil {
			attrs = append(attrs, "error", err.Error())
			slog.Error("s3 http request failed", attrs...)
			return resp, err
		}

		slog.Debug("s3 http request", attrs...)

		return resp, nil
	})
}

func parseResponseFields(resp *http.Response) map[string]any {
	responseFields := make(map[string]any)
	responseFields["status_code"] = resp.StatusCode
	responseFields["response_content_length"] = resp.ContentLength
	responseFields["request_id"] = resp.Header.Get("x-amz-request-id")
	responseFields["extended_request_id"] = resp.Header.Get("x-amz-id-2")
	return responseFields
}
