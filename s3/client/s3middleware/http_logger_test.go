package s3middleware

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HttpLogger", func() {
	var buf bytes.Buffer
	BeforeEach(func() {
		buf.Reset()
		logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		slog.SetDefault(logger)
	})

	Context("when transport returns response,", func() {
		It("log with 's3 http request' message", func() {
			mockTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode:    200,
					Body:          io.NopCloser(strings.NewReader("OK")),
					Header:        http.Header{"X-Amz-Request-Id": []string{"request-id"}, "X-Amz-Id-2": []string{"extended-request-id"}},
					ContentLength: 3,
				}, nil
			})
			loggingTransport := NewS3LoggingTransport(mockTransport)
			req := httptest.NewRequest("GET", "http://example.com/test", nil)
			_, _ = loggingTransport.RoundTrip(req) //nolint:errcheck
			logs := buf.String()
			Expect(logs).To(ContainSubstring(`"msg":"s3 http request"`))
			Expect(logs).To(ContainSubstring(`"method":"GET"`))
			Expect(logs).To(ContainSubstring(`"status_code":200`))
			Expect(logs).To(ContainSubstring(`"request_id":"request-id"`))
			Expect(logs).To(ContainSubstring(`"extended_request_id":"extended-request-id"`))
			Expect(logs).To(ContainSubstring(`"response_content_length":3`))
		})
	})

	Context("when transport returns error,", func() {
		It("log with 's3 http request failed' message", func() {

			hostNotFound := errors.New("dial tcp: lookup example.com: no such host")

			mockTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return nil, hostNotFound
			})
			loggingTransport := NewS3LoggingTransport(mockTransport)
			req := httptest.NewRequest("GET", "http://example.com/test", nil)
			_, _ = loggingTransport.RoundTrip(req) //nolint:errcheck
			logs := buf.String()
			Expect(logs).To(ContainSubstring(`"msg":"s3 http request failed"`))
			Expect(logs).To(ContainSubstring(`"method":"GET"`))
			Expect(logs).To(ContainSubstring(`"error"`))
			Expect(logs).To(ContainSubstring("no such host"))
		})
	})
})
