package middleware

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
		It("log with 'http response' message", func() {
			mockTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader("OK")),
					Header:     http.Header{"Content-Type": []string{"text/plain"}},
				}, nil
			})
			loggingTransport := NewLoggingTransport(mockTransport)
			req := httptest.NewRequest("GET", "http://example.com/test", nil)
			_, _ = loggingTransport.RoundTrip(req)
			logs := buf.String()
			Expect(logs).To(ContainSubstring(`"msg":"http response"`))
			Expect(logs).To(ContainSubstring(`"method":"GET"`))
			Expect(logs).To(ContainSubstring(`"status_code":200`))
		})
	})

	Context("when transport returns error,", func() {
		It("log with 'http request failed' message", func() {

			hostNotFound := errors.New("dial tcp: lookup example.com: no such host")

			mockTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return nil, hostNotFound
			})
			loggingTransport := NewLoggingTransport(mockTransport)
			req := httptest.NewRequest("GET", "http://example.com/test", nil)
			_, _ = loggingTransport.RoundTrip(req)
			logs := buf.String()
			Expect(logs).To(ContainSubstring(`"msg":"http request failed"`))
			Expect(logs).To(ContainSubstring(`"method":"GET"`))
			Expect(logs).To(ContainSubstring(`"error"`))
			Expect(logs).To(ContainSubstring("no such host"))
		})
	})

	Context("when transport make request,", func() {
		It("log with 'http request' message always", func() {
			mockTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader("OK")),
					Header:     http.Header{"Content-Type": []string{"text/plain"}},
				}, nil
			})
			loggingTransport := NewLoggingTransport(mockTransport)
			req := httptest.NewRequest("GET", "http://example.com/test", nil)
			_, _ = loggingTransport.RoundTrip(req)
			logs := buf.String()
			Expect(logs).To(ContainSubstring(`"msg":"http request"`))
			Expect(logs).To(ContainSubstring(`"method":"GET"`))
			Expect(logs).To(ContainSubstring(`"url":"http://example.com/test"`))
			Expect(logs).To(ContainSubstring(`"host":"example.com"`))
		})
	})

})
