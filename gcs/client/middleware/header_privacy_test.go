package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLoggingOmitsEncryptionAndAuthenticationHeaders(t *testing.T) {
	original := slog.Default()
	defer slog.SetDefault(original)
	var output bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug})))
	request := httptest.NewRequest("GET", "https://storage.googleapis.com/bucket/object", nil)
	for header, value := range map[string]string{"X-Goog-Encryption-Key": "fixture-encryption-key", "X-Goog-Encryption-Key-Sha256": "fixture-encryption-hash", "X-Goog-Copy-Source-Encryption-Key": "fixture-copy-key", "Authorization": "Bearer fixture-bearer-token", "Cookie": "fixture-cookie"} {
		request.Header.Set(header, value)
	}
	headers := request.Header.Clone()
	transport := NewLoggingTransport(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		for header := range headers {
			if req.Header.Get(header) != headers.Get(header) {
				t.Fatalf("changed transmitted header %s", header)
			}
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))}, nil
	}))
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	for _, values := range headers {
		for _, value := range values {
			if strings.Contains(output.String(), value) {
				t.Fatal("logged secret-bearing request header")
			}
		}
	}
	if !strings.Contains(output.String(), "http response") {
		t.Fatal("lost safe response diagnostics")
	}
}
